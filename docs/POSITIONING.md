---
title: Bench Positioning
type: product
status: active
tags:
  - bench
  - positioning
  - agents
---

# Bench Positioning

Bench is regression testing for infrastructure agents.

It runs real Kubernetes, Helm, Argo CD, Terraform, and AWS scenarios against
models, MCP servers, skills, and remote agents. The output is not a generic
score. It is a repeatable external benchmark report that shows whether an agent
is ready for a task class, where it failed, and what changed since the previous
run.

## Customer Promise

Know before shipping whether an agent can handle realistic infrastructure
incidents, whether a model or skill update regressed behavior, and where the
agent wasted turns, tokens, or unsafe actions.

## Product Frame

- Primary product site: `https://bench.evidra.cc`.
- Public surface: credible leaderboard and sample benchmark reports.
- Private surface: regression history, scheduled runs, private scenarios, and
  readiness reports.
- Technical surface: open CLI, scenario schema, runner protocol, and
  self-hosted service.

## What Bench Is Not

Bench is not a closed certification authority and does not require the Evidra
MCP project. External MCP servers and future agent protocols are systems under
test, connected through adapters into the same benchmark harness.
