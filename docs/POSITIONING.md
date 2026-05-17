---
title: Evidra Bench Positioning
type: product
status: active
tags:
  - bench
  - positioning
  - agents
---

# Evidra Bench Positioning

Evidra Bench is live infrastructure exams and regression testing for AI agents.

It runs real Kubernetes, Helm, Argo CD, Terraform, and AWS scenarios against
models, MCP servers, skills, and remote agents. The output is not a generic
score. It is a repeatable external benchmark report that shows whether an agent
is ready for a task class, where it failed, and what changed since the previous
run.

## Customer Promise

Know before shipping whether an agent can handle realistic infrastructure
incidents, whether a model or skill update regressed behavior, and where the
agent wasted turns, tokens, or unsafe actions.

## Priority Wedge

Public exam suites are the marketing wedge:

- Kubernetes admin/app/security tasks for AI agents
- GitOps, Terraform, and cloud-ops tracks
- public leaderboard and shareable readiness reports
- failure autopsy that shows why an agent failed, not only whether it failed

Private incident suites are the paid wedge:

- customer incidents converted into repeatable Bench cases
- no-MCP/native-tools baseline versus a customer's chosen MCP server
- scheduled regression runs and private readiness reports

## Product Frame

- Primary product site: `https://bench.evidra.cc`.
- Public surface: live exam suites, credible leaderboard, and sample benchmark
  reports.
- Private surface: regression history, scheduled runs, customer incident
  suites, private scenarios, and readiness reports.
- Technical surface: open CLI, scenario schema, runner protocol, and
  self-hosted service.

## What Bench Is Not

Evidra Bench is not an official certification authority and does not require the
user to adopt any particular MCP server. Exam-aligned suites can reference
public skill domains, but they are not affiliated with CNCF, Linux Foundation,
HashiCorp, or AWS. External MCP servers and future agent protocols are systems
under test, connected through adapters into the same benchmark harness.
