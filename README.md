# Evidra Bench

[![CI](https://github.com/vitas/evidra-bench/actions/workflows/ci.yml/badge.svg)](https://github.com/vitas/evidra-bench/actions/workflows/ci.yml)
[![Release](https://github.com/vitas/evidra-bench/actions/workflows/release.yml/badge.svg)](https://github.com/vitas/evidra-bench/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.25.11%2B-00ADD8.svg)](go.mod)
[![Bench](https://img.shields.io/badge/bench-live%20reports-4b5563.svg)](https://bench.evidra.cc)

Evidra Bench runs live regression tests for infrastructure agents. It puts a
model or agent into disposable Kubernetes incidents, lets it use real tools,
and catches failed or unsafe behavior before that configuration gets production
access.

Most AI agent benchmarks stop at a score, transcript, or vendor claim. Bench is
built for the harder questions platform teams ask before an agent touches
production:

- Did it diagnose the incident before changing infrastructure?
- Did it fix the root cause, or only make the final check green?
- Did it preserve safety constraints, ownership boundaries, and workload
  contracts?
- Did it loop, give up, burn tokens, or claim success too early?
- Did a new model, prompt, MCP server, tool server, or skill regress behavior?

The public report site is <https://bench.evidra.cc>. It hosts exam suites,
leaderboards, and inspectable benchmark reports produced by this harness.

![Benchmark overview](benchmark-overview.png)

## Try It In One Command

Docker is the only prerequisite. The runner creates a disposable kind cluster,
runs your model against three live incidents, verifies the outcome, writes
local reports, and removes the cluster:

```bash
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/evidra-results:/workspace/evidra-results" \
  -e OPENAI_API_KEY \
  ghcr.io/vitas/evidra-bench:latest \
  test --model openai/gpt-5
```

This runs locally without an Evidra account, login, central database, or result
upload. Your provider charges still apply to the model you select.

Or bring your own agent binary (`--agent` receives `kubectl` on PATH and
`INFRA_BENCH_SCENARIO` per case):

```bash
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/evidra-results:/workspace/evidra-results" \
  -v "$PWD/my-agent.sh:/fixtures/agent.sh:ro" \
  ghcr.io/vitas/evidra-bench:latest \
  test --agent /fixtures/agent.sh
```

You get a PASS / FAIL / UNSAFE verdict per case, a standalone
`evidra-results/report.html`, `result.json`, and signed evidence bundles for
every run under `evidra-results/bundles/` (open them with
`evidra validate --evidence-dir <bundle>` from the [Evidra
CLI](https://github.com/vitas/evidra)). The first run pulls a ~1 GB
Kubernetes node image, cached afterwards.

Mounting `/var/run/docker.sock` grants the runner host-level control through the
Docker daemon. Use this image only with agents and inputs you trust. Linux users
who opt into `--user` should add the socket's numeric group ID with
`--group-add "$(stat -c '%g' /var/run/docker.sock)"` and pre-create a writable
results directory.

## Start Here

| Goal | Read |
|---|---|
| Test a model or agent on live incidents now | Docker command above |
| Understand what Bench measures and how to read results | [Results And Reports](docs/RESULTS_AND_REPORTS.md) |
| Run your first local scenario | [Quickstart](docs/QUICKSTART.md) |
| Compare an MCP server, skill, or external agent | [Tool Server Integration](docs/TOOL_SERVER_INTEGRATION.md) |
| Write or review scenarios | [Scenario Authoring Guide](docs/SCENARIO_AUTHORING_GUIDE.md) |
| Browse the full documentation map | [Docs Home](docs/README.md) |

## What Bench Does

Bench creates a healthy infrastructure fixture, injects a known failure, runs
the selected agent adapter, collects artifacts, and verifies the final state.

```text
provision workspace
  -> bootstrap healthy baseline
  -> inject failure
  -> execute agent through adapter
  -> collect transcript, tool calls, timeline, and cost
  -> verify infrastructure outcome
  -> classify behavior findings
  -> publish local or hosted report data
```

Scenario checks are declarative. The agent can fix the problem any way it wants
as long as the final infrastructure state satisfies the checks and the action
path stays within the scenario safety expectations.

## What Makes It Different

| Capability | Why it matters |
|---|---|
| Live infrastructure exams | Runs real Kubernetes, Helm, Argo CD, Terraform, and AWS/LocalStack tasks instead of synthetic chat prompts. |
| Path-aware scoring | Separates safe passes from unsafe passes and catches shortcut fixes. |
| Artifact-backed evidence | Stores transcripts, tool calls, timelines, verifier output, run errors, scorecards, and failure autopsies. |
| Adapter-neutral evaluation | Tests provider loops, MCP servers, A2A agents, CLI agents, and skill prompts through one scenario harness. |
| Comparable report slices | Keeps model, scenario, runtime, and cluster settings fixed while comparing one axis. |
| Public and private workflows | Supports open benchmark reports, private procurement evaluations, and release regression tests. |

## Public Exam Suites

Bench groups scenarios into external-facing suites:

| Suite | What it tests |
|---|---|
| Kubernetes Admin Exam | Workloads, troubleshooting, networking, and storage in live clusters |
| Kubernetes Security Exam | Pod security, RBAC, runtime disruption, and safe remediation |
| GitOps And Release Exam | Helm and Argo CD drift, failed upgrades, rollback, and sync health |
| Terraform And Cloud Ops Exam | Terraform state, import, drift, AWS controls, and cloud recovery |
| MCP Server Readiness Exam | Native-tools baseline versus a selected MCP server on non-trivial and chaos scenarios |

These are exam-aligned benchmark suites, not official CNCF, Linux Foundation,
HashiCorp, or AWS certifications. See [Results And Reports](docs/RESULTS_AND_REPORTS.md)
for suite URLs, scoring labels, reproducibility rules, and report structure.

## Quick Start

Prerequisites: Go 1.25.11+, `kubectl`, `kind` or `k3d`, and `helm`.

```bash
# Build the CLI
make build

# List scenarios
bin/bench-cli scenario list

# Validate a scenario without starting a cluster
bin/bench-cli run --scenario kubernetes/broken-deployment --dry-run

# Run one live scenario
bin/bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model gemini-2.5-flash \
  --reuse-cluster
```

If you use an OpenAI-compatible provider route, point the `bifrost` provider at
it:

```bash
export INFRA_BENCH_BIFROST_URL=http://localhost:9090/v1
export INFRA_BENCH_BIFROST_AUTH_BEARER="$PROVIDER_API_KEY"
```

See [Quickstart](docs/QUICKSTART.md) for the first-run path and
[Bench Service Setup](docs/guides/bench-service-setup.md) for the local API and
runner control plane.

## Compare An MCP Server

Bench treats MCP servers as external tool backends. Keep the model, provider,
scenario set, timeout, memory window, and cluster settings fixed. Change only
the server command and labels.

```bash
# Native-tools baseline
bin/bench-cli bench \
  --scenario kubernetes \
  --provider bifrost \
  --model sonnet \
  --reuse-cluster

# Same scenario slice through a selected MCP server
bin/bench-cli bench \
  --scenario kubernetes \
  --provider bifrost \
  --model sonnet \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION" \
  --reuse-cluster
```

For paired baseline-versus-candidate reports, use `bench-cli report-pack`.
See [Tool Server Integration](docs/TOOL_SERVER_INTEGRATION.md) and
[Private Report Pack](docs/PRIVATE_REPORT_PACK.md).

## Scenario Catalog

The catalog covers operational domains and difficulty levels:

| Category | Runtime |
|---|---|
| Kubernetes | kind or k3d cluster |
| Helm | kind or k3d cluster |
| Argo CD | kind or k3d cluster |
| Terraform | local state |
| AWS | LocalStack, no cloud account required |

| Level | Name | What it tests |
|---|---|---|
| L1 | Fix | One clear problem, one fix |
| L2 | Diagnose | The agent must investigate before fixing |
| L3 | Judge | The fix has traps, trade-offs, or safety constraints |
| L4 | Investigate | Multi-step forensics and root-cause tracing |

See [Scenario Catalog](docs/SCENARIO_CATALOG.md) for the current inventory and
[Scenario Authoring Guide](docs/SCENARIO_AUTHORING_GUIDE.md) for writing new
scenarios.

## Development

```bash
make test                 # Go unit tests
make test-race            # Go tests with race detector
make fmt                  # gofmt
make lint                 # golangci-lint
make vuln                 # govulncheck
make smoke                # dry-run all scenarios
make public-smoke         # live public API smoke, requires BENCH_API_URL
make private-review-smoke # live private review write smoke
make ui-dev               # Vite dev server for local UI
make ui-build             # production UI build
```

See [Testing Guide](docs/TESTING.md) for local test coverage, CI gates, and
smoke-test workflows.

## License

[Apache License 2.0](LICENSE)
