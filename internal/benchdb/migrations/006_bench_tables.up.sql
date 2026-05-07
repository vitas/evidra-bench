-- 006_bench_tables.up.sql
-- Infrastructure agent benchmark results, artifacts, and scenario catalog.
-- Part of the bench intelligence layer (internal/bench/).

CREATE TABLE IF NOT EXISTS bench_runs (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    scenario_id TEXT NOT NULL,
    model TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    adapter TEXT NOT NULL DEFAULT 'cli',
    evidence_mode TEXT NOT NULL DEFAULT 'none',
    passed BOOLEAN NOT NULL DEFAULT FALSE,
    duration_seconds DOUBLE PRECISION NOT NULL DEFAULT 0,
    exit_code INTEGER NOT NULL DEFAULT 0,
    turns INTEGER NOT NULL DEFAULT 0,
    memory_window INTEGER NOT NULL DEFAULT -1,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    estimated_cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    checks_passed INTEGER NOT NULL DEFAULT 0,
    checks_total INTEGER NOT NULL DEFAULT 0,
    checks_json JSONB,
    metadata_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bench_runs_tenant ON bench_runs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bench_runs_model ON bench_runs(tenant_id, model);
CREATE INDEX IF NOT EXISTS idx_bench_runs_scenario ON bench_runs(tenant_id, scenario_id);
CREATE INDEX IF NOT EXISTS idx_bench_runs_evidence_mode ON bench_runs(tenant_id, evidence_mode);
CREATE INDEX IF NOT EXISTS idx_bench_runs_created ON bench_runs(tenant_id, created_at DESC);

-- Stores transcripts, tool-calls, timelines per run.
CREATE TABLE IF NOT EXISTS bench_artifacts (
    run_id TEXT NOT NULL REFERENCES bench_runs(id) ON DELETE CASCADE,
    artifact_type TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/json',
    data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (run_id, artifact_type)
);

-- Scenario catalog — global, not tenant-scoped.
-- Synced from bench repo, shared by all tenants.
CREATE TABLE IF NOT EXISTS bench_scenarios (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    difficulty TEXT NOT NULL DEFAULT 'medium',
    tools TEXT[] NOT NULL DEFAULT '{}',
    chaos BOOLEAN NOT NULL DEFAULT FALSE,
    evidra_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
