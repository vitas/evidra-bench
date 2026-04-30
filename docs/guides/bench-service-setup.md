# Bench Service Setup

This guide covers running the private bench service from this repo. It is the
first refactor phase: no managed cloud dependency is required.

## What Runs Here

`bench-cli serve` now owns:

- benchmark result APIs under `/v1/bench/*`
- trigger polling under `/v1/bench/trigger/*`
- poll-based runner control plane under `/v1/runners/*`
- the direct executor endpoint `POST /v1/certify`
- Postgres migrations for bench tables
- the local orchestration cluster used by `POST /v1/certify`

`../evidra` can continue to own evidence runtime, CLI/MCP protocol, and scoring
while this repo carries the benchmark API surface.

## Requirements

- Go 1.25+
- PostgreSQL
- `kind`, `kubectl`, and `helm` for local execution
- an API key value for Bearer auth

## Environment

```bash
export BENCH_DATABASE_URL=postgres://bench:bench@localhost:5432/bench?sslmode=disable
export EVIDRA_API_KEY=dev-secret
export BENCH_SERVICE_ADDR=:8090
```

Equivalent flags:

```bash
bench-cli serve \
  --database-url "$BENCH_DATABASE_URL" \
  --evidra-api-key "$EVIDRA_API_KEY"
```

`BENCH_DATABASE_URL` is also used by parallel bench execution. `EVIDRA_API_KEY`
is the static Bearer token for authenticated HTTP routes.

## Start

```bash
make build
bench-cli serve \
  --database-url "$BENCH_DATABASE_URL" \
  --evidra-api-key "$EVIDRA_API_KEY"
```

Verify:

```bash
curl http://localhost:8090/healthz
curl -H "Authorization: Bearer $EVIDRA_API_KEY" \
  http://localhost:8090/v1/bench/runs
```

The service runs pending migrations on startup. Bootstrap migrations are written
to tolerate an existing public bench schema when the tables or columns already
exist.

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
  --evidra-url http://localhost:8090 \
  --evidra-api-key "$EVIDRA_API_KEY"
```

## Trigger a Run

With a healthy runner registered:

```bash
curl -X POST http://localhost:8090/v1/bench/trigger \
  -H "Authorization: Bearer $EVIDRA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "sonnet",
    "provider": "anthropic",
    "evidence_mode": "smart",
    "execution_mode": "provider",
    "scenarios": ["broken-deployment"]
  }'
```

Poll the trigger snapshot:

```bash
curl -H "Authorization: Bearer $EVIDRA_API_KEY" \
  http://localhost:8090/v1/bench/trigger/<job-id>
```

## Register a Runner

```bash
curl -X POST http://localhost:8090/v1/runners/register \
  -H "Authorization: Bearer $EVIDRA_API_KEY" \
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
curl -H "Authorization: Bearer $EVIDRA_API_KEY" \
  "http://localhost:8090/v1/runners/jobs?runner_id=<runner-id>"
```

Complete the job after submitting run records:

```bash
curl -X POST http://localhost:8090/v1/runners/jobs/<job-id>/complete \
  -H "Authorization: Bearer $EVIDRA_API_KEY" \
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
VITE_EVIDRA_API_URL=http://localhost:8090 \
VITE_EVIDRA_API_KEY="$EVIDRA_API_KEY" \
npm run dev
```

Canonical UI routes are under `/bench/*`. Legacy `/scenarios`, `/runs`, and
`/results` redirect to the bench routes.

## Operational Notes

- `GET /v1/bench/leaderboard` is public.
- All other bench, trigger, and runner routes require Bearer auth.
- `POST /v1/bench/trigger` tries the runner queue before direct executor model
  validation, so runner-local aliases can work without a control-plane API key.
- If no eligible runner exists and no direct executor is configured, trigger
  returns `501 Not Implemented`.
- The runner janitor marks silent runners unhealthy and re-queues stale claimed
  jobs.
