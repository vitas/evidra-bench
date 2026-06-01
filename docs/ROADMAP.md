---
title: Roadmap
type: roadmap
status: active
tags:
  - bench
  - roadmap
  - oss
---

# Roadmap

This roadmap is intentionally public-safe. It describes the open-source
direction without exposing private customer scenarios, holdout exams, or hosted
deployment details.

## Current Focus

Bench is focused on live infrastructure exams for AI agents:

- grow the public Kubernetes, Helm, GitOps, Terraform, and cloud-ops scenario
  corpus
- make scenarios harder through real state, traps, and multi-step diagnosis
- compare models, MCP servers, skills, CLIs, and remote agents under the same
  verifier
- explain pass, unsafe pass, fail, turns, tokens, cost, and failure patterns

## Near Term

- Add more L3/L4 scenarios that require diagnosis before action.
- Improve failure autopsy coverage for missed diagnostics, unsafe shortcuts,
  retry loops, premature success, and excessive token burn.
- Publish more public benchmark reports that compare baseline tool use against
  selected MCP servers or external agents.
- Add per-failure-mode report breakdowns so readers can distinguish diagnosis,
  root-cause, patching, verification, safety, and efficiency failures instead
  of relying on one aggregate score.
- Publish methodology pages for each report family: scenario selection,
  scoring rules, verifier contracts, repeat policy, and artifact requirements.
- Add comparison views that keep aggregate results visible while leading with
  task-class and failure-mode deltas across tools and agents.
- Improve report reproducibility: command manifests, environment metadata, and
  artifact links should make public results easier to inspect.
- Keep adapters generic so Bench can evaluate new agent protocols without
  coupling to a specific MCP server or vendor.

## Later

- Scheduled regression runs for private release gates.
- Public scenario packs aligned to exam-like tracks.
- Better scenario authoring tools and fixture validation.
- Stronger artifact redaction and snapshot import workflows.
- Optional hosted integrations for teams that want private reports without
  running their own control plane.

## Out Of Scope For The Public Repo

- private customer scenarios
- holdout exam suites
- proprietary incident snapshots
- hosted production data
- secrets, deployment topology, billing, and customer operations data

See [Open Source Boundary](OPEN_SOURCE.md) for the full boundary.
