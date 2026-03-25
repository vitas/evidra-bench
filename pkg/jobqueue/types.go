// Package jobqueue provides River-based job scheduling for parallel bench runs.
package jobqueue

import "github.com/riverqueue/river"

// BenchJobArgs defines the arguments for a single scenario bench job.
type BenchJobArgs struct {
	JobID      string `json:"job_id"`
	TenantID   string `json:"tenant_id"`
	ScenarioID string `json:"scenario_id"`
	Model      string `json:"model"`
	Provider   string `json:"provider"`
	MCPServer  string `json:"mcp_server,omitempty"`
	WorkerID   int    `json:"worker_id"`
}

// Kind returns the River job kind identifier.
func (BenchJobArgs) Kind() string { return "bench_scenario" }

// InsertOpts returns default insert options.
func (BenchJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: 2,
	}
}
