package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/verifier"
)

func writeRunJSON(t *testing.T, dir string, bundle artifact.RunBundle) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadHistory_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	records := LoadHistory(dir)
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestLoadHistory_ReadsRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	writeRunJSON(t, filepath.Join(dir, "20260101-run1"), artifact.RunBundle{
		ScenarioID: "broken-deployment",
		Passed:     true,
		StartTime:  now.Add(-2 * time.Hour),
		EndTime:    now.Add(-2*time.Hour + time.Minute),
	})
	writeRunJSON(t, filepath.Join(dir, "20260102-run2"), artifact.RunBundle{
		ScenarioID: "broken-deployment",
		Passed:     false,
		StartTime:  now.Add(-1 * time.Hour),
		EndTime:    now.Add(-1*time.Hour + 2*time.Minute),
	})
	records := LoadHistory(dir)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	// Most recent first
	if records[0].Passed {
		t.Fatal("expected most recent run to be failed")
	}
}

func TestLoadHistory_ParsesChecks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	checks := verifier.VerifyResult{
		Passed: true,
		Checks: []verifier.CheckResult{
			{Name: "test", Type: "deployment-ready", Verdict: "pass"},
		},
	}
	checksJSON, _ := json.Marshal(checks)
	writeRunJSON(t, filepath.Join(dir, "run1"), artifact.RunBundle{
		ScenarioID: "test",
		Passed:     true,
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		Checks:     checksJSON,
	})
	records := LoadHistory(dir)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Checks == nil {
		t.Fatal("expected checks to be parsed")
	}
	if len(records[0].Checks.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(records[0].Checks.Checks))
	}
}

func TestHistoryForScenario(t *testing.T) {
	t.Parallel()
	records := []RunRecord{
		{RunBundle: artifact.RunBundle{ScenarioID: "a", Passed: true}},
		{RunBundle: artifact.RunBundle{ScenarioID: "b", Passed: false}},
		{RunBundle: artifact.RunBundle{ScenarioID: "a", Passed: false}},
	}
	result := HistoryForScenario(records, "a")
	if len(result) != 2 {
		t.Fatalf("expected 2 records for scenario a, got %d", len(result))
	}
}

func TestComputeStats(t *testing.T) {
	t.Parallel()
	records := []RunRecord{
		{RunBundle: artifact.RunBundle{Passed: false}},
		{RunBundle: artifact.RunBundle{Passed: true}},
		{RunBundle: artifact.RunBundle{Passed: true}},
	}
	stats := ComputeStats(records)
	if stats.TotalRuns != 3 {
		t.Fatalf("expected 3 total, got %d", stats.TotalRuns)
	}
	if stats.PassCount != 2 {
		t.Fatalf("expected 2 pass, got %d", stats.PassCount)
	}
	if stats.FailCount != 1 {
		t.Fatalf("expected 1 fail, got %d", stats.FailCount)
	}
	if stats.LastResult != "fail" {
		t.Fatalf("expected last=fail, got %s", stats.LastResult)
	}
}

func TestComputeStats_Empty(t *testing.T) {
	t.Parallel()
	stats := ComputeStats(nil)
	if stats.TotalRuns != 0 {
		t.Fatalf("expected 0, got %d", stats.TotalRuns)
	}
	if stats.LastResult != "" {
		t.Fatalf("expected empty, got %s", stats.LastResult)
	}
}

func TestRunRecord_Duration(t *testing.T) {
	t.Parallel()
	r := RunRecord{
		RunBundle: artifact.RunBundle{
			StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2026, 1, 1, 0, 2, 30, 0, time.UTC),
		},
	}
	if r.Duration() != 2*time.Minute+30*time.Second {
		t.Fatalf("expected 2m30s, got %v", r.Duration())
	}
}
