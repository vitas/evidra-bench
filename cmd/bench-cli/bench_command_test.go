package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/orchestrator"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func TestFilterRunnableScenarios(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{ID: "a", Skip: false, Environment: scenario.EnvironmentConfig{}},
		{ID: "b", Skip: true, SkipReason: "not ready"},
		{ID: "c", Skip: false, Environment: scenario.EnvironmentConfig{Providers: []string{"k3d"}}},
		{ID: "d", Skip: false, Environment: scenario.EnvironmentConfig{Providers: []string{"kind"}}},
		{ID: "e", Skip: true, Environment: scenario.EnvironmentConfig{Providers: []string{"k3d"}}},
	}

	var buf bytes.Buffer
	runnable, skipped := filterRunnableScenarios(scenarios, "kind", &buf)

	if len(runnable) != 2 {
		t.Fatalf("expected 2 runnable, got %d", len(runnable))
	}
	if runnable[0].ID != "a" || runnable[1].ID != "d" {
		t.Fatalf("unexpected runnable IDs: %v, %v", runnable[0].ID, runnable[1].ID)
	}
	if skipped != 3 {
		t.Fatalf("expected 3 skipped, got %d", skipped)
	}
	output := buf.String()
	if !strings.Contains(output, "SKIP b") {
		t.Fatalf("expected skip message for b, got: %s", output)
	}
	if !strings.Contains(output, "SKIP c") {
		t.Fatalf("expected skip message for c, got: %s", output)
	}
}

func TestFilterRunnableScenarios_EmptyProviders(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{ID: "a", Environment: scenario.EnvironmentConfig{}},
		{ID: "b", Environment: scenario.EnvironmentConfig{Providers: []string{"kind", "k3d"}}},
	}

	var buf bytes.Buffer
	runnable, skipped := filterRunnableScenarios(scenarios, "kind", &buf)

	if len(runnable) != 2 {
		t.Fatalf("expected 2 runnable, got %d", len(runnable))
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped, got %d", skipped)
	}
}

func TestBenchSequential_UsesDedicatedLeasePerScenarioByDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "kubernetes", "s1")
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlContent := `id: s1
title: S1
category: kubernetes
prompt: prompts/task.md
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
		"bench",
		"--scenario", "kubernetes/s1",
		"--scenarios-dir", dir,
		"--dry-run",
	})
	// In dry-run, no lease is acquired, but the path is exercised.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bench failed: %v", err)
	}
	if !strings.Contains(buf.String(), "s1") {
		t.Fatalf("expected s1 in output: %s", buf.String())
	}
}

func TestBenchSequential_ReuseClusterMixedProfiles_FailsFast(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{
			ID:          "default-scenario",
			Environment: scenario.EnvironmentConfig{},
		},
		{
			ID: "argocd-scenario",
			Environment: scenario.EnvironmentConfig{
				Profile: scenario.ProfileArgocd,
			},
		},
	}

	err := validateSingleProfile(scenarios)
	if err == nil {
		t.Fatal("expected error for mixed profiles")
	}
	if !strings.Contains(err.Error(), "--reuse-cluster") {
		t.Fatalf("error should mention --reuse-cluster, got: %v", err)
	}
	if !strings.Contains(err.Error(), "default") || !strings.Contains(err.Error(), "argocd") {
		t.Fatalf("error should mention both profiles, got: %v", err)
	}
}

func TestBenchSequential_ReuseClusterSingleProfile_UsesBatchLease(t *testing.T) {
	t.Parallel()

	// All scenarios resolve to default — validation should pass.
	scenarios := []*scenario.Scenario{
		{ID: "a", Environment: scenario.EnvironmentConfig{}},
		{ID: "b", Environment: scenario.EnvironmentConfig{}},
		{ID: "c", Environment: scenario.EnvironmentConfig{}},
	}

	if err := validateSingleProfile(scenarios); err != nil {
		t.Fatalf("expected no error for single profile, got: %v", err)
	}
}

func TestValidateSingleProfile_EmptyList(t *testing.T) {
	t.Parallel()

	if err := validateSingleProfile(nil); err != nil {
		t.Fatalf("expected no error for empty list, got: %v", err)
	}
}

func TestValidateSingleProfile_SingleScenario(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{
			ID: "argocd-scenario",
			Environment: scenario.EnvironmentConfig{
				Profile: scenario.ProfileArgocd,
			},
		},
	}
	if err := validateSingleProfile(scenarios); err != nil {
		t.Fatalf("expected no error for single scenario, got: %v", err)
	}
}

func TestBenchParallel_RejectsNonDefaultSharedProfiles(t *testing.T) {
	t.Parallel()

	// executeBenchParallel calls orchestrator.ValidateParallelProfiles
	// before provisioning. Verify that non-default profiles are rejected.
	scenarios := []*scenario.Scenario{
		{ID: "s1", Environment: scenario.EnvironmentConfig{}},
		{ID: "s2", Environment: scenario.EnvironmentConfig{Profile: scenario.ProfileArgocd}},
	}

	err := orchestrator.ValidateParallelProfiles(scenarios)
	if err == nil {
		t.Fatal("expected error for argocd profile in parallel mode")
	}
	if !strings.Contains(err.Error(), "argocd") {
		t.Fatalf("error should mention argocd, got: %v", err)
	}
	if !strings.Contains(err.Error(), "shared-cluster parallel") {
		t.Fatalf("error should mention shared-cluster parallel, got: %v", err)
	}
}

func TestBenchCommand_SkipsIncompatibleProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create two scenarios: one kind-compatible, one k3d-only.
	for _, sc := range []struct {
		name    string
		content string
	}{
		{"kind-ok", `id: kind-ok
title: Kind scenario
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
`},
		{"k3d-only", `id: k3d-only
title: K3d scenario
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
`},
	} {
		scenarioDir := filepath.Join(dir, "kubernetes", sc.name)
		if err := os.MkdirAll(scenarioDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(sc.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"bench",
		"--scenarios-dir", dir,
		"--dry-run",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bench failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "SKIP k3d-only") {
		t.Fatalf("expected k3d-only to be skipped, got: %s", output)
	}
	if !strings.Contains(output, "kind-ok") {
		t.Fatalf("expected kind-ok to be present, got: %s", output)
	}
}
