//go:build integration

package jobqueue

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelDryRun(t *testing.T) {
	dbURL := os.Getenv("BENCH_DATABASE_URL")
	if dbURL == "" {
		t.Skip("BENCH_DATABASE_URL not set")
	}

	var completed int64
	runFn := func(ctx context.Context, args BenchJobArgs, ns string) error {
		t.Logf("worker-%d: running %s in %s", args.NamespaceSlot, args.ScenarioID, ns)
		time.Sleep(50 * time.Millisecond) // simulate work
		atomic.AddInt64(&completed, 1)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := NewClient(ctx, dbURL, 3, runFn)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if err := client.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	scenarios := []string{"test/a", "test/b", "test/c", "test/d", "test/e"}
	if err := client.InsertBatch(ctx, scenarios, "test-model", "test", "", "", "", "", "", "", "", "", "test-job", "", 3); err != nil {
		t.Fatalf("InsertBatch: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.After(15 * time.Second)
	for {
		if atomic.LoadInt64(&completed) == int64(len(scenarios)) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout: only %d/%d completed", atomic.LoadInt64(&completed), len(scenarios))
		case <-time.After(100 * time.Millisecond):
		}
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	if err := client.Stop(stopCtx); err != nil {
		t.Errorf("Stop: %v", err)
	}

	if got := atomic.LoadInt64(&completed); got != int64(len(scenarios)) {
		t.Errorf("expected %d completed, got %d", len(scenarios), got)
	}
}
