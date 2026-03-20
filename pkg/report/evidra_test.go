package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReporter_WriteOffline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := NewReporter(Config{EvidencePath: dir})
	entries := []EvidenceEntry{
		{
			ID:         "test-1",
			Type:       "benchmark-run",
			Actor:      "test-agent",
			Timestamp:  time.Now(),
			ScenarioID: "broken-deployment",
			Adapter:    "cli",
			Passed:     true,
			ExitCode:   0,
			Duration:   30 * time.Second,
		},
	}
	if err := r.WriteOffline(entries); err != nil {
		t.Fatalf("write offline failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "evidence.jsonl"))
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if !strings.Contains(string(data), "broken-deployment") {
		t.Fatalf("evidence missing scenario: %s", data)
	}
}

func TestReporter_WriteOffline_MissingPath(t *testing.T) {
	t.Parallel()
	r := NewReporter(Config{})
	if err := r.WriteOffline(nil); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestReporter_WriteOffline_AppendsEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := NewReporter(Config{EvidencePath: dir})

	entry1 := EvidenceEntry{ID: "entry-1", ScenarioID: "s1", Timestamp: time.Now()}
	entry2 := EvidenceEntry{ID: "entry-2", ScenarioID: "s2", Timestamp: time.Now()}

	if err := r.WriteOffline([]EvidenceEntry{entry1}); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteOffline([]EvidenceEntry{entry2}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "evidence.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestReporter_Report_OfflineOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := NewReporter(Config{EvidencePath: dir})
	entries := []EvidenceEntry{
		{ID: "test-1", Timestamp: time.Now()},
	}
	if err := r.Report(entries); err != nil {
		t.Fatalf("report failed: %v", err)
	}
}

func TestReporter_WriteOffline_ValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := NewReporter(Config{EvidencePath: dir})
	entries := []EvidenceEntry{
		{ID: "valid-1", Timestamp: time.Now(), ScenarioID: "test"},
	}
	if err := r.WriteOffline(entries); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "evidence.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed EvidenceEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.ID != "valid-1" {
		t.Fatalf("unexpected ID: %s", parsed.ID)
	}
}
