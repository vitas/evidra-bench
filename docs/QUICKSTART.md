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

This guide gets you from Docker to a local infrastructure-agent report. No
Evidra account, source checkout, central server, or upload is required.

## Prerequisites

- Docker with access to `/var/run/docker.sock`
- a provider credential, an already-installed local Ollama model, or an
  external agent executable

The Docker socket gives the runner host-level control through the Docker
daemon. Run only agents and test inputs you trust.

## Run The Starter Suite

The default environment is kind. From any working directory:

```bash
# Linux: run as your own uid so the reports are yours, not root-owned
# (the docker.sock-owning group is added so kind stays drivable from
# inside). Omit --user/--group-add on Docker Desktop.
docker run --rm \
  --user "$(id -u):$(id -g)" \
  --group-add "$(stat -c %g /var/run/docker.sock)" \
  -e HOME=/workspace/evidra-results \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/evidra-results:/workspace/evidra-results" \
  -e OPENAI_API_KEY \
  ghcr.io/vitas/evidra-bench:latest \
  test --model openai/gpt-5
```

The first run downloads the Kubernetes node image. Later runs reuse Docker's
cache. The runner cleans up its disposable cluster even when a case fails.

To use k3d instead, add `--environment k3d`:

```bash
docker run --rm \
  --user "$(id -u):$(id -g)" \
  --group-add "$(stat -c %g /var/run/docker.sock)" \
  -e HOME=/workspace/evidra-results \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/evidra-results:/workspace/evidra-results" \
  -e OPENAI_API_KEY \
  ghcr.io/vitas/evidra-bench:latest \
  test --model openai/gpt-5 --environment k3d
```

Both environments use the same evaluation plan, scenarios, harness, verifier,
result model, and reporting pipeline.

## Run A Local Ollama Model

Local models are first-class `evidra test` targets, not a demo-only path. With
Ollama already running (`ollama serve`) and a tool-calling model installed
(`ollama pull qwen3:8b`), launch the runner with `--network host` so it can
reach the fixed local endpoint `127.0.0.1:11434`:

```bash
docker run --rm --network host \
  --user "$(id -u):$(id -g)" \
  --group-add "$(stat -c %g /var/run/docker.sock)" \
  -e HOME=/workspace/evidra-results \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/evidra-results:/workspace/evidra-results" \
  ghcr.io/vitas/evidra-bench:latest \
  test --model ollama/qwen3:8b --ci
```

Before creating any cluster, the runner discovers installed models, reads their
metadata, and validates tool calling; a missing or incompatible model aborts
immediately with an actionable error. Models are never downloaded
automatically, and the suite never falls back to a paid provider. Verdicts,
reports, and evidence are identical to any other provider.

`demo` is the thinnest entry point into the exact same evaluation: it selects
your only compatible installed model automatically and asks once when several
are installed:

```bash
docker run --rm -it --network host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/evidra-results:/workspace/evidra-results" \
  ghcr.io/vitas/evidra-bench:latest \
  demo
```

On macOS, Docker Desktop host networking must be enabled in Settings, and the
Ollama runtime itself has to be reachable from the Docker VM's loopback;
otherwise use the native `bin/bench-cli` build from the Advanced Native Setup
section below, which reaches Ollama on your Mac directly.

## Test An External Agent

Mount the agent executable into the runner and pass its container path:

```bash
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/evidra-results:/workspace/evidra-results" \
  -v "$PWD/my-agent.sh:/fixtures/agent.sh:ro" \
  ghcr.io/vitas/evidra-bench:latest \
  test --agent /fixtures/agent.sh
```

The agent receives `kubectl` on `PATH`, `KUBECONFIG`,
`INFRA_BENCH_SCENARIO`, `INFRA_BENCH_PROMPT`, and a writable workspace.

## Read The Result

The terminal prints PASS / FAIL / UNSAFE / INCOMPLETE for each case.
Verdicts rest on evidence captured inside the run — windowed API audit,
state snapshots, confined execution — and any lost or unattributable
evidence (or a faulted checker) lands the case on INCOMPLETE rather than
on a clean verdict; the result's gap list explains what was missing
(see [ADR 0001](adr/0001-process-safety-matching.md)). Local outputs include:

- `evidra-results/report.html` — standalone report opened directly in a browser;
- `evidra-results/result.json` — canonical machine-readable result;
- `evidra-results/runs/` — transcripts, verifier data, timelines, and autopsies;
- `evidra-results/bundles/` — portable signed evidence bundles when export succeeds.

Exit codes are stable for automation:

- `0`: required cases passed;
- `1`: a behavioral failure or unsafe result;
- `2`: the evaluation could not complete.

## Advanced Native Setup

For scenario authoring, advanced commands, or service development, clone the
repository and install Go 1.25.11+, `kubectl`, kind or k3d, and Helm:

```bash
make build
bin/bench-cli scenario list
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model gemini-2.5-flash \
  --reuse-cluster
```

A native live run uses the same underlying execution components as
`evidra test` in the Docker image.

## Choose The Next Command

| Goal | Command | Guide |
|---|---|---|
| Run many scenarios locally | `bin/bench-cli bench` | [Testing Methodology](TESTING_METHODOLOGY.md) |
| Score one certification-style track | `bin/bench-cli certify` | [Results And Reports](RESULTS_AND_REPORTS.md) |
| Compare baseline vs MCP/tool-server candidate | `bin/bench-cli report-pack` | [Private Report Pack](PRIVATE_REPORT_PACK.md) |
| Browse scenarios and artifacts in a terminal UI | `bin/bench-cli lab` | [Lab TUI Guide](LAB_TUI_GUIDE.md) |
| Start the local Bench API/control plane | `bin/bench-cli serve` | [Bench Service Setup](guides/bench-service-setup.md) |
| Compare prompt or skill variants | `bin/bench-cli skill-delta` | [Tool Server Integration](TOOL_SERVER_INTEGRATION.md) |

Hosted runners are an advanced API/control-plane workflow. Start with the
local Docker test, then move to [Bench Service Setup](guides/bench-service-setup.md)
and [Runner Architecture](RUNNER_ARCHITECTURE.md).

## Next Reading

- [Results And Reports](RESULTS_AND_REPORTS.md) — understand scoring, unsafe
  behavior, evidence, reproducibility, and report structure.
- [Tool Server Integration](TOOL_SERVER_INTEGRATION.md) — compare MCP servers,
  skills, and external agents.
- [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md) — write or review
  scenarios.
- [Threat Model](THREAT_MODEL.md) — understand runner, credential, and artifact
  boundaries before live evaluations.
