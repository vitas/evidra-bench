---
title: Quickstart
type: guide
status: active
tags:
  - bench
  - quickstart
  - oss
---

# Quickstart

This guide gets a new contributor from clone to first scenario validation.

## Happy Path

1. Build the CLI with `make build`.
2. Confirm scenarios load with `bin/bench-cli scenario list`.
3. Validate one scenario with `bin/bench-cli run --scenario kubernetes/broken-deployment --dry-run`.
4. Configure one provider.
5. Run one live scenario.
6. Inspect local artifacts under `runs/`.
7. Move to `bench`, `certify`, `report-pack`, `serve`, or `lab` only after the
   one-scenario path works.

## Prerequisites

- Go 1.25.10+
- `kubectl`
- `kind` or `k3d` for live Kubernetes scenarios
- `helm` for Helm scenarios
- Node.js 22+ only if you want to build the UI

## Build

```bash
make build
```

The binary is written to `bin/bench-cli`.

## Which Command Should I Use?

| Goal | Command |
|---|---|
| Validate or run one scenario | `bin/bench-cli run` |
| Run many scenarios locally | `bin/bench-cli bench` |
| Score one certification track | `bin/bench-cli certify` |
| Compare baseline vs MCP/tool-server candidate | `bin/bench-cli report-pack` |
| Browse local scenarios and run history in terminal | `bin/bench-cli lab` |
| Start the local Bench API/control plane | `bin/bench-cli serve` |
| Compare prompt or skill variants | `bin/bench-cli skill-delta` |

The local quickstart does not require the hosted runner pool. `bin/bench-cli
bench --parallel` uses a local queue. Hosted runners are an advanced
API/control-plane workflow covered by the runner architecture docs.

## Validate A Scenario Without A Cluster

Use dry-run mode to check that the scenario loads and validates:

```bash
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --dry-run
```

## Run A Live Scenario

Live runs provision or reuse an infrastructure environment, inject a failure,
execute the selected agent adapter, and verify final state.

Example with the generic provider loop:

```bash
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model gemini-2.5-flash \
  --reuse-cluster
```

If you use a local OpenAI-compatible provider gateway, point Bench at it:

```bash
export INFRA_BENCH_BIFROST_URL=http://localhost:9090/v1
export INFRA_BENCH_BIFROST_AUTH_BEARER="$PROVIDER_API_KEY"
```

## Run Through An MCP Server

Bench can evaluate any MCP server by passing the server command and a stable
identity:

```bash
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION"
```

## Run With A Skill Prompt

Skill prompts are local files on the runner host. Bench records their identity,
version, source, and SHA-256 digest for comparison:

```bash
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --skill-file skills/k8s-admin.md \
  --skill-id k8s-admin \
  --skill-version 2026-05-13
```

## Inspect Results

Local artifacts are written under `runs/`, which is intentionally ignored by
git. Do not commit raw run artifacts or private transcripts.

For hosted/private reporting, see [Private Report Pack](PRIVATE_REPORT_PACK.md).

## Next Reading

- [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md)
- [Scoring](SCORING.md)
- [Reproducibility](REPRODUCIBILITY.md)
- [Threat Model](THREAT_MODEL.md)
