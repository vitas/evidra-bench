---
title: Tool Server Integration
aliases:
  - MCP Integration
  - Agent Integration
  - Skill Comparison
type: guide
status: active
tags:
  - bench
  - mcp
  - integrations
  - agents
  - skills
---

# Tool Server Integration

Bench evaluates agent runtimes by keeping scenario setup and verification
inside the harness, then swapping how the agent acts. Use this guide when you
want to compare native Bench tools, an MCP server, a skill prompt, a CLI agent,
or a remote A2A agent.

## Comparison Rule

Change one axis at a time. Keep these fixed:

- scenario set
- model
- provider route
- timeout
- memory window
- cluster settings
- report ID when producing a shared report

Then vary one of:

- MCP/tool server command and version
- skill prompt
- CLI agent command
- A2A agent URL
- model or provider

## Native-Tools Baseline

A baseline run uses the Bench-owned provider loop and direct tools. It has an
empty `tool_server` identity in stored results.

```bash
bin/bench-cli bench \
  --scenario kubernetes \
  --provider bifrost \
  --model sonnet \
  --reuse-cluster
```

## MCP Server Run

All MCP servers use the same flag. Bench does not auto-start, auto-build, or
privilege any MCP server.

```bash
bin/bench-cli bench \
  --scenario kubernetes \
  --provider bifrost \
  --model sonnet \
  --mcp-server "npx -y @vendor/kubernetes-mcp --stdio" \
  --tool-server-id kubernetes-mcp \
  --tool-server-version 1.2.3 \
  --reuse-cluster
```

`--mcp-server` is the executable command. `--tool-server-id` and
`--tool-server-version` are stable comparison labels used by reports and
leaderboards. If labels are omitted, Bench infers a best-effort ID from the
command, but explicit labels are recommended.

## Stored Tool Server Identity

Reports use `tool_server` as the source of truth:

| Stored value | Meaning |
|---|---|
| empty `tool_server` | baseline or direct provider-loop run |
| non-empty `tool_server` | run used the named external tool server |

For tool-server runs, trigger requests and runner configs may carry:

- `mcp_server`: executable command for the runner
- `tool_server`: stable server identity used for filtering and comparison
- `tool_server_version`: stable version used for filtering and reports

Bench does not store a coarse run mode. The comparison identity is the
`tool_server` field.

## Skill Prompt Run

Skill prompts are local files on the runner host. Bench records their identity,
version, source, and SHA-256 digest for comparison.

```bash
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --skill-file skills/k8s-admin.md \
  --skill-id k8s-admin \
  --skill-version 2026-05-13
```

If `--skill-id` is omitted, Bench infers it from the file name. If
`--skill-sha256` is supplied, the runner verifies the local file content before
using it.

Bench does not download arbitrary skill URLs from the hosted API. To use an
external skill, download or copy it during runner setup, then pass the local
path through `--skill-file`.

## CLI And A2A Agents

External agents can run through adapter paths while Bench still owns scenario
setup and final verification.

```bash
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --adapter cli \
  --agent-command "/path/to/agent --stdio"
```

```bash
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --adapter a2a \
  --a2a-agent-url "$A2A_AGENT_URL"
```

Use these paths when the agent already has its own runtime and Bench should
evaluate behavior without embedding it into the provider loop.

## Optional Evidence Directory

Some scenarios can read local evidence artifacts when a run explicitly
provides an evidence directory:

```bash
bin/bench-cli run \
  --scenario kubernetes/privileged-pod-review \
  --provider bifrost \
  --model sonnet \
  --mcp-server "$MCP_SERVER" \
  --evidence-dir ./runs/evidence
```

This is compatibility behavior. Normal infrastructure checks always run
regardless of tool-server identity or evidence directory.

## Paired Report Workflow

For a private or public MCP readiness report, prefer `report-pack`. It runs a
native-tools baseline and a candidate MCP server over the same scenario slice,
sends both phases to Bench, and prints report URLs.

```bash
bin/bench-cli report-pack \
  --phase both \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY" \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION" \
  --reuse-cluster
```

See [Private Report Pack](PRIVATE_REPORT_PACK.md) for split-phase, multi-server,
and report-ID usage.

## API And Runner Contracts

This repo owns the benchmark API and runner control-plane surface used by the
Bench UI and remote runners:

- [Bench API Reference](BENCH_API_REFERENCE.md)
- [Executor Contract v1.0.0](contracts/EXECUTOR_CONTRACT_V1.md)
- [Bench Runner Control Plane Contract v1](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
- [Bench Service Setup](guides/bench-service-setup.md)
