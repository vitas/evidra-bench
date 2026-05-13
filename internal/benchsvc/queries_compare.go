package benchsvc

import (
	"context"
	"fmt"
)

// CompareModels returns per-scenario stats for two models side-by-side.
// Single query with conditional aggregation.
func (s *PgStore) CompareModels(ctx context.Context, tenantID, modelA, modelB string) ([]ScenarioModelComparison, error) {
	query := `
		SELECT scenario_id,
			COALESCE(100.0 * SUM(CASE WHEN model = $2 AND passed THEN 1 ELSE 0 END) /
				NULLIF(SUM(CASE WHEN model = $2 THEN 1 ELSE 0 END), 0), -1) AS a_pass_rate,
			COALESCE(100.0 * SUM(CASE WHEN model = $3 AND passed THEN 1 ELSE 0 END) /
				NULLIF(SUM(CASE WHEN model = $3 THEN 1 ELSE 0 END), 0), -1) AS b_pass_rate,
			COALESCE(AVG(CASE WHEN model = $2 THEN duration_seconds END), 0) AS a_duration,
			COALESCE(AVG(CASE WHEN model = $3 THEN duration_seconds END), 0) AS b_duration,
			COALESCE(AVG(CASE WHEN model = $2 THEN estimated_cost_usd END), 0) AS a_cost,
			COALESCE(AVG(CASE WHEN model = $3 THEN estimated_cost_usd END), 0) AS b_cost
		FROM bench_runs
		WHERE tenant_id = $1 AND archived_at IS NULL
			AND model IN ($2, $3)
		GROUP BY scenario_id
		HAVING SUM(CASE WHEN model = $2 THEN 1 ELSE 0 END) > 0
			AND SUM(CASE WHEN model = $3 THEN 1 ELSE 0 END) > 0
		ORDER BY scenario_id`

	args := []any{tenantID, modelA, modelB}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("bench.CompareModels: %w", err)
	}
	defer rows.Close()

	var results []ScenarioModelComparison
	for rows.Next() {
		var sc ScenarioModelComparison
		if err := rows.Scan(&sc.ScenarioID, &sc.APassRate, &sc.BPassRate,
			&sc.ADuration, &sc.BDuration, &sc.ACost, &sc.BCost); err != nil {
			return nil, fmt.Errorf("bench.CompareModels: scan: %w", err)
		}
		results = append(results, sc)
	}
	return results, rows.Err()
}
