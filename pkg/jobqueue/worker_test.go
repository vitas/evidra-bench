package jobqueue

import (
	"testing"
)

func TestBenchJobArgs_Kind(t *testing.T) {
	t.Parallel()
	args := BenchJobArgs{ScenarioID: "kubernetes/broken-deployment"}
	if args.Kind() != "bench_scenario" {
		t.Errorf("expected bench_scenario, got %s", args.Kind())
	}
}

func TestBenchJobArgs_InsertOpts(t *testing.T) {
	t.Parallel()
	args := BenchJobArgs{}
	opts := args.InsertOpts()
	if opts.MaxAttempts != MaxJobAttempts {
		t.Errorf("expected MaxAttempts=%d, got %d", MaxJobAttempts, opts.MaxAttempts)
	}
}
