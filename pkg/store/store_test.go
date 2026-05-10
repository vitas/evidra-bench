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
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return s
}

func mustInsert(t *testing.T, s *Store, r RunRecord) {
	t.Helper()
	if err := s.Insert(r); err != nil {
		t.Fatalf("insert: %v", err)
	}
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

	mustInsert(t, s, RunRecord{ID: "r1", ScenarioID: "s1", Model: "haiku", CreatedAt: now})
	mustInsert(t, s, RunRecord{ID: "r2", ScenarioID: "s1", Model: "sonnet", CreatedAt: now})
	mustInsert(t, s, RunRecord{ID: "r3", ScenarioID: "s2", Model: "haiku", CreatedAt: now})

	runs, _ := s.Query(QueryFilters{Model: "haiku"})
	if len(runs) != 2 {
		t.Fatalf("expected 2 haiku runs, got %d", len(runs))
	}
}

func TestQueryPassedOnly(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	now := time.Now().UTC()

	mustInsert(t, s, RunRecord{ID: "r1", ScenarioID: "s1", Passed: true, CreatedAt: now})
	mustInsert(t, s, RunRecord{ID: "r2", ScenarioID: "s1", Passed: false, CreatedAt: now})

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
		mustInsert(t, s, RunRecord{ID: fmt.Sprintf("r%d", i), ScenarioID: "s1", CreatedAt: now})
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

	mustInsert(t, s, RunRecord{ID: "r1", ScenarioID: "s1", Passed: true, CreatedAt: now})
	mustInsert(t, s, RunRecord{ID: "r2", ScenarioID: "s1", Passed: false, CreatedAt: now})
	mustInsert(t, s, RunRecord{ID: "r3", ScenarioID: "s2", Passed: true, CreatedAt: now})

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
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()

	mustInsert(t, s, RunRecord{ID: "r1", ScenarioID: "s1", Model: "haiku", CreatedAt: time.Now().UTC()})
	mustInsert(t, s, RunRecord{ID: "r2", ScenarioID: "s2", Model: "sonnet", CreatedAt: time.Now().UTC()})

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
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	mustInsert(t, s, RunRecord{ID: "r1", ScenarioID: "s1", Passed: true, CreatedAt: time.Now().UTC()})
	mustInsert(t, s, RunRecord{ID: "r2", ScenarioID: "s2", Passed: false, CreatedAt: time.Now().UTC()})
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	// Delete DB, keep JSONL
	if err := os.Remove(filepath.Join(dir, "bench.db")); err != nil {
		t.Fatalf("remove db: %v", err)
	}

	// Reopen and rebuild
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() {
		if err := s2.Close(); err != nil {
			t.Errorf("close rebuilt store: %v", err)
		}
	}()

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

func TestInsertAndQuery_PreservesEvidenceAndToolServerIdentity(t *testing.T) {
	t.Parallel()

	s := testStore(t)
	now := time.Now().UTC()

	rec := RunRecord{
		ID:                "run-tool-server",
		ScenarioID:        "s1",
		EvidenceMode:      "mcp",
		ToolServer:        "kubernetes-mcp",
		ToolServerVersion: "1.2.3",
		CreatedAt:         now,
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
	if runs[0].EvidenceMode != "mcp" {
		t.Fatalf("evidence_mode = %q, want mcp", runs[0].EvidenceMode)
	}
	if runs[0].ToolServer != "kubernetes-mcp" {
		t.Fatalf("tool_server = %q, want kubernetes-mcp", runs[0].ToolServer)
	}
	if runs[0].ToolServerVersion != "1.2.3" {
		t.Fatalf("tool_server_version = %q, want 1.2.3", runs[0].ToolServerVersion)
	}
}

func TestRebuild_PreservesToolServerIdentity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	mustInsert(t, s, RunRecord{
		ID:                "run-tool-server",
		ScenarioID:        "s1",
		EvidenceMode:      "mcp",
		ToolServer:        "kubernetes-mcp",
		ToolServerVersion: "1.2.3",
		CreatedAt:         time.Now().UTC(),
	})
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if err := os.Remove(filepath.Join(dir, "bench.db")); err != nil {
		t.Fatalf("remove db: %v", err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() {
		if err := s2.Close(); err != nil {
			t.Errorf("close rebuilt store: %v", err)
		}
	}()

	if _, err := s2.Rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	runs, err := s2.Query(QueryFilters{ScenarioID: "s1"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].ToolServer != "kubernetes-mcp" {
		t.Fatalf("tool_server = %q, want kubernetes-mcp", runs[0].ToolServer)
	}
	if runs[0].ToolServerVersion != "1.2.3" {
		t.Fatalf("tool_server_version = %q, want 1.2.3", runs[0].ToolServerVersion)
	}
}

func TestImportFromArtifacts_PreservesToolServerIdentity(t *testing.T) {
	t.Parallel()

	s := testStore(t)
	runsDir := t.TempDir()
	artifactDir := filepath.Join(runsDir, "run-1")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	body := `{
		"scenario_id": "s1",
		"adapter": "bench-cli",
		"start_time": "2026-05-10T10:00:00Z",
		"end_time": "2026-05-10T10:00:05Z",
		"exit_code": 0,
		"passed": true,
		"checks": {"checks":[{"verdict":"pass"}]},
		"metadata": {
			"evidence_mode": "mcp",
			"tool_server": "kubernetes-mcp",
			"tool_server_version": "1.2.3"
		}
	}`
	if err := os.WriteFile(filepath.Join(artifactDir, "run.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write run.json: %v", err)
	}

	count, err := s.ImportFromArtifacts(runsDir)
	if err != nil {
		t.Fatalf("import artifacts: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	runs, err := s.Query(QueryFilters{ScenarioID: "s1"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].ToolServer != "kubernetes-mcp" {
		t.Fatalf("tool_server = %q, want kubernetes-mcp", runs[0].ToolServer)
	}
	if runs[0].ToolServerVersion != "1.2.3" {
		t.Fatalf("tool_server_version = %q, want 1.2.3", runs[0].ToolServerVersion)
	}
}
