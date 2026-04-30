-- 008_bench_runs_archive.up.sql
-- Add archived_at column to bench_runs for soft-delete / archive support.

ALTER TABLE bench_runs ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_bench_runs_archived ON bench_runs(tenant_id, archived_at)
    WHERE archived_at IS NOT NULL;
