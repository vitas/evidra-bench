# Bench Executor Contract v1.0.0

This contract defines the direct executor API. A direct executor runs
benchmark scenarios against real infrastructure and reports results back to the
bench control plane.

In this repo, `bench-cli serve` implements the executor side of the contract via
`POST /v1/certify`. The dashboard-facing `POST /v1/bench/trigger` route can call
a direct executor when one is configured, or enqueue work for poll-based runners.

Poll-based runner registration, queue claim, and completion are documented in
[Bench Runner Control Plane Contract v1](BENCH_RUNNER_CONTROL_PLANE_V1.md).

## Versioning

All executor requests and callbacks use:

```json
{ "contract_version": "v1.0.0" }
```

New fields are additive within a major version. Breaking changes require a new
major version.

## Executor Endpoint

### POST /v1/certify

Start a benchmark run. The executor returns immediately and runs scenarios
asynchronously.

Request:

```json
{
  "contract_version": "v1.0.0",
  "job_id": "trigger-01KMH...",
  "model": "sonnet",
  "provider": "anthropic",
  "scenarios": ["broken-deployment", "repair-loop-escalation"],
  "config": {
    "timeout_per_scenario": 300,
    "adapter": "a2a",
    "a2a_agent_url": "http://agent:8080",
    "evidence_mode": "smart"
  },
  "callback": {
    "progress_url": "http://bench:8090/v1/bench/trigger/trigger-01KMH.../progress",
    "evidra_url": "http://bench:8090",
    "evidra_api_key": "dev-secret"
  }
}
```

Fields:

| Field | Required | Description |
|---|---|---|
| `contract_version` | yes | Must be `v1.0.0` |
| `job_id` | yes | Job ID assigned by the caller |
| `model` | yes | Model name or runner-local alias |
| `provider` | no | Provider label |
| `scenarios` | yes | Scenario IDs to run |
| `config.timeout_per_scenario` | no | Timeout per scenario in seconds |
| `config.adapter` | no | `provider`, `a2a`, `cli`, or `mcp` depending on executor support |
| `config.a2a_agent_url` | no | A2A endpoint when `adapter=a2a` |
| `config.evidence_mode` | no | `none` or `smart`; request value overrides worker default |
| `callback.progress_url` | yes | Trigger progress webhook |
| `callback.evidra_url` | yes | API base URL for run/artifact delivery |
| `callback.evidra_api_key` | yes | Bearer token for API auth |

Response:

```json
{
  "job_id": "trigger-01KMH...",
  "status": "accepted"
}
```

Status: `202 Accepted`.

### GET /healthz

Health check. Returns `200 OK` with `{"status":"ok"}`.

## Progress Callback

Executors send progress to `callback.progress_url`.

```http
POST /v1/bench/trigger/{id}/progress
Authorization: Bearer <api-key>
Content-Type: application/json
```

```json
{
  "contract_version": "v1.0.0",
  "job_id": "trigger-01KMH...",
  "scenario": "broken-deployment",
  "status": "passed",
  "run_id": "20260430-broken-deployment-sonnet",
  "completed": 1,
  "total": 2
}
```

Fields:

| Field | Required | Description |
|---|---|---|
| `contract_version` | yes | Must be `v1.0.0` |
| `job_id` | yes | Job ID from the original request |
| `scenario` | yes | Current scenario ID |
| `status` | yes | `running`, `passed`, `failed`, `error`, or `skipped` |
| `run_id` | no | Bench run ID after a scenario completes |
| `completed` | yes | Completed scenario count |
| `total` | yes | Total scenario count |

Send `running` before starting a scenario. Send `passed`, `failed`, `error`, or
`skipped` after a scenario finishes. The final update is where
`completed == total`.

## Data Delivery

The executor pushes run results and artifacts to the API identified by
`callback.evidra_url`. Authentication is `Bearer {callback.evidra_api_key}`.

### Run Result

```http
POST /v1/bench/runs
Authorization: Bearer <api-key>
Content-Type: application/json
```

```json
{
  "id": "20260430-broken-deployment-sonnet",
  "scenario_id": "broken-deployment",
  "model": "sonnet",
  "provider": "anthropic",
  "adapter": "a2a",
  "evidence_mode": "smart",
  "passed": true,
  "duration_seconds": 35.2,
  "exit_code": 0,
  "turns": 8,
  "checks_passed": 3,
  "checks_total": 3,
  "transcript": "optional text",
  "tool_calls": []
}
```

The `id` must be unique. Recommended format:
`{timestamp}-{scenario}-{model}`.

### Scenario Metadata

Executors may sync scenario metadata before runs:

```http
POST /v1/bench/scenarios/sync
Authorization: Bearer <api-key>
Content-Type: application/json
```

```json
{
  "scenarios": [
    {
      "id": "broken-deployment",
      "title": "Broken Deployment",
      "category": "kubernetes",
      "tags": ["deployment", "image"]
    }
  ]
}
```

## Execution Requirements

For each scenario, an executor must:

1. prepare the target environment
2. inject or seed the scenario failure
3. launch the agent with the requested model and execution mode
4. configure any evidence tooling to forward to `callback.evidra_url`
5. wait for completion or timeout
6. verify the outcome
7. submit the bench run result
8. send a progress callback

## Error Handling

- if a scenario fails to start, send `status: "error"`
- if a scenario times out, send `status: "failed"`
- if a job is already running and the executor is single-tenant/single-worker,
  return `409 Conflict`
- if the executor process crashes, the control plane or runner janitor is
  responsible for detecting stale state

## Reference Implementation

`bench-cli serve` is the reference direct executor:

```bash
BENCH_DATABASE_URL=postgres://bench:bench@localhost:5432/bench?sslmode=disable \
EVIDRA_API_KEY=dev-secret \
BENCH_SERVICE_ADDR=:8090 \
bench-cli serve
```

`--database-url` or `BENCH_DATABASE_URL` is required. `--evidra-api-key` or
`EVIDRA_API_KEY` is required for API auth.

Hosted control-plane deployments can start `bench-cli serve --control-plane-only`
or set `BENCH_CONTROL_PLANE_ONLY=true`. That mode intentionally disables this
direct executor endpoint and returns `501 Not Implemented` for `POST /v1/certify`.
