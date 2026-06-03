---
title: Private Report Pack
aliases:
  - Report Pack
  - MCP Readiness Report
type: guide
status: active
tags:
  - bench
  - reports
  - mcp
  - private-benchmark
---

# Private Report Pack

`bench-cli report-pack` creates a paired baseline-versus-candidate report. Use
it when you want to evaluate an MCP server or tool backend against the same
model and scenario slice used by a native-tools baseline.

The command is a thin orchestrator over normal scenario runs. It does not
introduce a privileged MCP mode.

## Phases

| Phase | What runs | Requirements |
|---|---|---|
| `baseline` | Direct Bench provider-loop tools | No MCP server required |
| `candidate` | Configured MCP tool server | Requires `--mcp-server` and `--tool-server-id` |
| `both` | Baseline, then candidate | Default for local paired reports |

Stored results use an empty `tool_server` for the baseline and the configured
`tool_server` / `tool_server_version` for the candidate.

## Paired Report

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

Use `--dry-run` first to inspect the selected scenarios and report links:

```bash
bin/bench-cli report-pack \
  --dry-run \
  --phase baseline \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc
```

## Pin A Scenario Slice

Pass `--scenario` repeatedly for a customer, release, or public report slice:

```bash
bin/bench-cli report-pack \
  --phase both \
  --scenario kubernetes/broken-deployment \
  --scenario kubernetes/safe-rollback-vs-broad-patch \
  --scenario kubernetes/network-policy-fix \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY" \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id acme-kubernetes-mcp \
  --tool-server-version 2026-05-customer-a
```

Use a stable `--report-id` for one-off customer reports and public comparison
campaigns so old online runs with the same tool-server labels are not mixed
into the slice.

## Multi-Server Public Report

For one baseline and multiple MCP candidate arms, reuse the same `REPORT_ID`.
Run the baseline once, then run one candidate phase per server.

```bash
REPORT_ID=kubernetes-mcp-readiness-2026-05

bin/bench-cli report-pack \
  --phase baseline \
  --report-id "$REPORT_ID" \
  --matrix-tool-server-id flux159-mcp-server-kubernetes \
  --matrix-tool-server-id containers-kubernetes-mcp-server \
  --scenario kubernetes/broken-deployment \
  --scenario kubernetes/network-policy-fix \
  --scenario kubernetes/safe-rollback-vs-broad-patch \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY"

bin/bench-cli report-pack \
  --phase candidate \
  --report-id "$REPORT_ID" \
  --scenario kubernetes/broken-deployment \
  --scenario kubernetes/network-policy-fix \
  --scenario kubernetes/safe-rollback-vs-broad-patch \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY" \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION"
```

For baseline-only public runs, add repeatable `--matrix-tool-server-id` flags
so dry-run output can print multi-arm matrix report links before any candidate
phase has run.

## Report Links

The printed report URLs point at:

- `https://bench.evidra.cc/bench/reports/tool-server`
- `https://api.evidra.cc/v1/bench/reports/tool-server?format=markdown`

The report is selected by exact `model`, `tool_server`,
`tool_server_version`, optional `report_id`, and scenario IDs.

## Operating Rules

- Keep model, provider, timeout, memory window, scenario set, and cluster
  settings fixed across phases.
- Change only the MCP server command and identity labels when comparing tool
  servers.
- Scenario verification failures are report data, not infrastructure errors.
  Add `--strict` if the command should exit non-zero on failed scenarios.
- A non-dry run requires `--bench-api-key` or `BENCH_API_KEY`; otherwise the
  online report may have no candidate entries.
- See [Results And Reports](RESULTS_AND_REPORTS.md) for scoring,
  reproducibility, unsafe passes, and report structure.
