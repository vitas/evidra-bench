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

Bench uses a generic integration model. Tool servers are external processes,
not project-specific modes. No MCP server is privileged or treated as the Bench
reference implementation.

## MCP Servers

All MCP servers are configured through the same flag:

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --mcp-server "$MCP_SERVER"
```

Bench does not auto-start or auto-build MCP server binaries. Install them in
the runner environment the same way you would install any other tool server.

`--mcp-server` is the executable command. Use `--tool-server-id` and
`--tool-server-version` as stable comparison labels when the command is not a
good product identity:

```bash
bench-cli bench \
  --scenario kubernetes \
  --provider bifrost \
  --model sonnet \
  --mcp-server "npx -y @vendor/kubernetes-mcp --stdio" \
  --tool-server-id kubernetes-mcp \
  --tool-server-version 1.2.3
```

If labels are omitted, Bench infers a best-effort ID from the command. Explicit
labels always win and are recommended for private reports and release
regression comparisons.

## Stored Tool Server Identity

Tool-server comparison and reports use `tool_server` as the source of truth:

- empty `tool_server`: baseline or direct provider-loop run
- non-empty `tool_server`: run used the selected external tool server

For tool-server runs the trigger endpoint and runner config may carry:

- `mcp_server`: executable command for the runner
- `tool_server`: stable server identity used for filtering/comparison
- `tool_server_version`: stable server version used for filtering/comparison
  and reports

Bench does not store a coarse run mode. The source of truth is `tool_server`:
empty means a native/direct provider-loop baseline, and non-empty means the
named external tool server. Compare by keeping model, provider, scenario set,
and runtime settings fixed while varying `tool_server` and
`tool_server_version`.

There are no Bench-specific MCP submodes and no reference MCP server. To test
a tool server, pass its command through `--mcp-server` and label the tested
server explicitly with `--tool-server-id` and `--tool-server-version`.

## Skill Prompts

Skill prompts are a separate comparison axis. Use a local file that already
exists on the runner host:

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --skill-file /tmp/bench-skills/k8s-admin.md \
  --skill-id k8s-admin \
  --skill-version 2026-05-13
```

Bench stores `skill_id`, `skill_version`, `skill_source`, and `skill_sha256`
on run records. If `--skill-id` is omitted, Bench infers it from the file name.
If `--skill-sha256` is supplied, the runner verifies the local file content
before using it.

Bench does not download arbitrary skill URLs from the hosted API. To use an
external skill, download or copy it in runner-side setup code, then pass the
local path through `--skill-file`.

Tool-server and skill comparisons can be combined. Keep model, provider,
scenario set, timeout, memory window, and cluster settings fixed while varying
only `tool_server`/`tool_server_version`, `skill_id`/`skill_version`, or both.

## Optional File-Based Checks

Some scenarios can read local evidence artifacts when a run explicitly provides
an evidence directory:

```bash
bench-cli run \
  --scenario kubernetes/privileged-pod-review \
  --provider bifrost \
  --model sonnet \
  --mcp-server "$MCP_SERVER" \
  --evidence-dir ./runs/evidence
```

This is compatibility behavior. Normal infrastructure checks always run
regardless of tool-server identity or evidence directory.

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

# Same benchmark through the selected MCP server
bench-cli bench \
  --scenario kubernetes \
  --model sonnet \
  --provider bifrost \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION" \
  --reuse-cluster

# Same benchmark through another MCP server
bench-cli bench \
  --scenario kubernetes \
  --model sonnet \
  --provider bifrost \
  --mcp-server "$OTHER_MCP_SERVER" \
  --tool-server-id other-kubernetes-mcp \
  --tool-server-version "$OTHER_MCP_VERSION" \
  --reuse-cluster
```

For a private report deliverable, prefer the paired workflow:

```bash
bench-cli report-pack \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY" \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION" \
  --reuse-cluster
```

It runs the direct baseline and candidate MCP server phases under the same
scenario slice, sends both sides to Bench, and prints live report links. See
[Private Report Pack](PRIVATE_REPORT_PACK.md).

## Bench Job Contracts

This repo owns the benchmark API and runner control-plane surface used by the
Bench UI and remote runners:

- [Bench API Reference](BENCH_API_REFERENCE.md)
- [Executor Contract v1.0.0](contracts/EXECUTOR_CONTRACT_V1.md)
- [Bench Runner Control Plane Contract v1](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
- [Bench Service Setup](guides/bench-service-setup.md)
