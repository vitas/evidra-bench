package benchsvc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// ListRuns returns runs matching filters with pagination (total count + page).
func (s *PgStore) ListRuns(ctx context.Context, tenantID string, f bench.RunFilters) ([]bench.RunRecord, int, error) {
	where, args := buildWhere(tenantID, f)

	// Count total.
	var total int
	countQ := "SELECT COUNT(*) FROM bench_runs" + where
	if err := s.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("bench.ListRuns: count: %w", err)
	}

	// Fetch page.
	orderCol := "created_at"
	validSortColumns := map[string]bool{
		"created_at": true, "duration_seconds": true, "estimated_cost_usd": true,
		"scenario_id": true, "model": true, "provider": true,
		"tool_server": true, "tool_server_version": true, "checks_passed": true, "turns": true, "passed": true,
	}
	if f.SortBy != "" && validSortColumns[f.SortBy] {
		orderCol = f.SortBy
	}
	orderDir := "DESC"
	if f.SortOrder == "asc" {
		orderDir = "ASC"
	}

	query := "SELECT " + runRecordColumns + " FROM bench_runs" + where +
		fmt.Sprintf(" ORDER BY %s %s", orderCol, orderDir)
	if f.Limit > 0 {
		args = append(args, f.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if f.Offset > 0 {
		args = append(args, f.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("bench.ListRuns: %w", err)
	}
	defer rows.Close()

	records, err := pgx.CollectRows(rows, scanRunRecord)
	if err != nil {
		return nil, 0, fmt.Errorf("bench.ListRuns: collect: %w", err)
	}
	return records, total, nil
}

// GetRun returns a single run by ID, scoped to the given tenant.
func (s *PgStore) GetRun(ctx context.Context, tenantID string, id string) (*bench.RunRecord, error) {
	query := "SELECT " + runRecordColumns + " FROM bench_runs WHERE tenant_id = $1 AND id = $2 AND archived_at IS NULL"
	rows, err := s.db.Query(ctx, query, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("bench.GetRun: %w", err)
	}
	defer rows.Close()

	r, err := pgx.CollectExactlyOneRow(rows, scanRunRecord)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("bench.GetRun: %w", err)
	}
	return &r, nil
}

// InsertRun inserts a single benchmark run record.
func (s *PgStore) InsertRun(ctx context.Context, tenantID string, r bench.RunRecord) error {
	query := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode, tool_server,
		tool_server_version, scenario_version,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`

	checksJSON := nullableJSONB(r.ChecksJSON)
	metadataJSON := nullableJSONB(r.MetadataJSON)
	createdAt := r.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err := s.db.Exec(ctx, query,
		r.ID, tenantID, r.ScenarioID, r.Model, r.Provider, r.Adapter, r.EvidenceMode, r.ToolServer,
		r.ToolServerVersion, r.ScenarioVersion,
		r.Passed, r.Duration, r.ExitCode, r.Turns, r.MemoryWindow,
		r.PromptTokens, r.CompletionTokens, r.EstimatedCost,
		r.ChecksPassed, r.ChecksTotal, checksJSON, metadataJSON, createdAt,
	)
	if err != nil {
		return fmt.Errorf("bench.InsertRun: %w", err)
	}
	return nil
}

// InsertRunBatch inserts multiple runs, skipping duplicates. Returns the number inserted.
func (s *PgStore) InsertRunBatch(ctx context.Context, tenantID string, runs []bench.RunRecord) (int, error) {
	if len(runs) == 0 {
		return 0, nil
	}

	query := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode, tool_server,
		tool_server_version, scenario_version,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
	ON CONFLICT (id) DO NOTHING`

	inserted := 0
	batch := &pgx.Batch{}
	for _, r := range runs {
		checksJSON := nullableJSONB(r.ChecksJSON)
		metadataJSON := nullableJSONB(r.MetadataJSON)
		createdAt := r.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		batch.Queue(query,
			r.ID, tenantID, r.ScenarioID, r.Model, r.Provider, r.Adapter, r.EvidenceMode, r.ToolServer,
			r.ToolServerVersion, r.ScenarioVersion,
			r.Passed, r.Duration, r.ExitCode, r.Turns, r.MemoryWindow,
			r.PromptTokens, r.CompletionTokens, r.EstimatedCost,
			r.ChecksPassed, r.ChecksTotal, checksJSON, metadataJSON, createdAt,
		)
	}

	br := s.db.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()

	for range runs {
		ct, err := br.Exec()
		if err != nil {
			return inserted, fmt.Errorf("bench.InsertRunBatch: %w", err)
		}
		inserted += int(ct.RowsAffected())
	}
	return inserted, nil
}

// DeleteRun deletes a single run by ID, scoped to the given tenant.
// Artifacts are cascade-deleted via the foreign key constraint.
func (s *PgStore) DeleteRun(ctx context.Context, tenantID, runID string) error {
	query := `DELETE FROM bench_runs WHERE id = $1 AND tenant_id = $2`
	ct, err := s.db.Exec(ctx, query, runID, tenantID)
	if err != nil {
		return fmt.Errorf("bench.DeleteRun: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ArchiveRuns sets archived_at on runs matching the request filters.
// Returns the number of runs archived.
func (s *PgStore) ArchiveRuns(ctx context.Context, tenantID string, req ArchiveRequest) (int, error) {
	clauses := []string{"tenant_id = $1", "archived_at IS NULL"}
	args := []any{tenantID}

	if req.Before != nil {
		args = append(args, *req.Before)
		clauses = append(clauses, fmt.Sprintf("created_at < $%d", len(args)))
	}
	if len(req.IDs) > 0 {
		args = append(args, req.IDs)
		clauses = append(clauses, fmt.Sprintf("id = ANY($%d)", len(args)))
	}
	if req.Model != "" {
		args = append(args, req.Model)
		clauses = append(clauses, fmt.Sprintf("model = $%d", len(args)))
	}

	query := "UPDATE bench_runs SET archived_at = NOW() WHERE " + strings.Join(clauses, " AND ")
	ct, err := s.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("bench.ArchiveRuns: %w", err)
	}
	return int(ct.RowsAffected()), nil
}
