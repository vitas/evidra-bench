package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func TestRunCommand_DryRun(t *testing.T) {
	// Create a temporary scenario directory.
	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "kubernetes", "broken-deployment")
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlContent := `id: broken-deployment
title: Fix broken deployment
category: kubernetes
prompt: prompts/task.md
break:
  type: kubectl
  command: "patch deployment web -n bench"
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"run",
		"--scenario", "kubernetes/broken-deployment",
		"--scenarios-dir", dir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(buf.String(), "broken-deployment") {
		t.Fatalf("unexpected output: %s", buf.String())
	}
}

func TestRunCommand_DryRun_ByScenarioID(t *testing.T) {
	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "kubernetes", "broken-deployment")
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlContent := `id: broken-deployment
title: Fix broken deployment
category: kubernetes
prompt: prompts/task.md
break:
  type: kubectl
  command: "patch deployment web -n bench"
bootstrap:
  - type: kubectl-apply
    path: fixtures/baseline.yaml
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"run",
		"--scenario", "broken-deployment",
		"--scenarios-dir", dir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(buf.String(), "broken-deployment") {
		t.Fatalf("unexpected output: %s", buf.String())
	}
}

func TestRunCommand_RejectsIncompatibleProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "kubernetes", "k3d-only")
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlContent := `id: k3d-only
title: K3d-only scenario
category: kubernetes
prompt: prompts/task.md
environment:
  providers: [k3d]
break:
  type: kubectl
  command: "get pods"
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCommand()
	cmd.SetArgs([]string{
		"run",
		"--scenario", "kubernetes/k3d-only",
		"--scenarios-dir", dir,
		"--dry-run",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for incompatible provider")
	}
	if !strings.Contains(err.Error(), "requires") || !strings.Contains(err.Error(), "k3d") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunCommand_AcceptsCompatibleProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "kubernetes", "kind-ok")
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlContent := `id: kind-ok
title: Kind-compatible scenario
category: kubernetes
prompt: prompts/task.md
environment:
  providers: [kind]
break:
  type: kubectl
  command: "get pods"
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"run",
		"--scenario", "kubernetes/kind-ok",
		"--scenarios-dir", dir,
		"--dry-run",
	})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error for compatible provider, got: %v", err)
	}
}

func TestRunCommand_ArgocdProfile_AcquiresDedicatedLease(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "argocd", "broken-guestbook")
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlContent := `id: broken-guestbook
title: Fix broken ArgoCD guestbook
category: argocd
prompt: prompts/task.md
environment:
  profile: argocd
  providers: [kind]
break:
  type: kubectl
  command: "get pods"
checks:
  - type: deployment-ready
    namespace: bench
    name: guestbook
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Dry-run resolves the profile but skips lease acquisition.
	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"run",
		"--scenario", "argocd/broken-guestbook",
		"--scenarios-dir", dir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(buf.String(), "broken-guestbook") {
		t.Fatalf("unexpected output: %s", buf.String())
	}

	// Verify the loaded scenario resolves to argocd profile.
	s, err := scenario.Resolve(dir, "argocd/broken-guestbook")
	if err != nil {
		t.Fatalf("resolve scenario: %v", err)
	}
	if got := s.ResolvedProfile(); got != scenario.ProfileArgocd {
		t.Fatalf("expected profile argocd, got %q", got)
	}
}

func TestRunCommand_RegistersToolServerFlags(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cmd := newRunCommand(&cfg)

	for _, flag := range []string{"tool-server-id", "tool-server-version"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Fatalf("expected --%s flag to be registered", flag)
		}
	}
}
