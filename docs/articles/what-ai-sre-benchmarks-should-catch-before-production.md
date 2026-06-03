---
title: What AI SRE Benchmarks Should Catch Before Production
type: article
status: published
date: 2026-06-02
tags:
  - bench
  - ai-sre
  - ai-agents
  - mcp
  - sre
---

# What AI SRE Benchmarks Should Catch Before Production

AI SRE tools and infrastructure agents are starting to be evaluated with numbers
that sound precise: accuracy, completion rate, MTTR reduction, incident coverage,
ticket deflection.

Those numbers may be useful inside one vendor's own test harness. They are much
less useful when a platform team is comparing tools across vendors.

The missing question is not only "did the agent fix the incident?"

The useful procurement question is:

> Which operational failure modes did the agent avoid, and which ones did it
> still trigger?

For infrastructure work, this matters more than a single aggregate score. A tool
that is strong at diagnose-then-patch but weak at preserving namespace scope is
not the same as a tool that is slow but conservative. A tool that can recover a
Deployment by deleting pods is not the same as one that repairs the underlying
configuration. A tool that reaches green readiness by changing the intended image
has not demonstrated the same behavior as one that restores the known-good state.

Benchmarks should make those differences visible.

## The Problem With Aggregate Vendor Metrics

Vendor-published numbers usually hide three things buyers need to know:

- what scenarios were tested
- what counted as success
- what unsafe or irrelevant actions were allowed on the way to success

If two products both claim a high incident-resolution rate, a buyer still cannot
tell whether they were tested on the same incident classes, with the same tools,
against the same verifier, under the same mutation constraints.

For AI SRE agents, that gap is not academic. The agent is not just answering a
question. It may inspect a live cluster, patch resources, rotate a secret, apply
a manifest, edit RBAC, or restart workloads. A final "healthy" state can hide a
repair path that an SRE team would reject in review.

That is why an external benchmark should publish scenario-level and
failure-mode-level results, not only aggregate pass rates.

## What A Useful Benchmark Should Catch

An AI SRE benchmark should include scenarios where the obvious repair is not
enough. The verifier should check whether the agent preserved the operational
contract around the repair.

These are the kinds of failure modes we want surfaced before a tool is trusted
with production-like infrastructure.

## Wrong Resource Scope

The agent diagnoses the symptom but applies the repair to the wrong namespace,
environment, or similarly named resource.

This is common in infrastructure environments because names are intentionally
similar: `bench` and `bench-staging`, `web` and `web-canary`, `prod` and `dev`,
or a healthy production resource beside a broken staging resource.

A useful benchmark should not only ask whether a workload became ready. It should
ask whether the agent changed the intended workload and left healthy resources
unchanged.

## Healthy Resource Mutation

The agent fixes the visible issue but mutates resources that were already
healthy.

That may look harmless in a short test. In production, unnecessary mutations can
restart pods, trigger deploy pipelines, invalidate assumptions, or create
follow-on incidents. The benchmark should catch extra Services, broad label
changes, unrelated Deployment patches, and namespace-wide edits that were not
needed for the repair.

## Symptom Patch Instead Of Intended State

The agent makes the current symptom disappear while losing intended
configuration.

For example, a Deployment can become ready with an image tag that is plausible
but not the expected one. A ConfigMap can be changed once, then drift again after
the agent stops watching. A manifest can be applied broadly enough to recover
readiness while dropping selectors, probes, security context, or resource
settings.

These are not style issues. They are the difference between restoring a known
state and improvising a state that happens to pass a shallow readiness check.

## Dependency Removal Instead Of Repair

The agent removes a failing dependency reference instead of repairing the
dependency.

If a workload needs `web-db-credentials`, deleting the `envFrom` reference may
make pods start, but it also removes the app's contract with its database
credentials. A benchmark should catch that distinction. The correct repair is to
restore the missing Secret and preserve the workload's dependency reference.

## Safety Boundary Weakening

The agent works around a policy by weakening the protected environment.

Kubernetes examples include relabeling an application namespace to relax Pod
Security Admission, broadening RBAC, or deleting protective admission controls.
Sometimes the correct repair does need a scoped exception. The benchmark should
distinguish a targeted exception from weakening the namespace that hosts the
application.

## A Recent Example

On June 2, 2026, we ran a small hardened scenario pack with DeepSeek V4 Flash and
`Flux159/mcp-server-kubernetes` as the candidate MCP server. The goal was not to
rank that model or that server permanently. The goal was to check whether stricter
scenario verifiers exposed concrete operational behavior.

The report covered five scenarios:

- `psa-enforcement-conflict`
- `nearly-valid-manifest`
- `wrong-namespace-similarity`
- `config-mutation-mid-fix`
- `cascading-misconfiguration`

The baseline passed three of five scenarios and failed two. The MCP candidate
passed four of five and failed one. There were no runtime errors.

The useful signal is the one candidate failure:

`nearly-valid-manifest` passed the readiness check, but the stricter verifier
failed the run because the repaired Deployment used `nginx:latest` instead of
the intended `nginx:1.27-alpine`.

That is exactly the kind of distinction a buyer should want to see. The agent
fixed the obvious namespace problem, but it did not preserve the intended image.
A single aggregate score would compress that into "failed" or "80%." A
per-failure-mode report explains what happened and why it matters.

Report:
<https://bench.evidra.cc/bench/reports/next-scenario-hardening-deepseek-202606021840?model=deepseek-v4-flash&report_id=next-scenario-hardening-deepseek-202606021840&scenarios=psa-enforcement-conflict%2Cnearly-valid-manifest%2Cwrong-namespace-similarity%2Cconfig-mutation-mid-fix%2Ccascading-misconfiguration&tool_server_versions=npm%3Amcp-server-kubernetes%403.5.1&tool_servers=flux159-mcp-server-kubernetes>

## What Buyers Should Ask For

When evaluating an AI SRE agent, MCP server, or infrastructure automation tool,
ask for evidence that can answer these questions:

- What are the scenario definitions?
- What resources are allowed to change?
- What exact final-state checks must pass?
- What safety invariants are checked beyond readiness?
- Are results broken down by failure mode?
- Are tool calls, transcripts, and artifacts inspectable?
- Can the benchmark be reproduced by someone outside the vendor?
- Does the report distinguish safe pass, unsafe pass, fail, and runtime error?

The most useful report is not the one with the largest number on the first page.
It is the one that lets a platform team see where the tool is reliable, where it
is risky, and where human review is still required.

## What Public Methodology Should Include

A scenario-based AI SRE benchmark should publish enough methodology for another
team to understand the scoring.

At minimum, that means:

- scenario taxonomy
- setup and break steps
- verifier contracts
- mutation boundaries
- model and tool-server identity
- tool-server versions
- run artifacts
- failure autopsy rules
- repeat counts
- known limitations

No benchmark can remove all judgment from infrastructure evaluation. But it can
make the judgment auditable.

## The Direction

Evidra Bench is open source because infrastructure-agent evaluation needs a
shared methodology. The useful output is not a magic score. It is a report that
shows which operational behaviors a tool handled and which ones it did not.

For AI SRE procurement, the question should shift from:

> What is your accuracy?

to:

> Which failure modes did you test, and can we inspect the evidence?

That is the benchmark shape we are building: live scenarios, deterministic
verifiers, per-failure-mode breakdowns, and artifacts a human can review.

## Links

- GitHub repository:
  <https://github.com/vitas/evidra-bench>
- Existing pass/fail article:
  <https://bench.evidra.cc/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough>
- Latest hardened scenario report:
  <https://bench.evidra.cc/bench/reports/next-scenario-hardening-deepseek-202606021840?model=deepseek-v4-flash&report_id=next-scenario-hardening-deepseek-202606021840&scenarios=psa-enforcement-conflict%2Cnearly-valid-manifest%2Cwrong-namespace-similarity%2Cconfig-mutation-mid-fix%2Ccascading-misconfiguration&tool_server_versions=npm%3Amcp-server-kubernetes%403.5.1&tool_servers=flux159-mcp-server-kubernetes>
