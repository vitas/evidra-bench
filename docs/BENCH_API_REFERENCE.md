# Bench API Reference

`bench-cli serve` exposes the local/private bench control plane used by the
dashboard and remote runners. The service owns benchmark results, scenario
metadata, trigger jobs, and runner registration.

```bash
BENCH_DATABASE_URL=postgres://bench:bench@localhost:5432/bench?sslmode=disable \
BENCH_API_KEY=dev-secret \
BENCH_SERVICE_ADDR=:8090 \
bench-cli serve
```

For hosted control-plane deployments backed by remote runners, start with
`BENCH_CONTROL_PLANE_ONLY=true` or `--control-plane-only`. In that mode the
service does not provision a local executor cluster and `POST /v1/certify`
returns `501 Not Implemented`.

Authentication uses `Authorization: Bearer $BENCH_API_KEY` for every
authenticated route. `GET /v1/bench/leaderboard` and `GET /healthz` are public.
Static-key auth maps all authenticated requests to tenant `default` in this
phase.

## Health

### GET /healthz

Returns `200 OK` with `{"status":"ok"}` when the HTTP process is running.

## Filters

Bench list and analytics endpoints accept exact evidence-mode filters:

| Query value | Meaning |
|---|---|
| empty | all runs |
| `none` | baseline runs only |
| `mcp` | runs that used an MCP server |

`POST /v1/bench/trigger` accepts only `none` or `mcp`.

## Public Endpoints

### GET /v1/bench/leaderboard

Model ranking by pass rate and pass^k reliability.

Query parameters:

| Name | Description |
|---|---|
| `evidence_mode` | filter using the evidence-mode contract above |
| `k` | pass^k trial count, 1-10, default `3` |

Response:

```json
{
  "models": [
    {
      "model": "claude-sonnet-4",
      "scenarios": 33,
      "runs": 40,
      "pass_rate": 97.5,
      "avg_duration": 72.0,
      "avg_cost": 0.24,
      "total_cost": 8.07,
      "pass_k": 85.2,
      "pass_k_trials": 3,
      "sufficient_scenarios": 28
    }
  ],
  "evidence_mode": ""
}
```

## Runs and Catalog

### GET /v1/bench/scenarios

Returns the global scenario catalog. Requires Bearer auth.

### POST /v1/bench/scenarios/sync

Upserts scenario metadata. Used by `bench-cli scenario push`.

```json
{
  "scenarios": [
    {
      "id": "broken-deployment",
      "title": "Broken Deployment",
      "category": "kubernetes",
      "tags": ["deployment", "image"],
      "chaos": false
    }
  ]
}
```

Response:

```json
{ "ok": true, "upserted": 75, "total": 75 }
```

### POST /v1/bench/runs

Submits one benchmark run. The body is a `pkg/bench.RunRecord` plus optional
artifact fields:

```json
{
  "id": "20260430-broken-deployment-sonnet",
  "scenario_id": "broken-deployment",
  "model": "sonnet",
  "provider": "anthropic",
  "adapter": "a2a",
  "evidence_mode": "mcp",
  "passed": true,
  "duration_seconds": 35.2,
  "exit_code": 0,
  "turns": 8,
  "checks_passed": 3,
  "checks_total": 3,
  "transcript": "optional text transcript",
  "tool_calls": [],
  "autopsy": { "outcome": "pass", "primary_failure": "" }
}
```

### POST /v1/bench/runs/batch

Batch submits runs. Body: `{"runs":[...]}`. Duplicate run IDs are ignored.

### GET /v1/bench/runs

Lists runs with pagination and filters.

Query parameters:

| Name | Description |
|---|---|
| `model` | exact model filter |
| `scenario` | exact scenario ID filter |
| `evidence_mode` | filter using the evidence-mode contract above |
| `since` | RFC3339 timestamp or `YYYY-MM-DD` |
| `passed` | `true` or `false` |
| `limit` | page size |
| `offset` | page offset |
| `sort_by` | `created_at`, `duration_seconds`, `estimated_cost_usd`, `scenario_id`, `model`, `provider`, `checks_passed`, `turns`, or `passed` |
| `sort_order` | `asc` or `desc` |

### GET /v1/bench/runs/{id}

Returns a single run detail.

### DELETE /v1/bench/runs/{id}

Deletes a run and cascades artifacts. Returns `204 No Content`.

### POST /v1/bench/runs/archive

Soft-archives runs matching a filter.

```json
{
  "before": "2026-04-01T00:00:00Z",
  "ids": ["run-1", "run-2"],
  "model": "sonnet"
}
```

Response:

```json
{ "archived": 12 }
```

### GET /v1/bench/runs/{id}/transcript

Returns transcript artifact as `text/plain`.

### GET /v1/bench/runs/{id}/tool-calls

Returns tool-call artifact JSON.

### GET /v1/bench/runs/{id}/timeline

Returns the decision timeline derived from stored artifacts.

### GET /v1/bench/runs/{id}/scorecard

Returns scorecard artifact JSON.

### GET /v1/bench/runs/{id}/autopsy

Returns failure autopsy artifact JSON when the run has one.

## Analytics

### GET /v1/bench/stats

Aggregate run counts and pass/fail breakdown. Accepts the same filters as
`GET /v1/bench/runs`.

### GET /v1/bench/catalog

Distinct models and providers observed in stored runs.

### GET /v1/bench/models

Lists models available to the authenticated tenant. A model is returned when the
platform has configured an `api_key_env` or the tenant has an enabled provider
override.

Response:

```json
{
  "models": [
    {
      "id": "gemini-2.5-flash",
      "display_name": "Gemini 2.5 Flash",
      "provider": "google",
      "api_base_url": "https://generativelanguage.googleapis.com/v1beta/openai",
      "available": true,
      "input_cost_per_mtok": 0.15,
      "output_cost_per_mtok": 0.6
    }
  ]
}
```

Per-tenant provider write routes exist in code but are intentionally not
registered until encrypted API-key storage is added.

### GET /v1/bench/compare/runs

Compares two runs. Requires `a` and `b` query parameters.

### GET /v1/bench/compare/models

Compares model performance. Pairwise mode uses `a` and `b`; matrix mode uses
comma-separated `models` and optional comma-separated `scenarios`.

### GET /v1/bench/signals

Aggregates behavioral signals parsed from scorecard artifacts.

### GET /v1/bench/regressions

Finds scenario/model pairs where the latest run failed after previous passes.

### GET /v1/bench/insights

Failure analysis for a scenario. Requires `?scenario=<id>`.

## Trigger API

The trigger API is the dashboard-facing run surface. It supports two execution
paths:

- runner queue: if a healthy runner advertises the requested model, the job is
  enqueued in Postgres and claimed via `/v1/runners/jobs`
- direct executor: if a `RunExecutor` is configured, the service can start a
  direct executor; control-plane-only mode returns `501` when no runner is
  eligible

See [Executor Contract v1.0.0](contracts/EXECUTOR_CONTRACT_V1.md) and
[Bench Runner Control Plane Contract v1](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md).

### POST /v1/bench/trigger

Starts a benchmark run. Requires `model`, `scenarios`, and `evidence_mode`.

```json
{
  "model": "sonnet",
  "provider": "anthropic",
  "execution_mode": "provider",
  "evidence_mode": "mcp",
  "runner_id": "01K...",
  "scenarios": ["broken-deployment"]
}
```

Response when queued for a runner:

```json
{ "id": "01K...", "status": "pending", "mode": "runner" }
```

Errors:

| Status | Meaning |
|---|---|
| `400` | invalid request or unavailable pinned runner |
| `401` | missing/invalid Bearer token |
| `501` | no eligible runner and no direct executor configured |

### GET /v1/bench/trigger/{id}

Returns the in-memory trigger snapshot. Supports SSE when
`Accept: text/event-stream` is set.

```json
{
  "id": "01K...",
  "status": "running",
  "model": "sonnet",
  "provider": "anthropic",
  "evidence_mode": "mcp",
  "execution_mode": "provider",
  "completed": 1,
  "passed": 1,
  "failed": 0,
  "total": 2,
  "current_scenario": "repair-loop-escalation",
  "run_ids": ["20260430-broken-deployment-sonnet"],
  "progress": [
    { "scenario": "broken-deployment", "status": "passed", "run_id": "20260430-broken-deployment-sonnet" },
    { "scenario": "repair-loop-escalation", "status": "running" }
  ]
}
```

### POST /v1/bench/trigger/{id}/progress

Progress webhook used by executors and runner bridges.

```json
{
  "contract_version": "v1.0.0",
  "scenario": "broken-deployment",
  "status": "passed",
  "run_id": "20260430-broken-deployment-sonnet",
  "completed": 1,
  "total": 2
}
```

Response: `200 OK`.

## Runner Control Plane

### POST /v1/runners/register

Registers a poll-based runner and advertises model capabilities.

```json
{
  "name": "local-runner",
  "models": ["sonnet", "gemini-2.5-flash"],
  "provider": "anthropic",
  "region": "local",
  "max_parallel": 1,
  "labels": { "cluster": "kind-local" }
}
```

Response:

```json
{ "runner_id": "01K...", "poll_interval": 5 }
```

### GET /v1/runners

Lists registered runners for tenant `default`.

### DELETE /v1/runners/{id}

Deletes a runner registration. Response: `204 No Content`.

### GET /v1/runners/jobs

Runner poll and heartbeat endpoint. Requires `runner_id`.

Response when a job is available:

```json
{
  "job_id": "01K...",
  "model": "sonnet",
  "provider": "anthropic",
  "evidence_mode": "mcp",
  "execution_mode": "provider",
  "scenarios": ["broken-deployment"],
  "timeout": 300
}
```

Response when no job is available: `204 No Content`.

### POST /v1/runners/jobs/{id}/complete

Marks a claimed job as complete. The runner must still own the job.

```json
{
  "runner_id": "01K...",
  "status": "completed",
  "passed": 1,
  "failed": 0,
  "message": ""
}
```

Response: `204 No Content`.

## Error Format

Errors return JSON:

```json
{ "error": "human-readable message" }
```
