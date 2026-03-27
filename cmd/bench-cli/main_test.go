package main

import (
	"bytes"
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
	want := "bench-cli v0.1.0-3-gabcdef0 (commit: abcdef0, built: 2026-03-15T12:00:00Z)"
	if got != want {
		t.Fatalf("buildVersionString() = %q, want %q", got, want)
	}
}

func TestResolveLocalAdapter_CLI(t *testing.T) {
	t.Parallel()

	got, err := resolveLocalAdapter("cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected CLI adapter")
	}
}

func TestResolveLocalAdapter_UnknownFails(t *testing.T) {
	t.Parallel()

	if _, err := resolveLocalAdapter("wat"); err == nil {
		t.Fatal("expected error for unknown adapter")
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

func TestSkillDeltaReportCommand_PrintsRemovalNotice(t *testing.T) {
	dir := t.TempDir()
	benchmark := skilldelta.BuildBenchmark(skilldelta.BenchmarkMetadata{
		Suite:       "skill-delta",
		GeneratedAt: "2026-03-15T18:10:00Z",
	}, []skilldelta.PairResult{
		{
			ScenarioID: "broken-deployment",
			Model:      "sonnet",
			Repeat:     1,
			WithoutSkill: skilldelta.RunSnapshot{
				Passed: false,
			},
			WithSkill: skilldelta.RunSnapshot{
				Passed: true,
				Scorecard: skilldelta.ScorecardMetrics{
					Available: true,
					Band:      "good",
					Signals:   []string{"repair_loop"},
				},
			},
			ComplianceDeltaPct: 100,
			TokenDelta: skilldelta.TokenDelta{
				TotalTokens: 350,
			},
		},
	})
	if err := skilldelta.WriteBenchmarkJSON(filepath.Join(dir, "benchmark.json"), benchmark); err != nil {
		t.Fatalf("WriteBenchmarkJSON: %v", err)
	}

	cmd := newRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"skill-delta", "report", "--dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill-delta report failed: %v", err)
	}
	if !strings.Contains(buf.String(), "removed") {
		t.Fatalf("expected removal notice, got: %s", buf.String())
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
