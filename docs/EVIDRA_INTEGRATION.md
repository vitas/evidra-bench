---
title: Tool Server And Evidence Compatibility
type: guide
status: active
tags:
  - bench
  - mcp
  - integrations
  - compatibility
---

# Tool Server And Evidence Compatibility

This file keeps its historical name for link stability. The current Bench
integration model is generic: Bench has no core dependency on the sibling
`../evidra` repo.

## MCP Servers

All MCP servers are configured through the same flag:

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --mcp-server "npx -y @anthropic/mcp-server-kubernetes"
```

To test `evidra-mcp`, pass it as a normal MCP server command:

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --mcp-server "evidra-mcp --signing-mode optional"
```

Bench does not auto-start or auto-build MCP server binaries. Install them in
the runner environment the same way you would install any other tool server.

## Evidence Modes

API and stored run records use two coarse modes:

| Mode | Meaning |
|---|---|
| `none` | baseline or direct provider-loop run |
| `mcp` | run used an MCP server |

The trigger endpoint accepts `evidence_mode` values `none` and `mcp`.

Use `tool_server` and `tool_server_version` metadata to distinguish individual
MCP servers when comparing results.

## Optional File-Based Checks

Some scenarios can read local evidence artifacts when a run explicitly provides
an evidence directory:

```bash
bench-cli run \
  --scenario kubernetes/privileged-pod-review \
  --provider bifrost \
  --model sonnet \
  --mcp-server "evidra-mcp --signing-mode optional" \
  --evidence-dir ./runs/evidence
```

This is compatibility behavior. Normal infrastructure checks always run
regardless of evidence mode or evidence directory.

## Comparison Pattern

To compare tool backends, keep the model, provider, scenario set, timeout,
memory window, and cluster settings fixed. Change only the MCP server command:

```bash
# Baseline
bench-cli bench \
  --scenario kubernetes \
  --model sonnet \
  --provider bifrost \
  --reuse-cluster

# Same benchmark through one MCP server
bench-cli bench \
  --scenario kubernetes \
  --model sonnet \
  --provider bifrost \
  --mcp-server "npx -y @anthropic/mcp-server-kubernetes" \
  --reuse-cluster

# Same benchmark through another MCP server
bench-cli bench \
  --scenario kubernetes \
  --model sonnet \
  --provider bifrost \
  --mcp-server "evidra-mcp --signing-mode optional" \
  --reuse-cluster
```

## Bench Job Contracts

This repo owns the benchmark API and runner control-plane surface used by the
Bench UI and remote runners:

- [Bench API Reference](BENCH_API_REFERENCE.md)
- [Executor Contract v1.0.0](contracts/EXECUTOR_CONTRACT_V1.md)
- [Bench Runner Control Plane Contract v1](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
- [Bench Service Setup](guides/bench-service-setup.md)
