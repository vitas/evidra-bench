package benchsvc

import (
	"context"
	"fmt"

	bench "github.com/vitas/evidra-bench/pkg/bench"
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
	var models, providers, toolServers, toolServerVersions, skillIDs, skillVersions []string
	toolServerVersionsByServer := map[string][]string{}
	skillVersionsByID := map[string][]string{}

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

	rows3, err := s.db.Query(ctx,
		"SELECT DISTINCT tool_server FROM bench_runs WHERE tenant_id = $1 AND archived_at IS NULL AND tool_server != '' ORDER BY tool_server", tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: tool servers: %w", err)
	}
	defer rows3.Close()
	for rows3.Next() {
		var toolServer string
		if err := rows3.Scan(&toolServer); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan tool server: %w", err)
		}
		toolServers = append(toolServers, toolServer)
	}
	if err := rows3.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: tool server rows: %w", err)
	}

	rows4, err := s.db.Query(ctx,
		"SELECT DISTINCT tool_server_version FROM bench_runs WHERE tenant_id = $1 AND archived_at IS NULL AND tool_server_version != '' ORDER BY tool_server_version", tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: tool server versions: %w", err)
	}
	defer rows4.Close()
	for rows4.Next() {
		var version string
		if err := rows4.Scan(&version); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan tool server version: %w", err)
		}
		toolServerVersions = append(toolServerVersions, version)
	}
	if err := rows4.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: tool server version rows: %w", err)
	}

	rows5, err := s.db.Query(ctx,
		`SELECT DISTINCT tool_server, tool_server_version
		 FROM bench_runs
		 WHERE tenant_id = $1
		   AND archived_at IS NULL
		   AND tool_server != ''
		   AND tool_server_version != ''
		 ORDER BY tool_server, tool_server_version`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: tool server version pairs: %w", err)
	}
	defer rows5.Close()
	for rows5.Next() {
		var toolServer, version string
		if err := rows5.Scan(&toolServer, &version); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan tool server version pair: %w", err)
		}
		toolServerVersionsByServer[toolServer] = append(toolServerVersionsByServer[toolServer], version)
	}
	if err := rows5.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: tool server version pair rows: %w", err)
	}

	rows6, err := s.db.Query(ctx,
		"SELECT DISTINCT skill_id FROM bench_runs WHERE tenant_id = $1 AND archived_at IS NULL AND skill_id != '' ORDER BY skill_id", tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: skill ids: %w", err)
	}
	defer rows6.Close()
	for rows6.Next() {
		var skillID string
		if err := rows6.Scan(&skillID); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan skill id: %w", err)
		}
		skillIDs = append(skillIDs, skillID)
	}
	if err := rows6.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: skill id rows: %w", err)
	}

	rows7, err := s.db.Query(ctx,
		"SELECT DISTINCT skill_version FROM bench_runs WHERE tenant_id = $1 AND archived_at IS NULL AND skill_version != '' ORDER BY skill_version", tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: skill versions: %w", err)
	}
	defer rows7.Close()
	for rows7.Next() {
		var version string
		if err := rows7.Scan(&version); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan skill version: %w", err)
		}
		skillVersions = append(skillVersions, version)
	}
	if err := rows7.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: skill version rows: %w", err)
	}

	rows8, err := s.db.Query(ctx,
		`SELECT DISTINCT skill_id, skill_version
		 FROM bench_runs
		 WHERE tenant_id = $1
		   AND archived_at IS NULL
		   AND skill_id != ''
		   AND skill_version != ''
		 ORDER BY skill_id, skill_version`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("bench.Catalog: skill version pairs: %w", err)
	}
	defer rows8.Close()
	for rows8.Next() {
		var skillID, version string
		if err := rows8.Scan(&skillID, &version); err != nil {
			return nil, fmt.Errorf("bench.Catalog: scan skill version pair: %w", err)
		}
		skillVersionsByID[skillID] = append(skillVersionsByID[skillID], version)
	}
	if err := rows8.Err(); err != nil {
		return nil, fmt.Errorf("bench.Catalog: skill version pair rows: %w", err)
	}

	return &bench.RunCatalog{
		Models:                     models,
		Providers:                  providers,
		ToolServers:                toolServers,
		ToolServerVersions:         toolServerVersions,
		ToolServerVersionsByServer: toolServerVersionsByServer,
		SkillIDs:                   skillIDs,
		SkillVersions:              skillVersions,
		SkillVersionsByID:          skillVersionsByID,
	}, nil
}

// ListScenarios returns all scenarios from the global catalog.
func (s *PgStore) ListScenarios(ctx context.Context) ([]bench.ScenarioSummary, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, category, title, tools, chaos, track, level
		 FROM bench_scenarios ORDER BY category, id`)
	if err != nil {
		return nil, fmt.Errorf("bench.ListScenarios: %w", err)
	}
	defer rows.Close()

	var scenarios []bench.ScenarioSummary
	for rows.Next() {
		var sc bench.ScenarioSummary
		var tools []string
		if err := rows.Scan(&sc.ID, &sc.Category, &sc.Title, &tools, &sc.Chaos, &sc.Track, &sc.Level); err != nil {
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
			`INSERT INTO bench_scenarios (id, category, title, description, tools, chaos, track, level, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
			 ON CONFLICT (id) DO UPDATE SET
			   category = EXCLUDED.category,
			   title = EXCLUDED.title,
			   description = EXCLUDED.description,
			   tools = EXCLUDED.tools,
			   chaos = EXCLUDED.chaos,
			   track = EXCLUDED.track,
			   level = EXCLUDED.level,
			   updated_at = NOW()`,
			sc.ID, sc.Category, sc.Title, sc.Description, tags, sc.Chaos, sc.Track, sc.Level)
		if err != nil {
			return upserted, fmt.Errorf("bench.UpsertScenarios(%s): %w", sc.ID, err)
		}
		upserted++
	}
	return upserted, nil
}
