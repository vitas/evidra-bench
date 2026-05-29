package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutionProfile_ExplicitProfileWins(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		Environment: EnvironmentConfig{
			Profile: ProfileArgocd,
			Cloud: CloudConfig{
				Provider: "localstack",
			},
		},
	}
	got := s.ResolvedProfile()
	if got != ProfileArgocd {
		t.Fatalf("ResolvedProfile() = %q, want %q", got, ProfileArgocd)
	}
}

func TestExecutionProfile_DefaultWhenUnset(t *testing.T) {
	t.Parallel()
	s := &Scenario{}
	got := s.ResolvedProfile()
	if got != ProfileDefault {
		t.Fatalf("ResolvedProfile() = %q, want %q", got, ProfileDefault)
	}
}

func TestExecutionProfile_LegacyLocalStackFallback(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		Environment: EnvironmentConfig{
			Cloud: CloudConfig{
				Provider: "localstack",
			},
		},
	}
	got := s.ResolvedProfile()
	if got != ProfileAWSLocalStack {
		t.Fatalf("ResolvedProfile() = %q, want %q", got, ProfileAWSLocalStack)
	}
}

func TestIsSupportedExecutionProfile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		profile ExecutionProfile
		want    bool
	}{
		{ProfileDefault, true},
		{ProfileArgocd, true},
		{ProfileAWSLocalStack, true},
		{"multi-node", true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsSupportedExecutionProfile(tt.profile); got != tt.want {
			t.Errorf("IsSupportedExecutionProfile(%q) = %v, want %v", tt.profile, got, tt.want)
		}
	}
}

func TestLoad_RejectsUnknownExecutionProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data := `id: test
title: test
category: kubernetes
prompt: prompts/task.md
environment:
  profile: mystery-profile
break:
  type: apply
  path: fixtures/broken.yaml
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix it."), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for unknown execution profile 'mystery-profile'")
	}
}

func TestLoad_AcceptsExplicitArgocdProfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data := `id: test-argocd
title: test argocd
category: argocd
prompt: prompts/task.md
environment:
  profile: argocd
  providers: [kind, k3d]
break:
  type: apply
  path: fixtures/broken.yaml
checks:
  - type: argocd-app-healthy
    name: web
`
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix it."), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if s.Environment.Profile != ProfileArgocd {
		t.Fatalf("Profile = %q, want %q", s.Environment.Profile, ProfileArgocd)
	}
	if got := s.ResolvedProfile(); got != ProfileArgocd {
		t.Fatalf("ResolvedProfile() = %q, want %q", got, ProfileArgocd)
	}
}

func TestLoad_AcceptsEmptyProfile(t *testing.T) {
	t.Parallel()
	dir := writeTestScenario(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if s.Environment.Profile != "" {
		t.Fatalf("Profile = %q, want empty", s.Environment.Profile)
	}
	if got := s.ResolvedProfile(); got != ProfileDefault {
		t.Fatalf("ResolvedProfile() = %q, want %q", got, ProfileDefault)
	}
}
