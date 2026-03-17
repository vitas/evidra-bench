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
