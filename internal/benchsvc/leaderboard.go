package benchsvc

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// Leaderboard returns aggregate stats per model, optionally filtered by evidence mode and scenario IDs.
// k controls the pass^k reliability metric: only scenarios with >= k trials contribute.
func (s *PgStore) Leaderboard(ctx context.Context, tenantID string, evidenceMode string, k int, scenarioIDs []string) ([]bench.LeaderboardEntry, error) {
	if k < 1 {
		k = 3
	}

	// Build the evidence mode WHERE clause using the shared alias logic.
	argN := 2
	modeClause := ""
	scenarioClause := ""
	args := []any{tenantID}

	if clause, clauseArgs := evidenceModeClause(argN, evidenceMode); clause != "" {
		modeClause = " AND " + clause
		args = append(args, clauseArgs...)
		argN += len(clauseArgs)
	}
	if len(scenarioIDs) > 0 {
		scenarioClause = fmt.Sprintf(" AND scenario_id = ANY($%d::text[])", argN)
		args = append(args, scenarioIDs)
		argN++
	}

	kArgN := argN
	args = append(args, k)

	query := fmt.Sprintf(`
		-- Existing run-weighted aggregation (unchanged semantics).
		-- Exclude infra errors (exit_code = -1) from all calculations.
		WITH run_agg AS (
			SELECT model,
				COUNT(DISTINCT scenario_id) AS scenarios,
				COUNT(*) AS runs,
				100.0 * SUM(CASE WHEN passed THEN 1 ELSE 0 END) / NULLIF(COUNT(*), 0) AS pass_rate,
				AVG(duration_seconds) AS avg_duration,
				AVG(estimated_cost_usd) AS avg_cost,
				SUM(estimated_cost_usd) AS total_cost
			FROM bench_runs
			WHERE tenant_id = $1 AND archived_at IS NULL AND exit_code >= 0%s%s
			GROUP BY model
		),
		-- Side CTE: per-scenario pass rates for pass^k.
		per_scenario AS (
			SELECT model, scenario_id,
				COUNT(*) AS trials,
				AVG(CASE WHEN passed THEN 1.0 ELSE 0.0 END) AS pass_rate
			FROM bench_runs
			WHERE tenant_id = $1 AND archived_at IS NULL AND exit_code >= 0%s%s
			GROUP BY model, scenario_id
		),
		pass_k_agg AS (
			SELECT model,
				COALESCE(100.0 * AVG(POWER(pass_rate, $%d)), 0) AS pass_k,
				COUNT(*)::int AS sufficient_scenarios
			FROM per_scenario
			WHERE trials >= $%d
			GROUP BY model
		)
		SELECT r.model, r.scenarios, r.runs, r.pass_rate,
			r.avg_duration, r.avg_cost, r.total_cost,
			COALESCE(p.pass_k, 0), $%d, COALESCE(p.sufficient_scenarios, 0)
		FROM run_agg r
		LEFT JOIN pass_k_agg p ON p.model = r.model
		ORDER BY r.pass_rate DESC, r.model
	`, modeClause, scenarioClause, modeClause, scenarioClause, kArgN, kArgN, kArgN)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("bench.Leaderboard: %w", err)
	}
	defer rows.Close()

	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (bench.LeaderboardEntry, error) {
		var e bench.LeaderboardEntry
		err := row.Scan(&e.Model, &e.Scenarios, &e.Runs, &e.PassRate,
			&e.AvgDuration, &e.AvgCost, &e.TotalCost,
			&e.PassK, &e.PassKTrials, &e.SufficientScenarios)
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("bench.Leaderboard: collect: %w", err)
	}
	return entries, nil
}
