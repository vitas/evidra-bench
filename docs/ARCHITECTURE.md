---
title: Bench Architecture
type: architecture
status: active
tags:
  - bench
  - architecture
  - agents
---

# Bench Architecture

Bench is a Go harness and control plane for testing infrastructure agents
against reproducible scenarios. It provisions or reuses test environments,
injects failures, executes an agent through a selected adapter, verifies the
final infrastructure state, and stores artifacts for comparison and regression
analysis.

## System Boundary

| Surface | Owner | Purpose |
|---|---|---|
| `bench-cli run`, `bench-cli bench`, `bench-cli certify` | this repo | Local scenario execution and certification |
| `bench-cli lab` | this repo | Local TUI for browsing scenarios and runs |
| `bench-cli serve` | this repo | Bench API, direct executor, runner queue, and local orchestration |
| `/v1/bench/*` | this repo | Runs, artifacts, analytics, triggers, leaderboard, scenario sync |
| `/v1/runners/*` | this repo | Poll-based remote runner registration, job claim, and completion |
| `/v1/certify` | this repo | Direct executor API for local service mode |
| Scenario catalog and schema | this repo | Infrastructure tasks, checks, levels, and tracks |
| Production deployment | private infrastructure repo | Compose, manifests, secrets, hosted topology, and operations |
| Optional external tools | outside this repo | MCP servers, remote A2A agents, CLIs, provider gateways |

Bench has no required sibling project dependency. It tests MCP servers, A2A
agents, CLIs, and provider gateways through generic execution adapters.

## Runtime Model

```text
scenario
  -> acquire lease
  -> provision workspace
  -> bootstrap healthy baseline
  -> inject failure
  -> execute agent through adapter
  -> collect trace and artifacts
  -> verify infrastructure outcome
  -> classify behavior
  -> store result
  -> report leaderboard/private regression result
```

Bench owns setup and verification. The selected adapter owns how the agent acts
between failure injection and final checks.

## Execution Modes

### Single Run

```text
bench-cli run
  -> acquire one lease
  -> run one scenario
  -> write local artifacts and store row
  -> release lease unless --reuse-cluster is set
```

Single runs are the simplest path for development and scenario debugging.

### Sequential Benchmark

```text
bench-cli bench
  -> acquire one compatible lease
  -> run scenarios in sequence
  -> store one run record per scenario
  -> release lease
```

Sequential benchmark mode is useful when cluster startup dominates runtime.

### Parallel Benchmark

```text
bench-cli bench --parallel N --database-url postgres://...
  -> provision shared cluster
  -> enqueue River jobs
  -> start workers
  -> copy scenario assets into per-worker workspaces
  -> rewrite namespaces
  -> run scenarios
  -> collect shared results
```

Parallel mode isolates each worker with a temporary workspace and namespace.
PostgreSQL is required for the River job queue.

### Bench Service

```text
bench-cli serve
  -> expose Bench API
  -> optionally provision a direct executor cluster
  -> accept trigger jobs
  -> serve runner queue and analytics
```

Hosted control-plane deployments normally use `BENCH_CONTROL_PLANE_ONLY=true`
or `--control-plane-only`. In that mode the API process does not provision a
local cluster and remote runners execute jobs.

## Agent Adapter Model

Bench normalizes scenario setup, task prompt delivery, artifacts, and
verification across several execution paths:

| Adapter | Code path | Responsibility |
|---|---|---|
| Built-in provider loop | `pkg/agent` | Send messages to a model, execute tool calls locally, feed results back |
| MCP server | `pkg/agent.MCPExecutor` | Expose tool calls through an MCP server command |
| A2A remote agent | `pkg/a2a` plus harness dispatch | Send the task to a remote A2A agent while Bench keeps setup and verification local |
| CLI process | `pkg/adapter` | Launch an external process and capture its output |
| Skill prompt | provider loop config | Compare behavior with alternate system prompts or role skills |

The target direction is one normalized run trace regardless of adapter:

```text
turn_started
assistant_message
tool_call_started
tool_call_finished
environment_observation
verification_check
agent_final_answer
timeout
token_usage
```

Current timeline support lives in `pkg/bench/timeline_*`. The failure-autopsy
layer will build on that timeline plus transcripts, tool calls, verifier output,
and run metrics.

## Data Flow

```text
Scenario YAML
  -> scenario loader
  -> provisioner lease
  -> bootstrap steps
  -> break steps
  -> agent adapter
  -> tool calls and transcript
  -> verifier checks
  -> artifact writer
  -> local store / Bench API
```

Stored run data includes:

- model, provider, adapter, and tool server metadata
- pass/fail outcome and exit code
- duration, turns, token counts, and estimated cost
- check results
- artifact directory
- transcript and tool-call artifacts when available
- derived timeline and scorecard artifacts when available

## Storage

Bench has two storage layers:

| Layer | Package | Purpose |
|---|---|---|
| Local SQLite plus JSONL backup | `pkg/store` | Local runs, rebuildable history, CLI workflows |
| PostgreSQL control-plane DB | `internal/benchdb`, `internal/benchsvc` | Hosted API, runners, analytics, leaderboard, trigger jobs |

The current PostgreSQL schema lives in
`internal/benchdb/migrations/001_init.up.sql`.

## Control Plane

Read-only benchmark API routes can be public and read from the configured
public tenant. Mutating routes, trigger routes, and runner routes require
static bearer auth, which maps requests to a tenant in this phase.

Key surfaces:

- `GET /healthz`
- public reads: `GET /v1/bench/leaderboard`, `GET /v1/bench/scenarios`,
  `GET /v1/bench/runs`, analytics, artifacts, comparisons
- `POST /v1/bench/runs`
- `POST /v1/bench/runs/batch`
- `POST /v1/bench/trigger`
- `GET /v1/runners/jobs`
- `POST /v1/runners/jobs/{id}/complete`

The authoritative contracts are:

- [Bench API Reference](BENCH_API_REFERENCE.md)
- [Runner Architecture](RUNNER_ARCHITECTURE.md)
- [Executor Contract](contracts/EXECUTOR_CONTRACT_V1.md)
- [Runner Control Plane Contract](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
- [Bench Service Setup](guides/bench-service-setup.md)

## Package Map

```text
cmd/bench-cli
  CLI, TUI entry point, service startup, certification, audit commands

internal/auth
  Static bearer-key auth middleware

internal/benchdb
  PostgreSQL connection and schema migration helpers

internal/benchsvc
  Bench API service, trigger jobs, runner queue, analytics, leaderboard

pkg/a2a
  A2A discovery and JSON-RPC task client

pkg/adapter
  Legacy CLI/MCP process adapter interface

pkg/agent
  Bifrost and Claude providers, tool-use loop, MCP executor, token costs

pkg/artifact
  Run bundle writer for transcripts, tool calls, timeline, scorecards

pkg/bench
  Run records, analytics types, timeline parsing and classification

pkg/config
  Runtime config, constants, version metadata

pkg/environment
  Kind/k3d providers, leases, bootstrap hooks, profile assets

pkg/harness
  Single scenario lifecycle: setup, break, agent, verify, artifacts, report

pkg/jobqueue
  River job queue client and worker implementation

pkg/orchestrator
  Parallel lifecycle and shared cluster orchestration

pkg/report
  Optional local JSONL evidence writer for compatibility workflows

pkg/scenario
  Scenario YAML loading, catalog resolution, provider compatibility

pkg/signalaudit
  Signal expectation auditing across run artifacts

pkg/skilldelta
  Paired with/without skill benchmark aggregation

pkg/store
  Local SQLite store and JSONL import/export

pkg/tui
  Bubble Tea local lab UI

pkg/verifier
  Declarative infrastructure checks

pkg/workspace
  Per-worker temporary workspace and namespace rewrite
```

## Workspace And Namespace Isolation

Parallel workers copy scenario assets into temporary directories before running.
This keeps agent writes, Terraform state, and generated files out of the source
tree.

Each worker also gets a namespace such as `bench-w0` or `bench-w1`. Namespace
rewrite happens before bootstrap so Kubernetes resources do not collide across
workers.

## Profiles And Leases

Scenarios can declare an execution profile such as `default`, `argocd`, or
`aws-localstack`. Profiles are implemented with checked-in cluster assets and
hooks rather than hardcoded Go logic:

```text
clusters/
  kind/default.yaml
  kind/argocd.yaml
  kind/aws-localstack.yaml
  k3d/default.yaml

profiles/
  default/
  argocd/install.sh
  argocd/healthcheck.sh
  argocd/cleanup.sh
  aws-localstack/install.sh
  aws-localstack/healthcheck.sh
  aws-localstack/cleanup.sh
```

The provisioner returns a lease with kubeconfig, provider metadata, and extra
environment variables. The harness receives a lease; it does not own cluster
lifetime decisions.

## Docker Images

This repo defines local build inputs. Production composition, hosted topology,
and secret wiring belong in a private infrastructure repository.

| Image | Dockerfile | Purpose |
|---|---|---|
| Bench UI | `ui/Dockerfile` | Static React UI served by nginx |
| Bench runner | `Dockerfile.bench` | `bench-cli serve` plus infrastructure tooling and scenarios |

The runner image uses the host Docker socket to create kind clusters as sibling
containers. It is not Docker-in-Docker.

## Documentation Map

- [Docs Home](README.md)
- [Testing Guide](TESTING.md)
- [Testing Methodology](TESTING_METHODOLOGY.md)
- [Agent Failure Autopsy](AGENT_FAILURE_AUTOPSY.md)
- [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md)
- [Runner Architecture](RUNNER_ARCHITECTURE.md)
- [Tool Server And Evidence Compatibility](TOOL_SERVER_INTEGRATION.md)
- [Bench API Reference](BENCH_API_REFERENCE.md)
- [Bench Service Setup](guides/bench-service-setup.md)
