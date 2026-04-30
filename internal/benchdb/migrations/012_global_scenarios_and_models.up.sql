-- 012_global_scenarios_and_models.up.sql
-- Data model v2: global catalogs, tenant providers, jobs, infra.
-- See docs/backlog/bench-data-model-v2.md for design document.

-- ── Extend bench_scenarios with full metadata ──

ALTER TABLE bench_scenarios ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE bench_scenarios ADD COLUMN IF NOT EXISTS timeout_seconds INTEGER NOT NULL DEFAULT 300;
ALTER TABLE bench_scenarios ADD COLUMN IF NOT EXISTS skip BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE bench_scenarios ADD COLUMN IF NOT EXISTS skip_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE bench_scenarios ADD COLUMN IF NOT EXISTS multi_stage BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE bench_scenarios ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_bench_scenarios_category ON bench_scenarios(category);
CREATE INDEX IF NOT EXISTS idx_bench_scenarios_track ON bench_scenarios(track);
CREATE INDEX IF NOT EXISTS idx_bench_scenarios_level ON bench_scenarios(level);

-- ── Global models catalog ──

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
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bench_models_provider ON bench_models(provider);
CREATE INDEX IF NOT EXISTS idx_bench_models_family ON bench_models(family);

-- ── Tenant provider config ──

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
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, model_id)
);

-- ── Infrastructure registry ──

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
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bench_infra_tenant ON bench_infra(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bench_infra_type ON bench_infra(type);
CREATE INDEX IF NOT EXISTS idx_bench_infra_status ON bench_infra(status);

-- ── Jobs (batch of runs) ──

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
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bench_jobs_tenant ON bench_jobs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bench_jobs_status ON bench_jobs(status);
CREATE INDEX IF NOT EXISTS idx_bench_jobs_model ON bench_jobs(model);
CREATE INDEX IF NOT EXISTS idx_bench_jobs_created ON bench_jobs(tenant_id, created_at DESC);

-- ── Link runs to jobs ──

ALTER TABLE bench_runs ADD COLUMN IF NOT EXISTS job_id TEXT REFERENCES bench_jobs(id);
CREATE INDEX IF NOT EXISTS idx_bench_runs_job ON bench_runs(job_id);

-- ── Seed known models ──

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
    updated_at = NOW();
