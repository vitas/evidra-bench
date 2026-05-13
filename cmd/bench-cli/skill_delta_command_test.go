package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/skilldelta"
)

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
