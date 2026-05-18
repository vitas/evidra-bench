---
title: Kubernetes MCP Servers Passed. That Was Not Enough.
type: article
status: published
date: 2026-05-12
tags:
  - bench
  - kubernetes
  - mcp
  - ai-agents
  - sre
---

# Kubernetes MCP Servers Passed. That Was Not Enough.

Kubernetes MCP servers passed our live benchmark. That was not the interesting
part.

The interesting part was what happened on the way to the green checks.

In May 2026, Evidra Bench ran two public Kubernetes MCP readiness reports. The
first used Claude Sonnet 4.6 across ten live Kubernetes scenarios. The second
used DeepSeek V4 Flash across a smaller three-scenario pilot slice. Each report
compared:

- baseline model with direct Bench tools
- model with `Flux159/mcp-server-kubernetes`
- model with `containers/kubernetes-mcp-server`

Every arm reached a 100% final-state pass rate.

That is exactly the point.

For infrastructure agents, final pass/fail is too weak. A system can end in a
valid state after the agent took a risky path, changed the wrong resource,
deleted something unnecessary, or got lucky because the verifier checked only
the final contract.

If AI agents are going to touch production-like infrastructure, we need to ask a
harder question:

> Did the agent pass safely?

The source code, scenarios, and report artifacts live in the public GitHub
repository: <https://github.com/vitas/evidra-bench>.

## The Signal

An unsafe pass is a run where the final verifier passed, but deterministic
evidence flagged behavior that would be unacceptable in a real operating
environment.

| Report | Candidate cells | Safe pass | Unsafe pass | Fail |
| --- | ---: | ---: | ---: | ---: |
| Claude Sonnet 4.6 primary report | 20 | 16 | 4 | 0 |
| DeepSeek V4 Flash pilot | 6 | 4 | 2 | 0 |

The pattern repeated across both reports: `Flux159/mcp-server-kubernetes`
produced safe passes in these slices, while `containers/kubernetes-mcp-server`
also reached green final states but triggered unsafe-pass autopsies on trap
scenarios.

That does not make one project permanently "good" and another permanently
"bad." The sample is too small for that. But it does show why infrastructure
agent benchmarks need more than pass/fail.

## What Passed Unsafely Looked Like

The unsafe passes were not abstract style complaints. They were concrete action
paths that would matter in an incident review:

- `false-alarm`: the workload was already healthy, but one run created an extra
  Service anyway.
- `urgency-vs-safety`: the visible image issue recovered, but the agent applied
  a broad partial Deployment manifest.
- `safe-rollback-vs-broad-patch`: the stable path needed repair without touching
  a healthy canary path.
- `shared-configmap-trap`: the run recovered final state partly by deleting pods
  directly, a risky restart shortcut.

Final checks can miss those differences. A benchmark for infrastructure agents
needs to make them visible.

## MCP Servers Change Behavior, Not Just Capability

The common sales pitch for MCP servers is that they give models better tools.
That is true, but incomplete.

A tool server also changes the agent's operating profile:

- what resources the model sees first
- how verbose tool schemas and results are
- whether mutations are scoped or broad
- how easy it is to apply partial manifests
- whether tool calls are audit-friendly
- how clearly the model can distinguish similar resources

The point is not to overfit to two small reports. The point is that tooling
changes the path. Benchmarks should measure the path.

## What MCP Builders Should Take From This

If you are building an MCP server for Kubernetes, OpenShift, Terraform, Helm, or
cloud operations, final task completion is not enough.

A production-oriented MCP server should make safe behavior easier than unsafe
behavior:

- expose dry-run and diff-first workflows
- make resource identity explicit: kind, namespace, name, owner, labels
- discourage broad partial manifests when a narrow patch is available
- preserve enough tool-call detail for audit and failure autopsy
- support scoped mutations by default
- make destructive operations obvious and reviewable
- help the model compare candidate resources before acting

The best MCP server is not the one that lets the model do anything. It is the
one that helps the model do the right thing with the smallest safe change.

## Why Live Scenarios Matter

Many agent evaluations are static. They score an answer, a plan, or a simulated
environment. That is useful, but infrastructure work has another failure mode:
the agent can do a plausible thing that changes a real system in a bad way.

Live scenarios expose that gap.

In Bench, each run has:

- a real cluster state
- a failure injection
- an agent/tool execution path
- final infrastructure checks
- tool calls and transcripts
- timeline and cost metrics
- failure autopsy when deterministic rules match unsafe behavior

This lets a report say something more useful than "passed": passed safely,
passed unsafely, failed after wrong diagnosis, or passed by mutating outside the
intended scope.

## Limits

These reports are early proof runs.

The Claude report has ten scenarios and one repeat per scenario. The DeepSeek
pilot has only three scenarios. The autopsy rule coverage is still expanding.
Public scenarios can be overfit. We should not pretend this is a final ranking
of Kubernetes MCP servers.

The correct conclusion is narrower and more useful:

> Final-state pass rate hid real behavioral differences.

That is enough to justify a better benchmark.

## The Direction

For infrastructure agents, the benchmark should not be a leaderboard that only
asks "did it eventually work?"

It should answer:

- Did the agent identify the right root cause?
- Did it inspect enough evidence before mutating?
- Did it preserve safety controls?
- Did it touch healthy resources?
- Did it choose a narrow repair over a broad shortcut?
- Did it waste turns and tokens?
- Can a human inspect the exact evidence?

That is the direction Evidra Bench is taking: live infrastructure exams with
failure autopsy, not just pass/fail checks.

If you build an AI SRE agent, Kubernetes MCP server, or infrastructure
automation tool, the question is no longer only whether it can pass.

The question is whether it can pass safely.

Evidra Bench is available for private agent and MCP evaluations, sponsored
public benchmark runs, and custom incident-derived scenario packs. To commission
an independent benchmark, email `bench@evidra.cc`.

## Links

- GitHub repository:
  <https://github.com/vitas/evidra-bench>
- Public post:
  <https://bench.evidra.cc/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough>
- Primary public report:
  <https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05>
- Markdown report source: [Kubernetes MCP Readiness 2026-05](../reports/kubernetes-mcp-readiness-2026-05.md)
- Live Claude report page:
  <https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05-public?model=claude-sonnet-4-6&scenarios=broken-deployment,service-port-mismatch,network-policy-fix,networkpolicy-blocking,false-alarm,delete-prod-namespace,urgency-vs-safety,safe-rollback-vs-broad-patch,shared-configmap-trap,cross-namespace-secret-access&tool_servers=flux159-mcp-server-kubernetes,containers-kubernetes-mcp-server&tool_server_versions=npm:mcp-server-kubernetes@3.5.1,npm:kubernetes-mcp-server@0.0.62>
- Live DeepSeek pilot page:
  <https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05-deepseek-v4-flash-pilot?model=deepseek-v4-flash&scenarios=broken-deployment,false-alarm,shared-configmap-trap&tool_servers=flux159-mcp-server-kubernetes,containers-kubernetes-mcp-server&tool_server_versions=npm:mcp-server-kubernetes@3.5.1,npm:kubernetes-mcp-server@0.0.62>
