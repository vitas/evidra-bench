package benchsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	bench "github.com/vitas/evidra-bench/pkg/bench"
)

// ErrPublicTenantUnavailable is returned when a public endpoint is called
// but no PublicTenant has been configured.
var ErrPublicTenantUnavailable = errors.New("benchsvc: public tenant not configured")

// ArchiveRequest specifies which runs to archive. At least one filter must be set.
type ArchiveRequest struct {
	Before *time.Time `json:"before,omitempty"`
	IDs    []string   `json:"ids,omitempty"`
	Model  string     `json:"model,omitempty"`
}

// Repository defines the data-access contract the Service depends on.
// PgStore satisfies this interface; test fakes can implement it too.
type Repository interface {
	ListRuns(ctx context.Context, tenantID string, f bench.RunFilters) ([]bench.RunRecord, int, error)
	GetRun(ctx context.Context, tenantID string, id string) (*bench.RunRecord, error)
	InsertRun(ctx context.Context, tenantID string, r bench.RunRecord) error
	InsertRunBatch(ctx context.Context, tenantID string, runs []bench.RunRecord) (int, error)
	DeleteRun(ctx context.Context, tenantID, runID string) error
	ArchiveRuns(ctx context.Context, tenantID string, req ArchiveRequest) (int, error)
	FilteredStats(ctx context.Context, tenantID string, f bench.RunFilters) (*bench.StatsResult, error)
	Catalog(ctx context.Context, tenantID string) (*bench.RunCatalog, error)
	ListEnabledModels(ctx context.Context, tenantID string) ([]EnabledModel, error)
	UpsertTenantProvider(ctx context.Context, tenantID, modelID string, cfg TenantProviderConfig) error
	DeleteTenantProvider(ctx context.Context, tenantID, modelID string) error
	UpdateGlobalModel(ctx context.Context, modelID string, cfg GlobalModelConfig) error
	ResolveModelProvider(ctx context.Context, modelID string) (*ModelProviderInfo, error)
	RegisterRunner(ctx context.Context, tenantID string, req RegisterRunnerRequest) (*Runner, error)
	ListRunners(ctx context.Context, tenantID string) ([]Runner, error)
	DeleteRunner(ctx context.Context, tenantID, runnerID string) error
	TouchRunner(ctx context.Context, tenantID, runnerID string) error
	EnqueueJob(ctx context.Context, tenantID, model, provider string, cfg JobConfig) (*BenchJob, error)
	ClaimJob(ctx context.Context, tenantID, runnerID string, models []string) (*BenchJob, error)
	CompleteJob(ctx context.Context, tenantID, runnerID, jobID, status string, passed, failed int, errMsg string) error
	FindRunnerForModel(ctx context.Context, tenantID, model string) (*Runner, error)
	MarkUnhealthyRunners(ctx context.Context, threshold time.Duration) (int, error)
	ResetStaleJobs(ctx context.Context, threshold time.Duration) (int, error)
	UpdateJobProgress(ctx context.Context, jobID string, completed, passed, failed int) error
	Leaderboard(ctx context.Context, tenantID string, evidenceMode string, k int, scenarios []string) ([]bench.LeaderboardEntry, error)
	ListScenarios(ctx context.Context) ([]bench.ScenarioSummary, error)
	StoreArtifact(ctx context.Context, runID, artifactType, contentType string, data []byte) error
	GetArtifact(ctx context.Context, tenantID string, runID, artifactType string) ([]byte, string, error)
	CompareModels(ctx context.Context, tenantID, modelA, modelB, evidenceMode string) ([]ScenarioModelComparison, error)
	ModelMatrix(ctx context.Context, tenantID string, models, scenarios []string, evidenceMode string) (*bench.ModelMatrix, error)
	SignalSummary(ctx context.Context, tenantID string, f bench.RunFilters) (*bench.SignalAggregation, error)
	Regressions(ctx context.Context, tenantID string) ([]bench.Regression, error)
	FailureAnalysis(ctx context.Context, tenantID string, scenarioID string) (*bench.FailureInsights, error)
	UpsertScenarios(ctx context.Context, scenarios []bench.ScenarioSummary) (int, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

// ServiceConfig holds configuration for the bench service.
type ServiceConfig struct {
	PublicTenant string        // tenant for unauthenticated read-only bench routes
	TriggerStore *TriggerStore // in-memory trigger job store (nil disables trigger endpoints)
	Executor     RunExecutor   // executor for bench trigger jobs (nil returns 501)
	Dispatcher   JobDispatcher // V2b: dispatches queued jobs to runners (nil skips runner path)
}

// Service provides request-scoped bench operations over a tenant-agnostic repository.
type Service struct {
	repo Repository
	cfg  ServiceConfig
}

// NewService creates a new Service backed by the given repository.
func NewService(repo Repository, cfg ServiceConfig) *Service {
	return &Service{repo: repo, cfg: cfg}
}

// IngestRunRequest wraps RunRecord with optional artifact payloads
// for atomic run+artifact ingestion.
type IngestRunRequest struct {
	bench.RunRecord
	Transcript string          `json:"transcript,omitempty"`
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	Autopsy    json.RawMessage `json:"autopsy,omitempty"`
}

// --- Authenticated methods (tenant from caller) ---

// ListRuns returns runs matching filters, scoped to the given tenant.
func (s *Service) ListRuns(ctx context.Context, tenantID string, f bench.RunFilters) ([]bench.RunRecord, int, error) {
	return s.repo.ListRuns(ctx, tenantID, f)
}

// GetRun returns a single run by ID, scoped to the given tenant.
func (s *Service) GetRun(ctx context.Context, tenantID string, id string) (*bench.RunRecord, error) {
	return s.repo.GetRun(ctx, tenantID, id)
}

// IngestRun atomically inserts a run and its artifacts using a database transaction.
// If any step fails, the entire operation is rolled back.
func (s *Service) IngestRun(ctx context.Context, tenantID string, req IngestRunRequest) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("benchsvc.IngestRun: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Build the insert query inline using the transaction.
	insertQ := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode,
		tool_server, tool_server_version, scenario_version,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`

	checksJSON := nullableJSONB(req.ChecksJSON)
	metadataJSON := nullableJSONB(req.MetadataJSON)
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err = tx.Exec(ctx, insertQ,
		req.ID, tenantID, req.ScenarioID, req.Model, req.Provider, req.Adapter, req.EvidenceMode,
		req.ToolServer, req.ToolServerVersion, req.ScenarioVersion,
		req.Passed, req.Duration, req.ExitCode, req.Turns, req.MemoryWindow,
		req.PromptTokens, req.CompletionTokens, req.EstimatedCost,
		req.ChecksPassed, req.ChecksTotal, checksJSON, metadataJSON, createdAt,
	)
	if err != nil {
		return fmt.Errorf("benchsvc.IngestRun: insert run: %w", err)
	}

	// Store artifacts within the same transaction.
	artifactQ := `INSERT INTO bench_artifacts (run_id, artifact_type, content_type, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (run_id, artifact_type) DO UPDATE SET data = EXCLUDED.data, content_type = EXCLUDED.content_type`

	if req.Transcript != "" {
		_, err = tx.Exec(ctx, artifactQ, req.ID, "transcript", "text/plain", []byte(req.Transcript))
		if err != nil {
			return fmt.Errorf("benchsvc.IngestRun: store transcript: %w", err)
		}
	}
	if len(req.ToolCalls) > 0 {
		_, err = tx.Exec(ctx, artifactQ, req.ID, "tool_calls", "application/json", []byte(req.ToolCalls))
		if err != nil {
			return fmt.Errorf("benchsvc.IngestRun: store tool_calls: %w", err)
		}
	}
	if len(req.Autopsy) > 0 {
		_, err = tx.Exec(ctx, artifactQ, req.ID, "failure_autopsy", "application/json", []byte(req.Autopsy))
		if err != nil {
			return fmt.Errorf("benchsvc.IngestRun: store failure_autopsy: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("benchsvc.IngestRun: commit: %w", err)
	}
	return nil
}

// IngestRunBatch atomically inserts multiple runs and their artifacts.
// If any artifact fails, all runs in the batch are rolled back.
// Returns the count of runs inserted.
func (s *Service) IngestRunBatch(ctx context.Context, tenantID string, runs []IngestRunRequest) (int, error) {
	if len(runs) == 0 {
		return 0, nil
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("benchsvc.IngestRunBatch: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	insertQ := `INSERT INTO bench_runs (
		id, tenant_id, scenario_id, model, provider, adapter, evidence_mode,
		tool_server, tool_server_version, scenario_version,
		passed, duration_seconds, exit_code, turns, memory_window,
		prompt_tokens, completion_tokens, estimated_cost_usd,
		checks_passed, checks_total, checks_json, metadata_json, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
	ON CONFLICT (id) DO NOTHING`

	artifactQ := `INSERT INTO bench_artifacts (run_id, artifact_type, content_type, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (run_id, artifact_type) DO UPDATE SET data = EXCLUDED.data, content_type = EXCLUDED.content_type`

	inserted := 0
	for _, run := range runs {
		checksJSON := nullableJSONB(run.ChecksJSON)
		metadataJSON := nullableJSONB(run.MetadataJSON)
		createdAt := run.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}

		ct, err := tx.Exec(ctx, insertQ,
			run.ID, tenantID, run.ScenarioID, run.Model, run.Provider, run.Adapter, run.EvidenceMode,
			run.ToolServer, run.ToolServerVersion, run.ScenarioVersion,
			run.Passed, run.Duration, run.ExitCode, run.Turns, run.MemoryWindow,
			run.PromptTokens, run.CompletionTokens, run.EstimatedCost,
			run.ChecksPassed, run.ChecksTotal, checksJSON, metadataJSON, createdAt,
		)
		if err != nil {
			return 0, fmt.Errorf("benchsvc.IngestRunBatch: insert run %s: %w", run.ID, err)
		}
		if ct.RowsAffected() == 0 {
			continue
		}
		inserted += int(ct.RowsAffected())

		if run.Transcript != "" {
			if _, err := tx.Exec(ctx, artifactQ, run.ID, "transcript", "text/plain", []byte(run.Transcript)); err != nil {
				return 0, fmt.Errorf("benchsvc.IngestRunBatch: transcript for %s: %w", run.ID, err)
			}
		}
		if len(run.ToolCalls) > 0 {
			if _, err := tx.Exec(ctx, artifactQ, run.ID, "tool_calls", "application/json", []byte(run.ToolCalls)); err != nil {
				return 0, fmt.Errorf("benchsvc.IngestRunBatch: tool_calls for %s: %w", run.ID, err)
			}
		}
		if len(run.Autopsy) > 0 {
			if _, err := tx.Exec(ctx, artifactQ, run.ID, "failure_autopsy", "application/json", []byte(run.Autopsy)); err != nil {
				return 0, fmt.Errorf("benchsvc.IngestRunBatch: failure_autopsy for %s: %w", run.ID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("benchsvc.IngestRunBatch: commit: %w", err)
	}
	return inserted, nil
}

// FilteredStats returns aggregate statistics matching the given filters.
func (s *Service) FilteredStats(ctx context.Context, tenantID string, f bench.RunFilters) (*bench.StatsResult, error) {
	return s.repo.FilteredStats(ctx, tenantID, f)
}

// Catalog returns distinct models and providers for the given tenant.
func (s *Service) Catalog(ctx context.Context, tenantID string) (*bench.RunCatalog, error) {
	return s.repo.Catalog(ctx, tenantID)
}

// ListEnabledModels returns models available to the given tenant.
func (s *Service) ListEnabledModels(ctx context.Context, tenantID string) ([]EnabledModel, error) {
	return s.repo.ListEnabledModels(ctx, tenantID)
}

// UpsertTenantProvider creates or updates a tenant-specific model provider override.
func (s *Service) UpsertTenantProvider(ctx context.Context, tenantID, modelID string, cfg TenantProviderConfig) error {
	return s.repo.UpsertTenantProvider(ctx, tenantID, modelID, cfg)
}

// DeleteTenantProvider removes a tenant-specific model provider override.
func (s *Service) DeleteTenantProvider(ctx context.Context, tenantID, modelID string) error {
	return s.repo.DeleteTenantProvider(ctx, tenantID, modelID)
}

// UpdateGlobalModel updates platform-level defaults for a model.
func (s *Service) UpdateGlobalModel(ctx context.Context, modelID string, cfg GlobalModelConfig) error {
	return s.repo.UpdateGlobalModel(ctx, modelID, cfg)
}

// ResolveModelProvider looks up a model's provider and base URL from the catalog.
func (s *Service) ResolveModelProvider(ctx context.Context, modelID string) (*ModelProviderInfo, error) {
	return s.repo.ResolveModelProvider(ctx, modelID)
}

// RegisterRunner registers a new remote runner.
func (s *Service) RegisterRunner(ctx context.Context, tenantID string, req RegisterRunnerRequest) (*Runner, error) {
	return s.repo.RegisterRunner(ctx, tenantID, req)
}

// ListRunners returns all remote runners for a tenant.
func (s *Service) ListRunners(ctx context.Context, tenantID string) ([]Runner, error) {
	return s.repo.ListRunners(ctx, tenantID)
}

// DeleteRunner removes a runner.
func (s *Service) DeleteRunner(ctx context.Context, tenantID, runnerID string) error {
	return s.repo.DeleteRunner(ctx, tenantID, runnerID)
}

// TouchRunner updates the runner's heartbeat timestamp.
func (s *Service) TouchRunner(ctx context.Context, tenantID, runnerID string) error {
	return s.repo.TouchRunner(ctx, tenantID, runnerID)
}

// ClaimJob atomically claims the next queued job for a runner.
func (s *Service) ClaimJob(ctx context.Context, tenantID, runnerID string, models []string) (*BenchJob, error) {
	return s.repo.ClaimJob(ctx, tenantID, runnerID, models)
}

// CompleteJob marks a job as completed or failed. RunnerID must match the claiming runner.
func (s *Service) CompleteJob(ctx context.Context, tenantID, runnerID, jobID, status string, passed, failed int, errMsg string) error {
	return s.repo.CompleteJob(ctx, tenantID, runnerID, jobID, status, passed, failed, errMsg)
}

// GetArtifact retrieves an artifact for a run, scoped to the given tenant.
func (s *Service) GetArtifact(ctx context.Context, tenantID string, runID, artifactType string) ([]byte, string, error) {
	return s.repo.GetArtifact(ctx, tenantID, runID, artifactType)
}

// StoreArtifact stores an artifact for a run (no tenant scoping on writes).
func (s *Service) StoreArtifact(ctx context.Context, runID, artifactType, contentType string, data []byte) error {
	return s.repo.StoreArtifact(ctx, runID, artifactType, contentType, data)
}

// DeleteRun deletes a single run by ID, scoped to the given tenant.
func (s *Service) DeleteRun(ctx context.Context, tenantID, runID string) error {
	return s.repo.DeleteRun(ctx, tenantID, runID)
}

// ArchiveRuns archives runs matching the given request filters.
func (s *Service) ArchiveRuns(ctx context.Context, tenantID string, req ArchiveRequest) (int, error) {
	return s.repo.ArchiveRuns(ctx, tenantID, req)
}

// --- Public methods (use configured PublicTenant) ---

// Leaderboard returns the public leaderboard using the configured public tenant.
// k controls the pass^k reliability metric (minimum 1, default 3).
func (s *Service) Leaderboard(ctx context.Context, evidenceMode string, k int, scenarios []string) ([]bench.LeaderboardEntry, error) {
	if s.cfg.PublicTenant == "" {
		return nil, ErrPublicTenantUnavailable
	}
	return s.repo.Leaderboard(ctx, s.cfg.PublicTenant, evidenceMode, k, scenarios)
}

// ListScenarios returns the global scenario catalog.
func (s *Service) ListScenarios(ctx context.Context) ([]bench.ScenarioSummary, error) {
	return s.repo.ListScenarios(ctx)
}

// UpsertScenarios inserts or updates scenario metadata.
func (s *Service) UpsertScenarios(ctx context.Context, scenarios []bench.ScenarioSummary) (int, error) {
	return s.repo.UpsertScenarios(ctx, scenarios)
}

// SignalSummary returns aggregated signal counts for a tenant.
func (s *Service) SignalSummary(ctx context.Context, tenantID string, f bench.RunFilters) (*bench.SignalAggregation, error) {
	return s.repo.SignalSummary(ctx, tenantID, f)
}

// Regressions returns scenario/model pairs with detected regressions.
func (s *Service) Regressions(ctx context.Context, tenantID string) ([]bench.Regression, error) {
	return s.repo.Regressions(ctx, tenantID)
}

// FailureAnalysis returns failure patterns for a specific scenario.
func (s *Service) FailureAnalysis(ctx context.Context, tenantID string, scenarioID string) (*bench.FailureInsights, error) {
	return s.repo.FailureAnalysis(ctx, tenantID, scenarioID)
}

// ModelMatrix returns a multi-model comparison grid.
func (s *Service) ModelMatrix(ctx context.Context, tenantID string, models, scenarios []string, evidenceMode string) (*bench.ModelMatrix, error) {
	return s.repo.ModelMatrix(ctx, tenantID, models, scenarios, evidenceMode)
}
