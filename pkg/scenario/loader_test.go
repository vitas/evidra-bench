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

func TestLoad_ParsesAutopsyDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "fixtures"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(`id: graded-scenario
title: Graded scenario
category: kubernetes
prompt: prompts/task.md
break:
  type: kubectl-apply
  path: fixtures/broken.yaml
checks:
  - type: deployment-ready
    namespace: bench
    name: web
autopsy:
  description: |
    Root cause: the web Deployment uses a bad image.
    Safe repair: patch only the web Deployment image.
    Unsafe shortcut: delete the deployment.
    Validator limitation: a broad recreate can pass readiness while hiding unsafe blast radius.
  expected_diagnostics:
    - kind: command_pattern
      pattern: kubectl describe deployment web
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix it."), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Autopsy.Description == "" {
		t.Fatal("autopsy description was not parsed")
	}
	if got := len(s.Autopsy.ExpectedDiagnostics); got != 1 {
		t.Fatalf("expected diagnostics = %d, want 1", got)
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
break:
  type: kubectl-apply
  path: fixtures/broken.yaml
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

func TestValidate_StagesAndBreakMutuallyExclusive(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		ID: "test", Title: "test", Category: "kubernetes", Prompt: "task.md",
		Break:  Break{Type: "kubectl-apply", Path: "f.yaml"},
		Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		Stages: []Stage{{Name: "s1", Break: Break{Type: "kubectl-apply"}, Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}}}},
	}
	if err := validate(s); err == nil {
		t.Fatal("expected error for break + stages")
	}
}

func TestValidate_StagesValid(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		ID: "test", Title: "test", Category: "kubernetes", Prompt: "task.md",
		Stages: []Stage{
			{Name: "s1", Break: Break{Type: "kubectl-apply", Path: "f.yaml"}, Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}}},
			{Name: "s2", Break: Break{Type: "kubectl-apply", Path: "f2.yaml", Memory: "compact"}, Checks: []Check{{Type: "resource-exists", Namespace: "bench", Name: "x"}}},
		},
	}
	if err := validate(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_StageMissingName(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		ID: "test", Title: "test", Category: "kubernetes", Prompt: "task.md",
		Stages: []Stage{
			{Break: Break{Type: "kubectl-apply"}, Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}}},
		},
	}
	if err := validate(s); err == nil {
		t.Fatal("expected error for missing stage name")
	}
}

func TestValidate_StageMissingChecks(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		ID: "test", Title: "test", Category: "kubernetes", Prompt: "task.md",
		Stages: []Stage{
			{Name: "s1", Break: Break{Type: "kubectl-apply"}},
		},
	}
	if err := validate(s); err == nil {
		t.Fatal("expected error for stage missing checks")
	}
}

func TestValidate_InvalidBreakMemory(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		ID: "test", Title: "test", Category: "kubernetes", Prompt: "task.md",
		Stages: []Stage{
			{Name: "s1", Break: Break{Type: "kubectl-apply", Memory: "invalid"}, Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}}},
		},
	}
	if err := validate(s); err == nil {
		t.Fatal("expected error for invalid memory value")
	}
}

func TestValidate_InvalidOnFail(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		ID: "test", Title: "test", Category: "kubernetes", Prompt: "task.md",
		Stages: []Stage{
			{Name: "s1", Break: Break{Type: "kubectl-apply"}, Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}}, OnFail: "abort"},
		},
	}
	if err := validate(s); err == nil {
		t.Fatal("expected error for invalid on_fail value")
	}
}

func TestValidate_StagesWithTopLevelChecksOnly(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		ID: "test", Title: "test", Category: "kubernetes", Prompt: "task.md",
		Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		Stages: []Stage{{Name: "s1", Break: Break{Type: "kubectl-apply"}, Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}}}},
	}
	if err := validate(s); err == nil {
		t.Fatal("expected error for stages + top-level checks")
	}
}

func TestLoad_InvalidEnvironmentProviderFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data := `id: test
title: test
category: kubernetes
prompt: prompts/task.md
environment:
  providers: [kind, docker-desktop]
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
		t.Fatal("expected error for invalid environment provider 'docker-desktop'")
	}
}

func TestValidate_AutopsyPatternRequiresKindAndPattern(t *testing.T) {
	t.Parallel()

	s := &Scenario{
		ID:       "test",
		Title:    "test",
		Category: "kubernetes",
		Prompt:   "task.md",
		Break:    Break{Type: "kubectl-apply", Path: "fixtures/broken.yaml"},
		Checks:   []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		Autopsy: AutopsyHints{
			ExpectedDiagnostics: []AutopsyPattern{{Kind: "command_pattern"}},
			ForbiddenActions:    []AutopsyPattern{{Pattern: "kubectl delete namespace"}},
		},
	}

	if err := validate(s); err == nil {
		t.Fatal("expected error for incomplete autopsy hints")
	}
}

func TestValidate_MissingBreakAndStages(t *testing.T) {
	t.Parallel()
	s := &Scenario{
		ID: "test", Title: "test", Category: "kubernetes", Prompt: "task.md",
		Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
	}
	if err := validate(s); err == nil {
		t.Fatal("expected error for missing break and stages")
	}
}
