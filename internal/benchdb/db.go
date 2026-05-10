// Package benchdb manages PostgreSQL connections and bench schema migrations.
package benchdb

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Connect creates a connection pool and runs pending migrations.
func Connect(databaseURL string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("benchdb.Connect: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("benchdb.Connect: ping: %w", err)
	}

	if err := runMigrations(databaseURL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("benchdb.Connect: migrate: %w", err)
	}
	if err := backfillToolServerIdentity(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("benchdb.Connect: backfill tool server identity: %w", err)
	}

	return pool, nil
}

const backfillToolServerSQL = `
UPDATE bench_runs
SET
    tool_server = CASE
        WHEN NULLIF(metadata_json->>'tool_server', '') IS NOT NULL THEN metadata_json->>'tool_server'
        ELSE 'legacy-mcp'
    END,
    tool_server_version = COALESCE(NULLIF(tool_server_version, ''), NULLIF(metadata_json->>'tool_server_version', ''), '')
WHERE evidence_mode = 'mcp'
  AND tool_server = ''
`

const ensureToolServerIndexSQL = `
CREATE INDEX IF NOT EXISTS idx_bench_runs_tool_server
ON bench_runs(tenant_id, tool_server)
`

func backfillToolServerIdentity(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, ensureToolServerIndexSQL); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, backfillToolServerSQL)
	return err
}

func runMigrations(databaseURL string) error {
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("open migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, "pgx5://"+stripScheme(databaseURL))
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// stripScheme removes the "postgres://" or "postgresql://" prefix so
// golang-migrate can use its own "pgx5://" scheme.
func stripScheme(url string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if len(url) > len(prefix) && url[:len(prefix)] == prefix {
			return url[len(prefix):]
		}
	}
	return url
}
