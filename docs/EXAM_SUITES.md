---
title: Public Exam Suites
type: reference
status: active
tags:
  - bench
  - exams
  - scenarios
---

# Public Exam Suites

Bench packages the live scenario catalog into public exam suites for AI
infrastructure agents. These suites make benchmark results easier to reproduce,
compare, and inspect across models, MCP servers, skills, and external agent
runtimes.

They are exam-aligned benchmark suites, not official certifications. Bench is
not affiliated with CNCF, the Linux Foundation, HashiCorp, AWS, or any other
vendor exam body.

## Suite Map

| Suite | Scenario source | What it proves |
|---|---|---|
| Kubernetes Admin Exam | `kubernetes` scenarios on `workloads`, `troubleshooting`, `networking`, and `storage` tracks | The agent can operate a live cluster without guessing or over-mutating. |
| Kubernetes Security Exam | `kubernetes` scenarios on `pod-security` and `runtime-security` tracks | The agent can fix security issues without weakening controls. |
| GitOps And Release Exam | `helm` and `argocd` scenarios on the `release-ops` track | The agent can recover release systems while preserving declarative intent. |
| Terraform And Cloud Ops Exam | `terraform` and `aws` scenarios, plus `platform-eng` track scenarios | The agent can reason about state and cloud controls before applying changes. |
| MCP Server Readiness Exam | L2/L3/L4 scenarios and chaos scenarios across all domains | A selected tool server improves diagnosis, safety, and cost versus a no-MCP/native-tools baseline under the same tasks. |

## Shareable URLs

The public catalog, leaderboard, and run browser accept an `exam` query
parameter:

| Suite | Catalog URL | Leaderboard URL | Runs URL |
|---|---|---|---|
| Kubernetes Admin Exam | `/bench/scenarios?exam=kubernetes-admin` | `/bench/leaderboard?exam=kubernetes-admin` | `/bench/runs?exam=kubernetes-admin` |
| Kubernetes Security Exam | `/bench/scenarios?exam=kubernetes-security` | `/bench/leaderboard?exam=kubernetes-security` | `/bench/runs?exam=kubernetes-security` |
| GitOps And Release Exam | `/bench/scenarios?exam=gitops-release` | `/bench/leaderboard?exam=gitops-release` | `/bench/runs?exam=gitops-release` |
| Terraform And Cloud Ops Exam | `/bench/scenarios?exam=terraform-cloud` | `/bench/leaderboard?exam=terraform-cloud` | `/bench/runs?exam=terraform-cloud` |
| MCP Server Readiness Exam | `/bench/scenarios?exam=mcp-readiness` | `/bench/leaderboard?exam=mcp-readiness` | `/bench/runs?exam=mcp-readiness` |

## Why This Layer Exists

The raw scenario catalog is useful for builders. Exam suites add a stable
comparison layer:

- They define named task slices for public benchmark reports.
- They make baseline, MCP-server, skill, and external-agent runs comparable.
- They keep scenario authors focused on balanced task portfolios.
- They let reports show per-suite and per-failure-mode results instead of only
  aggregate scores.

## Public Versus Non-Public Suites

Public exam suites should stay generic enough for reproducible open-source
evaluation: Kubernetes operations, security, GitOps, Terraform, cloud controls,
and MCP readiness.

Non-public suites can use organization-specific incidents, internal workflows,
provider constraints, and historical outages. Those fixtures and artifacts
should stay outside the public repository unless they are sanitized.

## Implementation Notes

The first implementation is intentionally thin:

- no new database table
- no migration
- no duplicate scenario metadata
- no forked scenario catalog

The UI derives exam-suite membership from existing scenario fields:
`category`, `track`, `level`, and `chaos`. If the suite definitions become
customer-specific or need stable historical membership, promote them to a
versioned data model later.
