---
title: Quickstart
aliases:
  - First Run
  - Local Quickstart
type: guide
status: active
tags:
  - bench
  - quickstart
  - local
  - oss
---

# Quickstart

This guide gets you from a fresh clone to one validated local scenario. Use it
before running batch benchmarks, hosted report workflows, or remote runners.

## Prerequisites

- Go 1.25.10+
- `kubectl`
- `kind` or `k3d` for live Kubernetes scenarios
- `helm` for Helm scenarios
- Node.js 22+ only if you want to build the UI

## Build The CLI

```bash
make build
```

The binary is written to `bin/bench-cli`.

## Confirm Scenarios Load

```bash
bin/bench-cli scenario list
```

Use the catalog to choose a scenario by ID. For the current inventory, see
[Scenario Catalog](SCENARIO_CATALOG.md).

## Validate Without A Cluster

Dry-run mode validates scenario YAML and command wiring without provisioning a
cluster:

```bash
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --dry-run
```

## Configure A Provider

Bench can route model requests through the generic `bifrost` provider path.
Point it at an OpenAI-compatible endpoint:

```bash
export INFRA_BENCH_BIFROST_URL=http://localhost:9090/v1
export INFRA_BENCH_BIFROST_AUTH_BEARER="$PROVIDER_API_KEY"
```

You can also point directly at a provider endpoint when it exposes an
OpenAI-compatible API.

## Run One Live Scenario

```bash
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model gemini-2.5-flash \
  --reuse-cluster
```

A live run provisions or reuses infrastructure, injects the scenario failure,
executes the selected agent adapter, records artifacts, and verifies final
state.

## Inspect Local Artifacts

Local artifacts are written under `runs/`, which is intentionally ignored by
git. A run can include:

- transcript
- tool calls
- timeline
- scorecard
- verifier output
- failure autopsy
- run review

Do not commit raw run artifacts or private transcripts.

## Choose The Next Command

| Goal | Command | Guide |
|---|---|---|
| Run many scenarios locally | `bin/bench-cli bench` | [Testing Methodology](TESTING_METHODOLOGY.md) |
| Score one certification-style track | `bin/bench-cli certify` | [Results And Reports](RESULTS_AND_REPORTS.md) |
| Compare baseline vs MCP/tool-server candidate | `bin/bench-cli report-pack` | [Private Report Pack](PRIVATE_REPORT_PACK.md) |
| Browse scenarios and artifacts in a terminal UI | `bin/bench-cli lab` | [Lab TUI Guide](LAB_TUI_GUIDE.md) |
| Start the local Bench API/control plane | `bin/bench-cli serve` | [Bench Service Setup](guides/bench-service-setup.md) |
| Compare prompt or skill variants | `bin/bench-cli skill-delta` | [Tool Server Integration](TOOL_SERVER_INTEGRATION.md) |

Hosted runners are an advanced API/control-plane workflow. Start with one
local run, then move to [Bench Service Setup](guides/bench-service-setup.md)
and [Runner Architecture](RUNNER_ARCHITECTURE.md).

## Next Reading

- [Results And Reports](RESULTS_AND_REPORTS.md) - understand scoring,
  unsafe passes, evidence, reproducibility, and report structure.
- [Tool Server Integration](TOOL_SERVER_INTEGRATION.md) - compare MCP servers,
  skills, and external agents.
- [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md) - write or review
  scenarios.
- [Threat Model](THREAT_MODEL.md) - understand runner, credential, and artifact
  boundaries before live evaluations.
