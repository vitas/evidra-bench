package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/skilldelta"
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

func TestBuildVersionString_UsesBuildMetadata(t *testing.T) {
	t.Parallel()

	originalVersion, originalCommit, originalDate := version, commit, date
	t.Cleanup(func() {
		version = originalVersion
		commit = originalCommit
		date = originalDate
	})

	version = "v0.1.0-3-gabcdef0"
	commit = "abcdef0"
	date = "2026-03-15T12:00:00Z"

	got := buildVersionString()
	want := "infra-bench v0.1.0-3-gabcdef0 (commit: abcdef0, built: 2026-03-15T12:00:00Z)"
	if got != want {
		t.Fatalf("buildVersionString() = %q, want %q", got, want)
	}
}

func TestSkillDeltaRunCommand_DryRunWritesPairJSON(t *testing.T) {
	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "kubernetes", "broken-deployment")
	if err := os.MkdirAll(filepath.Join(scenarioDir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "prompts", "task.md"), []byte("Fix it."), 0644); err != nil {
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

	noSkillPrompt := filepath.Join(dir, "no-skill.md")
	withSkillPrompt := filepath.Join(dir, "with-skill.md")
	if err := os.WriteFile(noSkillPrompt, []byte("no skill"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(withSkillPrompt, []byte("with skill"), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "runs")
	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"skill-delta", "run",
		"--scenario", "kubernetes/broken-deployment",
		"--model", "sonnet",
		"--provider", "claude",
		"--repeats", "1",
		"--dry-run",
		"--scenarios-dir", dir,
		"--no-skill-prompt", noSkillPrompt,
		"--with-skill-prompt", withSkillPrompt,
		"--out-dir", outDir,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill-delta run failed: %v", err)
	}

	pairPath := filepath.Join(outDir, "cases", "broken-deployment", "sonnet", "repeat-1", "pair.json")
	if _, err := os.Stat(pairPath); err != nil {
		t.Fatalf("pair.json missing: %v", err)
	}
	if !strings.Contains(buf.String(), pairPath) {
		t.Fatalf("output missing pair path: %s", buf.String())
	}
}

func TestSkillDeltaAggregateCommand_WritesBenchmarkArtifacts(t *testing.T) {
	dir := t.TempDir()
	pairPath := filepath.Join(dir, "cases", "broken-deployment", "sonnet", "repeat-1", "pair.json")
	if err := skilldelta.WritePairJSON(pairPath, skilldelta.PairResult{
		ScenarioID: "broken-deployment",
		Model:      "sonnet",
		Provider:   "claude",
		Repeat:     1,
		WithoutSkill: skilldelta.RunSnapshot{
			Passed:           false,
			DurationSeconds:  10,
			TotalTokens:      1250,
			EstimatedCostUSD: 0.015,
			Protocol: skilldelta.ProtocolMetrics{
				ComplianceRatePct: 0,
			},
		},
		WithSkill: skilldelta.RunSnapshot{
			Passed:           true,
			DurationSeconds:  14,
			TotalTokens:      1600,
			EstimatedCostUSD: 0.021,
			Protocol: skilldelta.ProtocolMetrics{
				ComplianceRatePct: 100,
			},
		},
		TokenDelta: skilldelta.TokenDelta{
			TotalTokens: 350,
		},
		ComplianceDeltaPct:   100,
		DurationDeltaSeconds: 4,
		CostDeltaUSD:         0.006,
	}); err != nil {
		t.Fatalf("WritePairJSON: %v", err)
	}

	cmd := newRootCommand()
	cmd.SetArgs([]string{"skill-delta", "aggregate", "--dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill-delta aggregate failed: %v", err)
	}

	for _, name := range []string{"benchmark.json", "benchmark.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
}
