package benchsvc

import (
	"context"
	"fmt"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// FilteredStats returns aggregate statistics matching the given filters.
func (s *PgStore) FilteredStats(ctx context.Context, tenantID string, f bench.RunFilters) (*bench.StatsResult, error) {
	where, args := buildWhere(tenantID, f)

	var st bench.StatsResult
	err := s.db.QueryRow(ctx,
		"SELECT COUNT(*), COALESCE(SUM(CASE WHEN passed THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN NOT passed THEN 1 ELSE 0 END),0) FROM bench_runs"+where,
		args...,
	).Scan(&st.TotalRuns, &st.PassCount, &st.FailCount)
	if err != nil {
		return nil, fmt.Errorf("bench.FilteredStats: %w", err)
	}

	rows, err := s.db.Query(ctx,
		"SELECT scenario_id, COUNT(*), SUM(CASE WHEN passed THEN 1 ELSE 0 END) FROM bench_runs"+where+" GROUP BY scenario_id ORDER BY scenario_id",
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("bench.FilteredStats: by scenario: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ss bench.ScenarioStat
		if err := rows.Scan(&ss.ScenarioID, &ss.Runs, &ss.Passed); err != nil {
			return nil, fmt.Errorf("bench.FilteredStats: scan: %w", err)
		}
		st.ByScenario = append(st.ByScenario, ss)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bench.FilteredStats: rows: %w", err)
	}
	return &st, nil
}

// Catalog returns distinct models and providers from bench_runs.
func (s *PgStore) Catalog(ctx context.Context, tenantID string) (*bench.RunCatalog, error) {
	var models, providers []string

	rows, err := s.db.Query(ctx,
		"SELECT DISTINCT model FROM bench_runs WHERE tenant_id = $1 AND archived_at IS NULL ORDER BY model", tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: models: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan model: %w", err)
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: models rows: %w", err)
	}

	rows2, err := s.db.Query(ctx,
		"SELECT DISTINCT provider FROM bench_runs WHERE tenant_id = $1 AND archived_at IS NULL AND provider != '' ORDER BY provider", tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: providers: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var p string
		if err := rows2.Scan(&p); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan provider: %w", err)
		}
		providers = append(providers, p)
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: providers rows: %w", err)
	}

	return &bench.RunCatalog{Models: models, Providers: providers}, nil
}

// ListScenarios returns all scenarios from the global catalog.
func (s *PgStore) ListScenarios(ctx context.Context) ([]bench.ScenarioSummary, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, category, title, tools, chaos, evidra_enabled, track, level
		 FROM bench_scenarios ORDER BY category, id`)
	if err != nil {
		return nil, fmt.Errorf("bench.ListScenarios: %w", err)
	}
	defer rows.Close()

	var scenarios []bench.ScenarioSummary
	for rows.Next() {
		var sc bench.ScenarioSummary
		var tools []string
		if err := rows.Scan(&sc.ID, &sc.Category, &sc.Title, &tools, &sc.Chaos, &sc.Evidra, &sc.Track, &sc.Level); err != nil {
			return nil, fmt.Errorf("bench.ListScenarios: scan: %w", err)
		}
		sc.Tags = tools
		scenarios = append(scenarios, sc)
	}
	return scenarios, rows.Err()
}

// UpsertScenarios inserts or updates scenario metadata in bench_scenarios.
func (s *PgStore) UpsertScenarios(ctx context.Context, scenarios []bench.ScenarioSummary) (int, error) {
	upserted := 0
	for _, sc := range scenarios {
		tags := sc.Tags
		if tags == nil {
			tags = []string{}
		}
		_, err := s.db.Exec(ctx,
			`INSERT INTO bench_scenarios (id, category, title, description, tools, chaos, evidra_enabled, track, level, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
			 ON CONFLICT (id) DO UPDATE SET
			   category = EXCLUDED.category,
			   title = EXCLUDED.title,
			   description = EXCLUDED.description,
			   tools = EXCLUDED.tools,
			   chaos = EXCLUDED.chaos,
			   evidra_enabled = EXCLUDED.evidra_enabled,
			   track = EXCLUDED.track,
			   level = EXCLUDED.level,
			   updated_at = NOW()`,
			sc.ID, sc.Category, sc.Title, sc.Description, tags, sc.Chaos, sc.Evidra, sc.Track, sc.Level)
		if err != nil {
			return upserted, fmt.Errorf("bench.UpsertScenarios(%s): %w", sc.ID, err)
		}
		upserted++
	}
	return upserted, nil
}
