---
title: Bench Documentation
aliases:
  - Bench Docs
  - Documentation Home
type: index
status: active
tags:
  - bench
  - docs
  - obsidian
---

# Bench Documentation

Bench is a live infrastructure exam and regression-testing system for AI
agents, MCP servers, skills, and remote agent runtimes. This page is the
Obsidian map of content and the GitHub documentation index.

## Start Here

- [Public README](../README.md) - GitHub entry point and quick start.
- [Quickstart](QUICKSTART.md) - clone, build, validate, and run a first
  scenario.
- [Bench Positioning](POSITIONING.md) - product frame, customer promise, and
  what Bench is not.
- [Public Exam Suites](EXAM_SUITES.md) - public suite map for leaderboard and
  readiness reports.
- [Roadmap](ROADMAP.md) - public-safe project direction.

## Run Bench

- [Testing Guide](TESTING.md) - local commands, CI gates, and package-level
  test coverage.
- [Bench Service Setup](guides/bench-service-setup.md) - local service,
  Postgres, control-plane-only mode, and runner setup.
- [Runner Architecture](RUNNER_ARCHITECTURE.md) - how remote runners register,
  poll, execute, report progress, and complete jobs.
- [Lab TUI Guide](LAB_TUI_GUIDE.md) - terminal UI usage.
- [Private Report Pack](PRIVATE_REPORT_PACK.md) - baseline versus MCP
  tool-server report workflow.
- [Human Review](guides/human-review.md) - review candidates, scenario
  improvements, public review visibility, and review artifacts.
- [Reproducibility](REPRODUCIBILITY.md) - what public and private reports
  should record.

## Understand Results

- [Testing Methodology](TESTING_METHODOLOGY.md) - what Bench measures and why.
- [Scoring](SCORING.md) - final-state outcomes, unsafe passes, behavior
  findings, and efficiency.
- [Agent Failure Autopsy](AGENT_FAILURE_AUTOPSY.md) - target product layer for
  explaining why an agent failed.
- [Sample Bench Report](SAMPLE_BENCH_REPORT.md) - example customer-facing
  report structure.
- [Kubernetes MCP Readiness 2026-05](reports/kubernetes-mcp-readiness-2026-05.md)
  - published public baseline versus two Kubernetes MCP servers.
  Public page: <https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05>

## Build Scenarios

- [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md) - how to write
  scenarios that test capability instead of obedience.
- [Scenario Schema](SCENARIO_SCHEMA.md) - YAML schema reference.
- [Scenario Catalog](SCENARIO_CATALOG.md) - implemented scenario inventory.

## Integrate Agents And Tool Servers

- [Tool Server And Evidence Compatibility](TOOL_SERVER_INTEGRATION.md) -
  generic MCP/tool-server integration, `tool_server` identity, and optional
  evidence-directory checks.
- [Bench API Reference](BENCH_API_REFERENCE.md) - HTTP API reference.
- [Executor Contract](contracts/EXECUTOR_CONTRACT_V1.md) - direct executor API.
- [Runner Control Plane Contract](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
  - poll-based remote runner API.
- [Run Review Contract v1](contracts/RUN_REVIEW_V1.md) - human review artifact
  schema.

## Architecture And Governance

- [Architecture](ARCHITECTURE.md) - execution model, control-plane boundary,
  package map, data flow, and deployment ownership.
- [Runner Architecture](RUNNER_ARCHITECTURE.md) - poll-based remote execution
  topology and lifecycle diagrams.
- [Threat Model](THREAT_MODEL.md) - runner, API, artifact, and credential
  boundaries.
- [Open Source Boundary](OPEN_SOURCE.md) - what belongs in the public repo and
  what stays private.

## Articles

- [Kubernetes MCP Servers Passed. That Was Not Enough.](articles/pass-fail-is-not-enough-for-ai-infra-agents.md)
  - published article based on the Claude and DeepSeek Kubernetes MCP readiness
    reports.
  Public post:
  <https://bench.evidra.cc/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough>

## Archive

- [Archived Architecture Review](archive/architecture-review-2026-04-23.md)
- [Archived Agent Certification Framework](archive/evidra-agent-certification-framework.md)
- [Archived Demo Playbook](archive/demo-playbook.md)
- [Archived Publication Checklist](archive/publication-checklist.md)
- [Archived Core Bench Plans](archive/core-bench-plans/)

Internal implementation plans are not part of the active documentation graph.
They are excluded from the public export and should not be linked from this
index.

## Obsidian Conventions

- Keep one active page per durable concept.
- Prefer this index over folder browsing.
- Use standard Markdown links so pages work in both Obsidian and GitHub.
- Use frontmatter `type`, `status`, and `tags` on active docs.
- Move outdated operational notes to `archive/` instead of keeping them in the
  active graph.
