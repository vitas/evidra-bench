# evidra-infra-bench

Standalone benchmark harness for testing infrastructure agents against realistic Kubernetes failure scenarios.

## What It Does

1. Provisions a disposable `kind` cluster
2. Bootstraps the healthy baseline declared by the scenario
3. Injects a known failure or drift condition
4. Executes your agent via a generic adapter (CLI or MCP)
5. Verifies infrastructure outcome with declarative checks
6. Verifies agent protocol compliance against Evidra evidence (opt-in)
7. Writes a complete local artifact bundle
8. Always writes local benchmark evidence and can optionally forward it to [Evidra](https://github.com/samebits/evidra)

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
```

## Scenarios

Scenarios are YAML-first and live under `scenarios/`:

```
scenarios/
  kubernetes/broken-deployment/   # Bad image tag
  helm/failed-upgrade/            # Failed Helm upgrade
  argocd/out-of-sync/             # Argo CD drift
```

Each scenario directory contains:
- `scenario.yaml` — metadata, break injection, checks, scope
- `prompts/task.md` — the task prompt given to the agent
- `fixtures/` — manifests used to inject failures

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

## CLI Flags

```
--scenario          scenario path (e.g., kubernetes/broken-deployment)
--adapter           cli or mcp (default: cli)
--agent-command     command to invoke the agent
--timeout           agent execution timeout (default: 5m)
--reuse-cluster     reuse an existing kind cluster and keep it after the run
--cluster-name      kind cluster name (default: infra-bench)
--dry-run           validate without executing
--evidra-url        Evidra API URL for online reporting
--evidra-api-key    Evidra API key
--evidra-evidence-dir   evidence directory for protocol verification (default: <runs-dir>/evidence)
```

## Artifacts

Each run writes to `runs/<timestamp>-<scenario>-<adapter>/`:

```
run.json            # Run metadata
prompt.txt          # Exact prompt input
transcript.txt      # Agent transcript
stdout.txt          # Process stdout
stderr.txt          # Process stderr
tool-calls.json     # Tool call log
verifier.json       # Check results
```

Offline benchmark evidence is appended under:

```
runs/evidra/evidence.jsonl
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
