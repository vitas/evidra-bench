package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	if err := os.MkdirAll(filepath.Join(runDir, "evidra"), 0755); err != nil {
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
	writeAuditJSONFixture(t, filepath.Join(runDir, "evidra", "scorecard.json"), map[string]any{
		"score":          90,
		"band":           "good",
		"signal_summary": signalSummary,
	})
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
