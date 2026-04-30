package benchsvc

import (
	"context"
	"log"
	"time"
)

// StartRunnerJanitor periodically marks unhealthy runners and resets stale jobs.
func StartRunnerJanitor(ctx context.Context, repo Repository, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := repo.MarkUnhealthyRunners(ctx, 15*time.Second); err != nil {
				log.Printf("[runner-janitor] mark unhealthy: %v", err)
			} else if n > 0 {
				log.Printf("[runner-janitor] marked %d runners unhealthy", n)
			}
			if n, err := repo.ResetStaleJobs(ctx, 5*time.Minute); err != nil {
				log.Printf("[runner-janitor] reset stale: %v", err)
			} else if n > 0 {
				log.Printf("[runner-janitor] reset %d stale jobs to queued", n)
			}
		}
	}
}
