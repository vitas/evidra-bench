# Bench API Reference

`bench-cli serve` exposes the local/private bench control plane used by the
dashboard and remote runners. The service owns benchmark results, scenario
metadata, trigger jobs, and runner registration.

```bash
BENCH_DATABASE_URL=postgres://bench:bench@localhost:5432/bench?sslmode=disable \
BENCH_API_KEY=dev-secret \
BENCH_PUBLIC_TENANT=default \
BENCH_SERVICE_ADDR=:8090 \
bench-cli serve
```

For hosted control-plane deployments backed by remote runners, start with
`BENCH_CONTROL_PLANE_ONLY=true` or `--control-plane-only`. In that mode the
service does not provision a local executor cluster and `POST /v1/certify`
returns `501 Not Implemented`.

Authentication uses `Authorization: Bearer $BENCH_API_KEY` for mutating routes,
trigger routes, runner routes, and model-provider configuration. Read-only
benchmark result, catalog, artifact, analytics, and comparison routes are
public and read from `BENCH_PUBLIC_TENANT`. If `BENCH_PUBLIC_TENANT` is omitted,
`bench-cli serve` uses the authenticated tenant, which defaults to `default`.

Static-key auth maps authenticated requests to `BENCH_DEFAULT_TENANT` in this
phase. `GET /healthz` is always public.

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

MCP runs can also carry `mcp_server`, `tool_server`, and
`tool_server_version`. `mcp_server` is the executable command for the runner;
`tool_server` and `tool_server_version` are stable labels used for filtering,
comparison, and private reports.

## Public Read Endpoints

### GET /v1/bench/leaderboard

Model ranking by pass rate and pass^k reliability.

Query parameters:

| Name | Description |
|---|---|
| `evidence_mode` | filter using the evidence-mode contract above |
| `k` | pass^k trial count, 1-10, default `3` |
| `scenarios` | comma-separated scenario IDs for suite or category slices |

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

Returns the global scenario catalog. Public read endpoint.

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
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
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
| `tool_server` | exact MCP tool-server identity filter |
| `tool_server_version` | exact MCP tool-server version filter |
| `scenario` | exact scenario ID filter |
| `scenarios` | comma-separated scenario IDs; ignored when `scenario` is set |
| `evidence_mode` | filter using the evidence-mode contract above |
| `since` | RFC3339 timestamp or `YYYY-MM-DD` |
| `passed` | `true` or `false` |
| `limit` | page size |
| `offset` | page offset |
| `sort_by` | `created_at`, `duration_seconds`, `estimated_cost_usd`, `scenario_id`, `model`, `provider`, `tool_server`, `tool_server_version`, `checks_passed`, `turns`, or `passed` |
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

Returns failure autopsy artifact JSON when the run has one. Current generated
artifacts use `version: "autopsy.v1"` and include a deterministic `confidence`
value. Older artifacts may omit `version`; clients should treat those as legacy
v0 reports and continue rendering the common fields.

## Analytics

### GET /v1/bench/stats

Aggregate run counts and pass/fail breakdown. Accepts the same filters as
`GET /v1/bench/runs`.

### GET /v1/bench/catalog

Distinct models, providers, MCP tool servers, and MCP tool-server versions
observed in stored runs.

```json
{
  "models": ["sonnet"],
  "providers": ["anthropic"],
  "tool_servers": ["kubernetes-mcp", "legacy-mcp"],
  "tool_server_versions": ["1.2.3"]
}
```

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

### GET /v1/bench/compare/tool-server

Compares no-MCP/native-tools baseline runs against runs for one selected MCP
tool server. Use this for private MCP readiness reports and release regression
reports where the model and scenario slice stay fixed and only the tool server
changes.

Query parameters:

| Name | Description |
|---|---|
| `model` | required exact model ID |
| `tool_server` | required exact MCP tool-server identity |
| `tool_server_version` | optional exact MCP tool-server version |
| `scenario` | optional exact scenario ID |
| `scenarios` | optional comma-separated scenario IDs; ignored when `scenario` is set |

Response:

```json
{
  "model": "sonnet",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
  "scenario_ids": ["broken-deployment", "stuck-rollout"],
  "baseline": {
    "runs": 2,
    "passed": 1,
    "pass_rate": 50,
    "avg_turns": 7,
    "avg_tokens": 850,
    "avg_cost_usd": 0.08,
    "avg_duration_seconds": 35.5
  },
  "candidate": {
    "runs": 2,
    "passed": 2,
    "pass_rate": 100,
    "avg_turns": 5,
    "avg_tokens": 620,
    "avg_cost_usd": 0.05,
    "avg_duration_seconds": 28
  },
  "delta": {
    "pass_rate_delta": 50,
    "avg_turns_delta": -2,
    "avg_tokens_delta": -230,
    "avg_cost_usd_delta": -0.03,
    "avg_duration_seconds_delta": -7.5
  },
  "scenarios": [
    {
      "scenario_id": "broken-deployment",
      "baseline": { "runs": 1, "passed": 0, "pass_rate": 0 },
      "candidate": { "runs": 1, "passed": 1, "pass_rate": 100 },
      "delta": { "pass_rate_delta": 100 }
    }
  ],
  "improved_scenarios": [
    {
      "scenario_id": "broken-deployment",
      "baseline": { "runs": 1, "passed": 0, "pass_rate": 0 },
      "candidate": { "runs": 1, "passed": 1, "pass_rate": 100 },
      "delta": { "pass_rate_delta": 100 }
    }
  ],
  "regressed_scenarios": []
}
```

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
  "mcp_server": "npx -y @vendor/kubernetes-mcp --stdio",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
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
  "mcp_server": "npx -y @vendor/kubernetes-mcp --stdio",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
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
