# Bench Service Integration Design

**Date:** 2026-03-25
**Status:** Approved

## Goal

Connect Evidra self-hosted to the private benchmark framework via a
REST service interface. Users trigger certification runs from Evidra
UI, the private bench service executes scenarios, and results flow
back to Evidra's existing bench and evidence pages.

## Architecture

```
Evidra UI          Evidra API              Bench Service (private)
─────────          ──────────              ───────────────────────

"Run" button  →  POST /v1/bench/trigger
                   → calls bench service  →  POST /v1/certify
                   → creates job            { model, scenarios,
                   → returns { id }           evidra_url, api_key }
                                                    ↓
SSE stream    ←  GET /v1/bench/trigger/{id}   Runs scenario 1...
  "1/5 ✓"    ←  ← POST .../progress          → POST /v1/evidence/forward
                                                → POST /v1/bench/runs
  "2/5 ✓"    ←  ← POST .../progress          Runs scenario 2...
                                                → POST /v1/evidence/forward
  "3/5 ..."  ←  ← POST .../progress            → POST /v1/bench/runs
                                                ...
  "5/5 ✓"    ←  ← POST .../complete          Done

redirect to    /bench/runs?model=X            All data already in Evidra
  results      /evidence
               /bench (leaderboard)
```

## New Evidra API Endpoints

### POST /v1/bench/trigger

Starts a remote benchmark run.

```json
Request:
{
  "model": "deepseek-chat",
  "provider": "deepseek",
  "scenarios": ["broken-deployment", "repair-loop-escalation"]
}

Response:
{
  "id": "trigger-01KMH...",
  "status": "pending"
}
```

Internally: calls `EVIDRA_BENCH_SERVICE_URL/v1/certify` with
the request + Evidra's own URL and API key for callbacks.

### GET /v1/bench/trigger/{id}

SSE stream of progress events.

```
event: progress
data: {"scenario":"broken-deployment","status":"running","completed":0,"total":5}

event: progress
data: {"scenario":"broken-deployment","status":"passed","completed":1,"total":5,"run_id":"20260325-..."}

event: progress
data: {"scenario":"repair-loop-escalation","status":"running","completed":1,"total":5}

event: complete
data: {"completed":5,"total":5,"passed":4,"failed":1,"run_ids":["20260325-...","..."]}
```

### POST /v1/bench/trigger/{id}/progress (webhook)

Called by the bench service to update progress.

```json
Request:
{
  "scenario": "broken-deployment",
  "status": "passed",
  "run_id": "20260325-broken-deployment-deepseek"
}
```

Pushes to the SSE stream for that trigger ID.

## Bench Service API (evidra-stand, private)

### POST /v1/certify

```json
Request:
{
  "model": "deepseek-chat",
  "provider": "deepseek",
  "scenarios": ["broken-deployment", "repair-loop-escalation"],
  "callback_url": "https://evidra-api:8080/v1/bench/trigger/trigger-01KMH.../progress",
  "evidra_url": "https://evidra-api:8080",
  "evidra_api_key": "ev1_..."
}
```

The bench service:
1. For each scenario:
   - Seeds the cluster
   - Runs kagent with evidra-mcp configured to forward to `evidra_url`
   - Evidence entries flow to Evidra during execution
   - Verifies the outcome
   - Submits bench run to `evidra_url/v1/bench/runs`
   - Calls `callback_url` with progress
2. On completion, calls callback with `status: complete`

## Evidra UI Changes

### /bench page — add "Run" button

Small addition to BenchLeaderboard or BenchDashboard:
- "Run Certification" button
- Modal: select model, provider, scenarios (checkboxes)
- Submit → POST /v1/bench/trigger
- Progress overlay: scenario list with checkmarks
- On complete → redirect to /bench/runs filtered by run IDs

### No new results pages

All results displayed on existing pages:
- `/bench` — leaderboard updates with new results
- `/bench/runs` — new runs appear, filterable
- `/bench/runs/{id}` — full detail with scorecard, signals, timeline
- `/evidence` — evidence entries from the run

## Configuration

```
EVIDRA_BENCH_SERVICE_URL=https://bench.internal:8090
```

When not set, the "Run" button is hidden. Evidra works without the
bench service — results can still be submitted via API.

## What Stays Private

- Scenario YAML manifests (in evidra-stand)
- Verification check logic
- Cluster management (Kind/k3d)
- Agent execution orchestration
- Bench service deployment

## What's Public (in Evidra)

- Bench run results (submitted via API)
- Evidence entries (forwarded during execution)
- Scorecards and signals (computed by Evidra)
- Leaderboard and comparison views
- Trigger endpoint (thin proxy)

## Microservice Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         USER BROWSER                            │
│                                                                 │
│  /bench          /bench/runs      /evidence       /bench/runs/X │
│  leaderboard     run list         evidence chain  run detail    │
│  + "Run" btn     + filter         + scorecard     + timeline    │
│       │                                                         │
└───────┼─────────────────────────────────────────────────────────┘
        │ SSE progress stream
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                      EVIDRA API (public)                        │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐    │
│  │ Trigger       │  │ Bench        │  │ Evidence           │    │
│  │               │  │              │  │                    │    │
│  │ POST /trigger │  │ POST /runs   │  │ POST /forward      │    │
│  │ GET  /trigger │  │ GET  /runs   │  │ GET  /entries      │    │
│  │   (SSE)       │  │ GET  /stats  │  │ GET  /scorecard    │    │
│  │               │  │ GET  /matrix │  │                    │    │
│  └───────┬───────┘  └──────▲───────┘  └──────▲─────────────┘    │
│          │                 │                 │                   │
│          │            stores runs       stores evidence          │
│          │                 │                 │                   │
│  ┌───────┴─────────────────┴─────────────────┘                  │
│  │ PostgreSQL                                                   │
│  │ bench_runs | bench_artifacts | evidence_entries               │
│  └──────────────────────────────────────────────────────────────┘│
│          │                                                       │
└──────────┼───────────────────────────────────────────────────────┘
           │ POST bench-service/v1/certify
           │ { model, scenarios, callback_url, evidra_url }
           ▼
┌─────────────────────────────────────────────────────────────────┐
│                   BENCH SERVICE (private)                        │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐    │
│  │ Certify API   │  │ Scenario     │  │ Agent Runner       │    │
│  │               │  │ Engine       │  │                    │    │
│  │ POST /certify │  │ seed cluster │  │ kagent + evidra-mcp│    │
│  │               │  │ run checks   │  │ per scenario       │    │
│  └───────┬───────┘  └──────┬───────┘  └──────┬─────────────┘    │
│          │                 │                 │                   │
│          │          Kind/k3d cluster    During execution:        │
│          │                              POST evidra/evidence ────┼──→ Evidra
│          │                              POST evidra/bench/runs ──┼──→ Evidra
│          │                                                       │
│          │──── POST callback_url/progress ───────────────────────┼──→ Evidra
│          │──── POST callback_url/complete ───────────────────────┼──→ Evidra
│                                                                 │
│  ┌──────────────────────────────────────────────────────────────┘│
│  │ Private assets: scenario YAMLs, check logic, cluster mgmt    │
│  └──────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Executor Contract (Versioned Specification)

The executor contract is an open specification. Anyone can implement
it — kagent team, third-party vendors, enterprise platform teams.
Evidra speaks the protocol; the executor runs the scenarios.

### Contract Version: v1.0.0

#### Request: Start a Run

```
POST {executor_url}/v1/certify
Content-Type: application/json

{
  "contract_version": "v1.0.0",
  "job_id": "trigger-01KMH...",
  "model": "deepseek-chat",
  "provider": "deepseek",
  "scenarios": ["broken-deployment", "repair-loop-escalation"],
  "config": {
    "timeout_per_scenario": 300,
    "adapter": "kagent"
  },
  "callback": {
    "progress_url": "https://evidra:8080/v1/bench/trigger/{id}/progress",
    "evidra_url": "https://evidra:8080",
    "evidra_api_key": "ev1_..."
  }
}
```

#### Callback: Progress Update

Executor calls back for each scenario completion:

```
POST {callback.progress_url}
Content-Type: application/json

{
  "contract_version": "v1.0.0",
  "job_id": "trigger-01KMH...",
  "scenario": "broken-deployment",
  "status": "passed",
  "run_id": "20260325-broken-deployment-deepseek",
  "completed": 1,
  "total": 5
}
```

Status values: `running`, `passed`, `failed`, `error`, `skipped`

#### Callback: Completion

```
POST {callback.progress_url}
Content-Type: application/json

{
  "contract_version": "v1.0.0",
  "job_id": "trigger-01KMH...",
  "status": "complete",
  "completed": 5,
  "total": 5,
  "passed": 4,
  "failed": 1,
  "run_ids": ["20260325-broken-deployment-deepseek", "..."]
}
```

#### Data Delivery

During execution, the executor pushes results to Evidra using
standard APIs (no executor-specific endpoints):

| Data | Evidra endpoint | When |
|------|----------------|------|
| Evidence entries | `POST /v1/evidence/forward` | During scenario execution |
| Bench run results | `POST /v1/bench/runs` | After each scenario |
| Scenario metadata | `POST /v1/bench/scenarios/sync` | Before first run (optional) |

These are standard Evidra APIs — the executor authenticates with
`callback.evidra_api_key`.

#### Contract Evolution

- `contract_version` is required on every request/callback
- Evidra validates the version and rejects unsupported versions
- New fields are additive (backward compatible within major version)
- Breaking changes increment the major version

### Third-Party Adoption

The contract is designed for third-party executors:

- **kagent team** builds an executor that benchmarks kagent against
  their own scenarios → results show in Evidra's leaderboard
- **Platform team** builds an executor that runs company-specific
  compliance scenarios → results in the same bench UI
- **Security vendor** builds an executor that tests agent behavior
  against adversarial scenarios → signals and scorecards in Evidra

Evidra is the analytics platform. Executors are the test runners.
The contract is the adoption surface.

### Go Interface

```go
// RunExecutor executes benchmark scenarios and reports results.
type RunExecutor interface {
    // Start begins a benchmark run and returns a job ID.
    Start(ctx context.Context, req RunRequest) (jobID string, err error)
    // Status returns the current progress of a job.
    Status(ctx context.Context, jobID string) (RunStatus, error)
}
```

The bench runner is a pluggable interface, not a specific implementation:

```go
// RunExecutor executes benchmark scenarios and reports results.
type RunExecutor interface {
    // Start begins a benchmark run and returns a job ID.
    Start(ctx context.Context, req RunRequest) (jobID string, err error)
    // Status returns the current progress of a job.
    Status(ctx context.Context, jobID string) (RunStatus, error)
}
```

Two implementations:

### LocalExecutor (default, OSS)

Runs scenarios in the evidra-mcp process. Uses `run_command` to
execute kubectl against the local kubeconfig. Evidence recorded
directly via the lifecycle service. No external dependencies.

Best for: single-machine setups, CI pipelines, OSS users.

### RemoteExecutor (configured)

Delegates to an external bench service via REST API. The service
runs scenarios on dedicated clusters with full isolation. Evidence
and bench runs flow back to Evidra via existing APIs.

Best for: SaaS deployments, enterprise, multi-tenant, dedicated
test clusters.

Activated by setting `EVIDRA_BENCH_SERVICE_URL`. When not set,
falls back to LocalExecutor.

```
POST /v1/bench/trigger { model, scenarios }
            ↓
    EVIDRA_BENCH_SERVICE_URL set?
        ├── yes → RemoteExecutor → POST bench-service/v1/certify
        └── no  → LocalExecutor  → run scenarios in-process
            ↓
    Same SSE progress stream
    Same bench results pages
    Same evidence viewer
```

## Architecture Principles

1. **Clean boundary** — Bench service is a black box. Evidra only
   knows the trigger/callback contract. Scenario logic stays private.

2. **Evidra is the system of record** — all runs, evidence, and
   scorecards are stored in Evidra. The bench service is stateless
   (fire and forget).

3. **Existing APIs only** — bench runs use `POST /v1/bench/runs`,
   evidence uses `POST /v1/evidence/forward`. No new storage.
   Only new surface: trigger + SSE progress.

4. **Graceful degradation** — when `EVIDRA_BENCH_SERVICE_URL` is
   not set, the "Run" button is hidden. All bench features still
   work with manually submitted results.

5. **Multi-tenant ready** — trigger passes the tenant's API key to
   the bench service. Results are scoped to that tenant.

## Implementation Order

1. Bench service: `POST /v1/certify` endpoint in evidra-stand
2. Evidra API: `POST /v1/bench/trigger` + webhook + SSE
3. Evidra UI: "Run" button + progress overlay
4. Architecture docs: update ARCHITECTURE.md + diagrams
5. Wire: EVIDRA_BENCH_SERVICE_URL config
6. Test end-to-end
