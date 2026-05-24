---
title: Testing Methodology
type: reference
status: active
tags:
  - bench
  - methodology
  - scenarios
  - agents
---

# Testing Methodology

Bench evaluates infrastructure agents on real operational tasks. A run is not a
quiz and not a scripted tutorial. The harness creates a working environment,
injects a known failure, asks the agent to restore the desired state, and then
verifies the result with declarative checks.

## What Bench Tests

### 1. Remediation Capability

Can the agent fix the infrastructure problem?

Each scenario starts from a healthy baseline and injects a realistic failure:
wrong image tags, missing secrets, broken NetworkPolicies, Helm drift, Argo CD
sync errors, Terraform state problems, or LocalStack-backed AWS issues.

Success is measured by final infrastructure checks, not by the method the agent
used. Examples:

- deployment becomes ready
- service endpoints exist
- Helm release is deployed
- Argo CD application becomes healthy
- resource still exists after remediation
- security constraint remains in place

### 2. Diagnostic Quality

Did the agent investigate enough before changing state?

Bench classifies tool calls into timeline phases:

- discovery
- diagnosis
- action
- verification
- explanation

The goal is not to force a specific command sequence. The goal is to catch
behavior such as immediate mutation without inspection, failure to check logs or
events, and repeated low-value inspection after the root cause is already clear.

### 3. Judgment Under Constraints

Can the agent make the right trade-off?

L3 and L4 scenarios include traps or ambiguity. A good agent must avoid broad
deletes, unsafe policy changes, weakening security controls, or fixing the
wrong component just because it is the most visible symptom.

Scenario constraints are outcome-based:

- "The application must remain functional."
- "Do not delete the deployment."
- "The security hardening must remain in place."
- "Minimize changes outside the affected namespace."

### 4. Failure Analysis

When the agent fails, Bench should explain why.

The failure-autopsy layer uses transcripts, tool calls, timelines, verifier
output, turns, tokens, and cost to classify failures such as:

- `gave_up`
- `timeout_no_progress`
- `retry_loop`
- `premature_success`
- `wrong_root_cause`
- `unsafe_action`
- `irrelevant_action`
- `missed_diagnostic_step`
- `tool_misuse`
- `excessive_token_burn`

See [Agent Failure Autopsy](AGENT_FAILURE_AUTOPSY.md).

### 5. Efficiency

Did the agent solve the task with reasonable time and budget?

Runs track:

- turns
- duration
- prompt tokens
- completion tokens
- estimated cost
- memory window

Efficiency only matters after correctness. A cheap failed run is still a failed
run. The useful comparison is between passing runs, or between a previously
passing run and a regression.

## Scenario Levels

| Level | Name | Prompt style | What it tests |
|---|---|---|---|
| L1 | Fix | Give the symptom | Can the agent execute a clear repair? |
| L2 | Diagnose | Give the affected area | Can the agent find the cause? |
| L3 | Judge | Give a concern and constraints | Can the agent choose a safe fix? |
| L4 | Investigate | Give an incident report | Can the agent trace across multiple resources? |

## Scenario Mix

A useful benchmark set should include:

- clean baseline tasks for fast sanity checks
- ambiguous operational tasks for diagnosis and judgment
- adversarial or cross-cutting tasks that expose unsafe shortcuts
- multi-stage tasks where the agent must adapt after the first fix
- chaos tasks where the environment changes during the run

The recommended distribution is:

- 20% clean baseline tasks
- 60% ambiguous operational tasks
- 20% adversarial or cross-cutting tasks

## Chaos Injection

Chaos is deliberately narrow and deterministic. It should create repeatable
operational pressure without turning Bench into a general chaos platform.

Current chaos patterns include:

- pod restarts during repair
- mounted ConfigMap drift during repair
- staged failures that appear after earlier checks pass

Chaos scenarios are useful when the question is:

> Does the agent stay reliable when the environment changes underneath it?

## Memory Window Testing

`--memory-window` controls how much conversation history the agent sees on each
turn:

| Value | Behavior | What it tests |
|---|---|---|
| `-1` | Full history | Baseline behavior with all context |
| `0` | Stateless | Can the agent solve each step from the last observation only? |
| `1` | Last exchange | Minimal short-term memory |
| `3` | Last 3 exchanges | Short plan retention |
| `10` | Last 10 exchanges | Moderate history with lower token cost |

Examples:

```bash
# Full memory
bench-cli run \
  --provider claude \
  --model sonnet \
  --scenario kubernetes/broken-deployment \
  --memory-window -1

# Stateless
bench-cli run \
  --provider claude \
  --model sonnet \
  --scenario kubernetes/broken-deployment \
  --memory-window 0
```

Useful findings:

- passes with `0`: the task may not require long-term planning
- fails with `0` but passes with `-1`: history matters for this scenario
- pass rate holds while tokens fall: smaller memory windows may be cheaper
- both fail: model/tooling/scenario difficulty is the limiting factor

## Model And Tool-Server Comparison

Bench comparisons should keep everything fixed except the variable under test:

- same scenario set
- same cluster profile
- same timeout
- same memory window
- same provider settings where possible
- one changed model, skill, tool server, or adapter

Examples:

```bash
# Same scenario, different models
bench-cli run --provider bifrost --model openai/gpt-4o --scenario kubernetes/broken-deployment
bench-cli run --provider bifrost --model google/gemini-2.5-flash --scenario kubernetes/broken-deployment

# Same model, selected MCP server
bench-cli run --provider bifrost --model sonnet --scenario kubernetes/broken-deployment
bench-cli run --provider bifrost --model sonnet --scenario kubernetes/broken-deployment \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION"
```

## Provider Path

In provider mode, Bench owns the tool-use loop:

```text
bench-cli
  -> Provider.Chat()
  -> model returns tool calls
  -> Bench executes tools locally
  -> tool results feed the next turn
```

Supported provider paths include Bifrost-compatible OpenAI-style APIs and the
Claude CLI provider.

```bash
INFRA_BENCH_BIFROST_URL=https://api.openai.com/v1 \
INFRA_BENCH_BIFROST_AUTH_BEARER=sk-proj-... \
bench-cli run --provider bifrost --model gpt-4o --scenario kubernetes/broken-deployment
```

Tool schemas are sanitized for strict providers that require complete JSON
schema metadata.

## Adapter Path

Adapter mode lets an external process or remote agent manage its own loop while
Bench keeps setup and verification local.

Examples:

- CLI process adapter
- MCP server execution
- A2A remote agent execution

Adapter comparisons are useful for evaluating tool servers, orchestration
systems, and agent runtimes without changing the scenario or verifier.

## Run Comparison

Compare two run directories:

```bash
bench-cli compare runs/<run-A>/ runs/<run-B>/
```

The comparison should focus on:

- pass/fail change
- check-level changes
- duration delta
- turns delta
- token and cost delta
- tool-call or timeline differences when artifacts exist

## Cost Tracking

Every provider-path run estimates cost from token usage when pricing is known.
Cost appears in:

- run metadata
- local DB queries
- compare output
- Bench API run records
- dashboard views

Cost should not be optimized in isolation. The practical target is lower cost
for the same or better pass rate.

## Results Storage

Every non-dry-run stores structured results locally:

```bash
bench-cli db stats
bench-cli db query --scenario broken-deployment
bench-cli db query --model haiku --failed
bench-cli db rebuild
bench-cli audit coverage --runs-dir runs
```

Storage model:

- `runs/bench.db` - local SQLite cache/index, gitignored, queryable
- `runs/results.jsonl` - append-only backup
- run artifacts - transcript, tool calls, verifier output, scorecard, timeline,
  failure autopsy, run error, and run events

Records include scenario, model, provider, adapter, pass/fail, duration, turns,
memory window, prompt tokens, completion tokens, estimated cost, and checks.
Hosted storage is separate: PostgreSQL owns the control-plane API, runners,
analytics, and leaderboard state. Local runs sync to hosted storage through the
Bench ingest API.

## Batch Benchmark Pipeline

`bench-cli bench` runs scenarios with repeatable post-processing:

```text
run scenarios
  -> write artifacts
  -> derive timeline and scorecard when available
  -> preserve run error and lifecycle events for failed attempts
  -> audit artifact coverage across stored runs when needed
  -> audit expected signals
  -> store local and optional API results
```

Example:

```bash
bench-cli bench --provider claude --model sonnet --reuse-cluster
```

Typical artifact layout:

```text
runs/bench/<timestamp>/
  summary.json
  signal-audit.json
  <scenario_model_r1>/
    run.json
    verifier.json
    transcript.txt
    tool-calls.json
    timeline.json
    failure-autopsy.json
    run-error.json
    run-events.json
    scorecard.json
```

Scenarios with `skip: true` are excluded from benchmark runs with a reason
printed to stdout.

## Artifact Coverage Audit

Artifact coverage audit checks whether stored runs have enough local artifacts
to support deterministic failure analysis. It reads `runs/bench.db`, inspects
each run's `artifact_dir`, validates required JSON artifacts, and writes
`artifact-coverage.json`.

```bash
bench-cli audit coverage --runs-dir runs
bench-cli audit coverage --runs-dir runs --fail-on-gaps
```

The audit expects every stored run to have `run.json`, `tool-calls.json`,
`timeline.json`, and `run-events.json`. Failed runs must also have
`failure-autopsy.json`; error runs with negative exit codes must have
`run-error.json`. It also checks that `timeline.total_steps` matches the
number of tool calls.

## Signal Audit

Signal audit compares observed run artifacts against expectations in
`configs/signal-audit.yaml`:

```yaml
broken-deployment:
  primary_signal: retry_loop
  expected_signals: [retry_loop]
  forbidden_signals: [blast_radius]
```

```bash
bench-cli audit signals --runs-dir runs/e2e
```

The audit reports:

- missing expected signals
- forbidden signals
- unexpected secondary signals
- unstable repeated-run groups

Signal audit is an intermediate layer. Failure autopsy should turn these
low-level findings into clearer product reports.

## Optional Compatibility Checks

Some scenarios can still read file-based evidence artifacts when the run
explicitly provides an evidence directory. Treat these as compatibility checks,
not as the core Bench product path.

Normal infrastructure verification always runs regardless of optional artifact
checks.
