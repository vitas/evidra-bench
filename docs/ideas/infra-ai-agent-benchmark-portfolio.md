---
title: Infra AI Agent Benchmark Portfolio
type: idea
status: active
reviewed: 2026-05-08
tags:
  - bench
  - scenarios
  - kubernetes
  - ai-agents
  - mcp
---

# Infra AI Agent Benchmark Portfolio

Bench should optimize for scenario quality before scenario count.

The strongest product story is not "we have hundreds of tasks." The strongest
story is:

> Bench evaluates whether infra agents think before they act.

The public showcase should make weak agents fail in visible, explainable ways:
they jump to an obvious but wrong fix, mutate too early, miss a diagnostic
signal, ignore blast radius, repeat failed commands, claim success too early,
or burn tokens without making progress.

## Product Direction

Build a small flagship set of hard scenarios first, then grow broader suites.

1. Start with 10-20 hard `L3-L5` scenarios that demonstrate judgment.
2. Make failure autopsy visible for every showcase run.
3. Use these scenarios for the public leaderboard and sales demos.
4. Grow scenario count only after the hard suite is compelling.

Scenario count is useful for coverage, but it is a weak differentiator by
itself. Discriminative scenarios are more valuable: they separate agents that
inspect, reason, act safely, and verify from agents that merely call tools.

## Top Priority: Public Exam Suites

The near-term priority is public exam-style suites for AI infrastructure
agents.

This is the marketing wedge:

- easy to understand: agents take live infrastructure exams
- easy to share: leaderboard, per-track grade, failure examples
- credible: real environments, real tools, real verifier results
- extensible: Kubernetes first, then GitOps, Terraform, cloud operations, MCP
  readiness

These suites can be aligned to public skill domains such as Kubernetes admin,
application, and security tasks. They should not be presented as official CKA,
CKAD, CKS, HashiCorp, or AWS certifications.

The paid wedge is customer-specific:

- turn postmortems, tickets, alerts, manifests, and logs into private cases
- compare baseline agents, MCP servers, skills, and models
- run scheduled regressions against the customer's own failure modes

## Live Bench Moat

Bench should prioritize real, live infrastructure benchmarks over read-only
simulators.

The moat is not the YAML catalog. The moat is the operational work required to
make hard scenarios repeatable:

- provisioning real environments
- injecting realistic faults
- letting agents take real actions
- detecting unsafe mutations and blast radius
- verifying recovery from the actual final state
- preserving artifacts for failure autopsy

This is expensive and tedious to reproduce. That is the point. A competitor can
copy scenario descriptions, but copying a reliable live benchmark range with
hard Kubernetes, GitOps, Terraform, and MCP readiness scenarios is much harder.

External snapshot benchmarks are useful for mining fault taxonomies and
diagnostic ideas. They should not pull Bench away from its strongest niche:
live remediation and readiness testing.

Snapshot/RCA simulation can still become a future accelerator. The right order
is live moat first, simulator scale second.

## Live-Derived Snapshots

If Bench adds snapshot/RCA evaluation later, the best source is Bench's own
live scenarios.

A live run should be the source of truth. After a scenario runs, Bench can
export an incident snapshot from real artifacts:

- initial broken state
- relevant Kubernetes or infrastructure object YAML
- events, logs, metrics, traces, and alerts when available
- tool calls and transcript
- verifier output
- final state
- expected root cause and expected evidence from scenario metadata
- actual agent path and failure autopsy labels

This keeps the simulator path tied to real remediation. It avoids building an
isolated dataset product and turns snapshots into a byproduct of the live
benchmark range.

Minimal scenario metadata needed for this:

- `root_cause`
- `affected_objects`
- `expected_evidence`
- `acceptable_fixes`
- `risky_actions`
- `forbidden_actions`

The first implementation does not need a replay engine. It can export a
`snapshot-case.json` artifact for reports, scenario review, autopsy, and future
RCA evaluation.

## Bench Recorder Wedge

There is a related niche product:

> Bring your incidents. Bench turns them into private agent eval cases.

This should not become a general observability platform. The product is a
recorder and case builder for agent readiness:

- install a short-lived recorder with read-only RBAC by default
- scope collection to namespace, workload, cluster, and time window
- collect Kubernetes objects, events, logs, metrics, traces, alerts, and change
  history
- redact secrets, tokens, PII, internal hostnames, and customer-specific data
- export `incident.benchcase.json` plus supporting artifacts
- let the customer annotate expected root cause, evidence, allowed fixes, risky
  actions, and forbidden actions
- run the case against agents, models, skills, and MCP servers
- produce an autopsy/readiness report

The important positioning:

> Bench does not replace your observability stack. Bench converts your
> incidents into repeatable agent tests.

This would make every customer build a private benchmark suite from its own
failure modes. That is more valuable than a generic public leaderboard for paid
readiness work.

Good build order:

1. Hard live scenarios.
2. Failure autopsy and reporting.
3. `snapshot-case.json` export from Bench's own live runs.
4. Customer-facing `bench-recorder`.

## Scenario Difficulty Tiers

| Tier | Purpose | Example |
|---|---|---|
| L1 Basic | Sanity check simple tool use | Fix a wrong service selector |
| L2 Diagnostic | Find the right signal before acting | Pod Pending due quota, taint, or PVC issue |
| L3 Judgment | Choose between plausible fixes | Drain blocked by PDB; scale first instead of forcing eviction |
| L4 Adversarial | Resist traps and misleading context | Do not delete a health check just to make status green |
| L5 Multi-stage | Sequence and recovery matter | Canary failure, rollback, then verify downstream dependencies |

The product narrative should emphasize `L3-L5`.

## Recommended Suite Portfolio

### 1. Hard Infra Agent Judgment Suite

This should be the flagship suite.

Purpose:

- show whether an agent investigates before mutation
- test rollback, refusal, scope discipline, and verification
- produce strong autopsy reports when agents fail

Candidate scenarios:

- false alarm during a normal rolling update where the correct answer is no
  mutation
- risky shortcut where deleting a probe or health check hides the issue
- namespace/resource similarity trap where the wrong object looks plausible
- PDB blocks node drain and the safe path is to scale before draining
- GitOps drift where some diffs are intentional and some are accidental
- canary failure where rollback is safest, but deeper diagnosis can earn a
  higher score
- prompt injection inside logs or tickets that asks the agent to run unsafe
  commands
- shared ConfigMap or Secret where a broad patch fixes one service and breaks
  another

### 2. Kubernetes RCA Suite

This suite tests diagnosis inside live or live-derived Bench runs. It should
help explain why an agent acted, not replace remediation benchmarks with a
read-only simulator.

Purpose:

- measure root-cause quality using events, logs, metrics, traces, and object
  state
- capture the diagnostic path before first mutation
- compare agent diagnostic paths, not only final answers

The agent should output the root cause, supporting evidence, impacted
resources, and recommended next action. Scoring should include correctness,
evidence quality, missed diagnostics, time to root cause, turns, tokens, and
whether the answer stayed within the scenario scope.

Good sources of inspiration:

- [Cloud-OpsBench](https://github.com/LLM4Ops/Cloud-OpsBench), because it uses
  snapshot-based Kubernetes RCA with stored cluster state, logs, alerts, tool
  cache, and golden diagnostic trajectories.
- [o11y-bench](https://o11ybench.ai/), because it scores observability tasks
  across logs, metrics, traces, dashboards, and incident workflows, with
  leaderboard reporting for pass consistency and cost.

### 3. Kubernetes Remediation Suite

This suite tests whether the agent can safely fix the environment.

Purpose:

- run live infrastructure scenarios where the final state must be verified
- measure unsafe writes, blast radius, rollback behavior, and verification
- support public showcase runs and private customer regression tests

Candidate domains:

- workload failures: CrashLoop, ImagePull, OOM, probes, resources
- networking: Service, DNS, NetworkPolicy, Ingress, Gateway API, HTTPRoute,
  GRPCRoute
- storage: PVC binding, reclaim policy, expansion, filesystem resize
- control plane and node failures: kubelet, kube-proxy, scheduler, admission
  webhook
- application failures: OpenTelemetry Demo, flagd feature flags, cascading
  dependency failures

### 4. MCP Server Readiness Suite

This is the most commercial suite.

Purpose:

- evaluate whether a customer MCP server improves or hurts agent performance
- run the same scenario baseline and MCP-assisted
- produce a readiness report showing pass rate, cost, turns, token use, tool
  errors, and failure autopsy

The important output is a delta:

| Metric | Baseline Agent | Agent + MCP Server |
|---|---:|---:|
| Pass rate | measured | measured |
| Time to root cause | measured | measured |
| Unsafe actions | measured | measured |
| Token use | measured | measured |
| Repeated/failed tool calls | measured | measured |

This should be Bench-owned. External projects can inspire the comparison, but
Bench should own the suite format, runner, scoring, reports, and customer
readiness workflow.

## What To Build Ourselves

Bench should own the productized benchmark layer:

- scenario schema and authoring guide
- runner and control-plane integration
- reproducible environment setup
- scoring and verifier contracts
- failure autopsy labels
- public leaderboard reports
- private readiness reports
- no-MCP/native-tools baseline versus selected MCP server comparison

External OSS projects should be used for scenario research, fixture ideas, and
optional upstream-compatible assets. They should not become required runtime
dependencies for the core product.

## External Landscape

| Project | What It Shows | What Bench Should Borrow | What Bench Should Avoid |
|---|---|---|---|
| [henrikrexed/k8s-ai-agent-benchmark](https://github.com/henrikrexed/k8s-ai-agent-benchmark) | Kubernetes observability benchmark comparing HolmesGPT, Kagent, and Sympozium across basic K8s, Gateway API, and OpenTelemetry Demo failures | Scenario taxonomy, Gateway API and flagd ideas, KPI set, same-prompt comparison | Proxmox/Dynatrace-heavy setup as required architecture, manual UI workflow |
| [Cloud-OpsBench](https://github.com/LLM4Ops/Cloud-OpsBench) | Snapshot-based RCA benchmark for Kubernetes cloud systems with many fault cases and golden trajectories | Fault taxonomy, RCA evidence concepts, expert diagnostic paths | Making snapshot replay the core product direction |
| [AIOpsLab](https://github.com/microsoft/AIOpsLab) | Framework for deploying apps, generating workload, injecting faults, exporting telemetry, and evaluating agents | Problem contract: application, task, fault, workload, evaluator | Adopting a large framework when Bench already has runner architecture |
| [NetArena](https://github.com/Froot-NetSys/NetArena) | Dynamic benchmarks for network automation agents with realistic emulator feedback | Dynamic task generation, network-policy style scenarios, A2A-style agent boundary | Network-specific scope as the main product |
| [ITBench](https://github.com/itbench-hub/ITBench) | IT automation benchmark across SRE, CISO, and FinOps with Kubernetes environments and leaderboards | Broader suite taxonomy: SRE, security, cost | Competing as a general IT benchmark too early |
| [o11y-bench](https://o11ybench.ai/) | Observability benchmark across logs, metrics, traces, dashboards, and incidents | Pass consistency, cost reporting, category breakdowns | Observability-only scope |
| [mcpbr](https://mcpbr.org/) | MCP-assisted vs baseline agent performance comparison | Delta reporting for tool-assisted runs | Generic task catalog without infra-specific depth |
| [agentevals](https://github.com/agentevals-dev/agentevals) | Agent evaluation from OpenTelemetry traces and expected tool trajectories | Trace-based scoring and tool trajectory checks | Replacing Bench's scenario/verifier layer |

## Competitive Risk From Incident/RCA Platforms

Companies such as Komodor, Robusta, RunWhen, Kubeshark, Grafana, Datadog,
Dynatrace, and New Relic are adjacent. They already collect many of the signals
needed to build incident snapshots: logs, metrics, events, traces, changes,
traffic, alerts, and deployment history.

Nothing technical prevents them from adding "turn this incident into an agent
eval" if the market becomes obvious. That is the main competitive risk.

Bench can still have a wedge because the product goal is different:

- Incident/RCA platforms optimize for resolving today's incident.
- Bench optimizes for testing whether an agent can handle tomorrow's incident.

Why they may not move first:

- Their buyer is usually the SRE/ops team trying to reduce MTTR, not the AI
  platform team validating agents and MCP servers.
- If they ship their own AI/RCA assistant, grading competing agents and
  customer MCP servers can create a neutrality problem.
- Recording signals is not enough. A benchmark needs scenario contracts,
  isolated execution, verifiers, scoring, safety checks, blast-radius analysis,
  and reproducible reports.
- Live remediation benchmarks are operationally annoying: environments must be
  provisioned, broken, repaired, verified, reset, and audited repeatedly.
- Customer incident data needs strong redaction, tenant isolation, consent, and
  export controls before it can become an eval asset.

Bench should assume these platforms can enter later. The defensive move is to
own the category early:

> external regression testing for infra agents and MCP tools

The strongest moat is not having a recorder. It is combining recorder output
with live remediation scenarios, failure autopsy, and neutral readiness reports
across agents and tool servers.

## Later Accelerator: Snapshot Simulator

Bench should not prioritize building a full read-only environment simulator
right now.

Simulator-style benchmarks are useful, but they are easier for others to copy
and easier to reduce to brittle trajectory matching. They also do not prove the
hardest thing customers care about: whether an agent can safely change real
infrastructure and verify recovery.

For now, use simulator and dataset projects as scenario research:

- extract realistic fault categories
- study expected diagnostic evidence
- identify common misleading signals
- adapt strong cases into live Bench scenarios

Do not build a snapshot replay engine until live remediation coverage and MCP
readiness reports are clearly stronger.

When live suites are credible, snapshot/RCA simulation may become useful as:

- a cheaper public leaderboard layer
- a fast regression filter before expensive live runs
- a way to compare many models across many incidents
- a way to collect sanitized customer incident cases
- an input to failure autopsy and diagnostic-path scoring
- a replay format derived from verified live Bench scenarios

The boundary should remain clear:

- snapshot mode asks: did the agent understand the incident?
- live Bench asks: did the agent safely fix the system?

## Notes On k8s-ai-agent-benchmark

This repository is useful as scenario research, not as a direct dependency.

It currently frames itself as an AI observability benchmark for Kubernetes. The
README describes comparison of HolmesGPT, Kagent, and Sympozium. Its setup uses
Cluster API-managed kubeadm clusters on Proxmox, a self-hosted LLM path,
Dynatrace, Gateway API, kgateway, Istio, OpenTelemetry Collector,
OpenTelemetry Demo, MCP servers, and tool-specific UIs.

The interesting scenario shape:

- Tier 1: basic Kubernetes failures such as CrashLoopBackOff, ImagePullBackOff,
  OOMKilled, Pending Pods, Service connectivity, and Gateway API
  misconfiguration
- Tier 2: Gateway API routing failures with HTTPRoute and GRPCRoute
- Tier 3: OpenTelemetry Demo application failures via flagd feature flags

The interesting KPIs:

- diagnosis accuracy
- token consumption
- CPU and memory resource usage
- time to root cause
- user interactions
- integrations available

Bench should adapt those ideas into automated, reproducible suites. It should
not require that vendor/tool stack as the core path.

## Quality Bar For Hard Scenarios

Every hard scenario should define:

- the trap or misleading shortcut
- expected diagnostics before first mutation
- allowed and forbidden mutating actions
- expected blast radius
- verifier checks for final state
- autopsy hints for common failure modes
- scoring beyond pass/fail

Recommended scoring dimensions:

- diagnostic depth before first write
- correctness of root cause
- unsafe action count
- wrong hypothesis count
- verification quality
- rollback behavior
- repeated action loops
- token and turn waste

These dimensions make Bench more useful than a pass/fail benchmark. They show
where the agent failed and what needs to improve.

## First Build Target

The next useful product milestone should be:

1. Add 5 hard `L3-L5` showcase scenarios.
2. Ensure each has autopsy expectations.
3. Run at least three agent/provider configurations.
4. Publish a public report that explains both wins and failures.
5. Use the same scenarios as the seed for private MCP readiness reports.

Good first five:

- false alarm with no safe mutation required
- risky shortcut health-check/probe trap
- PDB-blocked drain with safe scale-first path
- Gateway API route mismatch with misleading service health
- flagd or feature-flag cascade in OpenTelemetry Demo
