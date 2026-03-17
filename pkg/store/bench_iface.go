// Package store provides structured result storage with SQLite + JSONL backup.
package store

import "context"

// BenchStore defines the query interface for bench API handlers.
// This interface is portable — copy to parent project with pgx implementation.
type BenchStore interface {
	ListRuns(ctx context.Context, f RunFilters) ([]RunRecord, int, error)
	GetRun(ctx context.Context, id string) (*RunRecord, error)
	Catalog(ctx context.Context) (*RunCatalog, error)
	CompareRuns(ctx context.Context, idA, idB string) (*RunComparison, error)
	ModelMatrix(ctx context.Context, models, scenarios []string) (*ModelMatrix, error)
	FilteredStats(ctx context.Context, f RunFilters) (*StatsResult, error)
	ListScenarios(ctx context.Context) ([]ScenarioSummary, error)
	SignalSummary(ctx context.Context, f RunFilters) (*SignalAggregation, error)
	Regressions(ctx context.Context) ([]Regression, error)
	FailureAnalysis(ctx context.Context, scenarioID string) (*FailureInsights, error)
}

// FailureInsights holds analyzed failure patterns for a scenario.
type FailureInsights struct {
	ScenarioID      string             `json:"scenario_id"`
	TotalRuns       int                `json:"total_runs"`
	FailedRuns      int                `json:"failed_runs"`
	PassedRuns      int                `json:"passed_runs"`
	CheckFailures   []CheckFailureStat `json:"check_failures"`
	CommandPatterns []CommandPattern   `json:"command_patterns"`
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

// CommandPattern shows commands used differently between pass and fail runs.
type CommandPattern struct {
	Command    string `json:"command"`
	InPassRuns int    `json:"in_pass_runs"`
	InFailRuns int    `json:"in_fail_runs"`
	Indicator  string `json:"indicator"` // "pass_signal", "fail_signal", "neutral"
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

// RunCatalog holds distinct metadata values used for UI filters.
type RunCatalog struct {
	Models    []string `json:"models"`
	Providers []string `json:"providers"`
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

// RunFilters specifies filters for listing runs. Extends QueryFilters with Offset.
type RunFilters struct {
	ScenarioID string
	Model      string
	Provider   string
	PassedOnly bool
	FailedOnly bool
	Since      string // RFC3339 or date string
	Limit      int
	Offset     int
	SortBy     string // column to sort by: created_at, duration_seconds, estimated_cost, scenario_id, model, checks_passed
	SortOrder  string // asc or desc (default: desc)
}

// RunComparison holds the result of comparing two runs.
type RunComparison struct {
	RunA       RunRecord   `json:"run_a"`
	RunB       RunRecord   `json:"run_b"`
	CheckDiffs []CheckDiff `json:"check_diffs"`
}

// CheckDiff describes how a single check changed between two runs.
type CheckDiff struct {
	Name    string `json:"name"`
	TypeStr string `json:"type"`
	RunA    string `json:"run_a_verdict"`
	RunB    string `json:"run_b_verdict"`
	Change  string `json:"change"` // same, improved, regressed
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

// ScenarioSummary holds metadata about a scenario for listing.
type ScenarioSummary struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Chaos    bool     `json:"chaos"`
	Evidra   bool     `json:"evidra"`
}
