---
title: Public Exam Suites
type: product
status: active
tags:
  - bench
  - exams
  - scenarios
  - positioning
---

# Public Exam Suites

Bench packages the live scenario catalog into public exam suites for AI
infrastructure agents. These suites are the marketing surface: they make the
benchmark easy to understand, compare, and share.

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
| MCP Readiness Exam | L2/L3/L4 scenarios and chaos scenarios across all domains | A tool server improves diagnosis, safety, and cost under the same tasks. |

## Shareable URLs

The public catalog and leaderboard accept an `exam` query parameter:

| Suite | Catalog URL | Leaderboard URL |
|---|---|---|
| Kubernetes Admin Exam | `/bench/scenarios?exam=kubernetes-admin` | `/bench/leaderboard?exam=kubernetes-admin` |
| Kubernetes Security Exam | `/bench/scenarios?exam=kubernetes-security` | `/bench/leaderboard?exam=kubernetes-security` |
| GitOps And Release Exam | `/bench/scenarios?exam=gitops-release` | `/bench/leaderboard?exam=gitops-release` |
| Terraform And Cloud Ops Exam | `/bench/scenarios?exam=terraform-cloud` | `/bench/leaderboard?exam=terraform-cloud` |
| MCP Readiness Exam | `/bench/scenarios?exam=mcp-readiness` | `/bench/leaderboard?exam=mcp-readiness` |

## Why This Layer Exists

The raw scenario catalog is good for builders. Exam suites are better for
buyers and public comparison:

- They make the product story obvious: agents take live infrastructure exams.
- They create leaderboard slices that are easy to compare.
- They give scenario authors a target portfolio instead of a loose backlog.
- They let Bench grow a public moat through hard, real scenarios that are
  expensive for competitors to reproduce.

## Public Versus Private

Public exam suites should stay generic enough to be credible marketing proof:
Kubernetes operations, security, GitOps, Terraform, cloud controls, and MCP
readiness.

Private suites should use customer-specific incidents, internal workflows,
provider constraints, and historical outages. That is the paid product surface:
scheduled regressions, external readiness reports, and failure autopsy on the
customer's own cases.

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
