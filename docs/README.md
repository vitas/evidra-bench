---
title: Bench Documentation
aliases:
  - Bench Docs
  - Documentation Home
  - Evidra Bench Docs
type: index
status: active
tags:
  - bench
  - docs
  - obsidian
---

# Bench Documentation

Bench is a live infrastructure exam and regression-testing system for AI
agents, MCP servers, skills, and remote agent runtimes. This page is the main
Obsidian map and GitHub documentation index.

## Start Here

| Reader | First page | Purpose |
|---|---|---|
| Buyer, platform lead, or evaluator | [Results And Reports](RESULTS_AND_REPORTS.md) | Understand suites, scoring, unsafe passes, evidence, and reproducibility. |
| Agent, MCP server, or skill builder | [Quickstart](QUICKSTART.md) | Build the CLI, validate a scenario, run one live task, and inspect artifacts. |
| Contributor | [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md) | Write scenarios that test capability instead of obedience. |
| API or runner integrator | [Bench Service Setup](guides/bench-service-setup.md) | Run the local API and understand the runner control plane. |

If you are new to the project, read the public [README](../README.md), then
choose one of the two main lanes below.

## Lane 1: Understand Bench

- [Results And Reports](RESULTS_AND_REPORTS.md) - public exam suites, outcome
  labels, unsafe passes, failure autopsy, evidence, reproducibility, and sample
  report shape.
- [Testing Methodology](TESTING_METHODOLOGY.md) - deeper reference for what the
  benchmark measures: remediation, diagnosis, judgment, failure analysis,
  efficiency, chaos, memory windows, and comparison patterns.
- [Scenario Catalog](SCENARIO_CATALOG.md) - current public inventory by
  category, track, level, and skipped infrastructure requirements.
- [Kubernetes MCP Readiness 2026-05](reports/kubernetes-mcp-readiness-2026-05.md)
  - published public baseline-versus-MCP report.
  Public page: <https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05>

## Lane 2: Run And Evaluate

- [Quickstart](QUICKSTART.md) - first local run and artifact inspection.
- [Tool Server Integration](TOOL_SERVER_INTEGRATION.md) - compare native tools,
  MCP servers, skills, and external agent runtimes under fixed scenarios.
- [Private Report Pack](PRIVATE_REPORT_PACK.md) - paired baseline-versus-MCP
  report-pack workflow.
- [Lab TUI Guide](LAB_TUI_GUIDE.md) - terminal UI for browsing scenarios,
  running one task, and reviewing local artifacts.
- [Bench Service Setup](guides/bench-service-setup.md) - local service,
  Postgres, UI, private review writes, and runner setup.
- [Human Review](guides/human-review.md) - review candidates, scenario
  improvements, stored patch previews, validation rerun records, and review
  artifacts.

## Scenario Authoring

- [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md) - task prompt design,
  scenario structure, break design, check design, autopsy hints, traps,
  multi-stage scenarios, and authoring checklist.
- [Scenario Schema](SCENARIO_SCHEMA.md) - YAML field reference.
- [Scenario Catalog](SCENARIO_CATALOG.md) - implemented scenario inventory.

## Reference

- [Architecture](ARCHITECTURE.md) - execution model, control-plane boundary,
  package map, data flow, storage, profiles, leases, and deployment ownership.
- [Runner Architecture](RUNNER_ARCHITECTURE.md) - poll-based remote runner
  topology, job lifecycle, queue semantics, and contracts.
- [Bench API Reference](BENCH_API_REFERENCE.md) - HTTP API reference for runs,
  catalog, analytics, triggers, review endpoints, and runners.
- [Executor Contract](contracts/EXECUTOR_CONTRACT_V1.md) - direct executor API.
- [Runner Control Plane Contract](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
  - poll-based remote runner API.
- [Run Review Contract v1](contracts/RUN_REVIEW_V1.md) - human review artifact
  schema.
- [Testing Guide](TESTING.md) - local commands, CI gates, and package-level
  test coverage.
- [Threat Model](THREAT_MODEL.md) - runner, API, artifact, credential, and
  public-report boundaries.
- [Open Source Boundary](OPEN_SOURCE.md) - what belongs in the public repo and
  what stays private.
- [Roadmap](ROADMAP.md) - public-safe project direction.

## Articles

- [What AI SRE Benchmarks Should Catch Before Production](articles/what-ai-sre-benchmarks-should-catch-before-production.md)
  - buyer-facing article about scenario-based AI SRE benchmark failure modes.
  Public post:
  <https://bench.evidra.cc/bench/articles/what-ai-sre-benchmarks-should-catch-before-production>
- [Kubernetes MCP Servers Passed. That Was Not Enough.](articles/pass-fail-is-not-enough-for-ai-infra-agents.md)
  - article based on the Claude and DeepSeek Kubernetes MCP readiness reports.
  Public post:
  <https://bench.evidra.cc/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough>

## Local-Only Notes

Internal implementation plans and archive notes are ignored by git. They may
exist locally under `docs/plans/` and `docs/archive/`, but they are not part of
the public documentation graph.

## Obsidian Conventions

- Keep one active page per durable concept.
- Prefer this index over folder browsing.
- Use standard relative Markdown links so pages work in both Obsidian and
  GitHub.
- Use frontmatter `title`, `type`, `status`, and `tags` on active docs.
- Move outdated operational notes to `archive/` instead of keeping them in the
  active graph.
