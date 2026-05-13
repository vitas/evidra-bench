---
title: Bench Runner Control Plane Contract v1
type: contract
status: active
tags:
  - bench
  - runners
  - api
---

# Bench Runner Control Plane Contract v1

Poll-based runners let a managed or self-hosted bench service enqueue jobs in
Postgres while execution happens on external machines or clusters.

## Scope

This contract covers:

- runner registration in `bench_infra`
- persisted job queue in `bench_jobs`
- polling, claiming, heartbeat, and completion routes under `/v1/runners/*`
- compatibility progress updates under `/v1/bench/trigger/{id}/progress`

It does not replace the direct executor contract. See
[Executor Contract v1.0.0](EXECUTOR_CONTRACT_V1.md).

## Lifecycle

1. Runner registers capabilities with `POST /v1/runners/register`.
2. User or UI submits `POST /v1/bench/trigger`.
3. The control plane finds a healthy runner for the requested model and inserts
   a queued `bench_jobs` row.
4. Runner polls `GET /v1/runners/jobs?runner_id=...`.
5. Runner executes assigned scenarios.
6. Runner optionally sends per-scenario progress to
   `POST /v1/bench/trigger/{id}/progress`.
7. Runner sends final status to `POST /v1/runners/jobs/{id}/complete`.

## Trigger Enqueue

### POST /v1/bench/trigger

Request:

```json
{
  "model": "sonnet",
  "provider": "anthropic",
  "execution_mode": "provider",
  "runner_id": "01K...",
  "mcp_server": "npx -y @vendor/kubernetes-mcp --stdio",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
  "skill_file": "/tmp/bench-skills/k8s-admin.md",
  "skill_id": "k8s-admin",
  "skill_version": "2026-05-13",
  "skill_source": "local-temp",
  "skill_sha256": "abc123",
  "scenarios": ["broken-deployment", "repair-loop-escalation"]
}
```

Rules:

- `model` and `scenarios` are required.
- `execution_mode` is optional and must be `provider` or `a2a`; omitted means
  `provider`.
- `provider` may be supplied by the caller, inherited from the runner config,
  or resolved from the global model catalog.
- `runner_id` is optional. When present, the job is pinned to that runner.
- `mcp_server` is the executable command a runner should start for MCP tool
  execution.
- `tool_server` and `tool_server_version` are stable comparison labels stored
  on jobs and run records.
- `skill_file` is a local path on the runner host. The control plane does not
  download arbitrary remote skill URLs.
- `skill_id`, `skill_version`, `skill_source`, and `skill_sha256` are stable
  skill comparison and reproducibility labels.
- Pinned runner requests fail unless the runner is healthy and advertises the
  requested model.
- If no healthy runner is available and no direct executor is configured, the
  service returns `501 Not Implemented`.

Runner-mode response:

```json
{
  "id": "01K...",
  "status": "pending",
  "mode": "runner"
}
```

## Runner Registration

### POST /v1/runners/register

Request:

```json
{
  "name": "local-runner",
  "models": ["sonnet", "gemini-2.5-flash"],
  "provider": "anthropic",
  "region": "local",
  "max_parallel": 1,
  "labels": {
    "cluster": "kind-local"
  }
}
```

Response:

```json
{
  "runner_id": "01K...",
  "poll_interval": 5
}
```

The runner must retain `runner_id` and include it in every poll and completion
request.

### GET /v1/runners

Lists runners for the authenticated tenant.

```json
{
  "runners": [
    {
      "id": "01K...",
      "tenant_id": "default",
      "name": "local-runner",
      "region": "local",
      "status": "healthy",
      "config": {
        "models": ["sonnet", "gemini-2.5-flash"],
        "provider": "anthropic",
        "max_parallel": 1,
        "poll_interval": 5,
        "labels": { "cluster": "kind-local" }
      },
      "created_at": "2026-04-30T12:00:00Z",
      "updated_at": "2026-04-30T12:00:05Z"
    }
  ]
}
```

### DELETE /v1/runners/{id}

Deletes a runner registration. Response: `204 No Content`.

## Poll and Claim

### GET /v1/runners/jobs?runner_id={runner_id}

Polling is also the runner heartbeat.

Semantics:

- only healthy runners can poll successfully
- `TouchRunner` updates heartbeat state
- the claim query uses `FOR UPDATE SKIP LOCKED`
- pinned jobs can only be claimed by their pinned runner
- claimed jobs are assigned to the runner through `bench_jobs.infra_id`
- if no matching job is available, response is `204 No Content`

Response when a job is claimed:

```json
{
  "job_id": "01K...",
  "model": "sonnet",
  "provider": "anthropic",
  "execution_mode": "provider",
  "mcp_server": "npx -y @vendor/kubernetes-mcp --stdio",
  "tool_server": "kubernetes-mcp",
  "tool_server_version": "1.2.3",
  "skill_file": "/tmp/bench-skills/k8s-admin.md",
  "skill_id": "k8s-admin",
  "skill_version": "2026-05-13",
  "skill_source": "local-temp",
  "skill_sha256": "abc123",
  "scenarios": ["broken-deployment", "repair-loop-escalation"],
  "timeout": 300
}
```

Errors:

| Status | Meaning |
|---|---|
| `400` | missing `runner_id` |
| `404` | runner not found, unhealthy, or draining |
| `500` | queue or database failure |

## Progress

### POST /v1/bench/trigger/{id}/progress

Compatibility progress keeps the existing trigger polling/SSE API live while a
runner is executing a persisted job.

```json
{
  "contract_version": "v1.0.0",
  "scenario": "broken-deployment",
  "status": "running",
  "completed": 0,
  "total": 2
}
```

Progress updates also refresh `bench_jobs.last_progress_at`.

## Completion

### POST /v1/runners/jobs/{id}/complete

Request:

```json
{
  "runner_id": "01K...",
  "status": "completed",
  "passed": 2,
  "failed": 0,
  "message": ""
}
```

Rules:

- `status` must be `completed` or `failed`
- `runner_id` is required
- the completing runner must still own the job (`bench_jobs.infra_id`)
- completion updates the persisted job and the in-memory trigger snapshot used
  by `/v1/bench/trigger/{id}`

Response: `204 No Content`.

## Staleness and Janitor

`bench-cli serve` starts the runner janitor. It:

- marks silent healthy runners as unhealthy
- re-queues claimed jobs whose `last_progress_at` or `started_at` is stale
- prevents unhealthy runners from polling until they re-register

Runners should send progress during long jobs. If a runner only reports final
completion, the stale threshold must be longer than the longest expected job.
