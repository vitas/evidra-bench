package benchsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
)

// SignalSummary aggregates signal counts from scorecard artifacts across matching runs.
func (s *PgStore) SignalSummary(ctx context.Context, tenantID string, f bench.RunFilters) (*bench.SignalAggregation, error) {
	where, args := buildWhere(tenantID, f)

	// Left join so runs without scorecard artifacts still appear in totals.
	// bench_runs is not aliased so that buildWhere's bench_runs.created_at qualifier resolves
	// without ambiguity against bench_artifacts.created_at.
	query := `SELECT bench_runs.id, sc.data, fa.data
			FROM bench_runs
			LEFT JOIN bench_artifacts sc ON sc.run_id = bench_runs.id AND sc.artifact_type = '` + artifact.HostedScorecard + `'
			LEFT JOIN bench_artifacts fa ON fa.run_id = bench_runs.id AND fa.artifact_type = '` + artifact.HostedFailureAutopsy + `'` +
		where +
		` ORDER BY bench_runs.created_at DESC LIMIT 1000`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("bench.SignalSummary: %w", err)
	}
	defer rows.Close()

	agg := &bench.SignalAggregation{
		Signals: make(map[string]bench.SignalCount),
	}

	// Also count total runs (with or without scorecard).
	countQ := "SELECT COUNT(*) FROM bench_runs" + where
	if err := s.db.QueryRow(ctx, countQ, args...).Scan(&agg.TotalRuns); err != nil {
		return nil, fmt.Errorf("bench.SignalSummary: count: %w", err)
	}

	var scoreSum float64
	for rows.Next() {
		var runID string
		var scorecardData *[]byte // nullable for LEFT JOIN
		var autopsyData *[]byte   // nullable for LEFT JOIN
		if err := rows.Scan(&runID, &scorecardData, &autopsyData); err != nil {
			return nil, fmt.Errorf("bench.SignalSummary: scan: %w", err)
		}

		metrics := signalMetricsFromArtifacts(bytesOrNil(scorecardData), bytesOrNil(autopsyData))
		if metrics.hasScorecard {
			agg.RunsWithScorecard++
			if metrics.score > 0 {
				scoreSum += metrics.score
			}
		}
		for signal, count := range metrics.counts {
			entry := agg.Signals[signal]
			entry.Total += count
			if count > 0 {
				entry.RunCount++
			}
			agg.Signals[signal] = entry
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bench.SignalSummary: rows: %w", err)
	}

	if agg.RunsWithScorecard > 0 {
		agg.AvgScore = scoreSum / float64(agg.RunsWithScorecard)
	}

	return agg, nil
}

// scorecard matches the JSON structure stored in bench_artifacts.
type scorecard struct {
	Signals map[string]int `json:"signals"`
	Score   float64        `json:"score"`
	Band    string         `json:"band"`
}

type signalArtifactMetrics struct {
	counts       map[string]int
	hasScorecard bool
	score        float64
}

func signalMetricsFromArtifacts(scorecardData, autopsyData []byte) signalArtifactMetrics {
	if len(scorecardData) > 0 {
		var sc scorecard
		if json.Unmarshal(scorecardData, &sc) == nil {
			return signalArtifactMetrics{
				counts:       copySignalCounts(sc.Signals),
				hasScorecard: true,
				score:        sc.Score,
			}
		}
	}

	return signalArtifactMetrics{
		counts: autopsySignalCounts(autopsyData),
	}
}

func copySignalCounts(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for signal, count := range in {
		if strings.TrimSpace(signal) == "" || count <= 0 {
			continue
		}
		out[signal] = count
	}
	return out
}

func autopsySignalCounts(data []byte) map[string]int {
	counts := map[string]int{}
	if len(data) == 0 {
		return counts
	}
	var raw struct {
		Findings []struct {
			Kind string `json:"kind"`
		} `json:"findings"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return counts
	}
	for _, finding := range raw.Findings {
		kind := strings.TrimSpace(finding.Kind)
		if kind == "" {
			continue
		}
		counts[kind]++
	}
	return counts
}

func bytesOrNil(data *[]byte) []byte {
	if data == nil {
		return nil
	}
	return *data
}

// Regressions finds scenario/model pairs where the latest run failed but
// previous runs had passes -- indicating a regression.
func (s *PgStore) Regressions(ctx context.Context, tenantID string) ([]bench.Regression, error) {
	query := `
		WITH latest AS (
			SELECT DISTINCT ON (scenario_id, model)
				id, scenario_id, model, passed, created_at
			FROM bench_runs
			WHERE tenant_id = $1 AND archived_at IS NULL
			ORDER BY scenario_id, model, created_at DESC
		)
		SELECT l.scenario_id, l.model, l.id,
			(SELECT COUNT(*) FROM bench_runs r2
			 WHERE r2.tenant_id = $1 AND r2.archived_at IS NULL
			   AND r2.scenario_id = l.scenario_id AND r2.model = l.model
			   AND r2.passed = TRUE AND r2.id != l.id) AS prev_passed,
			(SELECT COUNT(*) FROM bench_runs r2
			 WHERE r2.tenant_id = $1 AND r2.archived_at IS NULL
			   AND r2.scenario_id = l.scenario_id AND r2.model = l.model
			   AND r2.id != l.id) AS prev_total
		FROM latest l
		WHERE l.passed = FALSE
		  AND (SELECT COUNT(*) FROM bench_runs r2
		       WHERE r2.tenant_id = $1 AND r2.archived_at IS NULL
		         AND r2.scenario_id = l.scenario_id AND r2.model = l.model
		         AND r2.passed = TRUE AND r2.id != l.id) > 0
		ORDER BY prev_passed DESC`

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Regressions: %w", err)
	}
	defer rows.Close()

	var results []bench.Regression
	for rows.Next() {
		var reg bench.Regression
		if err := rows.Scan(&reg.ScenarioID, &reg.Model, &reg.LatestRunID, &reg.PrevPassed, &reg.PrevTotal); err != nil {
			return nil, fmt.Errorf("bench.Regressions: scan: %w", err)
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

// FailureAnalysis computes failure patterns for a specific scenario.
func (s *PgStore) FailureAnalysis(ctx context.Context, tenantID string, scenarioID string) (*bench.FailureInsights, error) {
	// Fetch all runs for this scenario.
	runs, _, err := s.ListRuns(ctx, tenantID, bench.RunFilters{ScenarioID: scenarioID, Limit: 500})
	if err != nil {
		return nil, fmt.Errorf("bench.FailureAnalysis: %w", err)
	}

	insights := &bench.FailureInsights{ScenarioID: scenarioID, TotalRuns: len(runs)}

	var passRuns, failRuns []bench.RunRecord
	for _, r := range runs {
		if r.Passed {
			passRuns = append(passRuns, r)
		} else {
			failRuns = append(failRuns, r)
		}
	}
	insights.PassedRuns = len(passRuns)
	insights.FailedRuns = len(failRuns)

	// Check failure stats from checks_json.
	checkFails := map[string]*bench.CheckFailureStat{}
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
				stat = &bench.CheckFailureStat{CheckName: c.Name, CheckType: c.Type, Message: c.Message}
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

	// Model breakdown.
	modelMap := map[string]*bench.ModelFailureStat{}
	for _, r := range runs {
		stat, ok := modelMap[r.Model]
		if !ok {
			stat = &bench.ModelFailureStat{Model: r.Model}
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

	// Behavior comparison: pass vs fail averages.
	insights.BehaviorMetrics = bench.BehaviorComparison{
		PassAvgTurns:    avgField(passRuns, func(r bench.RunRecord) float64 { return float64(r.Turns) }),
		FailAvgTurns:    avgField(failRuns, func(r bench.RunRecord) float64 { return float64(r.Turns) }),
		PassAvgDuration: avgField(passRuns, func(r bench.RunRecord) float64 { return r.Duration }),
		FailAvgDuration: avgField(failRuns, func(r bench.RunRecord) float64 { return r.Duration }),
		PassAvgTokens:   avgField(passRuns, func(r bench.RunRecord) float64 { return float64(r.PromptTokens + r.CompletionTokens) }),
		FailAvgTokens:   avgField(failRuns, func(r bench.RunRecord) float64 { return float64(r.PromptTokens + r.CompletionTokens) }),
		PassAvgCost:     avgField(passRuns, func(r bench.RunRecord) float64 { return r.EstimatedCost }),
		FailAvgCost:     avgField(failRuns, func(r bench.RunRecord) float64 { return r.EstimatedCost }),
	}

	return insights, nil
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

func avgField(runs []bench.RunRecord, f func(bench.RunRecord) float64) float64 {
	if len(runs) == 0 {
		return 0
	}
	var sum float64
	for _, r := range runs {
		sum += f(r)
	}
	return sum / float64(len(runs))
}

// ModelMatrix builds a comparison grid of models across scenarios.
func (s *PgStore) ModelMatrix(ctx context.Context, tenantID string, models, scenarios []string) (*bench.ModelMatrix, error) {
	clauses := []string{"tenant_id = $1", "archived_at IS NULL"}
	args := []any{tenantID}

	if len(models) > 0 {
		placeholders := make([]string, len(models))
		for i, m := range models {
			args = append(args, m)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		clauses = append(clauses, "model IN ("+strings.Join(placeholders, ",")+")")
	}
	if len(scenarios) > 0 {
		placeholders := make([]string, len(scenarios))
		for i, sc := range scenarios {
			args = append(args, sc)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		clauses = append(clauses, "scenario_id IN ("+strings.Join(placeholders, ",")+")")
	}

	query := `SELECT scenario_id, model,
		COUNT(*) AS runs,
		SUM(CASE WHEN passed THEN 1 ELSE 0 END) AS passed,
		AVG(estimated_cost_usd) AS avg_cost,
		AVG(prompt_tokens + completion_tokens) AS avg_tokens,
		AVG(duration_seconds) AS avg_duration
		FROM bench_runs
		WHERE ` + strings.Join(clauses, " AND ") + `
		GROUP BY scenario_id, model
		ORDER BY scenario_id, model`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("bench.ModelMatrix: %w", err)
	}
	defer rows.Close()

	modelSet := map[string]bool{}
	scenarioSet := map[string]bool{}
	cells := map[string]map[string]bench.ModelMatrixCell{}

	for rows.Next() {
		var scenarioID, model string
		var cell bench.ModelMatrixCell
		var avgTokens float64
		if err := rows.Scan(&scenarioID, &model, &cell.Runs, &cell.Passed, &cell.AvgCost, &avgTokens, &cell.AvgDuration); err != nil {
			return nil, fmt.Errorf("bench.ModelMatrix: scan: %w", err)
		}
		cell.AvgTokens = int(avgTokens)
		if cell.Runs > 0 {
			cell.PassRate = float64(cell.Passed) / float64(cell.Runs) * 100
		}

		modelSet[model] = true
		scenarioSet[scenarioID] = true
		if cells[scenarioID] == nil {
			cells[scenarioID] = map[string]bench.ModelMatrixCell{}
		}
		cells[scenarioID][model] = cell
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bench.ModelMatrix: rows: %w", err)
	}

	return &bench.ModelMatrix{
		Models:    sortKeys(modelSet),
		Scenarios: sortKeys(scenarioSet),
		Cells:     cells,
	}, nil
}

func sortKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
