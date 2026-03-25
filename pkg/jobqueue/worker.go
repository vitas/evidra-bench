package jobqueue

import (
	"context"
	"fmt"
	"log"

	"github.com/riverqueue/river"
)

// RunFunc is the function that executes a single scenario.
// It receives the job args and the worker namespace.
type RunFunc func(ctx context.Context, args BenchJobArgs, namespace string) error

// BenchWorker implements river.Worker for scenario execution.
type BenchWorker struct {
	river.WorkerDefaults[BenchJobArgs]
	runFn RunFunc
}

// NewBenchWorker creates a worker with the given run function.
func NewBenchWorker(fn RunFunc) *BenchWorker {
	return &BenchWorker{runFn: fn}
}

// Work executes a single scenario in an isolated namespace.
func (w *BenchWorker) Work(ctx context.Context, job *river.Job[BenchJobArgs]) error {
	ns := fmt.Sprintf("bench-w%d", job.Args.WorkerID)
	log.Printf("[worker-%d] running %s / %s in namespace %s",
		job.Args.WorkerID, job.Args.ScenarioID, job.Args.Model, ns)

	if err := w.runFn(ctx, job.Args, ns); err != nil {
		log.Printf("[worker-%d] %s failed: %v", job.Args.WorkerID, job.Args.ScenarioID, err)
		return err
	}

	log.Printf("[worker-%d] %s completed", job.Args.WorkerID, job.Args.ScenarioID)
	return nil
}
