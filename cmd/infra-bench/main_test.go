package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/tui"
)

func TestMainHelp(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}
}

func TestRunCommand_MissingScenario(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"run"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing scenario")
	}
}

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

func TestScenarioListCommand(t *testing.T) {
	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "kubernetes", "broken-deployment")
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlContent := `id: broken-deployment
title: Fix broken deployment
category: kubernetes
prompt: prompts/task.md
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
	cmd.SetArgs([]string{"scenario", "list", "--scenarios-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(buf.String(), "kubernetes/broken-deployment") {
		t.Fatalf("expected relative scenario path in output, got %q", buf.String())
	}
}

func TestApplyLabFlagOverrides_PropagatesRunsDir(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.RunsDir = "/tmp/custom-runs"
	cfg.Provider = "bifrost"

	labCfg := tui.DefaultLabConfig()
	flags := pflag.NewFlagSet("lab", pflag.ContinueOnError)
	flags.String("runs-dir", "", "")
	flags.String("provider", "", "")
	if err := flags.Set("runs-dir", cfg.RunsDir); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("provider", cfg.Provider); err != nil {
		t.Fatal(err)
	}

	applyLabFlagOverrides(&labCfg, cfg, flags)

	if labCfg.RunsDir != cfg.RunsDir {
		t.Fatalf("runs dir = %q, want %q", labCfg.RunsDir, cfg.RunsDir)
	}
	if labCfg.Provider != cfg.Provider {
		t.Fatalf("provider = %q, want %q", labCfg.Provider, cfg.Provider)
	}
}
