package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenAndClose(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if s == nil {
		t.Fatal("store is nil")
	}
}

func TestInsertAndQuery(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	r := RunRecord{
		ID:         "run-1",
		ScenarioID: "broken-deployment",
		Model:      "haiku",
		Provider:   "claude",
		Passed:     true,
		Duration:   45.2,
		Turns:      11,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.Insert(r); err != nil {
		t.Fatalf("insert: %v", err)
	}

	runs, err := s.Query(QueryFilters{ScenarioID: "broken-deployment"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Model != "haiku" {
		t.Fatalf("expected haiku, got %s", runs[0].Model)
	}
}

func TestQueryByModel(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	now := time.Now().UTC()

	s.Insert(RunRecord{ID: "r1", ScenarioID: "s1", Model: "haiku", CreatedAt: now})
	s.Insert(RunRecord{ID: "r2", ScenarioID: "s1", Model: "sonnet", CreatedAt: now})
	s.Insert(RunRecord{ID: "r3", ScenarioID: "s2", Model: "haiku", CreatedAt: now})

	runs, _ := s.Query(QueryFilters{Model: "haiku"})
	if len(runs) != 2 {
		t.Fatalf("expected 2 haiku runs, got %d", len(runs))
	}
}

func TestQueryPassedOnly(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	now := time.Now().UTC()

	s.Insert(RunRecord{ID: "r1", ScenarioID: "s1", Passed: true, CreatedAt: now})
	s.Insert(RunRecord{ID: "r2", ScenarioID: "s1", Passed: false, CreatedAt: now})

	runs, _ := s.Query(QueryFilters{PassedOnly: true})
	if len(runs) != 1 {
		t.Fatalf("expected 1 passed run, got %d", len(runs))
	}
}

func TestQueryLimit(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	now := time.Now().UTC()

	for i := 0; i < 10; i++ {
		s.Insert(RunRecord{ID: fmt.Sprintf("r%d", i), ScenarioID: "s1", CreatedAt: now})
	}

	runs, _ := s.Query(QueryFilters{Limit: 3})
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}
}

func TestStats(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	now := time.Now().UTC()

	s.Insert(RunRecord{ID: "r1", ScenarioID: "s1", Passed: true, CreatedAt: now})
	s.Insert(RunRecord{ID: "r2", ScenarioID: "s1", Passed: false, CreatedAt: now})
	s.Insert(RunRecord{ID: "r3", ScenarioID: "s2", Passed: true, CreatedAt: now})

	st, err := s.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if st.TotalRuns != 3 {
		t.Fatalf("expected 3 total, got %d", st.TotalRuns)
	}
	if st.PassCount != 2 {
		t.Fatalf("expected 2 pass, got %d", st.PassCount)
	}
	if len(st.ByScenario) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(st.ByScenario))
	}
}

func TestJSONLBackup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, _ := Open(dir)
	defer s.Close()

	s.Insert(RunRecord{ID: "r1", ScenarioID: "s1", Model: "haiku", CreatedAt: time.Now().UTC()})
	s.Insert(RunRecord{ID: "r2", ScenarioID: "s2", Model: "sonnet", CreatedAt: time.Now().UTC()})

	// Check JSONL exists
	jsonlPath := filepath.Join(dir, "results.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Fatalf("expected 2 lines in jsonl, got %d", lines)
	}
}

func TestRebuild(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, _ := Open(dir)

	s.Insert(RunRecord{ID: "r1", ScenarioID: "s1", Passed: true, CreatedAt: time.Now().UTC()})
	s.Insert(RunRecord{ID: "r2", ScenarioID: "s2", Passed: false, CreatedAt: time.Now().UTC()})
	s.Close()

	// Delete DB, keep JSONL
	os.Remove(filepath.Join(dir, "bench.db"))

	// Reopen and rebuild
	s2, _ := Open(dir)
	defer s2.Close()

	count, err := s2.Rebuild()
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rebuilt, got %d", count)
	}

	runs, _ := s2.Query(QueryFilters{})
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs after rebuild, got %d", len(runs))
	}
}

func TestInsertAndQuery_PersistsMetadataJSONAndEstimatedCost(t *testing.T) {
	t.Parallel()

	s := testStore(t)
	now := time.Now().UTC()

	rec := RunRecord{
		ID:            "run-meta",
		ScenarioID:    "s1",
		EstimatedCost: 1.23,
		MetadataJSON:  `{"contract_version":"v1.0.1","prompt_version":"sha256:test"}`,
		CreatedAt:     now,
	}
	if err := s.Insert(rec); err != nil {
		t.Fatalf("insert: %v", err)
	}

	runs, err := s.Query(QueryFilters{ScenarioID: "s1"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].EstimatedCost != 1.23 {
		t.Fatalf("estimated_cost = %v", runs[0].EstimatedCost)
	}
	if runs[0].MetadataJSON != rec.MetadataJSON {
		t.Fatalf("metadata_json = %q", runs[0].MetadataJSON)
	}

	data, err := os.ReadFile(s.jsonlPath)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	var decoded RunRecord
	if err := json.Unmarshal(data[:len(data)-1], &decoded); err != nil {
		t.Fatalf("decode jsonl: %v", err)
	}
	if decoded.MetadataJSON != rec.MetadataJSON {
		t.Fatalf("jsonl metadata_json = %q", decoded.MetadataJSON)
	}
}
