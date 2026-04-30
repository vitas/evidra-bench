package benchdb

import (
	"embed"
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
	found := make(map[string]bool, len(entries))
	for _, entry := range entries {
		found[entry.Name()] = true
	}
	for _, want := range []string{
		"001_tenants_and_keys.up.sql",
		"006_bench_tables.up.sql",
		"008_bench_runs_archive.up.sql",
		"009_bench_scenarios_track_level.up.sql",
		"010_bench_runs_tool_server.up.sql",
		"011_bench_runs_versions.up.sql",
		"012_global_scenarios_and_models.up.sql",
		"013_bench_jobs_progress_tracking.up.sql",
		"013_bench_jobs_progress_tracking.down.sql",
	} {
		if !found[want] {
			t.Fatalf("missing embedded migration %s", want)
		}
	}
}

func TestConnect_InvalidURL(t *testing.T) {
	t.Parallel()
	_, err := Connect("postgres://invalid:5432/nonexistent?connect_timeout=1")
	if err == nil {
		t.Fatal("expected error for invalid database URL")
	}
}

func TestBootstrapMigrationsAreIdempotentForExistingBenchSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file string
		want []string
	}{
		{
			file: "migrations/006_bench_tables.up.sql",
			want: []string{
				"CREATE TABLE IF NOT EXISTS bench_runs",
				"CREATE INDEX IF NOT EXISTS idx_bench_runs_tenant",
				"CREATE INDEX IF NOT EXISTS idx_bench_runs_model",
				"CREATE INDEX IF NOT EXISTS idx_bench_runs_scenario",
				"CREATE INDEX IF NOT EXISTS idx_bench_runs_evidence_mode",
				"CREATE INDEX IF NOT EXISTS idx_bench_runs_created",
				"CREATE TABLE IF NOT EXISTS bench_artifacts",
				"CREATE TABLE IF NOT EXISTS bench_scenarios",
			},
		},
		{
			file: "migrations/008_bench_runs_archive.up.sql",
			want: []string{
				"ALTER TABLE bench_runs ADD COLUMN IF NOT EXISTS archived_at",
				"CREATE INDEX IF NOT EXISTS idx_bench_runs_archived",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			body, err := testMigrations.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("read %s: %v", tt.file, err)
			}
			sql := string(body)
			for _, want := range tt.want {
				if !strings.Contains(sql, want) {
					t.Fatalf("%s is missing idempotent statement %q", tt.file, want)
				}
			}
		})
	}
}
