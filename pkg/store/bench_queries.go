package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Verify Store implements BenchStore.
var _ BenchStore = (*Store)(nil)

// ListRuns returns runs matching filters with pagination (total count + page).
func (s *Store) ListRuns(ctx context.Context, f RunFilters) ([]RunRecord, int, error) {
	where, args := buildWhere(f)

	// Count total
	var total int
	countQ := "SELECT COUNT(*) FROM runs" + where
	if err := s.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("store.ListRuns: count: %w", err)
	}

	// Fetch page
	query := "SELECT id, scenario_id, model, provider, adapter, passed, duration_seconds, exit_code, turns, memory_window, prompt_tokens, completion_tokens, estimated_cost, checks_passed, checks_total, checks_json, metadata_json, artifact_dir, created_at FROM runs" + where + " ORDER BY created_at DESC"
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	if f.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", f.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("store.ListRuns: %w", err)
	}
	defer rows.Close()

	var records []RunRecord
	for rows.Next() {
		var r RunRecord
		if err := rows.Scan(&r.ID, &r.ScenarioID, &r.Model, &r.Provider, &r.Adapter, &r.Passed, &r.Duration, &r.ExitCode, &r.Turns, &r.MemoryWindow, &r.PromptTokens, &r.CompletionTokens, &r.EstimatedCost, &r.ChecksPassed, &r.ChecksTotal, &r.ChecksJSON, &r.MetadataJSON, &r.ArtifactDir, &r.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("store.ListRuns: scan: %w", err)
		}
		records = append(records, r)
	}
	return records, total, rows.Err()
}

// GetRun returns a single run by ID.
func (s *Store) GetRun(ctx context.Context, id string) (*RunRecord, error) {
	var r RunRecord
	err := s.db.QueryRowContext(ctx, "SELECT id, scenario_id, model, provider, adapter, passed, duration_seconds, exit_code, turns, memory_window, prompt_tokens, completion_tokens, estimated_cost, checks_passed, checks_total, checks_json, metadata_json, artifact_dir, created_at FROM runs WHERE id = ?", id).
		Scan(&r.ID, &r.ScenarioID, &r.Model, &r.Provider, &r.Adapter, &r.Passed, &r.Duration, &r.ExitCode, &r.Turns, &r.MemoryWindow, &r.PromptTokens, &r.CompletionTokens, &r.EstimatedCost, &r.ChecksPassed, &r.ChecksTotal, &r.ChecksJSON, &r.MetadataJSON, &r.ArtifactDir, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("store.GetRun: %w", err)
	}
	return &r, nil
}

// CompareRuns loads two runs and computes check diffs.
func (s *Store) CompareRuns(ctx context.Context, idA, idB string) (*RunComparison, error) {
	a, err := s.GetRun(ctx, idA)
	if err != nil {
		return nil, fmt.Errorf("store.CompareRuns: run A: %w", err)
	}
	b, err := s.GetRun(ctx, idB)
	if err != nil {
		return nil, fmt.Errorf("store.CompareRuns: run B: %w", err)
	}

	diffs := computeCheckDiffs(a.ChecksJSON, b.ChecksJSON)
	return &RunComparison{RunA: *a, RunB: *b, CheckDiffs: diffs}, nil
}

// ModelMatrix builds a comparison grid of models across scenarios.
func (s *Store) ModelMatrix(ctx context.Context, models, scenarios []string) (*ModelMatrix, error) {
	where := " WHERE 1=1"
	var args []any

	if len(models) > 0 {
		placeholders := make([]string, len(models))
		for i, m := range models {
			placeholders[i] = "?"
			args = append(args, m)
		}
		where += " AND model IN (" + strings.Join(placeholders, ",") + ")"
	}
	if len(scenarios) > 0 {
		placeholders := make([]string, len(scenarios))
		for i, sc := range scenarios {
			placeholders[i] = "?"
			args = append(args, sc)
		}
		where += " AND scenario_id IN (" + strings.Join(placeholders, ",") + ")"
	}

	query := `SELECT scenario_id, model,
		COUNT(*) as runs,
		SUM(CASE WHEN passed THEN 1 ELSE 0 END) as passed,
		AVG(estimated_cost) as avg_cost,
		AVG(prompt_tokens + completion_tokens) as avg_tokens,
		AVG(duration_seconds) as avg_duration
		FROM runs` + where + `
		GROUP BY scenario_id, model
		ORDER BY scenario_id, model`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store.ModelMatrix: %w", err)
	}
	defer rows.Close()

	modelSet := map[string]bool{}
	scenarioSet := map[string]bool{}
	cells := map[string]map[string]ModelMatrixCell{}

	for rows.Next() {
		var scenarioID, model string
		var cell ModelMatrixCell
		var avgTokens float64
		if err := rows.Scan(&scenarioID, &model, &cell.Runs, &cell.Passed, &cell.AvgCost, &avgTokens, &cell.AvgDuration); err != nil {
			return nil, fmt.Errorf("store.ModelMatrix: scan: %w", err)
		}
		cell.AvgTokens = int(avgTokens)
		if cell.Runs > 0 {
			cell.PassRate = float64(cell.Passed) / float64(cell.Runs) * 100
		}

		modelSet[model] = true
		scenarioSet[scenarioID] = true
		if cells[scenarioID] == nil {
			cells[scenarioID] = map[string]ModelMatrixCell{}
		}
		cells[scenarioID][model] = cell
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortedModels := sortKeys(modelSet)
	sortedScenarios := sortKeys(scenarioSet)

	return &ModelMatrix{
		Models:    sortedModels,
		Scenarios: sortedScenarios,
		Cells:     cells,
	}, nil
}

// FilteredStats returns aggregate statistics matching the given filters.
func (s *Store) FilteredStats(ctx context.Context, f RunFilters) (*StatsResult, error) {
	where, args := buildWhere(f)

	var st StatsResult
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*), COALESCE(SUM(CASE WHEN passed THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN NOT passed THEN 1 ELSE 0 END),0) FROM runs"+where,
		args...,
	).Scan(&st.TotalRuns, &st.PassCount, &st.FailCount)
	if err != nil {
		return nil, fmt.Errorf("store.FilteredStats: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT scenario_id, COUNT(*), SUM(CASE WHEN passed THEN 1 ELSE 0 END) FROM runs"+where+" GROUP BY scenario_id ORDER BY scenario_id",
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store.FilteredStats: by scenario: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ss ScenarioStat
		if err := rows.Scan(&ss.ScenarioID, &ss.Runs, &ss.Passed); err != nil {
			return nil, err
		}
		st.ByScenario = append(st.ByScenario, ss)
	}
	return &st, rows.Err()
}

// ListScenarios returns distinct scenarios from the runs table.
// Note: full metadata (title, tags, etc.) comes from scenario YAML files,
// which the API handler enriches. This returns only what the DB knows.
func (s *Store) ListScenarios(ctx context.Context) ([]ScenarioSummary, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT scenario_id FROM runs ORDER BY scenario_id")
	if err != nil {
		return nil, fmt.Errorf("store.ListScenarios: %w", err)
	}
	defer rows.Close()
	var summaries []ScenarioSummary
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		summaries = append(summaries, ScenarioSummary{ID: id})
	}
	return summaries, rows.Err()
}

// buildWhere constructs a WHERE clause and args from RunFilters.
func buildWhere(f RunFilters) (string, []any) {
	var clauses []string
	var args []any

	if f.ScenarioID != "" {
		clauses = append(clauses, "scenario_id = ?")
		args = append(args, f.ScenarioID)
	}
	if f.Model != "" {
		clauses = append(clauses, "model = ?")
		args = append(args, f.Model)
	}
	if f.Provider != "" {
		clauses = append(clauses, "provider = ?")
		args = append(args, f.Provider)
	}
	if f.PassedOnly {
		clauses = append(clauses, "passed = 1")
	}
	if f.FailedOnly {
		clauses = append(clauses, "passed = 0")
	}
	if f.Since != "" {
		t, err := time.Parse(time.RFC3339, f.Since)
		if err != nil {
			t, err = time.Parse("2006-01-02", f.Since)
		}
		if err == nil {
			clauses = append(clauses, "created_at >= ?")
			args = append(args, t)
		}
	}

	if len(clauses) == 0 {
		return " WHERE 1=1", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func sortKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// checksResult is used for parsing checks JSON.
type checksResult struct {
	Passed bool         `json:"passed"`
	Checks []checkEntry `json:"checks"`
}

type checkEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Verdict string `json:"verdict"`
	Message string `json:"message,omitempty"`
}

func computeCheckDiffs(checksA, checksB string) []CheckDiff {
	var a, b checksResult
	json.Unmarshal([]byte(checksA), &a)
	json.Unmarshal([]byte(checksB), &b)

	aMap := map[string]checkEntry{}
	for _, c := range a.Checks {
		key := c.Type + "/" + c.Name
		aMap[key] = c
	}
	bMap := map[string]checkEntry{}
	for _, c := range b.Checks {
		key := c.Type + "/" + c.Name
		bMap[key] = c
	}

	seen := map[string]bool{}
	var diffs []CheckDiff

	for key, ca := range aMap {
		seen[key] = true
		cb, ok := bMap[key]
		change := "same"
		bVerdict := ""
		if ok {
			bVerdict = cb.Verdict
			if ca.Verdict != cb.Verdict {
				if cb.Verdict == "pass" {
					change = "improved"
				} else {
					change = "regressed"
				}
			}
		} else {
			change = "removed"
		}
		diffs = append(diffs, CheckDiff{
			Name:    ca.Name,
			TypeStr: ca.Type,
			RunA:    ca.Verdict,
			RunB:    bVerdict,
			Change:  change,
		})
	}

	for key, cb := range bMap {
		if seen[key] {
			continue
		}
		diffs = append(diffs, CheckDiff{
			Name:    cb.Name,
			TypeStr: cb.Type,
			RunA:    "",
			RunB:    cb.Verdict,
			Change:  "new",
		})
	}

	return diffs
}
