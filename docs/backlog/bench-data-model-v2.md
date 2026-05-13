# Bench Data Model v2 — Design Document

Status: implemented in this repo's bench service migrations
`012_global_scenarios_and_models.up.sql` and
`013_bench_jobs_progress_tracking.up.sql`. Earlier bootstrap migrations are
idempotent so the private bench service can start against an existing public
bench Postgres schema when tables or columns already exist.

## Version History

| Version | Date | Changes |
|---|---|---|
| v1.0 | 2026-03-16 | Initial bench_runs, bench_artifacts, bench_scenarios |
| v1.1 | 2026-03-23 | Track/level on scenarios, tool_server on runs |
| v1.2 | 2026-03-24 | tool_server_version, scenario_version on runs |
| **v2.0** | **2026-03-25** | **Global models, tenant providers, jobs, infra — this doc** |

## Problem

The current schema stores everything flat in `bench_runs`:
- Model name as a text string — no cost data, no provider config
- Scenario ID as text — no FK, metadata synced separately
- No concept of "batch" — runs are individual, no grouping
- No infra tracking — can't distinguish kind vs EKS results
- Tenant model credentials hardcoded in env vars — not SaaS-ready

## Goals

1. Global catalog tables for models and scenarios (shared, not per-tenant)
2. Per-tenant provider configuration (API keys, rate limits, budgets)
3. Job grouping — a batch of runs with status tracking
4. Infrastructure registry — track what cluster each job ran on
5. Foundation for River job queue and parallel execution

## Entity Relationship

```
bench_models              1──────M  bench_tenant_providers
  (global catalog)                    (per tenant config)
                                         │
                                         │ tenant_id
                                         ▼
bench_scenarios           ◄─────── bench_runs ──────► bench_artifacts
  (global catalog)          scenario_id  │               (per run)
                                         │ job_id
                                         ▼
                            bench_jobs ──────► bench_infra
                          (per tenant)    infra_id  (per tenant or shared)
```

## Tables

### bench_models (global)

Shared catalog of LLM models. Maintained by platform operators.
Tenants read from this to see what's available.

```sql
CREATE TABLE bench_models (
    id                    TEXT PRIMARY KEY,        -- "gemini-2.5-flash"
    display_name          TEXT NOT NULL DEFAULT '', -- "Gemini 2.5 Flash"
    provider              TEXT NOT NULL DEFAULT '', -- "google", "anthropic", "openai"
    family                TEXT NOT NULL DEFAULT '', -- "gemini", "claude", "gpt"
    api_base_url          TEXT NOT NULL DEFAULT '', -- default provider URL
    api_key_env           TEXT NOT NULL DEFAULT '', -- env var hint for local CLI
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
```

**Indexes:** `provider`, `family`

**Seed data:** gemini-2.5-flash, gemini-2.5-pro, gpt-4.1, gpt-5.2,
claude-sonnet-4-20250514, deepseek-chat, qwen-plus

### bench_tenant_providers (per tenant)

Tenant-specific credentials and preferences per model.
Separates sensitive data (keys) from the global catalog.

```sql
CREATE TABLE bench_tenant_providers (
    tenant_id        TEXT NOT NULL REFERENCES tenants(id),
    model_id         TEXT NOT NULL REFERENCES bench_models(id),
    api_key_enc      TEXT NOT NULL DEFAULT '',     -- encrypted API key
    api_base_url     TEXT NOT NULL DEFAULT '',     -- override URL (e.g. own proxy)
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    priority         INTEGER NOT NULL DEFAULT 0,  -- preferred order for auto-selection
    rate_limit       INTEGER NOT NULL DEFAULT 0,  -- max concurrent runs (0 = unlimited)
    monthly_budget   DOUBLE PRECISION NOT NULL DEFAULT 0, -- spend cap USD (0 = unlimited)
    monthly_spent    DOUBLE PRECISION NOT NULL DEFAULT 0, -- current month spend
    budget_reset_at  TIMESTAMPTZ,                 -- when monthly_spent was last reset
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, model_id)
);
```

**Flow:**
1. Tenant sees global model list from `bench_models`
2. Configures API key + preferences in `bench_tenant_providers`
3. Job submission resolves credentials from tenant config
4. No config = model unavailable for that tenant
5. Budget exceeded = job rejected with clear error

### bench_scenarios (global, extended)

Extended from v1.0 with full metadata. Synced from bench repo YAML files.

```sql
CREATE TABLE bench_scenarios (
    id                TEXT PRIMARY KEY,            -- "broken-deployment"
    category          TEXT NOT NULL DEFAULT '',     -- "kubernetes", "terraform", "helm"
    title             TEXT NOT NULL DEFAULT '',
    description       TEXT NOT NULL DEFAULT '',
    track             TEXT NOT NULL DEFAULT '',     -- "workloads", "troubleshooting", etc.
    level             TEXT NOT NULL DEFAULT '',     -- "L1", "L2", "L3", "L4"
    difficulty        TEXT NOT NULL DEFAULT 'medium',
    tools             TEXT[] NOT NULL DEFAULT '{}', -- ["kubectl", "helm"]
    tags              TEXT[] NOT NULL DEFAULT '{}', -- ["pvc", "storage"]
    timeout_seconds   INTEGER NOT NULL DEFAULT 300,
    chaos             BOOLEAN NOT NULL DEFAULT FALSE,
    multi_stage       BOOLEAN NOT NULL DEFAULT FALSE,
    evidra_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    skip              BOOLEAN NOT NULL DEFAULT FALSE,
    skip_reason       TEXT NOT NULL DEFAULT '',
    version           TEXT NOT NULL DEFAULT '',     -- scenario content hash
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Indexes:** `category`, `track`, `level`

### bench_infra (per tenant or shared)

Tracks the execution environment. A kind cluster, vCluster, EKS cluster, etc.
Many jobs can reference the same infra (reuse-cluster pattern).

```sql
CREATE TABLE bench_infra (
    id             TEXT PRIMARY KEY,               -- ULID
    tenant_id      TEXT NOT NULL REFERENCES tenants(id),
    type           TEXT NOT NULL DEFAULT 'kind',   -- "kind", "vcluster", "eks", "gke", "aks"
    name           TEXT NOT NULL DEFAULT '',        -- "infra-bench", "tenant-acme-vcluster"
    version        TEXT NOT NULL DEFAULT '',        -- k8s version "v1.31.2"
    region         TEXT NOT NULL DEFAULT 'local',  -- "local", "eu-west-1", "us-east-1"
    runtime        TEXT NOT NULL DEFAULT '',        -- "macos-arm64", "linux-amd64", "k8s-pod"
    executor       TEXT NOT NULL DEFAULT 'local',  -- "local", "dind-pod", "remote"
    config_json    JSONB,                          -- node count, addons (ArgoCD, Istio), features
    status         TEXT NOT NULL DEFAULT 'active', -- "active", "hibernated", "destroyed"
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Indexes:** `tenant_id`, `type`, `status`

**Lifecycle:**
- Local Mac: one infra record per kind cluster, persists across jobs
- SaaS DinD: infra created per job, destroyed after
- Enterprise vCluster: persistent infra, many jobs reference it

### bench_jobs (per tenant)

A batch of scenario runs. Maps to a River job group.
Created by CLI (`bench --parallel`), API (`POST /v1/certify`), or scheduler.

```sql
CREATE TABLE bench_jobs (
    id               TEXT PRIMARY KEY,              -- ULID or River job ID
    tenant_id        TEXT NOT NULL REFERENCES tenants(id),
    infra_id         TEXT REFERENCES bench_infra(id),
    type             TEXT NOT NULL DEFAULT 'bench', -- "bench", "certify", "single"
    model            TEXT NOT NULL,                 -- model used for this batch
    provider         TEXT NOT NULL DEFAULT 'bifrost',
    tool_server      TEXT NOT NULL DEFAULT '',      -- MCP server command or empty
    tool_server_ver  TEXT NOT NULL DEFAULT '',
    evidence_mode    TEXT NOT NULL DEFAULT 'none',
    parallel         INTEGER NOT NULL DEFAULT 1,    -- worker count
    status           TEXT NOT NULL DEFAULT 'queued', -- "queued","running","completed","failed","cancelled"
    total            INTEGER NOT NULL DEFAULT 0,
    completed        INTEGER NOT NULL DEFAULT 0,
    passed           INTEGER NOT NULL DEFAULT 0,
    failed           INTEGER NOT NULL DEFAULT 0,
    config_json      JSONB,                         -- full run config snapshot
    triggered_by     TEXT NOT NULL DEFAULT 'cli',   -- "cli", "api", "scheduler"
    error_message    TEXT NOT NULL DEFAULT '',
    started_at       TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Indexes:** `tenant_id`, `status`, `model`, `created_at DESC`

**Status transitions:**
```
queued → running → completed
                 → failed
       → cancelled
```

### bench_runs (per job)

One run per scenario within a job. Extended with `job_id` FK.

```sql
-- Existing table, add column:
ALTER TABLE bench_runs ADD COLUMN IF NOT EXISTS job_id TEXT REFERENCES bench_jobs(id);
```

**Indexes:** `job_id`

Existing columns remain unchanged. `job_id` is nullable for backward
compatibility with runs created before jobs existed.

### bench_artifacts (unchanged)

No changes. Already linked to runs via `run_id` FK.

## Query Patterns

**"Show all jobs for a tenant"**
```sql
SELECT j.*, i.type as infra_type, i.name as infra_name
FROM bench_jobs j
LEFT JOIN bench_infra i ON j.infra_id = i.id
WHERE j.tenant_id = $1
ORDER BY j.created_at DESC;
```

**"Compare model performance across jobs"**
```sql
SELECT j.model,
       COUNT(*) as total_runs,
       SUM(CASE WHEN r.passed THEN 1 ELSE 0 END) as passed,
       AVG(r.duration_seconds) as avg_duration,
       SUM(r.estimated_cost_usd) as total_cost
FROM bench_jobs j
JOIN bench_runs r ON r.job_id = j.id
WHERE j.tenant_id = $1
GROUP BY j.model;
```

**"Get tenant's available models with spend"**
```sql
SELECT m.*, tp.enabled, tp.monthly_budget, tp.monthly_spent,
       (tp.api_key_enc != '') as has_key
FROM bench_models m
LEFT JOIN bench_tenant_providers tp
  ON tp.model_id = m.id AND tp.tenant_id = $1
WHERE m.recommended = TRUE
ORDER BY tp.priority DESC NULLS LAST, m.display_name;
```

**"Kind vs EKS pass rates"**
```sql
SELECT i.type, COUNT(*) as runs,
       AVG(CASE WHEN r.passed THEN 1.0 ELSE 0.0 END) as pass_rate
FROM bench_runs r
JOIN bench_jobs j ON r.job_id = j.id
JOIN bench_infra i ON j.infra_id = i.id
WHERE j.tenant_id = $1
GROUP BY i.type;
```

**"Job progress for live UI"**
```sql
SELECT id, status, total, completed, passed, failed,
       started_at, completed_at
FROM bench_jobs
WHERE id = $1 AND tenant_id = $2;
```

## Migration Strategy

1. Migration `012_global_scenarios_and_models.up.sql` creates new tables
2. Existing `bench_runs` gets `job_id` column (nullable, backward compat)
3. Existing `bench_scenarios` gets extended columns (additive, no breaks)
4. Seed `bench_models` with known models
5. No data migration needed — old runs work without `job_id`
6. New runs created through River always have `job_id`

## API Endpoints (planned)

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/bench/models` | List available models |
| `GET` | `/v1/bench/models/:id` | Model details + cost |
| `PUT` | `/v1/bench/providers/:model_id` | Configure tenant credentials |
| `GET` | `/v1/bench/infra` | List tenant's infra |
| `POST` | `/v1/bench/infra` | Register infra |
| `GET` | `/v1/bench/jobs` | List jobs |
| `POST` | `/v1/bench/jobs` | Create job (enqueue via River) |
| `GET` | `/v1/bench/jobs/:id` | Job status + progress |
| `DELETE` | `/v1/bench/jobs/:id` | Cancel job |
| `GET` | `/v1/bench/jobs/:id/runs` | Runs within a job |

## Security

- `api_key_enc` in `bench_tenant_providers` is encrypted at rest
  using AES-256-GCM with a per-tenant derived key
- Tenant isolation enforced via `tenant_id` on all queries
- Row-level security (RLS) as defense-in-depth
- API keys never returned in GET responses — only `has_key: true/false`
- Budget enforcement checked before job enqueueing, not after
