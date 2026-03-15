# Testing Methodology

## What infra-bench Tests

infra-bench evaluates AI infrastructure agents on three dimensions:

### 1. Remediation Capability

Can the agent diagnose and fix a real infrastructure problem?

Each scenario breaks something in a Kubernetes cluster (wrong image, missing
ConfigMap, failed Helm upgrade, ArgoCD drift) and asks the agent to fix it.
Success is measured by declarative checks: deployment ready, service endpoints
reachable, Helm release deployed, ArgoCD app healthy.

Some scenarios also inject deterministic runtime chaos while the agent is
working, such as deleting pods mid-repair or mutating a ConfigMap after the
agent has already started reasoning about it.

This tests **operational competence** — does the agent understand Kubernetes
well enough to find and fix real problems?

### 2. Protocol Compliance

Does the agent follow the prescribe/report protocol while working?

The Evidra protocol requires agents to declare intent (prescribe) before
mutating infrastructure and report the outcome after. This creates an evidence
chain that enables reliability scoring. Scenarios with `evidra:` expectations
verify:

- Every mutation was prescribed before execution
- Every prescription has exactly one matching report
- Risk levels match expectations
- Declined verdicts are recorded with reasoning

This tests **operational discipline** — does the agent follow safety protocols
even when under pressure to fix things quickly?

### 3. Behavioral Signals

Does the agent's behavior trigger the right reliability signals?

The Evidra signal engine detects behavioral patterns in evidence chains:

| Signal | What it means | Healthy? |
|--------|--------------|----------|
| `retry_loop` | Agent retries the same failed action | No — stuck |
| `thrashing` | Agent tries many different approaches, all failing | No — flailing |
| `repair_loop` | Agent fails, adapts, succeeds | Yes — self-correcting |
| `artifact_drift` | Agent changes what it executes after prescribing | No — inconsistent |
| `blast_radius` | Destructive operation on many resources | Risky |
| `protocol_violation` | Missing prescribe or report | No — protocol gap |
| `risk_escalation` | Agent escalates beyond its baseline risk level | Informational |

This tests **behavioral quality** — not just whether the agent succeeds,
but how it gets there.

## Chaos Injection

Not every scenario should use chaos. Static failures are still valuable for
baseline remediation. Chaos scenarios are for a narrower question:

**does the agent stay reliable when the environment changes underneath it?**

infra-bench uses a deliberately small chaos model:

- deterministic timed steps
- the same command vocabulary as bootstrap/break steps
- no CRDs, controllers, or external chaos framework
- artifacts that record exactly what changed and when

Current chaos scenarios focus on:

- pod restarts during repair
- mounted ConfigMap drift during repair

This is enough to surface signals like verification gaps, retry loops, and
plan/action drift without turning the benchmark into a full chaos engineering
platform.

## Memory Window Testing

### Why Test Memory

AI agents accumulate context over a conversation. Each tool call result,
each error message, each observation adds to the message history. This has
two effects:

1. **More context helps reasoning** — the agent can learn from earlier
   mistakes, remember what it already tried, and build a coherent plan.

2. **More context costs tokens** — longer conversations consume more
   input tokens, increasing latency and cost.

The memory window parameter lets you test the trade-off explicitly.

### What `--memory-window` Does

Controls how much conversation history the agent sees on each turn:

| Value | Behavior | Tests |
|-------|----------|-------|
| `-1` (default) | Full history — agent sees everything | Baseline: how well does the agent perform with complete context? |
| `0` | Stateless — agent only sees system prompt + task + last tool result | Can the agent solve problems without memory? Tests pure reasoning. |
| `1` | Last exchange — agent sees previous action and result | Minimal memory: enough to react to the last result, no long-term plan. |
| `3` | Last 3 exchanges | Short-term memory: can maintain a brief plan. |
| `10` | Last 10 exchanges | Moderate memory: enough for most multi-step fixes. |

### What Memory Testing Reveals

**If the agent passes with `--memory-window 0`:** The problem is simple enough
to solve from scratch each step. The agent doesn't need planning or history.

**If the agent fails with `--memory-window 0` but passes with `-1`:** The
agent relies on conversation history — it plans across steps, learns from
failures, or needs to remember what it already tried.

**If the agent fails with both:** The problem is hard regardless of memory.

**If tokens drop significantly with smaller windows but pass rate stays
similar:** You can save cost without losing quality.

### Running Memory Window Tests

```bash
# Full memory (baseline)
infra-bench run --provider claude --model sonnet \
  --scenario kubernetes/broken-deployment \
  --memory-window -1

# Stateless
infra-bench run --provider claude --model sonnet \
  --scenario kubernetes/broken-deployment \
  --memory-window 0

# Sliding window of 3
infra-bench run --provider claude --model sonnet \
  --scenario kubernetes/broken-deployment \
  --memory-window 3
```

Compare results in the HTML report:

```bash
infra-bench report
open runs/report.html
```

## Model Comparison

### Why Compare Models

Different models have different strengths:

- **Smaller models** (Haiku, GPT-4o-mini) — faster, cheaper, but may
  struggle with complex multi-step reasoning
- **Larger models** (Opus, GPT-4o) — better reasoning, but slower and
  more expensive
- **Different providers** — Anthropic vs OpenAI vs Google may have
  different strengths on infrastructure tasks

### Running Model Comparisons

```bash
# Same scenario, different models
infra-bench run --provider bifrost --model anthropic/claude-3-5-sonnet --scenario ...
infra-bench run --provider bifrost --model openai/gpt-4o --scenario ...
infra-bench run --provider claude --model haiku --scenario ...

# Generate comparison report
infra-bench report
```

The HTML report shows results grouped by scenario with model, provider,
duration, turns, and token usage — making it easy to compare.

## Provider Architecture

infra-bench supports two execution paths:

### Provider Path (recommended for benchmarking)

infra-bench owns the tool-use loop. It sends prompts to the LLM, executes
tool calls locally (kubectl, helm, evidra CLI), and feeds results back.

```
infra-bench → Provider.Chat() → LLM response with tool calls
                                         ↓
                               infra-bench executes tools locally
                                         ↓
                               feed results back → next turn
```

Providers: `bifrost` (any model via API proxy), `claude` (Claude CLI).

### Adapter Path (legacy)

The agent is an external process that manages its own tool loop.
infra-bench just launches it and captures output.

```
infra-bench → spawn agent process → agent manages tools internally
```

Adapters: `cli` (any command), `mcp` (MCP-capable command).

## HTML Report

Generate a human-readable report from all run artifacts:

```bash
infra-bench report                           # → runs/report.html
infra-bench report my-report.html            # custom path
infra-bench report --runs-dir ./other-runs   # different runs dir
```

The report includes:
- Summary statistics (total runs, pass rate)
- Scenario matrix (all scenarios, run counts, pass rates)
- Per-scenario run details (model, provider, duration, checks, turns, memory, tokens, chaos mode)
- Color-coded pass/fail badges and check marks
