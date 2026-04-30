// Package bench provides structured result storage and queries for
// infrastructure agent benchmark runs (PostgreSQL / pgx).
package bench

import (
	"time"
)

// LeaderboardEntry represents one model's aggregate benchmark performance.
type LeaderboardEntry struct {
	Model               string  `json:"model"`
	Scenarios           int     `json:"scenarios"`
	Runs                int     `json:"runs"`
	PassRate            float64 `json:"pass_rate"`
	AvgDuration         float64 `json:"avg_duration"`
	AvgCost             float64 `json:"avg_cost"`
	TotalCost           float64 `json:"total_cost"`
	PassK               float64 `json:"pass_k"`               // pass^k reliability (0-100)
	PassKTrials         int     `json:"pass_k_trials"`        // k value used
	SufficientScenarios int     `json:"sufficient_scenarios"` // scenarios with >= k trials
}

// RunRecord represents a single benchmark run stored in bench_runs.
type RunRecord struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	ScenarioID        string    `json:"scenario_id"`
	Model             string    `json:"model"`
	Provider          string    `json:"provider"`
	Adapter           string    `json:"adapter"`
	EvidenceMode      string    `json:"evidence_mode"`       // direct, proxy, smart, or none; evidra is a query alias for non-none
	ToolServer        string    `json:"tool_server"`         // MCP server used (empty = baseline/direct exec)
	ToolServerVersion string    `json:"tool_server_version"` // version of MCP server binary
	ScenarioVersion   string    `json:"scenario_version"`    // version/hash of scenario definition
	Passed            bool      `json:"passed"`
	Duration          float64   `json:"duration_seconds"`
	ExitCode          int       `json:"exit_code"`
	Turns             int       `json:"turns"`
	MemoryWindow      int       `json:"memory_window"`
	PromptTokens      int       `json:"prompt_tokens"`
	CompletionTokens  int       `json:"completion_tokens"`
	EstimatedCost     float64   `json:"estimated_cost_usd"`
	ChecksPassed      int       `json:"checks_passed"`
	ChecksTotal       int       `json:"checks_total"`
	ChecksJSON        string    `json:"checks_json,omitempty"`
	MetadataJSON      string    `json:"metadata_json,omitempty"`
	ArtifactDir       string    `json:"artifact_dir,omitempty"` // local filesystem path (bench runner only)
	CreatedAt         time.Time `json:"created_at"`
}

// RunFilters specifies filters for listing runs.
type RunFilters struct {
	ScenarioID    string
	Model         string
	Provider      string
	EvidenceMode  string // proxy, direct, smart, none -- empty means all; evidra aliases non-none
	PassedOnly    bool
	FailedOnly    bool
	Since         *time.Time // cutoff time — handler parses, store just uses
	Limit         int
	Offset        int
	SortBy        string // column to sort by
	SortOrder     string // asc or desc (default: desc)
	ExcludeErrors bool   // exclude infra errors (exit_code < 0)
}

// RunCatalog holds distinct metadata values used for UI filters.
type RunCatalog struct {
	Models    []string `json:"models"`
	Providers []string `json:"providers"`
}

// StatsResult holds aggregate run statistics.
type StatsResult struct {
	TotalRuns  int            `json:"total_runs"`
	PassCount  int            `json:"pass_count"`
	FailCount  int            `json:"fail_count"`
	ByScenario []ScenarioStat `json:"by_scenario"`
}

// ScenarioStat holds per-scenario stats.
type ScenarioStat struct {
	ScenarioID string `json:"scenario_id"`
	Runs       int    `json:"runs"`
	Passed     int    `json:"passed"`
}

// ScenarioSummary holds metadata about a scenario for listing.
type ScenarioSummary struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Category    string   `json:"category"`
	Track       string   `json:"track,omitempty"`
	Level       string   `json:"level,omitempty"`
	Timeout     string   `json:"timeout,omitempty"`
	Tags        []string `json:"tags"`
	Chaos       bool     `json:"chaos"`
	Evidra      bool     `json:"evidra"`
	Skip        bool     `json:"skip,omitempty"`
}

// SignalAggregation holds aggregated signal counts across runs.
type SignalAggregation struct {
	TotalRuns         int                    `json:"total_runs"`
	RunsWithScorecard int                    `json:"runs_with_scorecard"`
	Signals           map[string]SignalCount `json:"signals"`
	AvgScore          float64                `json:"avg_score"`
}

// SignalCount holds detection stats for a single signal type.
type SignalCount struct {
	Total    int `json:"total"`     // total detections
	RunCount int `json:"run_count"` // runs where detected > 0
}

// Regression describes a scenario/model pair where the latest run failed
// but previous runs had a positive pass rate.
type Regression struct {
	ScenarioID   string  `json:"scenario_id"`
	Model        string  `json:"model"`
	LatestRunID  string  `json:"latest_run_id"`
	LatestPassed bool    `json:"latest_passed"`
	PrevPassed   int     `json:"prev_passed"`
	PrevTotal    int     `json:"prev_total"`
	PrevRate     float64 `json:"prev_rate"`
	Severity     string  `json:"severity"` // critical, warning
}

// FailureInsights holds analyzed failure patterns for a scenario.
type FailureInsights struct {
	ScenarioID      string             `json:"scenario_id"`
	TotalRuns       int                `json:"total_runs"`
	FailedRuns      int                `json:"failed_runs"`
	PassedRuns      int                `json:"passed_runs"`
	CheckFailures   []CheckFailureStat `json:"check_failures"`
	ModelBreakdown  []ModelFailureStat `json:"model_breakdown"`
	BehaviorMetrics BehaviorComparison `json:"behavior_metrics"`
}

// CheckFailureStat shows how often a specific check fails.
type CheckFailureStat struct {
	CheckName string  `json:"check_name"`
	CheckType string  `json:"check_type"`
	FailCount int     `json:"fail_count"`
	FailRate  float64 `json:"fail_rate"` // percentage of failed runs where this check failed
	Message   string  `json:"message,omitempty"`
}

// ModelFailureStat shows pass/fail per model for a scenario.
type ModelFailureStat struct {
	Model  string  `json:"model"`
	Runs   int     `json:"runs"`
	Passed int     `json:"passed"`
	Failed int     `json:"failed"`
	Rate   float64 `json:"rate"`
}

// BehaviorComparison shows metric differences between pass and fail runs.
type BehaviorComparison struct {
	PassAvgTurns    float64 `json:"pass_avg_turns"`
	FailAvgTurns    float64 `json:"fail_avg_turns"`
	PassAvgDuration float64 `json:"pass_avg_duration"`
	FailAvgDuration float64 `json:"fail_avg_duration"`
	PassAvgTokens   float64 `json:"pass_avg_tokens"`
	FailAvgTokens   float64 `json:"fail_avg_tokens"`
	PassAvgCost     float64 `json:"pass_avg_cost"`
	FailAvgCost     float64 `json:"fail_avg_cost"`
}

// ModelMatrix holds a comparison grid across models and scenarios.
type ModelMatrix struct {
	Models    []string                              `json:"models"`
	Scenarios []string                              `json:"scenarios"`
	Cells     map[string]map[string]ModelMatrixCell `json:"cells"` // [scenario][model]
}

// ModelMatrixCell holds aggregate metrics for one scenario/model pair.
type ModelMatrixCell struct {
	Runs        int     `json:"runs"`
	Passed      int     `json:"passed"`
	PassRate    float64 `json:"pass_rate"`
	AvgCost     float64 `json:"avg_cost"`
	AvgTokens   int     `json:"avg_tokens"`
	AvgDuration float64 `json:"avg_duration"`
}
