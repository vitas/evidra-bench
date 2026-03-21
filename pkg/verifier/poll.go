package verifier

import (
	"context"
	"time"
)

// PollChecks runs all checkers in a loop until all pass or the context is cancelled.
// It polls at the given interval. Returns the last VerifyResult.
func PollChecks(ctx context.Context, kubeconfigPath string, checkers []Checker, interval time.Duration) *VerifyResult {
	for {
		result := RunChecks(ctx, kubeconfigPath, checkers)
		if result.Passed {
			return result
		}
		select {
		case <-ctx.Done():
			return result
		case <-time.After(interval):
		}
	}
}
