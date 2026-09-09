package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/scenario"
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

func TestNewKindProviderDetectsContainerRunner(t *testing.T) {
	t.Setenv("EVIDRA_CONTAINERIZED", "1")
	t.Setenv("HOSTNAME", "runner-container")

	provider := newKindProvider(config.Default(), environment.ContainerNetworkNative)
	if provider.ContainerName != "runner-container" {
		t.Fatalf("ContainerName = %q, want runner-container", provider.ContainerName)
	}
}

func TestNewK3dProviderDetectsContainerRunner(t *testing.T) {
	t.Setenv("EVIDRA_CONTAINERIZED", "1")
	t.Setenv("HOSTNAME", "runner-container")

	provider := newK3dProvider(config.Default(), environment.ContainerNetworkNative)
	if provider.ContainerName != "runner-container" {
		t.Fatalf("ContainerName = %q, want runner-container", provider.ContainerName)
	}
}

func TestResolveRunnerNetworkModeDetectsOnceAndWiresProviders(t *testing.T) {
	t.Setenv("EVIDRA_CONTAINERIZED", "1")
	t.Setenv("HOSTNAME", "runner-x")
	calls := 0
	original := containerNetworkProbe
	containerNetworkProbe = func(_ context.Context, _ environment.CommandRunner, name string) (environment.ContainerNetworkMode, error) {
		calls++
		if name != "runner-x" {
			t.Errorf("probe container name = %q, want runner-x", name)
		}
		return environment.ContainerNetworkHost, nil
	}
	defer func() { containerNetworkProbe = original }()

	mode, err := resolveRunnerNetworkMode()
	if err != nil || mode != environment.ContainerNetworkHost {
		t.Fatalf("resolveRunnerNetworkMode() = %q, %v", mode, err)
	}
	if calls != 1 {
		t.Fatalf("probe calls = %d, want 1", calls)
	}
	if got := newKindProvider(config.Default(), mode).NetworkMode; got != environment.ContainerNetworkHost {
		t.Fatalf("kind NetworkMode = %q", got)
	}
	if got := newK3dProvider(config.Default(), mode).NetworkMode; got != environment.ContainerNetworkHost {
		t.Fatalf("k3d NetworkMode = %q", got)
	}
	if existing := newKindProvider(config.Default(), mode).ContainerName; existing != "runner-x" {
		t.Fatalf("kind ContainerName = %q, want runner-x", existing)
	}
}

func TestResolveRunnerNetworkModeNativeWithoutContainerization(t *testing.T) {
	t.Setenv("EVIDRA_CONTAINERIZED", "")
	original := containerNetworkProbe
	probed := false
	containerNetworkProbe = func(context.Context, environment.CommandRunner, string) (environment.ContainerNetworkMode, error) {
		probed = true
		return environment.ContainerNetworkNative, nil
	}
	defer func() { containerNetworkProbe = original }()
	mode, err := resolveRunnerNetworkMode()
	if err != nil || mode != environment.ContainerNetworkNative || probed {
		t.Fatalf("mode = %q, err = %v, probed = %v", mode, err, probed)
	}
}

func TestResolveRunnerNetworkModeFailureIsActionable(t *testing.T) {
	t.Setenv("EVIDRA_CONTAINERIZED", "1")
	t.Setenv("HOSTNAME", "runner-x")
	original := containerNetworkProbe
	containerNetworkProbe = func(context.Context, environment.CommandRunner, string) (environment.ContainerNetworkMode, error) {
		return "", errors.New("cannot inspect runner container: docker socket missing")
	}
	defer func() { containerNetworkProbe = original }()
	if _, err := resolveRunnerNetworkMode(); err == nil || !strings.Contains(err.Error(), "runner container") {
		t.Fatalf("err = %v", err)
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

func TestRunCommand_RegistersReportIdentityFlags(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cmd := newRunCommand(&cfg)

	for _, flag := range []string{"tool-server-id", "tool-server-version", "report-id"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Fatalf("expected --%s flag to be registered", flag)
		}
	}
}
