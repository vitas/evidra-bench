---
title: Bench Business Model
type: product
status: active
tags:
  - bench
  - business
  - product
---

# Bench Business Model

Private planning note for a small profitable developer-tool business.

## Positioning

Sell live infrastructure exams and reliable agent regression testing, not a
closed benchmark format.

Bench should be trusted because teams can run it themselves: the CLI, scenario
schema, API contracts, and runner protocol stay open. The paid product removes
operational work and adds durable history, scheduling, private scenarios, and
failure reports.

Public exam suites are the marketing wedge. Customer incident suites are the
paid wedge.

The primary product site is `https://bench.evidra.cc`. Keep API endpoints and
deployment internals separate from this public product URL.

## Customer Problem

Teams adopting infrastructure agents need to know:

- whether the agent can handle realistic incidents
- whether the agent can pass public Kubernetes, GitOps, Terraform, and cloud
  operations exam-style scenarios
- which model, prompt, MCP server, or skill changed behavior
- when a previously passing scenario regressed
- where a failed agent wasted turns and tokens
- whether the agent took unsafe shortcuts
- whether a private production-like scenario remains covered over time
- whether their own historical incidents can become repeatable agent tests

Most teams do not want to maintain clusters, runners, result databases,
dashboards, scheduled jobs, and scenario packs just to answer those questions.

## Product Shape

| Tier | Customer | Offer |
|---|---|---|
| Self-hosted | OSS users, individual teams | CLI, scenario schema, local service, runner protocol |
| Managed SaaS | agent builders, small platform teams | hosted dashboard, durable history, scheduled runs, runner registration |
| Enterprise | platform/security teams | private scenarios, dedicated runners, compliance packs, support |

The first paid wedge should be managed SaaS for people who already believe in
agent testing but do not want to operate the benchmark control plane.

## What Remains Open

- `bench-cli`
- scenario schema
- public scenario catalog
- API contracts
- runner protocol
- self-hosted bench service
- local dashboard/TUI workflows

This supports adoption and lets serious users verify the system independently.

## What Is Paid

- managed dashboard and durable result history
- scheduled private regression runs
- private scenario packs
- failure-autopsy reports
- hosted runner operations
- model/provider history across teams
- external benchmark reports / readiness reports
- support for custom runners and enterprise network constraints

## Why Users Pay If Self-Hosting Exists

Running benchmarks is operationally annoying:

- clusters need provisioning and cleanup
- runners need liveness, retries, and logs
- Postgres and artifacts need backups
- model credentials and budgets need governance
- scenario catalogs need updates
- scheduled runs need alerting
- teams want a URL and a report, not another service to maintain

The paid product sells time saved, repeatability, and private reporting.

## Revenue Lines

1. Managed SaaS subscription with run limits.
2. Dedicated runner add-on for private infrastructure.
3. Scenario packs for Kubernetes, security, Terraform, compliance, and vendor
   benchmark suites.
4. Incident-to-benchmark sprints that turn customer outages and postmortems
   into private Bench suites.
5. Professional services for custom scenario authoring.
6. External benchmark reports and readiness reports for agent vendors.

## Near-Term Business Focus

1. Make the self-hosted story boring and reliable.
2. Make public exam suites and leaderboard results credible and easy to share.
3. Build one hosted loop: create project, register runner, run scenario, view
   result, compare against prior runs.
4. Add failure autopsy before adding broad analytics.
5. Add a manual incident-to-benchmark service before building a production
   recorder.
6. Charge for managed history, scheduling, private packs, and reports once the
   loop is used repeatedly.

## Product Priority

Bench should prioritize:

1. reliable scenario execution
2. public exam suites as marketing proof
3. failure autopsy
4. private regression history
5. protocol-agnostic adapters
6. public leaderboard
7. scheduled runs and alerts
8. readiness reports

Generic analytics should come after the failure-reporting layer. The valuable
question is not just "what changed?" It is "why did the agent fail, and what
should the team fix?"

## Repo Boundary

| Repo | Role |
|---|---|
| `evidra-bench` | scenarios, execution harness, adapters, bench API, runner protocol, dashboard |
| private infrastructure repo | production deployment topology, compose/manifests, secrets, hosted operations |
| external tool-server repos | optional MCP/tooling projects that Bench can test like any other tool server |

Bench should not depend on any external tool-server project as a core product
requirement.
