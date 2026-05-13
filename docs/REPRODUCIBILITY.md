---
title: Reproducibility
type: methodology
status: active
tags:
  - bench
  - reproducibility
  - reports
---

# Reproducibility

Bench public results should be inspectable and repeatable enough for readers to
understand what was tested, even when hosted production artifacts are not
included in the repository.

## What To Record

Every public benchmark report should include:

- repo commit
- scenario ids and suite name
- model id and provider route
- adapter type
- MCP/tool-server id and version when applicable
- run command or report-pack command
- environment runtime (`kind`, `k3d`, LocalStack, or other)
- pass/fail/safe-pass/unsafe-pass result
- turns, tokens, duration, and estimated cost when available
- artifact links or sanitized evidence excerpts

## Local Reproduction

Build the CLI:

```bash
make build
```

Run a single dry-run validation:

```bash
bench-cli run --scenario kubernetes/broken-deployment --dry-run
```

Run one live scenario with a selected provider:

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model gemini-2.5-flash \
  --reuse-cluster
```

Run a baseline-versus-MCP report pack:

```bash
bench-cli report-pack \
  --provider bifrost \
  --model sonnet \
  --bench-url "$BENCH_API_URL" \
  --bench-api-key "$BENCH_API_KEY" \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION"
```

## Limits

Live infrastructure benchmarks are not pure deterministic unit tests. Results
can vary with model version, provider routing, tool-server version, cluster
runtime, network timing, and scenario fixture changes.

This is why reports should preserve command metadata, version metadata, and raw
evidence links. The goal is not to claim perfect determinism; it is to make the
run auditable.

## Data Hygiene

Do not publish private transcripts, customer incident data, API keys, database
dumps, or unredacted hosted artifacts. Public reports should use sanitized
evidence and public scenario fixtures.
