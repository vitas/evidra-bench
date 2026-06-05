package benchsvc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
)

const benchJobSelectColumns = `id, tenant_id, infra_id, model, provider, status, total,
	completed, passed, failed, tool_server, tool_server_ver, skill_id, skill_version,
	error_message, config_json, created_at`

// RegisterRunner inserts a new remote runner into bench_infra.
func (s *PgStore) RegisterRunner(ctx context.Context, tenantID string, req RegisterRunnerRequest) (*Runner, error) {
	id := ulid.Make().String()
	cfg := RunnerConfig{
		Models:       req.Models,
		Provider:     req.Provider,
		MaxParallel:  req.MaxParallel,
		PollInterval: 5,
		Labels:       req.Labels,
	}
	if cfg.MaxParallel < 1 {
		cfg.MaxParallel = 1
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.RegisterRunner: marshal config: %w", err)
	}

	name := req.Name
	if name == "" {
		name = id[:8]
	}
	region := req.Region
	if region == "" {
		region = "local"
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO bench_infra (id, tenant_id, type, name, region, executor, config_json, status)
		VALUES ($1, $2, 'kind', $3, $4, 'remote', $5, 'healthy')
	`, id, tenantID, name, region, cfgJSON)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.RegisterRunner: %w", err)
	}

	return &Runner{
		ID:       id,
		TenantID: tenantID,
		Name:     name,
		Region:   region,
		Status:   "healthy",
		Config:   cfg,
	}, nil
}

// ListRunners returns all remote runners for a tenant.
func (s *PgStore) ListRunners(ctx context.Context, tenantID string) ([]Runner, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, tenant_id, name, region, status, config_json, created_at, updated_at
		FROM bench_infra
		WHERE tenant_id = $1 AND executor = 'remote'
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.ListRunners: %w", err)
	}
	defer rows.Close()

	var runners []Runner
	for rows.Next() {
		var r Runner
		var cfgJSON []byte
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Region, &r.Status, &cfgJSON, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("benchsvc.ListRunners scan: %w", err)
		}
		if len(cfgJSON) > 0 {
			_ = json.Unmarshal(cfgJSON, &r.Config)
		}
		runners = append(runners, r)
	}
	return runners, rows.Err()
}

// DeleteRunner removes a runner from bench_infra.
func (s *PgStore) DeleteRunner(ctx context.Context, tenantID, runnerID string) error {
	result, err := s.db.Exec(ctx, `
		DELETE FROM bench_infra WHERE id = $1 AND tenant_id = $2
	`, runnerID, tenantID)
	if err != nil {
		return fmt.Errorf("benchsvc.DeleteRunner: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchRunner updates the runner's updated_at and restores healthy status.
// Rejects runners that are draining or unhealthy. An unhealthy runner must
// re-register; a poll from an unhealthy runner returns 404.
func (s *PgStore) TouchRunner(ctx context.Context, tenantID, runnerID string) error {
	result, err := s.db.Exec(ctx, `
		UPDATE bench_infra SET updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND status = 'healthy'
	`, runnerID, tenantID)
	if err != nil {
		return fmt.Errorf("benchsvc.TouchRunner: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EnqueueJob inserts a new benchmark job into bench_jobs with status 'queued'.
func (s *PgStore) EnqueueJob(ctx context.Context, tenantID, model, provider string, cfg JobConfig) (*BenchJob, error) {
	id := ulid.Make().String()
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.EnqueueJob: marshal: %w", err)
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO bench_jobs (id, tenant_id, model, provider, status, total, config_json, tool_server, tool_server_ver, skill_id, skill_version)
		VALUES ($1, $2, $3, $4, 'queued', $5, $6, $7, $8, $9, $10)
	`, id, tenantID, model, provider, len(cfg.Scenarios), cfgJSON, cfg.ToolServer, cfg.ToolServerVersion, cfg.SkillID, cfg.SkillVersion)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.EnqueueJob: %w", err)
	}

	return &BenchJob{
		ID:                id,
		TenantID:          tenantID,
		Model:             model,
		Provider:          provider,
		Status:            "queued",
		Total:             len(cfg.Scenarios),
		ToolServer:        cfg.ToolServer,
		ToolServerVersion: cfg.ToolServerVersion,
		SkillID:           cfg.SkillID,
		SkillVersion:      cfg.SkillVersion,
		ConfigJSON:        cfgJSON,
	}, nil
}

// ClaimJob atomically claims the next queued job matching the runner's models.
// If a job has a runner_id pinned in config_json, only that runner can claim it.
// Returns nil if no job is available.
func (s *PgStore) ClaimJob(ctx context.Context, tenantID, runnerID string, models []string) (*BenchJob, error) {
	row := s.db.QueryRow(ctx, `
		UPDATE bench_jobs SET
			status = 'claimed',
			infra_id = $3,
			started_at = NOW()
		WHERE id = (
			SELECT id FROM bench_jobs
			WHERE tenant_id = $1
			  AND status = 'queued'
			  AND model = ANY($2)
			  AND (config_json->>'runner_id' = '' OR config_json->>'runner_id' IS NULL OR config_json->>'runner_id' = $3)
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING `+benchJobSelectColumns, tenantID, models, runnerID)
	job, err := scanBenchJobRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // no job available
	}
	if err != nil {
		return nil, fmt.Errorf("benchsvc.ClaimJob: %w", err)
	}
	return job, nil
}

// GetJob returns a persisted job by ID, scoped to a tenant.
func (s *PgStore) GetJob(ctx context.Context, tenantID, jobID string) (*BenchJob, error) {
	row := s.db.QueryRow(ctx, `
		SELECT `+benchJobSelectColumns+`
		FROM bench_jobs
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, jobID)
	job, err := scanBenchJobRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("benchsvc.GetJob: %w", err)
	}
	return job, nil
}

// CompleteJob marks a job as completed or failed with final counts.
// The infra_id (runner) must match to prevent stale runners from overwriting state.
func (s *PgStore) CompleteJob(ctx context.Context, tenantID, runnerID, jobID, status string, passed, failed int, errMsg string) error {
	result, err := s.db.Exec(ctx, `
		UPDATE bench_jobs SET
			status = $4,
			completed = $5 + $6,
			passed = $5,
			failed = $6,
			error_message = $7,
			completed_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND infra_id = $3
	`, jobID, tenantID, runnerID, status, passed, failed, errMsg)
	if err != nil {
		return fmt.Errorf("benchsvc.CompleteJob: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkUnhealthyRunners marks runners as unhealthy if they haven't checked in within the threshold.
func (s *PgStore) MarkUnhealthyRunners(ctx context.Context, threshold time.Duration) (int, error) {
	result, err := s.db.Exec(ctx, `
		UPDATE bench_infra SET status = 'unhealthy'
		WHERE executor = 'remote' AND status = 'healthy'
		  AND updated_at < NOW() - $1::interval
	`, threshold.String())
	if err != nil {
		return 0, fmt.Errorf("benchsvc.MarkUnhealthyRunners: %w", err)
	}
	return int(result.RowsAffected()), nil
}

// ResetStaleJobs resets claimed jobs back to queued if they haven't made progress.
func (s *PgStore) ResetStaleJobs(ctx context.Context, threshold time.Duration) (int, error) {
	result, err := s.db.Exec(ctx, `
		UPDATE bench_jobs SET status = 'queued', infra_id = NULL, started_at = NULL, last_progress_at = NULL
		WHERE status = 'claimed'
		  AND COALESCE(last_progress_at, started_at) < NOW() - $1::interval
	`, threshold.String())
	if err != nil {
		return 0, fmt.Errorf("benchsvc.ResetStaleJobs: %w", err)
	}
	return int(result.RowsAffected()), nil
}

// UpdateJobProgress updates a job's progress counters and last_progress_at timestamp.
func (s *PgStore) UpdateJobProgress(ctx context.Context, jobID string, completed, passed, failed int) error {
	_, err := s.db.Exec(ctx, `
		UPDATE bench_jobs SET
			status = 'running',
			completed = $2,
			passed = $3,
			failed = $4,
			last_progress_at = NOW()
		WHERE id = $1 AND status IN ('claimed', 'running')
	`, jobID, completed, passed, failed)
	if err != nil {
		return fmt.Errorf("benchsvc.UpdateJobProgress: %w", err)
	}
	return nil
}

// FindRunnerForModel finds a healthy runner that supports the given model.
func (s *PgStore) FindRunnerForModel(ctx context.Context, tenantID, model string) (*Runner, error) {
	var r Runner
	var cfgJSON []byte
	err := s.db.QueryRow(ctx, `
		SELECT id, tenant_id, name, region, status, config_json, created_at, updated_at
		FROM bench_infra
		WHERE tenant_id = $1
		  AND executor = 'remote'
		  AND status = 'healthy'
		  AND config_json->'models' ? $2
		ORDER BY updated_at DESC
		LIMIT 1
	`, tenantID, model).Scan(&r.ID, &r.TenantID, &r.Name, &r.Region, &r.Status, &cfgJSON, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("benchsvc.FindRunnerForModel: %w", err)
	}
	if len(cfgJSON) > 0 {
		_ = json.Unmarshal(cfgJSON, &r.Config)
	}
	return &r, nil
}

func scanBenchJobRow(row pgx.Row) (*BenchJob, error) {
	var job BenchJob
	var cfgJSON []byte
	var infraID sql.NullString
	err := row.Scan(
		&job.ID, &job.TenantID, &infraID, &job.Model, &job.Provider,
		&job.Status, &job.Total, &job.Completed, &job.Passed, &job.Failed,
		&job.ToolServer, &job.ToolServerVersion, &job.SkillID, &job.SkillVersion,
		&job.ErrorMessage, &cfgJSON, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if infraID.Valid {
		job.InfraID = infraID.String
	}
	job.ConfigJSON = cfgJSON
	return &job, nil
}
