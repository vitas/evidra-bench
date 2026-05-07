-- Folded bench database baseline.
-- This is the complete schema for fresh bench databases.

CREATE TABLE IF NOT EXISTS tenants (
    id          TEXT PRIMARY KEY,
    label       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tenants (id, label) VALUES ('default', 'Default Tenant')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS api_keys (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL REFERENCES tenants(id),
    key_hash     BYTEA NOT NULL,
    prefix       TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys(tenant_id);

CREATE TABLE IF NOT EXISTS bench_models (
    id                    TEXT PRIMARY KEY,
    display_name          TEXT NOT NULL DEFAULT '',
    provider              TEXT NOT NULL DEFAULT '',
    family                TEXT NOT NULL DEFAULT '',
    api_base_url          TEXT NOT NULL DEFAULT '',
    api_key_env           TEXT NOT NULL DEFAULT '',
    input_cost_per_mtok   DOUBLE PRECISION NOT NULL DEFAULT 0,
    output_cost_per_mtok  DOUBLE PRECISION NOT NULL DEFAULT 0,
    context_window        INTEGER NOT NULL DEFAULT 0,
    max_output_tokens     INTEGER NOT NULL DEFAULT 0,
    supports_tool_use     BOOLEAN NOT NULL DEFAULT TRUE,
    supports_vision       BOOLEAN NOT NULL DEFAULT FALSE,
    recommended           BOOLEAN NOT NULL DEFAULT FALSE,
    notes                 TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bench_models_provider ON bench_models(provider);
CREATE INDEX IF NOT EXISTS idx_bench_models_family ON bench_models(family);

INSERT INTO bench_models (id, display_name, provider, family, api_base_url, api_key_env,
    input_cost_per_mtok, output_cost_per_mtok, context_window, recommended, notes)
VALUES
    ('gemini-2.5-flash', 'Gemini 2.5 Flash', 'google', 'gemini',
     'https://generativelanguage.googleapis.com/v1beta/openai', 'GEMINI_API_KEY',
     0.15, 0.60, 1048576, TRUE, 'Fast, cheap, 76-80% pass rate'),
    ('gemini-2.5-pro', 'Gemini 2.5 Pro', 'google', 'gemini',
     'https://generativelanguage.googleapis.com/v1beta/openai', 'GEMINI_API_KEY',
     1.25, 10.00, 1048576, TRUE, '84% pass rate, best Gemini'),
    ('gpt-4.1', 'GPT-4.1', 'openai', 'gpt',
     'https://api.openai.com/v1', 'OPENAI_API_KEY',
     2.00, 8.00, 1047576, TRUE, 'Fast, cheap for OpenAI tier'),
    ('gpt-5.2', 'GPT-5.2', 'openai', 'gpt',
     'https://api.openai.com/v1', 'OPENAI_API_KEY',
     2.50, 10.00, 128000, TRUE, ''),
    ('claude-sonnet-4-20250514', 'Claude Sonnet 4', 'anthropic', 'claude',
     'https://api.anthropic.com/v1', 'ANTHROPIC_API_KEY',
     3.00, 15.00, 200000, TRUE, 'Default benchmark model'),
    ('deepseek-chat', 'DeepSeek Chat', 'deepseek', 'deepseek',
     'https://api.deepseek.com/v1', 'DEEPSEEK_API_KEY',
     0.14, 0.28, 65536, TRUE, 'Very cheap, slower'),
    ('qwen-plus', 'Qwen Plus', 'alibaba', 'qwen',
     'https://dashscope-intl.aliyuncs.com/compatible-mode/v1', 'DASHSCOPE_API_KEY',
     0.80, 2.00, 131072, TRUE, '78% pass rate')
ON CONFLICT (id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    provider = EXCLUDED.provider,
    family = EXCLUDED.family,
    api_base_url = EXCLUDED.api_base_url,
    api_key_env = EXCLUDED.api_key_env,
    input_cost_per_mtok = EXCLUDED.input_cost_per_mtok,
    output_cost_per_mtok = EXCLUDED.output_cost_per_mtok,
    context_window = EXCLUDED.context_window,
    recommended = EXCLUDED.recommended,
    notes = EXCLUDED.notes,
    updated_at = now();

CREATE TABLE IF NOT EXISTS bench_tenant_providers (
    tenant_id        TEXT NOT NULL REFERENCES tenants(id),
    model_id         TEXT NOT NULL REFERENCES bench_models(id),
    api_key_enc      TEXT NOT NULL DEFAULT '',
    api_base_url     TEXT NOT NULL DEFAULT '',
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    priority         INTEGER NOT NULL DEFAULT 0,
    rate_limit       INTEGER NOT NULL DEFAULT 0,
    monthly_budget   DOUBLE PRECISION NOT NULL DEFAULT 0,
    monthly_spent    DOUBLE PRECISION NOT NULL DEFAULT 0,
    budget_reset_at  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, model_id)
);

CREATE TABLE IF NOT EXISTS bench_infra (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL REFERENCES tenants(id),
    type           TEXT NOT NULL DEFAULT 'kind',
    name           TEXT NOT NULL DEFAULT '',
    version        TEXT NOT NULL DEFAULT '',
    region         TEXT NOT NULL DEFAULT 'local',
    runtime        TEXT NOT NULL DEFAULT '',
    executor       TEXT NOT NULL DEFAULT 'local',
    config_json    JSONB,
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bench_infra_tenant ON bench_infra(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bench_infra_type ON bench_infra(type);
CREATE INDEX IF NOT EXISTS idx_bench_infra_status ON bench_infra(status);

CREATE TABLE IF NOT EXISTS bench_jobs (
    id               TEXT PRIMARY KEY,
    tenant_id        TEXT NOT NULL REFERENCES tenants(id),
    infra_id         TEXT REFERENCES bench_infra(id),
    type             TEXT NOT NULL DEFAULT 'bench',
    model            TEXT NOT NULL,
    provider         TEXT NOT NULL DEFAULT 'bifrost',
    tool_server      TEXT NOT NULL DEFAULT '',
    tool_server_ver  TEXT NOT NULL DEFAULT '',
    evidence_mode    TEXT NOT NULL DEFAULT 'none',
    parallel         INTEGER NOT NULL DEFAULT 1,
    status           TEXT NOT NULL DEFAULT 'queued',
    total            INTEGER NOT NULL DEFAULT 0,
    completed        INTEGER NOT NULL DEFAULT 0,
    passed           INTEGER NOT NULL DEFAULT 0,
    failed           INTEGER NOT NULL DEFAULT 0,
    config_json      JSONB,
    triggered_by     TEXT NOT NULL DEFAULT 'cli',
    error_message    TEXT NOT NULL DEFAULT '',
    started_at       TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    last_progress_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bench_jobs_tenant ON bench_jobs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bench_jobs_status ON bench_jobs(status);
CREATE INDEX IF NOT EXISTS idx_bench_jobs_model ON bench_jobs(model);
CREATE INDEX IF NOT EXISTS idx_bench_jobs_created ON bench_jobs(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS bench_runs (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    scenario_id TEXT NOT NULL,
    model TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    adapter TEXT NOT NULL DEFAULT 'cli',
    evidence_mode TEXT NOT NULL DEFAULT 'none',
    tool_server TEXT NOT NULL DEFAULT '',
    tool_server_version TEXT NOT NULL DEFAULT '',
    scenario_version TEXT NOT NULL DEFAULT '',
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
    archived_at TIMESTAMPTZ,
    job_id TEXT REFERENCES bench_jobs(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bench_runs_tenant ON bench_runs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bench_runs_model ON bench_runs(tenant_id, model);
CREATE INDEX IF NOT EXISTS idx_bench_runs_scenario ON bench_runs(tenant_id, scenario_id);
CREATE INDEX IF NOT EXISTS idx_bench_runs_evidence_mode ON bench_runs(tenant_id, evidence_mode);
CREATE INDEX IF NOT EXISTS idx_bench_runs_created ON bench_runs(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bench_runs_archived ON bench_runs(tenant_id, archived_at)
    WHERE archived_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_bench_runs_job ON bench_runs(job_id);

CREATE TABLE IF NOT EXISTS bench_artifacts (
    run_id TEXT NOT NULL REFERENCES bench_runs(id) ON DELETE CASCADE,
    artifact_type TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/json',
    data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, artifact_type)
);

CREATE TABLE IF NOT EXISTS bench_scenarios (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    difficulty TEXT NOT NULL DEFAULT 'medium',
    tools TEXT[] NOT NULL DEFAULT '{}',
    chaos BOOLEAN NOT NULL DEFAULT FALSE,
    evidra_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    track TEXT NOT NULL DEFAULT '',
    level TEXT NOT NULL DEFAULT '',
    tags TEXT[] NOT NULL DEFAULT '{}',
    timeout_seconds INTEGER NOT NULL DEFAULT 300,
    skip BOOLEAN NOT NULL DEFAULT FALSE,
    skip_reason TEXT NOT NULL DEFAULT '',
    multi_stage BOOLEAN NOT NULL DEFAULT FALSE,
    version TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bench_scenarios_category ON bench_scenarios(category);
CREATE INDEX IF NOT EXISTS idx_bench_scenarios_track ON bench_scenarios(track);
CREATE INDEX IF NOT EXISTS idx_bench_scenarios_level ON bench_scenarios(level);
