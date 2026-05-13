---
title: Threat Model
type: security
status: active
tags:
  - bench
  - security
  - threat-model
---

# Threat Model

Bench executes agents against live infrastructure fixtures. That is useful, but
it creates real security boundaries that contributors and operators need to
respect.

## Assets

- API keys and provider credentials
- hosted Bench API credentials
- private run transcripts and artifacts
- customer incident scenarios and snapshots
- local kubeconfigs and cluster state
- runner host filesystem and Docker socket
- public report integrity

## Trust Boundaries

| Boundary | Risk |
|---|---|
| Browser UI to Bench API | Public pages must not embed write-capable secrets. |
| Bench service to runner | Runner jobs can execute infrastructure actions. |
| Runner to local host | Docker socket and kubeconfig access can affect the host. |
| Agent output to verifier/report | Prompt-injected or misleading text must not decide scoring alone. |
| Private artifacts to public repo | Transcripts and snapshots can leak secrets or customer data. |

## Main Risks

- exposing `BENCH_API_KEY` or provider keys in browser builds
- committing private run artifacts or database dumps
- running untrusted agent commands on a non-disposable host
- letting agent self-reports override verifier checks
- publishing private customer scenario details
- using broad Kubernetes or cloud credentials in local tests

## Current Mitigations

- generated runs and databases are ignored by git
- UI secret hygiene is checked in CI
- public read APIs are separated from authenticated write paths
- scenario verification uses structured checks instead of trusting agent text
- contribution docs require sanitized fixtures and artifacts
- private deployment topology and secrets are outside the public repo

## Operator Guidance

- Run live scenarios in disposable clusters and isolated workspaces.
- Use least-privilege credentials for providers, Kubernetes, and cloud fixtures.
- Do not mount production kubeconfigs into local benchmark runs.
- Treat external MCP servers and CLI agents as untrusted code.
- Redact transcripts before sharing them publicly.

## Non-Goals

Bench is not a sandbox for arbitrary untrusted code. It is a benchmark harness
for controlled evaluation environments. Operators are responsible for choosing
safe runners, credentials, and network boundaries.
