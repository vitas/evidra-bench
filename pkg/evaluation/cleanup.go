package evaluation

import (
	"context"
	"fmt"
	"time"
)

// ReleaseLease releases an owned environment even when execution was
// cancelled. Callers that borrow a lease must leave cleanup to its owner.
func ReleaseLease(ctx context.Context, lease Lease, timeout time.Duration) (CleanupResult, error) {
	result := CleanupResult{}
	if lease == nil {
		return result, nil
	}
	if timeout <= 0 {
		timeout = defaultCleanupTimeout
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	result.Attempted = true
	if err := lease.Release(cleanupCtx); err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("cleanup: %w", err)
	}
	result.Succeeded = true
	return result, nil
}
