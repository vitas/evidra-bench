package benchsvc

import "context"

// JobDispatcher is called after a job is enqueued to notify the runner backend.
// V2b: PoolDispatcher (no-op — runners poll for work).
// V3:  GitHubDispatcher (triggers workflow_dispatch).
type JobDispatcher interface {
	Dispatch(ctx context.Context, job *BenchJob, runner *Runner) error
}

// PoolDispatcher is a no-op dispatcher for poll-based runners.
// Jobs are enqueued in bench_jobs; runners discover them via GET /v1/runners/jobs.
type PoolDispatcher struct{}

func (d *PoolDispatcher) Dispatch(_ context.Context, _ *BenchJob, _ *Runner) error {
	return nil
}
