package scenario

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testScenarioYAML = `id: broken-deployment
title: Fix a broken deployment
category: kubernetes
tags: [deployment, readiness]
prompt: prompts/task.md
timeout: "3m"
bootstrap:
  - type: kubectl-apply
    path: fixtures/baseline.yaml
after_break:
  - type: kubectl-apply
    path: fixtures/observe.yaml
break:
  type: apply
  path: fixtures/broken.yaml
checks:
  - type: deployment-ready
    namespace: bench
    name: web
scope:
  namespaces: [bench]
`

const testScenarioWithChaosYAML = `id: broken-deployment
title: Fix a broken deployment
category: kubernetes
tags: [deployment, readiness]
prompt: prompts/task.md
timeout: "3m"
bootstrap:
  - type: kubectl-apply
    path: fixtures/baseline.yaml
break:
  type: apply
  path: fixtures/broken.yaml
chaos:
  mode: repeat
  stop_on_agent_done: true
  steps:
    - at: 20s
      name: kill-web
      type: kubectl
      args: [delete, pod, -n, bench, web-0]
      allow_failure: true
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`

func writeTestScenario(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "fixtures"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(testScenarioYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix the broken deployment."), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoad_ValidScenario(t *testing.T) {
	t.Parallel()
	dir := writeTestScenario(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if s.ID != "broken-deployment" {
		t.Fatalf("unexpected id: %s", s.ID)
	}
	if s.Category != "kubernetes" {
		t.Fatalf("unexpected category: %s", s.Category)
	}
	if len(s.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(s.Checks))
	}
}

func TestLoad_ParsesChaos(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "fixtures"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(testScenarioWithChaosYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix the broken deployment."), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if s.Chaos.Mode != "repeat" {
		t.Fatalf("chaos mode = %q, want repeat", s.Chaos.Mode)
	}
	if !s.Chaos.StopOnAgentDone {
		t.Fatal("chaos stop_on_agent_done = false, want true")
	}
	if len(s.Chaos.Steps) != 1 {
		t.Fatalf("expected 1 chaos step, got %d", len(s.Chaos.Steps))
	}
	if s.Chaos.Steps[0].At.Duration != 20*time.Second {
		t.Fatalf("chaos step at = %s, want 20s", s.Chaos.Steps[0].At.Duration)
	}
	if !s.Chaos.Steps[0].AllowFailure {
		t.Fatal("chaos step allow_failure = false, want true")
	}
}

func TestLoad_ResolvesPromptPath(t *testing.T) {
	t.Parallel()
	dir := writeTestScenario(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(s.Prompt) {
		t.Fatalf("prompt path not resolved: %s", s.Prompt)
	}
}

func TestLoad_ResolvesBreakPath(t *testing.T) {
	t.Parallel()
	dir := writeTestScenario(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(s.Break.Path) {
		t.Fatalf("break path not resolved: %s", s.Break.Path)
	}
}

func TestLoad_PreservesRemoteBootstrapPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(`id: remote-bootstrap
title: Remote bootstrap
category: argocd
prompt: prompts/task.md
bootstrap:
  - type: kubectl-apply
    path: https://example.com/install.yaml
checks:
  - type: argocd-app-healthy
    name: web
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix it."), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Bootstrap[0].Path; got != "https://example.com/install.yaml" {
		t.Fatalf("bootstrap path = %q, want remote URL", got)
	}
}

func TestLoad_ResolvesBootstrapPaths(t *testing.T) {
	t.Parallel()
	dir := writeTestScenario(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Bootstrap) != 1 {
		t.Fatalf("expected 1 bootstrap step, got %d", len(s.Bootstrap))
	}
	if !filepath.IsAbs(s.Bootstrap[0].Path) {
		t.Fatalf("bootstrap path not resolved: %s", s.Bootstrap[0].Path)
	}
}

func TestLoad_ResolvesAfterBreakPaths(t *testing.T) {
	t.Parallel()
	dir := writeTestScenario(t)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.AfterBreak) != 1 {
		t.Fatalf("expected 1 after_break step, got %d", len(s.AfterBreak))
	}
	if !filepath.IsAbs(s.AfterBreak[0].Path) {
		t.Fatalf("after_break path not resolved: %s", s.AfterBreak[0].Path)
	}
}

func TestLoad_ChaosStepMissingAt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	data := `id: broken-deployment
title: Fix a broken deployment
category: kubernetes
prompt: prompts/task.md
chaos:
  steps:
    - name: kill-web
      type: kubectl
      args: [delete, pod, -n, bench, web-0]
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix it."), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for chaos step missing at")
	}
}

func TestLoad_MissingID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data := `title: test
category: test
prompt: task.md
checks:
  - type: deployment-ready
`
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestLoad_MissingChecks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data := `id: test
title: test
category: test
prompt: task.md
`
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for missing checks")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadAll_FindsScenarios(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	catDir := filepath.Join(base, "kubernetes", "broken-deployment")
	if err := os.MkdirAll(catDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catDir, "scenario.yaml"), []byte(testScenarioYAML), 0644); err != nil {
		t.Fatal(err)
	}

	scenarios, err := LoadAll(base)
	if err != nil {
		t.Fatalf("load all failed: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	if scenarios[0].Path != "kubernetes/broken-deployment" {
		t.Fatalf("unexpected scenario path: %s", scenarios[0].Path)
	}
}

func TestResolve_ByID(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	catDir := filepath.Join(base, "kubernetes", "broken-deployment")
	if err := os.MkdirAll(catDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catDir, "scenario.yaml"), []byte(testScenarioYAML), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Resolve(base, "broken-deployment")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if s.ID != "broken-deployment" {
		t.Fatalf("unexpected id: %s", s.ID)
	}
}
