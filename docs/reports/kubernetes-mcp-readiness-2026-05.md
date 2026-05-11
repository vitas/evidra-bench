---
title: Kubernetes MCP Readiness 2026-05
type: runbook
status: draft
tags:
  - bench
  - reports
  - kubernetes
  - mcp
---

# Kubernetes MCP Readiness 2026-05

Public report comparing one native-tools baseline against two Kubernetes MCP
server candidates under the same model, scenario slice, and `report_id`.

## Report Identity

| Field | Value |
|---|---|
| Report ID | `kubernetes-mcp-readiness-2026-05` |
| Public page | `https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05` |
| Matrix API | `https://api.evidra.cc/v1/bench/reports/tool-server-matrix?model=sonnet&report_id=kubernetes-mcp-readiness-2026-05&tool_servers=flux159-mcp-server-kubernetes%2Ccontainers-kubernetes-mcp-server` |
| Markdown API | `https://api.evidra.cc/v1/bench/reports/tool-server-matrix?model=sonnet&report_id=kubernetes-mcp-readiness-2026-05&tool_servers=flux159-mcp-server-kubernetes%2Ccontainers-kubernetes-mcp-server&format=markdown` |

## Fixed Inputs

```bash
REPORT_ID=kubernetes-mcp-readiness-2026-05
MODEL=sonnet
PROVIDER=bifrost
BENCH_URL=https://api.evidra.cc
BENCH_UI_URL=https://bench.evidra.cc
REPEATS=1
```

Keep these fixed across baseline and candidate phases:

- model and provider
- scenario list
- timeout and memory window
- cluster provider and reuse policy
- system prompt, role, and contract version
- `report_id`

## Scenario Slice

The initial public slice is Kubernetes-heavy and intentionally includes traps
where an agent must diagnose before acting:

```bash
SCENARIO_ARGS=(
  --scenario kubernetes/broken-deployment
  --scenario kubernetes/wrong-service-selector
  --scenario kubernetes/network-policy-fix
  --scenario kubernetes/misleading-ingress
  --scenario kubernetes/false-alarm
  --scenario kubernetes/safe-rollback-vs-broad-patch
  --scenario kubernetes/urgency-vs-safety
  --scenario kubernetes/cascading-misconfiguration
  --scenario kubernetes/repair-loop-escalation
  --scenario kubernetes/risky-shortcut
)
```

## MCP Server Arms

Baseline:

- `tool_server`: empty
- command: direct Bench provider-loop tools

Flux159 candidate:

- repository: `https://github.com/Flux159/mcp-server-kubernetes`
- `tool_server`: `flux159-mcp-server-kubernetes`
- `tool_server_version`: exact git commit, package version, or image digest
- command env:

```bash
FLUX159_KUBERNETES_MCP_SERVER='<validated stdio launch command>'
FLUX159_KUBERNETES_MCP_VERSION='<exact commit-or-release>'
```

Containers candidate:

- repository: `https://github.com/containers/kubernetes-mcp-server`
- `tool_server`: `containers-kubernetes-mcp-server`
- `tool_server_version`: exact git commit, package version, or image digest
- command env:

```bash
CONTAINERS_KUBERNETES_MCP_SERVER='<validated stdio launch command>'
CONTAINERS_KUBERNETES_MCP_VERSION='<exact commit-or-release>'
```

## Version Pinning

Before running the public report:

1. Resolve each upstream repository to an immutable commit, release, package
   version, or container image digest.
2. Store that value in `--tool-server-version`.
3. Save the exact launch command in the article notes and final report.
4. Do not rerun only one candidate after changing the scenario list, model, or
   cluster settings.

## Baseline Command

```bash
bench-cli report-pack \
  --phase baseline \
  --report-id "$REPORT_ID" \
  --matrix-tool-server-id flux159-mcp-server-kubernetes \
  --matrix-tool-server-id containers-kubernetes-mcp-server \
  --matrix-tool-server-version "$FLUX159_KUBERNETES_MCP_VERSION" \
  --matrix-tool-server-version "$CONTAINERS_KUBERNETES_MCP_VERSION" \
  "${SCENARIO_ARGS[@]}" \
  --model "$MODEL" \
  --provider "$PROVIDER" \
  --bench-url "$BENCH_URL" \
  --bench-ui-url "$BENCH_UI_URL" \
  --bench-api-key "$BENCH_API_KEY" \
  --repeats "$REPEATS" \
  --reuse-cluster
```

## Flux159 Candidate Command

```bash
bench-cli report-pack \
  --phase candidate \
  --report-id "$REPORT_ID" \
  "${SCENARIO_ARGS[@]}" \
  --model "$MODEL" \
  --provider "$PROVIDER" \
  --bench-url "$BENCH_URL" \
  --bench-ui-url "$BENCH_UI_URL" \
  --bench-api-key "$BENCH_API_KEY" \
  --repeats "$REPEATS" \
  --mcp-server "$FLUX159_KUBERNETES_MCP_SERVER" \
  --tool-server-id flux159-mcp-server-kubernetes \
  --tool-server-version "$FLUX159_KUBERNETES_MCP_VERSION" \
  --reuse-cluster
```

## Containers Candidate Command

```bash
bench-cli report-pack \
  --phase candidate \
  --report-id "$REPORT_ID" \
  "${SCENARIO_ARGS[@]}" \
  --model "$MODEL" \
  --provider "$PROVIDER" \
  --bench-url "$BENCH_URL" \
  --bench-ui-url "$BENCH_UI_URL" \
  --bench-api-key "$BENCH_API_KEY" \
  --repeats "$REPEATS" \
  --mcp-server "$CONTAINERS_KUBERNETES_MCP_SERVER" \
  --tool-server-id containers-kubernetes-mcp-server \
  --tool-server-version "$CONTAINERS_KUBERNETES_MCP_VERSION" \
  --reuse-cluster
```

## Pilot Command Set

Use a smaller `REPORT_ID` before the public run:

```bash
REPORT_ID=kubernetes-mcp-readiness-2026-05-pilot
SCENARIO_ARGS=(
  --scenario kubernetes/broken-deployment
  --scenario kubernetes/network-policy-fix
  --scenario kubernetes/safe-rollback-vs-broad-patch
)
```

Then run the same three commands: baseline, Flux159 candidate, and containers
candidate. Inspect the matrix API before reusing the final public `REPORT_ID`.

## Pre-Publish Checklist

- Baseline, Flux159, and containers arms all have runs for the same scenarios.
- `report_id` is present in every run metadata payload.
- Tool-server versions are immutable and visible in the report.
- Failure autopsy artifacts exist for failed or unsafe-pass candidate cells.
- Public page loads: `https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05`.
- Markdown API returns a complete report for article drafts.
- Raw evidence links open for representative pass, fail, and unsafe-pass cells.
- Sponsored or vendor-provided runs are clearly labeled in the article.
- The article states that sponsors do not control scoring or findings.
