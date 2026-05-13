# Demo Playbook

## Overview

5-minute video demo showing the Bench value prop: run agents against real infrastructure, compare models, and inspect why runs pass or fail.

## Pre-Demo Setup

### Night Before: Seed the Dashboard

Run ~180 fresh runs across 3 models to fill the leaderboard with pass^k data.

```bash
source .env && export $(grep -v '^#' .env | grep -v '^$' | xargs)

# Gemini — kubernetes + helm scenarios × 3 repeats
export INFRA_BENCH_BIFROST_URL=https://generativelanguage.googleapis.com/v1beta/openai
export INFRA_BENCH_BIFROST_AUTH_BEARER=$GEMINI_API_KEY
bench-cli bench \
  --scenario kubernetes --scenario helm \
  --model gemini-2.5-flash --provider bifrost \
  --repeats 3 --environment k3d --reuse-cluster --mcp-server "$MCP_SERVER" \
  --bench-url $BENCH_API_URL --bench-api-key $BENCH_API_KEY

# DeepSeek V4 Flash — same scenarios
export INFRA_BENCH_BIFROST_URL=https://api.deepseek.com
export INFRA_BENCH_BIFROST_AUTH_BEARER=$DEEPSEEK_API_KEY
bench-cli bench \
  --scenario kubernetes --scenario helm \
  --model deepseek-v4-flash --provider bifrost \
  --repeats 3 --environment k3d --reuse-cluster --mcp-server "$MCP_SERVER" \
  --bench-url $BENCH_API_URL --bench-api-key $BENCH_API_KEY

# Qwen — same scenarios
export INFRA_BENCH_BIFROST_URL=https://dashscope-intl.aliyuncs.com/compatible-mode/v1
export INFRA_BENCH_BIFROST_AUTH_BEARER=$DASHSCOPE_API_KEY
bench-cli bench \
  --scenario kubernetes --scenario helm \
  --model qwen-plus --provider bifrost \
  --repeats 3 --environment k3d --reuse-cluster --mcp-server "$MCP_SERVER" \
  --bench-url $BENCH_API_URL --bench-api-key $BENCH_API_KEY
```

### Morning Of: E2E Verification Checklist

Run all 7 before recording. Every one must pass.

| # | What | Command | Expected |
|---|------|---------|----------|
| 1 | Baseline PASS | `bench-cli run --scenario kubernetes/broken-deployment --model gemini-2.5-flash --provider bifrost --environment k3d` | PASS, <60s |
| 2 | MCP PASS | same + `--mcp-server "$MCP_SERVER" --reuse-cluster` | PASS, MCP run reported to API |
| 3 | ArgoCD PASS | `bench-cli run --scenario argocd/out-of-sync --model gemini-2.5-flash --provider bifrost --environment k3d --reuse-cluster --cluster-name argo-demo` | PASS, <3min |
| 4 | Leaderboard | `curl -sf -H "Authorization: Bearer $BENCH_API_KEY" $BENCH_API_URL/v1/bench/leaderboard` | gemini-2.5-flash in results |
| 5 | Evidence filter | Open the run detail page and inspect tool calls | Run details shown |
| 6 | Skipped scenario | `bench-cli run --scenario kubernetes/apparmor-profile-pod --dry-run` | Error: "skipped" |
| 7 | Profile rejection | `bench-cli bench --scenario argocd/out-of-sync --parallel 2 --database-url ...` | Error: "shared-cluster parallel" |

After verification, clean up clusters:
```bash
k3d cluster delete infra-bench argo-demo
```

## Video Demo Script (5 min)

### Act 1: "What does your agent actually do?" (1 min)

Live terminal. Show the agent fixing a broken deployment.

```bash
bench-cli run --scenario kubernetes/broken-deployment \
  --model gemini-2.5-flash --provider bifrost \
  --environment k3d --mcp-server "$MCP_SERVER"
```

**What judges see:**
- Agent diagnoses ErrImagePull
- Finds the bad image tag `nginx:99.99-nonexistent`
- Fixes it with `kubectl set image`
- Deployment goes green
- **PASS** in ~30s
- Tool calls and transcript recorded

**Talking point:** "One command. Real cluster. Real agent. Real verification."

### Act 2: "Same agent, harder problem" (1.5 min)

```bash
bench-cli run --scenario kubernetes/wrong-service-selector \
  --model gemini-2.5-flash --provider bifrost \
  --environment k3d --reuse-cluster --mcp-server "$MCP_SERVER"
```

**What judges see:**
- This time the agent must investigate — it's not a clear error message
- Agent examines the service, compares selectors, finds the mismatch
- Takes 5-8 turns of diagnosis
- Fixes the selector, service routes correctly
- **PASS**

**Talking point:** "L1 scenarios test if the agent can follow a runbook. L2 tests if it can think."

### Act 3: "Why did it fail?" (1.5 min)

Switch to the Bench run detail or local artifacts.

1. Open the latest run.
2. Show pass/fail, turns, duration, token usage, and estimated cost.
3. Open transcript and tool calls.
4. Point out whether the agent diagnosed before acting.
5. Show the verifier output that decided the run.

**Talking point:** "The important question is not only did it pass. It is where the agent spent turns, what it inspected, and why the final checks passed or failed."

### Act 4: "How does your agent compare?" (1 min)

Switch to the Bench leaderboard.

1. Show 10 models ranked by pass rate
2. Hover over "Reliability" — explain pass^k: "not just does it pass, but does it pass consistently"
3. Sonnet at 92.8% reliability, Gemini Flash at 53.5%
4. "190 runs for Sonnet, 360 for Gemini Flash — this is real data"

**Talking point:** "75 scenarios, 10 models, thousands of runs. This is how you know which agent to deploy."

## Model Selection for Demo

| Model | Role in Demo | Why |
|-------|-------------|-----|
| **gemini-2.5-flash** | Live runs | Cheapest (~$0.001/run), fastest (30s), good pass rate. Low risk of timeout. |
| **claude-sonnet-4** | Leaderboard reference | Highest reliability (92.8%). Show on screen, don't run live — expensive. |
| **deepseek-chat** | Contrast on leaderboard | Shows "passes but slower, more turns" — different agent personality. |

## Scenario Selection for Demo

| Scenario | Level | Time | Why |
|----------|-------|------|-----|
| `broken-deployment` | L1 Fix | ~30s | Fast, visual, everyone understands "bad image" |
| `wrong-service-selector` | L2 Diagnose | ~60s | Agent must investigate, shows reasoning |
| `argocd/out-of-sync` | L1 Fix | ~2min | Shows profiles, GitOps, real ArgoCD |

## What NOT to Demo

- Skipped scenarios (13 still skipped — multi-node, AppArmor, etc.)
- DeepSeek live runs (slower, historically used `kubectl -w` which hangs)
- Parallel mode (needs PostgreSQL — adds setup complexity)
- Sonnet live runs (expensive — $0.10+ per run, save budget for batch)
- The designer page (works but not the core value prop)

## Key Messages for Judges

1. **"Skills aren't universally good"** — a 5-line troubleshooting skill cuts L1 from 17 turns to 4, but makes L2 fail. You need to measure.
2. **"Failure analysis, not just outcomes"** — show turns, token burn, tool calls, checks, and the reason a run failed.
3. **"Real infrastructure"** — disposable Kind/k3d clusters, real kubectl, real Helm, real ArgoCD. Not mocks.
4. **"Model-agnostic"** — 10 models benchmarked through one framework. Swap `--model` and compare.
5. **"Open source"** — 75 scenarios, YAML-first, anyone can add scenarios without writing Go.

## Backup Plans

| Problem | Fix |
|---------|-----|
| k3d won't start | Use `--environment kind` instead |
| API key expired | Check `.env`, re-export |
| Scenario fails live | "This is exactly why we benchmark — let me show you a passing one" |
| Bench API is down | Show local results: `cat runs/bench/*/summary.json` |
| Agent times out | "DeepSeek sometimes does this — that's why reliability matters" |
