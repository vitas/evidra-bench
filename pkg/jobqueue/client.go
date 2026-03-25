package jobqueue

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// Client wraps a River client with bench-specific configuration.
// Client wraps a River client with bench-specific configuration.
// Lifecycle: NewClient → Migrate → Insert/InsertBatch → Start → Stop.
// Migrate must be called before Start. Stop must be called exactly once.
type Client struct {
	river *river.Client[pgx.Tx]
	pool  *pgxpool.Pool
}

// NewClient creates a River client connected to PostgreSQL.
// parallel controls the max concurrent workers.
func NewClient(ctx context.Context, databaseURL string, parallel int, runFn RunFunc) (*Client, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("jobqueue: connect: %w", err)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, NewBenchWorker(runFn))

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: parallel},
		},
		Workers: workers,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("jobqueue: create client: %w", err)
	}

	return &Client{river: riverClient, pool: pool}, nil
}

// Migrate runs River's internal migrations to create its job tables.
func (c *Client) Migrate(ctx context.Context) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(c.pool), nil)
	if err != nil {
		return fmt.Errorf("jobqueue: migrator: %w", err)
	}
	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	return err
}

// Start begins processing jobs. Blocks until ctx is cancelled.
func (c *Client) Start(ctx context.Context) error {
	log.Printf("[jobqueue] starting workers")
	return c.river.Start(ctx)
}

// Stop gracefully stops the client.
func (c *Client) Stop(ctx context.Context) error {
	if err := c.river.Stop(ctx); err != nil {
		return err
	}
	c.pool.Close()
	return nil
}

// Stopped returns a channel that is closed when all workers have stopped.
func (c *Client) Stopped() <-chan struct{} {
	return c.river.Stopped()
}

// Insert enqueues a bench scenario job.
func (c *Client) Insert(ctx context.Context, args BenchJobArgs) error {
	_, err := c.river.Insert(ctx, args, nil)
	if err != nil {
		return fmt.Errorf("jobqueue: insert: %w", err)
	}
	return nil
}

// InsertBatch enqueues multiple scenario jobs, assigning worker IDs round-robin.
func (c *Client) InsertBatch(ctx context.Context, scenarios []string, model, provider, mcpServer, jobID, tenantID string, parallel int) error {
	params := make([]river.InsertManyParams, 0, len(scenarios))
	for i, sid := range scenarios {
		params = append(params, river.InsertManyParams{
			Args: BenchJobArgs{
				JobID:         jobID,
				TenantID:      tenantID,
				ScenarioID:    sid,
				Model:         model,
				Provider:      provider,
				MCPServer:     mcpServer,
				NamespaceSlot: i % parallel,
				Parallel:      parallel,
			},
		})
	}
	_, err := c.river.InsertMany(ctx, params)
	if err != nil {
		return fmt.Errorf("jobqueue: insert batch: %w", err)
	}
	log.Printf("[jobqueue] enqueued %d scenarios for %s", len(scenarios), model)
	return nil
}
