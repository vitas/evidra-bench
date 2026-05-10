package benchdb

import (
	"context"
	"embed"
	"os"
	"strings"
	"testing"
)

//go:embed migrations/*.sql
var testMigrations embed.FS

func TestMigrationsEmbedded(t *testing.T) {
	t.Parallel()
	entries, err := testMigrations.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("embedded migrations = %d files, want one folded baseline", len(entries))
	}
	if got, want := entries[0].Name(), "001_init.up.sql"; got != want {
		t.Fatalf("embedded migration = %s, want %s", got, want)
	}
}

func TestConnect_InvalidURL(t *testing.T) {
	t.Parallel()
	_, err := Connect("postgres://invalid:5432/nonexistent?connect_timeout=1")
	if err == nil {
		t.Fatal("expected error for invalid database URL")
	}
}

func TestFoldedBaselineMigrationContainsFinalBenchSchema(t *testing.T) {
	t.Parallel()

	body, err := testMigrations.ReadFile("migrations/001_init.up.sql")
	if err != nil {
		t.Fatalf("read folded baseline: %v", err)
	}
	sql := string(body)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS tenants",
		"CREATE TABLE IF NOT EXISTS api_keys",
		"CREATE TABLE IF NOT EXISTS bench_models",
		"CREATE TABLE IF NOT EXISTS bench_tenant_providers",
		"CREATE TABLE IF NOT EXISTS bench_infra",
		"CREATE TABLE IF NOT EXISTS bench_jobs",
		"last_progress_at TIMESTAMPTZ",
		"CREATE TABLE IF NOT EXISTS bench_runs",
		"evidence_mode TEXT NOT NULL DEFAULT 'none'",
		"archived_at TIMESTAMPTZ",
		"tool_server TEXT NOT NULL DEFAULT ''",
		"tool_server_version TEXT NOT NULL DEFAULT ''",
		"scenario_version TEXT NOT NULL DEFAULT ''",
		"job_id TEXT REFERENCES bench_jobs(id)",
		"CREATE TABLE IF NOT EXISTS bench_artifacts",
		"CREATE TABLE IF NOT EXISTS bench_scenarios",
		"track TEXT NOT NULL DEFAULT ''",
		"level TEXT NOT NULL DEFAULT ''",
		"tags TEXT[] NOT NULL DEFAULT '{}'",
		"timeout_seconds INTEGER NOT NULL DEFAULT 300",
		"version TEXT NOT NULL DEFAULT ''",
		"CREATE INDEX IF NOT EXISTS idx_bench_runs_archived",
		"CREATE INDEX IF NOT EXISTS idx_bench_runs_tool_server",
		"INSERT INTO bench_models",
		"ON CONFLICT (id) DO UPDATE SET",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("folded baseline is missing %q", want)
		}
	}
}

func TestBackfillToolServerSQLMarksLegacyMCPRows(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"UPDATE bench_runs",
		"evidence_mode = 'mcp'",
		"tool_server = ''",
		"legacy-mcp",
		"metadata_json->>'tool_server'",
		"metadata_json->>'tool_server_version'",
	} {
		if !strings.Contains(backfillToolServerSQL, want) {
			t.Fatalf("backfill SQL is missing %q", want)
		}
	}
}

func TestEnsureToolServerIndexSQLIsIdempotent(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"CREATE INDEX IF NOT EXISTS idx_bench_runs_tool_server",
		"bench_runs(tenant_id, tool_server)",
	} {
		if !strings.Contains(ensureToolServerIndexSQL, want) {
			t.Fatalf("index SQL is missing %q", want)
		}
	}
}

func TestConnectAppliesFoldedBaseline(t *testing.T) {
	databaseURL := os.Getenv("BENCHDB_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("BENCHDB_TEST_DATABASE_URL not set")
	}

	pool, err := Connect(databaseURL)
	if err != nil {
		t.Fatalf("connect and migrate: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx := context.Background()
	for _, table := range []string{
		"tenants",
		"api_keys",
		"bench_models",
		"bench_tenant_providers",
		"bench_infra",
		"bench_jobs",
		"bench_runs",
		"bench_artifacts",
		"bench_scenarios",
	} {
		var exists bool
		if err := pool.QueryRow(ctx, "select to_regclass($1) is not null", "public."+table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("table %s was not created", table)
		}
	}

	var version int
	var dirty bool
	if err := pool.QueryRow(ctx, "select version, dirty from schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	if version != 1 || dirty {
		t.Fatalf("schema_migrations = version %d dirty %v, want version 1 dirty false", version, dirty)
	}

	var seededModels int
	if err := pool.QueryRow(ctx, "select count(*) from bench_models").Scan(&seededModels); err != nil {
		t.Fatalf("count seeded models: %v", err)
	}
	if seededModels == 0 {
		t.Fatal("bench_models seed data was not inserted")
	}
}
