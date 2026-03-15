# evidra-infra-bench

Standalone benchmark harness for testing infrastructure agents against realistic Kubernetes failure scenarios.

## What It Does

1. Provisions a disposable `kind` cluster
2. Bootstraps the healthy baseline declared by the scenario
3. Injects a known failure or drift condition
4. Optionally injects deterministic runtime chaos while the agent is working
5. Executes your agent via a generic adapter (CLI or MCP)
6. Verifies infrastructure outcome with declarative checks
7. Verifies agent protocol compliance against Evidra evidence (opt-in)
8. Writes a complete local artifact bundle
9. Always writes local benchmark evidence and can optionally forward it to [Evidra](https://github.com/samebits/evidra)

## Prerequisites

- Go 1.24+
- [kind](https://kind.sigs.k8s.io/)
- kubectl
- helm (for Helm scenarios)

## Quick Start

```bash
# Build
make build

# List available scenarios
./bin/infra-bench scenario list

# The list output can be passed back to run directly, or you can use the scenario id
./bin/infra-bench run --scenario broken-deployment --dry-run

# Dry-run a scenario (validates without provisioning a cluster)
./bin/infra-bench run \
  --scenario kubernetes/broken-deployment \
  --dry-run

# Run a scenario against your agent
./bin/infra-bench run \
  --scenario kubernetes/broken-deployment \
  --adapter cli \
  --agent-command /path/to/your/agent

# Interactive TUI — browse, select, and run scenarios
./bin/infra-bench lab
```

## Scenarios

Scenarios are YAML-first and live under `scenarios/`:

```
scenarios/
  kubernetes/broken-deployment/   # Bad image tag
  kubernetes/pod-kill-during-repair/ # Broken deployment + runtime pod restarts
  kubernetes/config-mutation-mid-fix/ # Mounted config drifts during repair
  helm/failed-upgrade/            # Failed Helm upgrade
  argocd/out-of-sync/             # Argo CD drift
```

Each scenario directory contains:
- `scenario.yaml` — metadata, break injection, checks, scope
- `prompts/task.md` — the task prompt given to the agent
- `fixtures/` — manifests used to inject failures

Scenarios can also declare an optional `chaos:` block to inject deterministic
runtime disruptions during agent execution:

```yaml
chaos:
  stop_on_agent_done: true
  steps:
    - at: 10s
      name: kill-web-pods
      type: kubectl
      args: [delete, pod, -n, bench, -l, app=web, --force, --grace-period=0]
```

### Adding a Scenario

Create a new directory under `scenarios/<category>/<name>/` with a `scenario.yaml`:

```yaml
id: my-scenario
title: Fix something broken
category: kubernetes
prompt: prompts/task.md
timeout: "3m"
bootstrap:
  - name: deploy-baseline
    type: kubectl-apply
    path: ../../../manifests/baseline
break:
  type: kubectl-apply
  path: fixtures/broken.yaml
checks:
  - type: deployment-ready
    namespace: bench
    name: web
scope:
  namespaces: [bench]
```

## Agent Adapters

### CLI Adapter

Launches your agent as an external process with environment variables:

- `KUBECONFIG` — path to the cluster kubeconfig
- `INFRA_BENCH_WORKSPACE` — workspace directory
- `INFRA_BENCH_SCENARIO` — scenario ID
- `INFRA_BENCH_PROMPT` — path to the prompt file

### MCP Adapter

Launches an MCP-capable agent process with the same environment variables plus `INFRA_BENCH_ADAPTER=mcp`.

## Two-Dimensional Evaluation

infra-bench evaluates agents on two independent axes:

**Infrastructure outcome** — did the agent fix the problem?
Declarative checks verify cluster state: deployment ready, service endpoints
reachable, Helm release deployed, ArgoCD app healthy.

**Protocol compliance** — did the agent follow the prescribe/report protocol?
When `evidra:` expectations are declared in a scenario, the harness reads the
Evidra evidence chain after the run and asserts: every mutation was prescribed
before execution, every prescribe has exactly one report, risk levels match
expectations, declined verdicts are recorded with context.

A scenario can pass on infrastructure but fail on protocol (agent fixed it
but skipped prescribe), or pass on protocol but fail on infrastructure
(agent followed protocol perfectly but didn't solve the problem). Both
dimensions matter for reliable AI infrastructure agents.

Scenarios without `evidra:` block are evaluated on infrastructure outcome only.

## Interactive Lab TUI

`infra-bench lab` launches an interactive terminal UI for browsing and running
scenarios without remembering CLI flags or editing YAML.

Features:
- Filterable catalog with category and text search
- Scenario detail view with checks and evidra expectations
- One-key execution with live result display
- Persistent run config (adapter, agent command, dry-run)
- Pass/fail badges on previously run scenarios

```bash
# Launch with defaults (dry-run mode)
./bin/infra-bench lab

# Launch with agent configured
./bin/infra-bench lab --agent-command /path/to/agent --adapter cli
```

Key bindings: `j/k` navigate, `/` search, `t` filter category, `p` cycle
provider, `m` cycle model, `h` run history, `Enter` run, `d` toggle dry-run,
`e` edit config, `?` help, `q` quit.

See `docs/LAB_TUI_GUIDE.md` for the full user guide.

## Pluggable Providers

infra-bench can drive any LLM through a multi-turn tool-use loop:

```bash
# Claude CLI (default model: haiku)
infra-bench run --provider claude --model sonnet --scenario ...

# Any model via Bifrost proxy
infra-bench run --provider bifrost --model openai/gpt-4o --scenario ...
infra-bench run --provider bifrost --model anthropic/claude-3-5-sonnet --scenario ...
```

The agent calls `run_command`, `evidra_prescribe`, and `evidra_report` tools.
infra-bench executes them locally and feeds results back. Rate-limited
requests are retried automatically with adaptive backoff.

## Memory Window Testing

Test how much conversation history an agent needs:

```bash
infra-bench run --provider claude --model sonnet --memory-window -1 ...  # full history
infra-bench run --provider claude --model sonnet --memory-window 0 ...   # stateless
infra-bench run --provider claude --model sonnet --memory-window 3 ...   # last 3 exchanges
```

See `docs/TESTING_METHODOLOGY.md` for interpretation guidance.

## CLI Flags

```
--scenario            scenario path (e.g., kubernetes/broken-deployment)
--provider            LLM provider for tool-use loop (bifrost, claude)
--model               model name (e.g. sonnet, opus, haiku, openai/gpt-4o)
--adapter             cli or mcp — legacy external agent path (default: cli)
--agent-command       command to invoke external agent
--memory-window       agent context window (-1=full, 0=stateless, N=last N exchanges)
--timeout             agent execution timeout (default: 5m)
--reuse-cluster       reuse an existing kind cluster
--cluster-name        kind cluster name (default: infra-bench)
--dry-run             validate without executing
--evidra-bin          path to evidra binary for protocol tools
--evidra-evidence-dir evidence directory for protocol verification
--evidra-url          Evidra API URL for online reporting
--evidra-api-key      Evidra API key
```

## Results & Reports

```bash
# HTML report from all runs
infra-bench report

# Compare two runs side by side
infra-bench compare runs/<run-A>/ runs/<run-B>/

# Query results database
infra-bench db stats                               # aggregate statistics
infra-bench db query --model haiku                 # filter by model
infra-bench db query --scenario broken-deployment  # filter by scenario
infra-bench db query --failed --limit 10           # recent failures

# Rebuild database from JSONL backup
infra-bench db rebuild
```

Results are stored in SQLite (`runs/bench.db`, gitignored) with a JSONL backup
(`runs/results.jsonl`, committable). The DB is always rebuildable from JSONL.

## Artifacts

Each run writes to `runs/<timestamp>-<scenario>-<adapter>/`:

```
run.json            # Run metadata (scenario, model, pass/fail, tokens, cost)
prompt.txt          # Exact prompt input
transcript.txt      # Agent transcript (all turns)
stdout.txt          # Process stdout
stderr.txt          # Process stderr
tool-calls.json     # Tool call log
verifier.json       # Check results
chaos.json          # Structured chaos timeline, when enabled
chaos.log           # Human-readable chaos event log, when enabled
```

## Development

```bash
make test           # Run all tests
make test-race      # Run with race detector
make fmt            # Format code
make lint           # Run linter
```

## License

Apache License 2.0
