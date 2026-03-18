package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	orderCol := "created_at"
	validSortColumns := map[string]bool{
		"created_at": true, "duration_seconds": true, "estimated_cost": true,
		"scenario_id": true, "model": true, "provider": true,
		"checks_passed": true, "turns": true, "passed": true,
	}
	if f.SortBy != "" && validSortColumns[f.SortBy] {
		orderCol = f.SortBy
	}
	orderDir := "DESC"
	if f.SortOrder == "asc" {
		orderDir = "ASC"
	}
	query := "SELECT " + runRecordColumns + " FROM runs" + where + fmt.Sprintf(" ORDER BY %s %s", orderCol, orderDir)
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
		r, err := scanRunRecord(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("store.ListRuns: scan: %w", err)
		}
		records = append(records, r)
	}
	return records, total, rows.Err()
}

// GetRun returns a single run by ID.
func (s *Store) GetRun(ctx context.Context, id string) (*RunRecord, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+runRecordColumns+" FROM runs WHERE id = ?", id)
	r, err := scanRunRecordRow(row)
	if err != nil {
		return nil, fmt.Errorf("store.GetRun: %w", err)
	}
	return &r, nil
}

// Catalog returns distinct models and providers from stored runs.
func (s *Store) Catalog(ctx context.Context) (*RunCatalog, error) {
	models, err := distinctStringColumn(ctx, s.db, "model")
	if err != nil {
		return nil, fmt.Errorf("store.Catalog models: %w", err)
	}
	providers, err := distinctStringColumn(ctx, s.db, "provider")
	if err != nil {
		return nil, fmt.Errorf("store.Catalog providers: %w", err)
	}
	return &RunCatalog{
		Models:    models,
		Providers: providers,
	}, nil
}

// scanner is satisfied by both *sql.Rows and *sql.Row.
type scanner interface {
	Scan(dest ...any) error
}

// runRecordColumns is the SELECT column list for RunRecord scans.
const runRecordColumns = "id, scenario_id, model, provider, adapter, evidence_mode, passed, duration_seconds, exit_code, turns, memory_window, prompt_tokens, completion_tokens, estimated_cost, checks_passed, checks_total, checks_json, metadata_json, artifact_dir, created_at"

func scanRunRecord(s scanner) (RunRecord, error) {
	var r RunRecord
	var createdAt string
	err := s.Scan(&r.ID, &r.ScenarioID, &r.Model, &r.Provider, &r.Adapter, &r.EvidenceMode, &r.Passed,
		&r.Duration, &r.ExitCode, &r.Turns, &r.MemoryWindow,
		&r.PromptTokens, &r.CompletionTokens, &r.EstimatedCost,
		&r.ChecksPassed, &r.ChecksTotal, &r.ChecksJSON, &r.MetadataJSON,
		&r.ArtifactDir, &createdAt)
	if err != nil {
		return r, err
	}
	r.CreatedAt = parseTime(createdAt)
	return r, nil
}

func scanRunRecordRow(row *sql.Row) (RunRecord, error) {
	var r RunRecord
	var createdAt string
	err := row.Scan(&r.ID, &r.ScenarioID, &r.Model, &r.Provider, &r.Adapter, &r.EvidenceMode, &r.Passed,
		&r.Duration, &r.ExitCode, &r.Turns, &r.MemoryWindow,
		&r.PromptTokens, &r.CompletionTokens, &r.EstimatedCost,
		&r.ChecksPassed, &r.ChecksTotal, &r.ChecksJSON, &r.MetadataJSON,
		&r.ArtifactDir, &createdAt)
	if err != nil {
		return r, err
	}
	r.CreatedAt = parseTime(createdAt)
	return r, nil
}

func distinctStringColumn(ctx context.Context, db *sql.DB, column string) ([]string, error) {
	query := fmt.Sprintf("SELECT DISTINCT %s FROM runs WHERE %s != '' ORDER BY %s", column, column, column)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

// parseTime tries common SQLite time formats.
func parseTime(s string) time.Time {
	// Try standard formats first with original string.
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05.999999 -0700",
		"2006-01-02 15:04:05 -0700",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	// Strip trailing timezone abbreviation (e.g. " CET", " UTC")
	// Go's time.Parse doesn't handle these. Only strip if the last
	// space-separated token is all uppercase letters.
	if idx := strings.LastIndex(s, " "); idx > 0 {
		suffix := s[idx+1:]
		allUpper := true
		for _, c := range suffix {
			if c < 'A' || c > 'Z' {
				allUpper = false
				break
			}
		}
		if allUpper && len(suffix) >= 2 {
			return parseTime(s[:idx])
		}
	}
	return time.Time{}
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
	if f.EvidenceMode != "" {
		clauses = append(clauses, "evidence_mode = ?")
		args = append(args, f.EvidenceMode)
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

// SignalSummary aggregates signal counts from scorecard.json files across matching runs.
func (s *Store) SignalSummary(ctx context.Context, f RunFilters) (*SignalAggregation, error) {
	runs, _, err := s.ListRuns(ctx, RunFilters{
		ScenarioID: f.ScenarioID,
		Model:      f.Model,
		Provider:   f.Provider,
		PassedOnly: f.PassedOnly,
		FailedOnly: f.FailedOnly,
		Since:      f.Since,
		Limit:      1000,
	})
	if err != nil {
		return nil, fmt.Errorf("store.SignalSummary: %w", err)
	}

	agg := &SignalAggregation{
		TotalRuns: len(runs),
		Signals:   make(map[string]SignalCount),
	}

	var scoreSum float64
	for _, run := range runs {
		if run.ArtifactDir == "" {
			continue
		}
		sc, err := readScorecard(run.ArtifactDir)
		if err != nil {
			continue // scorecard not available for this run
		}
		agg.RunsWithScorecard++
		if sc.Score > 0 {
			scoreSum += sc.Score
		}
		for signal, count := range sc.Signals {
			entry := agg.Signals[signal]
			entry.Total += count
			if count > 0 {
				entry.RunCount++
			}
			agg.Signals[signal] = entry
		}
	}
	if agg.RunsWithScorecard > 0 {
		agg.AvgScore = scoreSum / float64(agg.RunsWithScorecard)
	}

	return agg, nil
}

type scorecard struct {
	Signals map[string]int `json:"signals"`
	Score   float64        `json:"score"`
	Band    string         `json:"band"`
}

func readScorecard(artifactDir string) (*scorecard, error) {
	data, err := os.ReadFile(filepath.Join(artifactDir, "scorecard.json"))
	if err != nil {
		return nil, err
	}
	var sc scorecard
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

// FailureAnalysis computes failure patterns for a specific scenario.
func (s *Store) FailureAnalysis(ctx context.Context, scenarioID string) (*FailureInsights, error) {
	runs, _, err := s.ListRuns(ctx, RunFilters{ScenarioID: scenarioID, Limit: 500})
	if err != nil {
		return nil, fmt.Errorf("store.FailureAnalysis: %w", err)
	}

	insights := &FailureInsights{ScenarioID: scenarioID, TotalRuns: len(runs)}

	var passRuns, failRuns []RunRecord
	for _, r := range runs {
		if r.Passed {
			passRuns = append(passRuns, r)
		} else {
			failRuns = append(failRuns, r)
		}
	}
	insights.PassedRuns = len(passRuns)
	insights.FailedRuns = len(failRuns)

	// Check failure stats
	checkFails := map[string]*CheckFailureStat{}
	for _, r := range failRuns {
		var cr checksResult
		if json.Unmarshal([]byte(r.ChecksJSON), &cr) != nil {
			continue
		}
		for _, c := range cr.Checks {
			if c.Verdict != "fail" {
				continue
			}
			key := c.Type + "/" + c.Name
			stat, ok := checkFails[key]
			if !ok {
				stat = &CheckFailureStat{CheckName: c.Name, CheckType: c.Type, Message: c.Message}
				checkFails[key] = stat
			}
			stat.FailCount++
		}
	}
	for _, stat := range checkFails {
		if insights.FailedRuns > 0 {
			stat.FailRate = float64(stat.FailCount) / float64(insights.FailedRuns) * 100
		}
		insights.CheckFailures = append(insights.CheckFailures, *stat)
	}
	sort.Slice(insights.CheckFailures, func(i, j int) bool {
		return insights.CheckFailures[i].FailCount > insights.CheckFailures[j].FailCount
	})

	// Command patterns — compare commands used in pass vs fail runs
	passCommands := extractCommands(passRuns)
	failCommands := extractCommands(failRuns)
	allCommands := map[string]bool{}
	for cmd := range passCommands {
		allCommands[cmd] = true
	}
	for cmd := range failCommands {
		allCommands[cmd] = true
	}
	for cmd := range allCommands {
		pc := passCommands[cmd]
		fc := failCommands[cmd]
		indicator := "neutral"
		if pc > 0 && fc == 0 {
			indicator = "pass_signal"
		} else if fc > 0 && pc == 0 {
			indicator = "fail_signal"
		}
		if indicator != "neutral" {
			insights.CommandPatterns = append(insights.CommandPatterns, CommandPattern{
				Command:    cmd,
				InPassRuns: pc,
				InFailRuns: fc,
				Indicator:  indicator,
			})
		}
	}
	sort.Slice(insights.CommandPatterns, func(i, j int) bool {
		if insights.CommandPatterns[i].Indicator != insights.CommandPatterns[j].Indicator {
			return insights.CommandPatterns[i].Indicator < insights.CommandPatterns[j].Indicator
		}
		return insights.CommandPatterns[i].InPassRuns+insights.CommandPatterns[i].InFailRuns >
			insights.CommandPatterns[j].InPassRuns+insights.CommandPatterns[j].InFailRuns
	})
	if len(insights.CommandPatterns) > 15 {
		insights.CommandPatterns = insights.CommandPatterns[:15]
	}

	// Model breakdown
	modelMap := map[string]*ModelFailureStat{}
	for _, r := range runs {
		stat, ok := modelMap[r.Model]
		if !ok {
			stat = &ModelFailureStat{Model: r.Model}
			modelMap[r.Model] = stat
		}
		stat.Runs++
		if r.Passed {
			stat.Passed++
		} else {
			stat.Failed++
		}
	}
	for _, stat := range modelMap {
		if stat.Runs > 0 {
			stat.Rate = float64(stat.Passed) / float64(stat.Runs) * 100
		}
		insights.ModelBreakdown = append(insights.ModelBreakdown, *stat)
	}
	sort.Slice(insights.ModelBreakdown, func(i, j int) bool {
		return insights.ModelBreakdown[i].Rate > insights.ModelBreakdown[j].Rate
	})

	// Behavior comparison
	insights.BehaviorMetrics = BehaviorComparison{
		PassAvgTurns:    avgField(passRuns, func(r RunRecord) float64 { return float64(r.Turns) }),
		FailAvgTurns:    avgField(failRuns, func(r RunRecord) float64 { return float64(r.Turns) }),
		PassAvgDuration: avgField(passRuns, func(r RunRecord) float64 { return r.Duration }),
		FailAvgDuration: avgField(failRuns, func(r RunRecord) float64 { return r.Duration }),
		PassAvgTokens:   avgField(passRuns, func(r RunRecord) float64 { return float64(r.PromptTokens + r.CompletionTokens) }),
		FailAvgTokens:   avgField(failRuns, func(r RunRecord) float64 { return float64(r.PromptTokens + r.CompletionTokens) }),
		PassAvgCost:     avgField(passRuns, func(r RunRecord) float64 { return r.EstimatedCost }),
		FailAvgCost:     avgField(failRuns, func(r RunRecord) float64 { return r.EstimatedCost }),
	}

	return insights, nil
}

func extractCommands(runs []RunRecord) map[string]int {
	counts := map[string]int{}
	for _, r := range runs {
		if r.ArtifactDir == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.ArtifactDir, "tool-calls.json"))
		if err != nil {
			continue
		}
		var calls []struct {
			Tool string         `json:"tool"`
			Args map[string]any `json:"args"`
		}
		if json.Unmarshal(data, &calls) != nil {
			continue
		}
		for _, c := range calls {
			if c.Tool != "run_command" {
				continue
			}
			cmd, _ := c.Args["command"].(string)
			if cmd == "" {
				continue
			}
			// Normalize: extract the base command (first 3 words)
			parts := strings.Fields(cmd)
			key := strings.Join(parts[:min(len(parts), 3)], " ")
			counts[key]++
		}
	}
	return counts
}

func avgField(runs []RunRecord, f func(RunRecord) float64) float64 {
	if len(runs) == 0 {
		return 0
	}
	var sum float64
	for _, r := range runs {
		sum += f(r)
	}
	return sum / float64(len(runs))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Regressions finds scenario/model pairs where the latest run failed but
// previous runs had passes — indicating a regression.
func (s *Store) Regressions(ctx context.Context) ([]Regression, error) {
	query := `
		SELECT r.scenario_id, r.model, r.id,
			(SELECT COUNT(*) FROM runs r2
			 WHERE r2.scenario_id = r.scenario_id AND r2.model = r.model
			   AND r2.passed = 1 AND r2.id != r.id) as prev_passed,
			(SELECT COUNT(*) FROM runs r2
			 WHERE r2.scenario_id = r.scenario_id AND r2.model = r.model
			   AND r2.id != r.id) as prev_total
		FROM runs r
		WHERE r.passed = 0
		  AND r.created_at = (
		    SELECT MAX(r3.created_at) FROM runs r3
		    WHERE r3.scenario_id = r.scenario_id AND r3.model = r.model
		  )
		  AND (SELECT COUNT(*) FROM runs r2
		       WHERE r2.scenario_id = r.scenario_id AND r2.model = r.model
		         AND r2.passed = 1 AND r2.id != r.id) > 0
		ORDER BY prev_passed DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("store.Regressions: %w", err)
	}
	defer rows.Close()

	var results []Regression
	for rows.Next() {
		var reg Regression
		if err := rows.Scan(&reg.ScenarioID, &reg.Model, &reg.LatestRunID, &reg.PrevPassed, &reg.PrevTotal); err != nil {
			return nil, fmt.Errorf("store.Regressions: scan: %w", err)
		}
		reg.LatestPassed = false
		if reg.PrevTotal > 0 {
			reg.PrevRate = float64(reg.PrevPassed) / float64(reg.PrevTotal) * 100
		}
		if reg.PrevRate >= 80 {
			reg.Severity = "critical"
		} else {
			reg.Severity = "warning"
		}
		results = append(results, reg)
	}
	return results, rows.Err()
}

func computeCheckDiffs(checksA, checksB string) []CheckDiff {
	var a, b checksResult
	// Unmarshal errors are intentional no-ops: missing or malformed checks_json
	// produces an empty diff, which is the correct behavior for runs without checks.
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
