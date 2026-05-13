# Bench Service Setup

This guide covers running the private bench service from this repo. It is the
first refactor phase: no managed cloud dependency is required.

Production deployment is out of scope for this repository. Keep deployment
topology, compose/manifests, secrets wiring, and environment-specific settings
in a separate private infrastructure repository.

## What Runs Here

`bench-cli serve` now owns:

- benchmark result APIs under `/v1/bench/*`
- trigger polling under `/v1/bench/trigger/*`
- poll-based runner control plane under `/v1/runners/*`
- the direct executor endpoint `POST /v1/certify`
- Postgres migrations for bench tables
- the local orchestration cluster used by `POST /v1/certify` when direct
  executor mode is enabled

## Requirements

- Go 1.25+
- PostgreSQL, preferably local or self-managed for this phase
- `kind`, `kubectl`, and `helm` for local execution
- an API key value for Bearer auth

Do not point the first-phase smoke test at a hosted Postgres instance in
Hetzner. A local Docker/Postgres.app database or a disposable self-managed
development database is enough.

## Environment

```bash
export BENCH_DATABASE_URL=postgres://bench:bench@localhost:5432/bench?sslmode=disable
export BENCH_API_KEY=dev-secret
export BENCH_PUBLIC_TENANT=default
export BENCH_SERVICE_ADDR=:8090
```

Equivalent flags:

```bash
bench-cli serve \
  --database-url "$BENCH_DATABASE_URL" \
  --bench-api-key "$BENCH_API_KEY"
```

`BENCH_DATABASE_URL` is also used by parallel bench execution. `BENCH_API_KEY`
is the static Bearer token for authenticated HTTP routes.
`BENCH_PUBLIC_TENANT` is the tenant used by unauthenticated read-only benchmark
routes. If it is omitted, `bench-cli serve` uses `BENCH_DEFAULT_TENANT`, or
`default` when both tenant env vars are empty.

For hosted control-plane deployments that use remote runners, disable the local
direct executor:

```bash
export BENCH_CONTROL_PLANE_ONLY=true
bench-cli serve --control-plane-only
```

In this mode startup does not provision kind/k3d, `/v1/bench/*` and
`/v1/runners/*` stay available, and `POST /v1/certify` returns
`501 Not Implemented`.

## Start

```bash
make build
bench-cli serve \
  --database-url "$BENCH_DATABASE_URL" \
  --bench-api-key "$BENCH_API_KEY"
```

Verify:

```bash
curl http://localhost:8090/healthz
curl http://localhost:8090/v1/bench/runs
```

Run the same public read checks used after hosted deploys:

```bash
BENCH_API_URL=http://localhost:8090 make public-smoke
```

The service runs pending migrations on startup. The migration history is folded
into a single baseline, `001_init.up.sql`, for new bench databases.

If an existing database already ran the older split migration history through
version `014`, deploy an image with the folded baseline first, then reset only
the migration marker:

```sql
UPDATE schema_migrations SET version = 1, dirty = false;
```

Do not rewrite benchmark data during this reset.

## Database Tables

Core tables:

| Table | Purpose |
|---|---|
| `tenants` | phase-0 static tenant, currently seeded as `default` |
| `api_keys` | future key-store auth path |
| `bench_runs` | benchmark run records |
| `bench_artifacts` | transcripts, tool calls, scorecards, timelines |
| `bench_scenarios` | global scenario catalog |
| `bench_models` | global model catalog and provider defaults |
| `bench_tenant_providers` | future tenant-specific model/provider config |
| `bench_infra` | registered runners and execution infrastructure |
| `bench_jobs` | persisted trigger jobs |

## Sync Scenario Catalog

The UI reads scenarios from Postgres. Seed or refresh them from local YAML:

```bash
bench-cli scenario push \
  --bench-url http://localhost:8090 \
  --bench-api-key "$BENCH_API_KEY"
```

## Trigger a Run

With a healthy runner registered:

```bash
curl -X POST http://localhost:8090/v1/bench/trigger \
  -H "Authorization: Bearer $BENCH_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "sonnet",
    "provider": "anthropic",
    "evidence_mode": "mcp",
    "execution_mode": "provider",
    "scenarios": ["broken-deployment"]
  }'
```

Poll the trigger snapshot:

```bash
curl -H "Authorization: Bearer $BENCH_API_KEY" \
  http://localhost:8090/v1/bench/trigger/<job-id>
```

## Register a Runner

```bash
curl -X POST http://localhost:8090/v1/runners/register \
  -H "Authorization: Bearer $BENCH_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "local-runner",
    "models": ["sonnet"],
    "provider": "anthropic",
    "region": "local",
    "max_parallel": 1
  }'
```

Then poll:

```bash
curl -H "Authorization: Bearer $BENCH_API_KEY" \
  "http://localhost:8090/v1/runners/jobs?runner_id=<runner-id>"
```

Complete the job after submitting run records:

```bash
curl -X POST http://localhost:8090/v1/runners/jobs/<job-id>/complete \
  -H "Authorization: Bearer $BENCH_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"runner_id":"<runner-id>","status":"completed","passed":1,"failed":0}'
```

## UI

Build the dashboard:

```bash
make ui-build
```

For local Vite development:

```bash
cd ui
VITE_BENCH_API_URL=http://localhost:8090 \
npm run dev
```

Canonical UI routes are under `/bench/*`. Legacy `/scenarios`, `/runs`, and
`/results` redirect to the bench routes.

## Operational Notes

- Read-only benchmark result, catalog, artifact, analytics, and comparison
  routes are public and use `BENCH_PUBLIC_TENANT`.
- Mutating bench routes, model configuration routes, trigger routes, and runner
  routes require Bearer auth. Do not put `BENCH_API_KEY` into the browser UI;
  use `bench-cli` or a server-side proxy/user-auth flow for authenticated
  actions.
- `POST /v1/bench/trigger` tries the runner queue before direct executor model
  validation, so runner-local aliases can work without a control-plane API key.
- If no eligible runner exists and no direct executor is configured, trigger
  returns `501 Not Implemented`.
- `--control-plane-only` or `BENCH_CONTROL_PLANE_ONLY=true` disables direct
  executor provisioning for production control-plane deployments.
- The runner janitor marks silent runners unhealthy and re-queues stale claimed
  jobs.
