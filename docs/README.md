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
- [Lab TUI Guide](LAB_TUI_GUIDE.md) - terminal UI usage.
- [Private Report Pack](PRIVATE_REPORT_PACK.md) - baseline versus MCP
  tool-server report workflow.
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

## Build Scenarios

- [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md) - how to write
  scenarios that test capability instead of obedience.
- [Scenario Schema](SCENARIO_SCHEMA.md) - YAML schema reference.
- [Scenario Catalog](SCENARIO_CATALOG.md) - implemented scenario inventory.
- [Infra AI Agent Benchmark Portfolio](ideas/infra-ai-agent-benchmark-portfolio.md)
  - research-backed suite ideas.

## Integrate Agents And Tool Servers

- [Tool Server And Evidence Compatibility](TOOL_SERVER_INTEGRATION.md) -
  generic MCP/tool-server integration, `tool_server` identity, and legacy
  `evidence_mode` compatibility.
- [Bench API Reference](BENCH_API_REFERENCE.md) - HTTP API reference.
- [Executor Contract](contracts/EXECUTOR_CONTRACT_V1.md) - direct executor API.
- [Runner Control Plane Contract](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
  - poll-based remote runner API.

## Architecture And Governance

- [Architecture](ARCHITECTURE.md) - execution model, control-plane boundary,
  package map, data flow, and deployment ownership.
- [Threat Model](THREAT_MODEL.md) - runner, API, artifact, and credential
  boundaries.
- [Open Source Boundary](OPEN_SOURCE.md) - what belongs in the public repo and
  what stays private.
- [Bench Business Model](product/bench-business-model.md) - paid wedge,
  self-hosted boundary, and go-to-market notes.

## Articles

- [Pass/Fail Is Not Enough for AI Infrastructure Agents](articles/pass-fail-is-not-enough-for-ai-infra-agents.md)
  - draft article based on the Claude and DeepSeek Kubernetes MCP readiness
    reports.

## Ideas And Backlog

- [CKA Scenario Ideas](ideas/cka-scenario-ideas.md)
- [CKS Scenario Ideas](ideas/cks-scenario-ideas.md)
- [Helm And Argo CD Scenario Ideas](ideas/helm-scenario-ideas.md)
- [Terraform Scenario Ideas](ideas/terraform-scenario-ideas.md)
- [AWS Scenario Ideas](ideas/aws-scenario-ideas.md)
- [Terraform And AWS Scenario Ideas](ideas/terraform-aws-scenario-ideas.md)
- [Parallel Bench Runner](backlog/parallel-bench-runner.md)
- [Bench Data Model v2](backlog/bench-data-model-v2.md)
- [Progressive Skill Loading](backlog/2026-03-23-progressive-skill-loading.md)
- [Next-Generation Scenario Ideas](backlog/2026-03-17-next-gen-scenario-ideas.md)

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
