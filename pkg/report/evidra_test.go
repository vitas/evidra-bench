package report

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestConfig_IsOnline(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		cfg    Config
		online bool
	}{
		{"empty", Config{}, false},
		{"url only", Config{EvidraURL: "http://example.com"}, false},
		{"key only", Config{EvidraAPIKey: "key"}, false},
		{"both", Config{EvidraURL: "http://example.com", EvidraAPIKey: "key"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.cfg.IsOnline(); got != tt.online {
				t.Fatalf("IsOnline() = %v, want %v", got, tt.online)
			}
		})
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

func TestReporter_Report_UploadsBenchmarkRunWhenOnline(t *testing.T) {
	t.Parallel()

	var seenAuth string
	var seenBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/benchmark/run" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run_id":"run-123","status":"accepted"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	r := NewReporter(Config{
		EvidencePath: dir,
		EvidraURL:    srv.URL,
		EvidraAPIKey: "secret",
	})
	entries := []EvidenceEntry{
		{ID: "test-1", Timestamp: time.Now(), ScenarioID: "broken-deployment"},
	}

	if err := r.Report(entries); err != nil {
		t.Fatalf("report failed: %v", err)
	}
	if seenAuth != "Bearer secret" {
		t.Fatalf("unexpected auth header: %q", seenAuth)
	}
	if seenBody["suite"] != "infra-bench" {
		t.Fatalf("unexpected suite payload: %#v", seenBody)
	}
	results, ok := seenBody["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("unexpected benchmark payload: %#v", seenBody)
	}
	result, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected result payload: %#v", results[0])
	}
	if result["case_id"] != "broken-deployment" {
		t.Fatalf("unexpected case_id payload: %#v", result)
	}
}
