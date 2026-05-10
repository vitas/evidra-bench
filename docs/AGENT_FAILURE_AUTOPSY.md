---
title: Agent Failure Autopsy
type: product
status: active
tags:
  - bench
  - agents
  - failure-analysis
  - analytics
---

# Agent Failure Autopsy

Agent failure autopsy is the product layer that explains why a run failed, not
only that it failed. The goal is to turn raw transcripts, tool calls, timeline
events, verifier output, tokens, and costs into a report a platform team can
act on.

The MVP scope and implementation sequence are tracked in
[Autopsy MVP Design](plans/2026-05-10-autopsy-mvp-design.md) and
[Autopsy MVP Implementation Plan](plans/2026-05-10-autopsy-mvp.md).

## Problem

Pass/fail is too coarse for agent regression testing. A failed run can mean many
different things:

- the agent never found the broken resource
- it found the issue but fixed the wrong cause
- it repeated the same command until timeout
- it changed a risky resource outside the scenario scope
- it claimed success before verification passed
- it spent most of its token budget on irrelevant inspection

These failures imply different fixes. Some need a better model, some need a
better tool server, some need prompt changes, and some need scenario or policy
changes.

## Target Report

```text
Outcome: FAIL
Root failure: premature_success
Gave up at: turn 11
Wasted turns: 7 / 14
Wasted tokens: 8420 / 13100
Loop detected: repeated kubectl get pods 4 times
Missed diagnostic: never inspected deployment events
Wrong action: restarted pod instead of fixing image
Verification failure: deployment/web never became ready
```

The report should be deterministic when possible. LLM-generated narrative can
be added later, but the MVP should come from structured artifacts and simple
classifiers.

## Normalized Run Trace

Every execution adapter should eventually feed the same trace model:

```text
turn_started
assistant_message
tool_call_started
tool_call_finished
environment_observation
verification_check
agent_final_answer
timeout
token_usage
```

Current Bench artifacts already cover part of this:

- `transcript.txt`
- `tool-calls.json`
- verifier `checks_json`
- run metadata: turns, duration, tokens, estimated cost
- timeline from `pkg/bench/timeline_*`
- scorecard artifacts when available

The first implementation can derive most findings from tool calls, verifier
results, run metadata, and final transcript text.

## Failure Taxonomy

| Failure | Meaning | Likely data source |
|---|---|---|
| `gave_up` | Agent explicitly stops or says it cannot continue before checks pass | final answer, transcript |
| `timeout_no_progress` | Run times out after repeated low-information turns | timeout, timeline, repeated commands |
| `retry_loop` | Same action or same failed diagnostic repeats several times | tool-call command normalization |
| `premature_success` | Agent claims completion before verifier checks pass | final answer, verifier output |
| `wrong_root_cause` | Agent fixes a plausible but incorrect cause | scenario expectations, failed checks, tool sequence |
| `unsafe_action` | Agent changes resources outside the allowed scope | scenario scope, mutating commands |
| `irrelevant_action` | Agent spends turns on unrelated tools or resources | command/resource extraction |
| `missed_diagnostic_step` | Agent skips a diagnostic expected for this scenario class | scenario metadata, timeline phases |
| `tool_misuse` | Agent calls a tool with invalid args or repeatedly hits usage errors | tool-call result text |
| `excessive_token_burn` | Token use is high relative to comparable passing runs | run metrics and historical baseline |

## MVP Scope

The first useful slice should stay small:

1. Normalize command strings from `tool-calls.json`.
2. Detect repeated command loops.
3. Detect premature success from final answer plus failed verifier checks.
4. Detect missing diagnosis depth before first mutation.
5. Detect excessive turn and token use against scenario/model history.
6. Emit a machine-readable `failure-autopsy.json`.
7. Show the summary in run detail pages and private reports.

## Scenario Expectations

Some findings need scenario context. The scenario schema can grow optional
failure-analysis hints later:

```yaml
autopsy:
  expected_diagnostics:
    - kubectl describe deployment
    - kubectl get events
  forbidden_actions:
    - kubectl delete namespace
  primary_failure_modes:
    - missed_diagnostic_step
    - premature_success
```

These hints should not tell the agent what to do. They are only for post-run
analysis.

## Adapter Independence

Failure autopsy should not depend on a specific agent protocol. The same report
should work for:

- built-in provider loop runs
- MCP server runs
- A2A remote-agent runs
- CLI process runs
- future provider-native traces

Adapters can provide richer events when available, but the classifier should
fall back to transcripts, tool calls, and verifier output.

## Future Work

- Cross-run clustering: "this model fails all network-policy scenarios the
  same way."
- Provider/tool-server attribution: "same model passes directly but fails
  through this MCP server."
- Cost-aware recommendations: "smaller memory window preserved pass rate and
  cut tokens by 42%."
- LLM-generated narrative on top of deterministic findings.
- Regression alerts for scheduled private runs.
