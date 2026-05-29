package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/orchestrator"
)

type noopParallelRunner struct {
	done      chan struct{}
	scenarios []string
	runCfg    config.Config
}

func newNoopParallelRunner() *noopParallelRunner {
	return &noopParallelRunner{done: make(chan struct{}, 1)}
}

func (r *noopParallelRunner) RunParallel(_ context.Context, runCfg config.Config, _ orchestrator.ProgressReporter, scenarios []string, _ []string, _ int, _ int, _ string) (*orchestrator.RunResult, error) {
	r.runCfg = runCfg
	r.scenarios = scenarios
	r.done <- struct{}{}
	return &orchestrator.RunResult{}, nil
}

func (r *noopParallelRunner) waitCalled(t *testing.T) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
		t.Fatal("runner was not called within timeout")
	}
}

// writeTestScenario creates a minimal scenario YAML in dir/category/name/scenario.yaml.
func writeTestScenario(t *testing.T, dir, category, id string, providers []string) {
	t.Helper()
	scenarioDir := filepath.Join(dir, category, id)
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	providerLine := ""
	if len(providers) > 0 {
		providerLine = fmt.Sprintf("  providers: [%s]\n", strings.Join(providers, ", "))
	}
	yaml := fmt.Sprintf(`id: %s
title: Test %s
category: %s
prompt: prompts/task.md
environment:
%sbreak:
  type: kubectl
  command: "get pods"
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`, id, id, category, providerLine)
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeTestScenarioWithProfile creates a scenario YAML with an explicit execution profile.
func writeTestScenarioWithProfile(t *testing.T, dir, category, id, profile string) {
	t.Helper()
	scenarioDir := filepath.Join(dir, category, id)
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := fmt.Sprintf(`id: %s
title: Test %s
category: %s
prompt: prompts/task.md
environment:
  profile: %s
break:
  type: kubectl
  command: "get pods"
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`, id, id, category, profile)
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

type fakeServeOrchestrator struct {
	provisionCalls int
	teardownCalls  int
}

func (f *fakeServeOrchestrator) Provision(context.Context) (string, error) {
	f.provisionCalls++
	return "/tmp/kubeconfig", nil
}

func (f *fakeServeOrchestrator) Teardown(context.Context) {
	f.teardownCalls++
}

func (f *fakeServeOrchestrator) RunParallel(context.Context, config.Config, orchestrator.ProgressReporter, []string, []string, int, int, string) (*orchestrator.RunResult, error) {
	return &orchestrator.RunResult{}, nil
}
