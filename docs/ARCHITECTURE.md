# Architecture

## Overview

evidra-infra-bench is a Go harness that runs AI agents against real Kubernetes
clusters to measure infrastructure remediation skills. It provisions clusters,
injects faults, lets agents fix them, and verifies outcomes.

## Control Plane Boundary

This repo owns scenario execution, workspace isolation, namespace isolation,
verification, and result reporting. It does not own the multi-tenant bench job
contracts exposed by the hosted Evidra API.

| Surface | Owner | Purpose |
|---------|-------|---------|
| `bench-cli run`, `bench-cli bench`, `bench-cli serve` | `evidra-infra-bench` | Run scenarios directly or through the local River-backed orchestrator |
| `POST /v1/certify` | `evidra-infra-bench` | Local standalone enqueue API for the orchestrator; request-level `evidence_mode` overrides the worker default |
| `POST /v1/bench/trigger`, `GET /v1/bench/trigger/{id}`, `POST /v1/bench/trigger/{id}/progress`, `/v1/runners/*` | sibling `evidra` repo | Hosted trigger, persisted job queue, runner registration, progress, and completion |

When this harness is used inside the hosted Evidra stack, it participates in
one of two control-plane modes:

- direct executor mode: Evidra accepts `POST /v1/bench/trigger` and invokes an
  executor implementation that runs scenarios immediately; hosted trigger
  requests only accept the coarse `none|smart` evidence modes
- poll-based runner mode: Evidra persists the job, a registered runner claims
  it through `GET /v1/runners/jobs`, and the runner completes it through
  `POST /v1/runners/jobs/{id}/complete`; claimed jobs include `evidence_mode`

The authoritative HTTP contract for those hosted surfaces lives in the sibling
`evidra` repo:

- `../evidra/docs/contracts/EXECUTOR_CONTRACT_V1.md`
- `../evidra/docs/contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md`
- `../evidra/docs/api-reference.md`

## Module Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│  Entry Points                                                       │
│                                                                     │
│  cmd/bench-cli/main.go     CLI: run, bench, certify, lab, serve    │
│  cmd/bench-cli/serve.go    REST API: POST /v1/certify              │
└───────────┬─────────────────────────────┬───────────────────────────┘
            │ sequential                  │ parallel (--parallel N)
            ▼                             ▼
┌───────────────────────┐   ┌─────────────────────────────────────────┐
│  Direct Execution     │   │  pkg/orchestrator                       │
│                       │   │                                         │
│  runScenarioOnce()    │   │  Orchestrator.Provision()  ← cluster   │
│    ↓                  │   │  Orchestrator.RunParallel() ← River    │
│  harness.Run()        │   │  Orchestrator.Teardown()   ← cleanup   │
│                       │   └──────────┬──────────────────────────────┘
└───────────┬───────────┘              │
            │                          ▼
            │              ┌───────────────────────┐
            │              │  pkg/jobqueue          │
            │              │                        │
            │              │  River Client          │
            │              │  BenchWorker           │
            │              │  BenchJobArgs          │
            │              │    ↓                   │
            │              │  PostgreSQL job queue   │
            │              └──────────┬────────────┘
            │                         │ per job
            │                         ▼
            │              ┌───────────────────────┐
            │              │  pkg/workspace         │
            │              │                        │
            │              │  New() → copy scenarios │
            │              │  RewriteNamespace()    │
            │              │  Cleanup()             │
            │              └──────────┬────────────┘
            │                         │
            ▼                         ▼
┌─────────────────────────────────────────────────────────────────────┐
│  pkg/harness                                                        │
│                                                                     │
│  Harness.Run(RunRequest)                                           │
│    1. Provision cluster (or skip if KubeconfigPath set)            │
│    2. Create namespace                                              │
│    3. Bootstrap (baseline manifests, ArgoCD, addons)               │
│    4. Start LocalStack (if AWS scenario)                           │
│    5. Inject break (fault)                                          │
│    6. Execute agent (provider loop or MCP)                         │
│    7. Wait for rollouts                                             │
│    8. Verify outcome (checks)                                      │
│    9. Write artifacts                                               │
│   10. Report to Evidra API                                         │
│   11. Store results (SQLite + JSONL)                               │
│   12. Teardown (or skip if --reuse-cluster)                        │
└───────────┬─────────────────────────────────────────────────────────┘
            │ uses
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Supporting Packages                                                │
│                                                                     │
│  pkg/environment/    KindProvider, K3dProvider, Bootstrapper       │
│  pkg/scenario/       Load, LoadAll, Resolve (YAML schema)         │
│  pkg/agent/          Bifrost/Claude providers, tool-use loop       │
│  pkg/agent/          MCPExecutor (MCP server stdio transport)      │
│  pkg/verifier/       deployment-ready, service-reachable, etc.     │
│  pkg/artifact/       Run bundle writer (JSON, transcripts)         │
│  pkg/report/         Evidra API reporter (online/offline JSONL)    │
│  pkg/store/          SQLite + JSONL backup                         │
│  pkg/config/         Config struct, version collection             │
│  pkg/adapter/        Legacy CLI/MCP adapter interface              │
│  pkg/tui/            Bubble Tea interactive lab                    │
│  pkg/skilldelta/     Paired with/without skill benchmarks          │
│  pkg/signalaudit/    Signal expectation auditing                   │
└─────────────────────────────────────────────────────────────────────┘
```

## Execution Modes

### Sequential (default)

```
CLI: bench-cli run --scenario kubernetes/broken-deployment

main.go → runScenarioOnce() → harness.Run()
  ↓
  provision kind cluster
  ↓
  bootstrap → break → agent → verify → store
  ↓
  teardown (or reuse)
```

One scenario at a time. No database required. Results in SQLite + JSONL.

### Parallel (--parallel N)

```
CLI: bench-cli bench --parallel 4 --database-url postgres://...

main.go → executeBenchParallel()
  ↓
  orchestrator.Provision()         ← kind cluster (once)
  ↓
  orchestrator.RunParallel()       ← River job queue
    ├── Worker 0 (bench-w0)        ← workspace + namespace isolation
    ├── Worker 1 (bench-w1)
    ├── Worker 2 (bench-w2)
    └── Worker 3 (bench-w3)
           ↓ each worker:
           workspace.New()          ← copy scenarios/manifests/charts
           workspace.RewriteNamespace()  ← bench → bench-wN
           harness.Run(KubeconfigPath)   ← skip provision
           workspace.Cleanup()      ← remove temp dir
  ↓
  orchestrator.Teardown()          ← destroy cluster (once)
```

Requires PostgreSQL for River job queue. Each worker gets:
- Isolated temp directory (workspace) — agent writes don't touch repo
- Isolated namespace (bench-w0..bench-wN) — no cross-worker interference
- Shared kubeconfig — all workers use the same pre-provisioned cluster
- Shared results store — results survive workspace cleanup

### REST API (serve)

```
CLI: bench-cli serve --database-url postgres://... --parallel 4

serve.go → serveAPI()
  ↓
  orchestrator.Provision()         ← kind cluster (once at startup)
  ↓
  POST /v1/certify                 ← enqueues River jobs
    ↓
  orchestrator.RunParallel()       ← async, returns immediately
  ↓
  GET /healthz                     ← liveness check
```

Same orchestrator, same lifecycle. The API just enqueues and returns.

This local `POST /v1/certify` surface is separate from the hosted
`POST /v1/bench/trigger` contract. The Run UI in this repo targets the hosted
trigger API, not the standalone certify API.
The local certify request can override the worker's default evidence mode; the
request value wins over the worker default and does not change the hosted
trigger aliasing.

## Data Flow

```
Scenario YAML                Agent LLM
     │                           │
     ▼                           ▼
┌──────────┐              ┌──────────────┐
│ scenario │──bootstrap──▶│   kind       │◀──tool calls──┤
│ .yaml    │──break──────▶│   cluster    │               │
│ fixtures │              │   (bench-wN) │               │
└──────────┘              └──────┬───────┘               │
                                 │                       │
                          verify │              ┌────────┴────────┐
                                 ▼              │  pkg/agent      │
                          ┌──────────────┐      │  Bifrost/Claude │
                          │ pkg/verifier │      │  MCPExecutor    │
                          │ checks pass? │      └─────────────────┘
                          └──────┬───────┘
                                 │
                    ┌────────────┼────────────┐
                    ▼            ▼            ▼
              ┌──────────┐ ┌─────────┐ ┌──────────┐
              │ SQLite   │ │ JSONL   │ │ Evidra   │
              │ bench.db │ │ backup  │ │ API      │
              └──────────┘ └─────────┘ └──────────┘
```

## Package Dependencies

```
cmd/bench-cli
  ├── pkg/orchestrator  ← parallel lifecycle
  │     ├── pkg/jobqueue     ← River workers
  │     ├── pkg/workspace    ← copy + namespace rewrite
  │     ├── pkg/environment  ← cluster provisioning
  │     ├── pkg/config       ← constants, settings
  │     └── pkg/store        ← shared results
  ├── pkg/harness       ← single scenario lifecycle
  │     ├── pkg/environment  ← bootstrap, cluster
  │     ├── pkg/agent        ← LLM providers, MCP
  │     ├── pkg/verifier     ← outcome checks
  │     ├── pkg/artifact     ← run bundles
  │     ├── pkg/report       ← Evidra reporting
  │     ├── pkg/store        ← SQLite + JSONL
  │     └── pkg/config       ← settings, versions
  ├── pkg/scenario      ← YAML loading
  ├── pkg/tui           ← interactive lab
  └── pkg/skilldelta    ← skill benchmarks
```

## Key Design Decisions

### Workspace Isolation
Each parallel worker copies `scenarios/`, `manifests/`, and `charts/` to a temp
directory. Agent writes (terraform state, modified fixtures) stay in the workspace
and are cleaned up after the run. The source repo is never modified.

### Namespace Isolation
Each worker gets its own Kubernetes namespace (`bench-w0`, `bench-w1`, etc.)
via text-based rewriting of YAML and shell files. The harness creates the
namespace before bootstrap and passes it to all kubectl commands.

### Pre-provisioned Cluster
When `RunRequest.KubeconfigPath` is set, the harness skips cluster create/destroy.
The orchestrator provisions once, shares the kubeconfig with all workers, and
tears down once after all workers complete.

### River Job Queue
PostgreSQL-backed job queue provides persistence (crash recovery), retries,
and parallelism control. Jobs are enqueued by the CLI or API, processed by
worker goroutines, and tracked via atomic counters.

### Shared Results Store
Parallel workers write to a shared SQLite store (opened by the orchestrator)
rather than workspace-local stores. This ensures results survive workspace
cleanup and are available even if the Evidra API is unreachable.
