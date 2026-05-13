package jobqueue

import (
	"context"
	"fmt"
	"log"

	"github.com/riverqueue/river"
	"github.com/vitas/evidra-bench/pkg/config"
)

// RunFunc is the function that executes a single scenario.
// It receives the job args and the worker namespace.
// Returning an error triggers a River retry (MaxAttempts=2).
// Return nil even on scenario failure to avoid retries.
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
	ns := config.DefaultNamespace
	if job.Args.Parallel > 1 {
		ns = fmt.Sprintf("%s-w%d", config.DefaultNamespace, job.Args.NamespaceSlot)
	}
	log.Printf("[worker-%d] running %s / %s in namespace %s",
		job.Args.NamespaceSlot, job.Args.ScenarioID, job.Args.Model, ns)

	if err := w.runFn(ctx, job.Args, ns); err != nil {
		log.Printf("[worker-%d] %s failed: %v", job.Args.NamespaceSlot, job.Args.ScenarioID, err)
		return err
	}

	log.Printf("[worker-%d] %s completed", job.Args.NamespaceSlot, job.Args.ScenarioID)
	return nil
}
