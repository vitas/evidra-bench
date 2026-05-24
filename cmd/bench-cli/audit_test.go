package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/localstore"
)

func TestSignalAuditCommand_WritesJSON(t *testing.T) {
	runsDir := t.TempDir()
	manifestPath := writeAuditManifestFixture(t, runsDir, `
broken-deployment:
  primary_signal: retry_loop
  expected_signals: [retry_loop]
`)
	writeAuditedRunFixture(t, filepath.Join(runsDir, "run-a"), "broken-deployment", "sonnet", "claude", map[string]any{
		"retry_loop": map[string]any{"count": 1},
	})

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"audit", "signals", "--runs-dir", runsDir, "--manifest", manifestPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit signals failed: %v", err)
	}

	outputPath := filepath.Join(runsDir, "signal-audit.json")
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("signal-audit.json missing: %v", err)
	}
	if !strings.Contains(buf.String(), "audited runs: 1") {
		t.Fatalf("summary missing audited run count: %s", buf.String())
	}
}

func TestSignalAuditCommand_DefaultManifestPath_Works(t *testing.T) {
	runsDir := t.TempDir()
	writeAuditedRunFixture(t, filepath.Join(runsDir, "run-a"), "broken-deployment", "sonnet", "claude", map[string]any{
		"retry_loop": map[string]any{"count": 1},
	})

	cmd := newRootCommand()
	cmd.SetArgs([]string{"audit", "signals", "--runs-dir", runsDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit signals failed with default manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runsDir, "signal-audit.json")); err != nil {
		t.Fatalf("signal-audit.json missing: %v", err)
	}
}

func TestSignalAuditCommand_FiltersByScenario(t *testing.T) {
	runsDir := t.TempDir()
	manifestPath := writeAuditManifestFixture(t, runsDir, `
broken-deployment:
  primary_signal: retry_loop
  expected_signals: [retry_loop]
networkpolicy-blocking:
  primary_signal: blast_radius
  expected_signals: [blast_radius]
`)
	writeAuditedRunFixture(t, filepath.Join(runsDir, "run-a"), "broken-deployment", "sonnet", "claude", map[string]any{
		"retry_loop": map[string]any{"count": 1},
	})
	writeAuditedRunFixture(t, filepath.Join(runsDir, "run-b"), "networkpolicy-blocking", "sonnet", "claude", map[string]any{
		"blast_radius": map[string]any{"count": 1},
	})

	cmd := newRootCommand()
	cmd.SetArgs([]string{"audit", "signals", "--runs-dir", runsDir, "--manifest", manifestPath, "--scenario", "broken-deployment"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit signals failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(runsDir, "signal-audit.json"))
	if err != nil {
		t.Fatalf("read signal-audit.json: %v", err)
	}
	var report struct {
		RunCount         int `json:"run_count"`
		ScenarioFindings []struct {
			ScenarioID string `json:"scenario_id"`
		} `json:"scenario_findings"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode signal-audit.json: %v", err)
	}
	if report.RunCount != 1 {
		t.Fatalf("run_count = %d, want 1", report.RunCount)
	}
	for _, finding := range report.ScenarioFindings {
		if finding.ScenarioID != "broken-deployment" {
			t.Fatalf("unexpected scenario finding: %s", finding.ScenarioID)
		}
	}
}

func TestArtifactCoverageAuditCommand_WritesJSON(t *testing.T) {
	runsDir := t.TempDir()
	runDir := filepath.Join(runsDir, "run-complete")
	writeCoverageRunArtifacts(t, runDir, 1, true)

	resultsStore, err := localstore.Open(runsDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := resultsStore.Insert(localstore.RunRecord{
		ID:          "run-complete",
		ScenarioID:  "broken-deployment",
		Model:       "sonnet",
		Provider:    "claude",
		Adapter:     "cli",
		Passed:      true,
		ExitCode:    0,
		ArtifactDir: runDir,
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := resultsStore.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"audit", "coverage", "--runs-dir", runsDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit coverage failed: %v", err)
	}

	outputPath := filepath.Join(runsDir, "artifact-coverage.json")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("artifact-coverage.json missing: %v", err)
	}
	var report struct {
		TotalRuns       int     `json:"total_runs"`
		CompleteRuns    int     `json:"complete_runs"`
		CoveragePercent float64 `json:"coverage_percent"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode artifact coverage: %v", err)
	}
	if report.TotalRuns != 1 || report.CompleteRuns != 1 || report.CoveragePercent != 100 {
		t.Fatalf("report = %+v, want one complete run", report)
	}
	if !strings.Contains(buf.String(), "artifact coverage: 1/1 complete (100.0%)") {
		t.Fatalf("summary missing coverage line: %s", buf.String())
	}
}

func TestArtifactCoverageAuditCommand_FailOnGaps(t *testing.T) {
	runsDir := t.TempDir()
	runDir := filepath.Join(runsDir, "run-missing")
	writeAuditJSONFixture(t, filepath.Join(runDir, "run.json"), map[string]any{"scenario_id": "broken-deployment"})

	resultsStore, err := localstore.Open(runsDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := resultsStore.Insert(localstore.RunRecord{
		ID:          "run-missing",
		ScenarioID:  "broken-deployment",
		Model:       "sonnet",
		Provider:    "claude",
		Adapter:     "cli",
		Passed:      false,
		ExitCode:    -1,
		ArtifactDir: runDir,
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := resultsStore.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"audit", "coverage", "--runs-dir", runsDir, "--fail-on-gaps"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected fail-on-gaps error")
	}
	if !strings.Contains(err.Error(), "artifact coverage gaps") {
		t.Fatalf("error = %v, want coverage gaps", err)
	}
	if !strings.Contains(buf.String(), "missing by artifact:") {
		t.Fatalf("summary missing gaps: %s", buf.String())
	}
}

func writeAuditManifestFixture(t *testing.T, dir, body string) string {
	t.Helper()

	path := filepath.Join(dir, "signal-audit.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0644); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}
	return path
}

func writeAuditedRunFixture(t *testing.T, runDir, scenarioID, model, provider string, signalSummary map[string]any) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(runDir, "evidence"), 0755); err != nil {
		t.Fatalf("mkdir run fixture: %v", err)
	}
	runJSON := map[string]any{
		"scenario_id": scenarioID,
		"passed":      true,
		"start_time":  time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		"end_time":    time.Date(2026, 3, 15, 12, 1, 0, 0, time.UTC),
		"metadata": map[string]string{
			"model":    model,
			"provider": provider,
		},
	}
	writeAuditJSONFixture(t, filepath.Join(runDir, "run.json"), runJSON)
	writeAuditJSONFixture(t, filepath.Join(runDir, "evidence", "scorecard.json"), map[string]any{
		"score":          90,
		"band":           "good",
		"signal_summary": signalSummary,
	})
}

func writeCoverageRunArtifacts(t *testing.T, runDir string, totalSteps int, passed bool) {
	t.Helper()
	writeAuditJSONFixture(t, filepath.Join(runDir, "run.json"), map[string]any{
		"scenario_id": "broken-deployment",
		"passed":      passed,
	})
	toolCalls := make([]map[string]any, 0, totalSteps)
	for i := 0; i < totalSteps; i++ {
		toolCalls = append(toolCalls, map[string]any{"tool": "run_command"})
	}
	writeAuditJSONFixture(t, filepath.Join(runDir, "tool-calls.json"), toolCalls)
	writeAuditJSONFixture(t, filepath.Join(runDir, "timeline.json"), map[string]any{
		"total_steps": totalSteps,
	})
	writeAuditJSONFixture(t, filepath.Join(runDir, "run-events.json"), []map[string]any{
		{"phase": "run", "status": "started"},
		{"phase": "run", "status": "completed"},
	})
	if !passed {
		writeAuditJSONFixture(t, filepath.Join(runDir, "failure-autopsy.json"), map[string]any{
			"outcome": "fail",
		})
	}
}

func writeAuditJSONFixture(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir json fixture: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write json fixture: %v", err)
	}
}
