---
title: Open Source Boundary
type: governance
status: active
tags:
  - bench
  - oss
  - governance
---

# Open Source Boundary

Evidra Bench is open source where openness improves trust: scenario format,
runner behavior, verifier logic, public reports, and the public scenario
corpus should be inspectable and reproducible.

## In Scope

The public repository contains:

- CLI and local runner code
- scenario schema and public scenario fixtures
- verifier and scoring behavior
- adapter contracts for MCP servers, CLI agents, A2A agents, and provider loops
- failure-autopsy schema and deterministic classifiers
- public report UI and documentation
- sanitized examples, sample reports, and methodology docs

These pieces should be understandable without access to hosted production data.

## Not Promised As Open Source

The public repository does not promise to include:

- private customer scenarios
- holdout exam suites
- proprietary incident snapshots
- private transcripts, artifacts, or report packs
- hosted production database contents
- credentials, runner secrets, or deployment-specific configuration
- commercial scheduling, billing, or customer operations data

This boundary lets the public project build trust while preserving a business
model around private regression testing, sponsored public benchmark runs, and
custom live scenario packs.

## Data Hygiene

Only sanitized data belongs in git. Generated runs, private report artifacts,
database dumps, API keys, and customer-specific incident material must stay out
of the repository.

Internal planning notes are local-only and intentionally ignored by git.

## Contribution Focus

The most valuable public contributions are:

- harder real-infrastructure scenarios
- stronger final-state verification
- safer failure-analysis rules
- better adapter coverage for agent protocols and MCP servers
- clearer public reports that explain cost, turns, and failure modes
