---
title: Bench Documentation
type: index
status: active
tags:
  - bench
  - docs
  - obsidian
---

# Bench Documentation

Bench is a regression testing and evaluation system for infrastructure agents.
Use this page as the documentation home in Obsidian and GitHub.

## Product

- Product site: `https://bench.evidra.cc`.
- [Public README](../README.md) - GitHub entry point and quick start.
- [Positioning](POSITIONING.md) - short product frame and customer promise.
- [Public Exam Suites](EXAM_SUITES.md) - product-facing scenario packs for
  leaderboard and readiness reports.
- [Business Model](product/bench-business-model.md) - paid wedge, self-hosted
  boundary, and go-to-market notes.
- [Agent Failure Autopsy](AGENT_FAILURE_AUTOPSY.md) - target product design
  for explaining where and why an agent failed.
- [Autopsy MVP Design](plans/2026-05-10-autopsy-mvp-design.md) - next
  first-class failure-analysis release slice.

## Architecture

- [Architecture](ARCHITECTURE.md) - execution model, control-plane boundary,
  packages, data flow, and deployment ownership.
- [Testing Methodology](TESTING_METHODOLOGY.md) - what Bench measures and why.
- [Evidence and Tool-Server Integration](TOOL_SERVER_INTEGRATION.md) - optional MCP
  and file-based compatibility notes.
- [Private Report Pack](PRIVATE_REPORT_PACK.md) - CLI workflow for private
  baseline versus MCP tool-server reports.
- [Evidence Modes](EVIDENCE_MODES.md) - run-record modes used by API filters
  and MCP runs.

## Running Bench

- [Bench Service Setup](guides/bench-service-setup.md) - local service,
  database, control-plane-only mode, and runner setup.
- [Testing Guide](testing.md) - local test commands and package coverage.
- [Demo Playbook](DEMO_PLAYBOOK.md) - scripted demo flow.
- [Lab TUI Guide](LAB_TUI_GUIDE.md) - local terminal UI usage.

## Scenario Authoring

- [Scenario Authoring Guide](SCENARIO_AUTHORING_GUIDE.md) - how to write
  scenarios that test agent capability instead of obedience.
- [Scenario Schema](SCENARIO_SCHEMA.md) - YAML schema reference.
- [Scenario Catalog](SCENARIO_CATALOG.md) - implemented scenario catalog.

## API And Runner Contracts

- [Bench API Reference](BENCH_API_REFERENCE.md) - HTTP API reference.
- [Executor Contract](contracts/EXECUTOR_CONTRACT_V1.md) - direct executor API.
- [Runner Control Plane Contract](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
  - poll-based remote runner API.

## Scenario Ideas

- [Infra AI Agent Benchmark Portfolio](ideas/infra-ai-agent-benchmark-portfolio.md)
- [CKA Scenario Ideas](ideas/cka-scenario-ideas.md)
- [CKS Scenario Ideas](ideas/cks-scenario-ideas.md)
- [Helm And Argo CD Scenario Ideas](ideas/helm-scenario-ideas.md)
- [Terraform Scenario Ideas](ideas/terraform-scenario-ideas.md)
- [AWS Scenario Ideas](ideas/aws-scenario-ideas.md)
- [Terraform And AWS Scenario Ideas](ideas/terraform-aws-scenario-ideas.md)

## Backlog And Archive

- [Parallel Bench Runner](backlog/parallel-bench-runner.md)
- [Bench Data Model v2](backlog/bench-data-model-v2.md)
- [Progressive Skill Loading](backlog/2026-03-23-progressive-skill-loading.md)
- [Archived Core Bench Plans](archive/core-bench-plans/)
- [Implementation Plans](plans/)

## Current Center Of Gravity

Bench should stay focused on repeatable evaluation:

1. Run agents against real infrastructure scenarios.
2. Compare models, skills, MCP servers, and remote agents under the same
   conditions.
3. Store pass rate, cost, turns, token use, and artifacts.
4. Explain failures in a way that helps teams improve agents and prevent
   regressions.

External MCP servers and artifact sources are tested through generic Bench
interfaces; none are required project dependencies.
