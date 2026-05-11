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

Use `--phase` to split the work across machines, retries, or multiple MCP
servers:

- `--phase baseline`: direct Bench tools only; does not require `--mcp-server`
  or `--tool-server-id`.
- `--phase candidate`: configured MCP tool server only; requires
  `--mcp-server` and `--tool-server-id`.
- `--phase both`: default two-phase behavior.

## Command

```bash
bench-cli report-pack \
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

Use `--dry-run` first to inspect the selected scenarios and report links:

```bash
bench-cli report-pack \
  --dry-run \
  --phase baseline \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc
```

By default, the command uses a small Kubernetes-heavy private report pack. Pass
`--scenario` repeatedly to pin a customer or release-specific slice:

```bash
bench-cli report-pack \
  --phase both \
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
`tool_server_version`, optional `report_id`, and scenario IDs. Use a stable
`--report-id` for one-off customer reports and public comparison campaigns so
old online runs with the same tool-server labels are not mixed into the slice.

## Public Multi-Server Workflow

For a public report with one baseline and multiple MCP candidate arms, reuse
the same `REPORT_ID` for every phase:

```bash
REPORT_ID=kubernetes-mcp-readiness-2026-05

bench-cli report-pack \
  --phase baseline \
  --report-id "$REPORT_ID" \
  --scenario kubernetes/broken-deployment \
  --scenario kubernetes/network-policy-fix \
  --scenario kubernetes/safe-rollback-vs-broad-patch \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY"

bench-cli report-pack \
  --phase candidate \
  --report-id "$REPORT_ID" \
  --scenario kubernetes/broken-deployment \
  --scenario kubernetes/network-policy-fix \
  --scenario kubernetes/safe-rollback-vs-broad-patch \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY" \
  --mcp-server "$FLUX159_KUBERNETES_MCP_SERVER" \
  --tool-server-id flux159-mcp-server-kubernetes \
  --tool-server-version "$FLUX159_KUBERNETES_MCP_VERSION"

bench-cli report-pack \
  --phase candidate \
  --report-id "$REPORT_ID" \
  --scenario kubernetes/broken-deployment \
  --scenario kubernetes/network-policy-fix \
  --scenario kubernetes/safe-rollback-vs-broad-patch \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY" \
  --mcp-server "$CONTAINERS_KUBERNETES_MCP_SERVER" \
  --tool-server-id containers-kubernetes-mcp-server \
  --tool-server-version "$CONTAINERS_KUBERNETES_MCP_VERSION"
```

## Operating Notes

- Keep model, provider, timeout, memory window, scenario set, and cluster
  settings fixed across both phases.
- Change only the MCP server command and identity labels when comparing tool
  servers.
- Scenario verification failures are report data, not infrastructure errors.
  Add `--strict` if the command should exit non-zero on failed scenarios.
- A non-dry run requires `--bench-api-key` or `BENCH_API_KEY`, otherwise the
  online report may have no candidate entries.
