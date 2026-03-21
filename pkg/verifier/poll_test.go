package verifier

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type countingChecker struct {
	calls     atomic.Int32
	passAfter int32
}

func (c *countingChecker) Check(_ context.Context, _ string) CheckResult {
	n := c.calls.Add(1)
	if n >= c.passAfter {
		return CheckResult{Name: "test", Verdict: VerdictPass}
	}
	return CheckResult{Name: "test", Verdict: VerdictFail, Message: "not yet"}
}

func TestPollChecks_PassesAfterRetries(t *testing.T) {
	t.Parallel()
	checker := &countingChecker{passAfter: 3}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := PollChecks(ctx, "", []Checker{checker}, 100*time.Millisecond)
	if !result.Passed {
		t.Fatal("expected pass")
	}
	if checker.calls.Load() < 3 {
		t.Errorf("expected at least 3 calls, got %d", checker.calls.Load())
	}
}

func TestPollChecks_TimeoutFails(t *testing.T) {
	t.Parallel()
	checker := &countingChecker{passAfter: 1000}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	result := PollChecks(ctx, "", []Checker{checker}, 100*time.Millisecond)
	if result.Passed {
		t.Fatal("expected fail on timeout")
	}
}

func TestPollChecks_ImmediatePass(t *testing.T) {
	t.Parallel()
	checker := &countingChecker{passAfter: 1}
	ctx := context.Background()

	result := PollChecks(ctx, "", []Checker{checker}, 100*time.Millisecond)
	if !result.Passed {
		t.Fatal("expected immediate pass")
	}
	if checker.calls.Load() != 1 {
		t.Errorf("expected 1 call, got %d", checker.calls.Load())
	}
}
