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

## Prerequisites

- Go 1.26.3+
- `kubectl`
- `kind` or `k3d` for live Kubernetes scenarios
- `helm` for Helm scenarios
- Node.js 22+ only if you want to build the UI

## Build

```bash
make build
```

The binary is written to `bin/bench-cli`.

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

## Inspect Results

Local artifacts are written under `runs/`, which is intentionally ignored by
git. Do not commit raw run artifacts or private transcripts.

For hosted/private reporting, see [Private Report Pack](PRIVATE_REPORT_PACK.md).

## Next Reading

- [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md)
- [Scoring](SCORING.md)
- [Reproducibility](REPRODUCIBILITY.md)
- [Threat Model](THREAT_MODEL.md)
