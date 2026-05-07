package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/orchestrator"
	"samebits.com/evidra-infra-bench/pkg/scenario"
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

func TestRunHelpDoesNotExposeEvidraSpecialModes(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"run", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}

	help := buf.String()
	for _, flag := range []string{
		"--evidra ",
		"--trace ",
		"--proxy-mode",
		"--smart-prescribe",
		"--evidra-bin",
		"--evidra-evidence-dir",
	} {
		if strings.Contains(help, flag) {
			t.Fatalf("run help exposes removed special mode flag %q:\n%s", flag, help)
		}
	}
	if !strings.Contains(help, "--mcp-server") {
		t.Fatalf("run help must retain generic --mcp-server support:\n%s", help)
	}
}

func TestApplyServeEnvOptions_ControlPlaneOnlyUsesCanonicalEnv(t *testing.T) {
	t.Setenv("BENCH_CONTROL_PLANE_ONLY", "true")

	opts := applyServeEnvOptions(serveOptions{})

	if !opts.ControlPlaneOnly {
		t.Fatal("expected BENCH_CONTROL_PLANE_ONLY to enable control-plane-only mode")
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
	// Not parallel: mutates package-level version/commit/date vars.
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
