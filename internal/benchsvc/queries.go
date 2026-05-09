package benchsvc

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// ErrNotFound is returned when a query matches zero rows.
var ErrNotFound = errors.New("not found")

// runRecordColumns is the SELECT column list for RunRecord scans.
const runRecordColumns = `id, tenant_id, scenario_id, model, provider, adapter, evidence_mode, tool_server,
	tool_server_version, scenario_version,
	passed, duration_seconds, exit_code, turns, memory_window,
	prompt_tokens, completion_tokens, estimated_cost_usd,
	checks_passed, checks_total, checks_json, metadata_json, created_at`

// EnabledModel is a model available to a tenant via platform default or tenant override.
type EnabledModel struct {
	ID                string  `json:"id"`
	DisplayName       string  `json:"display_name"`
	Provider          string  `json:"provider"`
	APIBaseURL        string  `json:"api_base_url,omitempty"`
	APIKeyEnv         string  `json:"-"` // never exposed to clients
	InputCostPerMtok  float64 `json:"input_cost_per_mtok"`
	OutputCostPerMtok float64 `json:"output_cost_per_mtok"`
}

// TenantProviderConfig holds mutable tenant-specific provider settings.
type TenantProviderConfig struct {
	APIKeyEnc     string  `json:"api_key"`
	APIBaseURL    string  `json:"api_base_url,omitempty"`
	RateLimit     int     `json:"rate_limit,omitempty"`
	MonthlyBudget float64 `json:"monthly_budget,omitempty"`
}

// ModelProviderInfo holds the provider and base URL for a model, used to
// resolve credentials at trigger time.
type ModelProviderInfo struct {
	Provider   string `json:"provider"`
	APIBaseURL string `json:"api_base_url"`
	APIKeyEnv  string `json:"-"`
}

// GlobalModelConfig holds platform-level configuration for a model.
type GlobalModelConfig struct {
	APIBaseURL string `json:"api_base_url,omitempty"`
	APIKeyEnv  string `json:"api_key_env,omitempty"`
}

// RunnerConfig holds the capabilities a runner reports at registration.
type RunnerConfig struct {
	Models       []string          `json:"models"`
	Provider     string            `json:"provider,omitempty"`
	MaxParallel  int               `json:"max_parallel,omitempty"`
	PollInterval int               `json:"poll_interval,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// Runner represents a registered runner from bench_infra.
type Runner struct {
	ID        string       `json:"id"`
	TenantID  string       `json:"tenant_id"`
	Name      string       `json:"name"`
	Region    string       `json:"region"`
	Status    string       `json:"status"`
	Config    RunnerConfig `json:"config"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// RegisterRunnerRequest is the payload for POST /v1/runners/register.
type RegisterRunnerRequest struct {
	Name        string            `json:"name"`
	Models      []string          `json:"models"`
	Provider    string            `json:"provider,omitempty"`
	Region      string            `json:"region,omitempty"`
	MaxParallel int               `json:"max_parallel,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// BenchJob represents a queued or running benchmark job from bench_jobs.
type BenchJob struct {
	ID           string          `json:"id"`
	TenantID     string          `json:"tenant_id"`
	InfraID      string          `json:"infra_id,omitempty"`
	Model        string          `json:"model"`
	Provider     string          `json:"provider"`
	Status       string          `json:"status"`
	Total        int             `json:"total"`
	Completed    int             `json:"completed"`
	Passed       int             `json:"passed"`
	Failed       int             `json:"failed"`
	ErrorMessage string          `json:"error_message,omitempty"`
	ConfigJSON   json.RawMessage `json:"config_json,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// JobConfig holds the scenario list and options stored in bench_jobs.config_json.
type JobConfig struct {
	Scenarios     []string `json:"scenarios"`
	Timeout       int      `json:"timeout,omitempty"`
	RunnerID      string   `json:"runner_id,omitempty"` // manual pinning
	EvidenceMode  string   `json:"evidence_mode,omitempty"`
	ExecutionMode string   `json:"execution_mode,omitempty"`
}

// scanRunRecord scans a row into a bench.RunRecord.
func scanRunRecord(row pgx.CollectableRow) (bench.RunRecord, error) {
	var r bench.RunRecord
	var checksJSON, metadataJSON *string
	err := row.Scan(
		&r.ID, &r.TenantID, &r.ScenarioID, &r.Model, &r.Provider, &r.Adapter, &r.EvidenceMode, &r.ToolServer,
		&r.ToolServerVersion, &r.ScenarioVersion,
		&r.Passed, &r.Duration, &r.ExitCode, &r.Turns, &r.MemoryWindow,
		&r.PromptTokens, &r.CompletionTokens, &r.EstimatedCost,
		&r.ChecksPassed, &r.ChecksTotal, &checksJSON, &metadataJSON, &r.CreatedAt,
	)
	if err != nil {
		return r, err
	}
	if checksJSON != nil {
		r.ChecksJSON = *checksJSON
	}
	if metadataJSON != nil {
		r.MetadataJSON = *metadataJSON
	}
	return r, nil
}

// buildWhere constructs a WHERE clause with numbered PostgreSQL placeholders.
// The tenant_id filter is always applied.
func buildWhere(tenantID string, f bench.RunFilters) (string, []any) {
	clauses := []string{"tenant_id = $1", "archived_at IS NULL"}
	args := []any{tenantID}

	if f.ScenarioID != "" {
		args = append(args, f.ScenarioID)
		clauses = append(clauses, fmt.Sprintf("scenario_id = $%d", len(args)))
	} else if len(f.ScenarioIDs) > 0 {
		args = append(args, f.ScenarioIDs)
		clauses = append(clauses, fmt.Sprintf("scenario_id = ANY($%d::text[])", len(args)))
	}
	if f.Model != "" {
		args = append(args, f.Model)
		clauses = append(clauses, fmt.Sprintf("model = $%d", len(args)))
	}
	if f.Provider != "" {
		args = append(args, f.Provider)
		clauses = append(clauses, fmt.Sprintf("provider = $%d", len(args)))
	}
	if clause, clauseArgs := evidenceModeClause(len(args)+1, f.EvidenceMode); clause != "" {
		clauses = append(clauses, clause)
		args = append(args, clauseArgs...)
	}
	if f.PassedOnly {
		clauses = append(clauses, "passed = TRUE")
	}
	if f.FailedOnly {
		clauses = append(clauses, "passed = FALSE")
	}
	if f.Since != nil {
		args = append(args, *f.Since)
		clauses = append(clauses, fmt.Sprintf("bench_runs.created_at >= $%d", len(args)))
	}
	if f.ExcludeErrors {
		clauses = append(clauses, "exit_code >= 0")
	}

	return " WHERE " + strings.Join(clauses, " AND "), args
}

// evidenceModeClause returns a SQL predicate for an exact evidence_mode filter.
func evidenceModeClause(argPos int, evidenceMode string) (string, []any) {
	switch evidenceMode {
	case "":
		return "", nil
	default:
		return fmt.Sprintf("evidence_mode = $%d", argPos), []any{evidenceMode}
	}
}

// nullableJSONB returns nil for empty strings (maps to SQL NULL for JSONB columns),
// or the string pointer for non-empty JSON.
func nullableJSONB(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
