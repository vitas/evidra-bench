---
title: Private Report Pack
type: guide
status: active
tags:
  - bench
  - reports
  - mcp
  - private-benchmark
---

# Private Report Pack

`bench-cli report-pack` runs the same model against the same scenario slice in
two phases:

1. `baseline`: direct Bench provider-loop tools, stored with empty
   `tool_server`.
2. `candidate`: the configured MCP tool server, stored with `tool_server` and
   `tool_server_version`.

The command is a thin orchestrator over the normal scenario runner. It does not
introduce a special benchmark mode or privileged MCP server.

## Command

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

Use `--dry-run` first to inspect the selected scenarios and report links:

```bash
bench-cli report-pack \
  --dry-run \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID"
```

By default, the command uses a small Kubernetes-heavy private report pack. Pass
`--scenario` repeatedly to pin a customer or release-specific slice:

```bash
bench-cli report-pack \
  --scenario kubernetes/broken-deployment \
  --scenario safe-rollback-vs-broad-patch \
  --scenario network-policy-fix \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY" \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id acme-kubernetes-mcp \
  --tool-server-version 2026-05-customer-a
```

## Report Slice

The printed live report URLs point at:

- `https://bench.evidra.cc/bench/reports/tool-server`
- `https://api.evidra.cc/v1/bench/reports/tool-server?format=markdown`

The report is selected by exact `model`, `tool_server`,
`tool_server_version`, and scenario IDs. If old online runs have the same
labels, they are part of the same aggregate. For one-off customer reports, use
a version label that identifies the evaluation window, release, or customer
run.

## Operating Notes

- Keep model, provider, timeout, memory window, scenario set, and cluster
  settings fixed across both phases.
- Change only the MCP server command and identity labels when comparing tool
  servers.
- Scenario verification failures are report data, not infrastructure errors.
  Add `--strict` if the command should exit non-zero on failed scenarios.
- A non-dry run requires `--bench-api-key` or `BENCH_API_KEY`, otherwise the
  online report may have no candidate entries.
