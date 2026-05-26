# Evidra Bench

[![CI](https://github.com/vitas/evidra-bench/actions/workflows/ci.yml/badge.svg)](https://github.com/vitas/evidra-bench/actions/workflows/ci.yml)
[![Release](https://github.com/vitas/evidra-bench/actions/workflows/release.yml/badge.svg)](https://github.com/vitas/evidra-bench/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.25.10%2B-00ADD8.svg)](go.mod)
[![Bench](https://img.shields.io/badge/bench-live%20reports-4b5563.svg)](https://bench.evidra.cc)

Evidra Bench is an AI infrastructure agent benchmark and regression testing
system. Run the same real Kubernetes, Helm, Argo CD, Terraform, and
AWS/LocalStack scenarios across models, MCP servers, skills, and remote agents.
Track pass rate, cost, turns, token use, and failure patterns over time.

Bench answers the questions that matter before an agent touches production:

- Can it fix the incident?
- Did it diagnose before acting?
- Did it loop, give up, or claim success too early?
- Did a new model, prompt, MCP server, or skill regress behavior?
- How many tokens and turns did the run waste?

The primary product site is `https://bench.evidra.cc`. Public exam suites and
the leaderboard are the marketing surface. Private regression history,
scheduled runs, customer incident suites, and failure reports are the product
surface.

## Main Features

| Feature | Why it matters |
|---|---|
| Live infrastructure exams | Runs real Kubernetes, Helm, Argo CD, Terraform, and AWS/LocalStack scenarios instead of synthetic chat prompts. |
| Path-aware scoring | Measures not only whether the final state passed, but whether the agent diagnosed first, looped, skipped evidence, or used unsafe shortcuts. |
| Artifact-backed evidence | Stores transcripts, tool calls, timelines, scorecards, run events, errors, and failure autopsies so every result can be inspected. |
| Agent failure autopsy | Turns raw traces into behavior findings such as missed diagnostics, retry loops, premature success, and unsafe actions. |
| MCP server readiness testing | Compares native-tools baselines against selected MCP servers on the same scenario slice, with reportable evidence links. |
| Adapter-neutral agent testing | Tests built-in model loops, MCP servers, A2A agents, CLI agents, and skill prompts through one scenario harness. |
| Model, prompt, and skill regression history | Shows whether a model upgrade, prompt change, tool server, or skill improved L3/L4 behavior or only helped easy tasks. |
| Public reports and leaderboard | Publishes shareable benchmark results with evidence, not just a score badge. |
| Private and self-hosted deployments | Uses the same product and API for public reports, private incident suites, team regression history, and customer readiness reports. |
| Remote runner control plane | Lets hosted Bench queue jobs while remote runners execute scenarios in controlled infrastructure. |
| Local CLI and TUI workflow | Supports fast local scenario development, dry runs, artifact browsing, and repeatable lab workflows. |
| Scenario Studio human review | Planned review layer for turning human labels, notes, and evidence snippets into reusable scenario rules. |

## Why

Agent quality is not a single pass/fail number. The same prompt or tool server
can make an easy scenario faster and make a harder scenario fail by skipping
diagnosis. You need repeatable tests with real infrastructure state, artifacts,
and comparable run history.

The public suites are exam-aligned marketing proof: Kubernetes, security,
GitOps, Terraform, and cloud-ops tasks that show how agents behave in real
environments. They are not official CNCF, Linux Foundation, HashiCorp, or AWS
certifications.

## Public Exam Suites

Bench packages the catalog into public suites that are easy to compare on a
leaderboard and easy to explain in readiness reports:

| Suite | What it tests |
|---|---|
| Kubernetes Admin Exam | Workloads, troubleshooting, networking, and storage in live clusters |
| Kubernetes Security Exam | Pod security, RBAC, runtime disruption, and safe remediation |
| GitOps And Release Exam | Helm and Argo CD drift, failed upgrades, rollback, and sync health |
| Terraform And Cloud Ops Exam | Terraform state, import, drift, AWS controls, and cloud recovery |
| MCP Server Readiness Exam | No-MCP/native-tools baseline versus a selected MCP server on non-trivial and chaos scenarios |

See [Public Exam Suites](docs/EXAM_SUITES.md) for the current suite map.

```bash
# Baseline model behavior
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model gemini-2.5-flash

# Same model with a skill prompt
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model gemini-2.5-flash \
  --skill-file skills/k8s-admin.md \
  --skill-id k8s-admin

# Same scenario through a selected MCP server
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model gemini-2.5-flash \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION"
```

## Use Cases

| User | Question Bench answers |
|---|---|
| Platform teams | Can this agent handle realistic incidents before we deploy it? |
| Agent builders | Which model, prompt, or tool stack regressed this scenario? |
| MCP server builders | Does this tool server improve outcomes without raising cost? |
| Skill authors | Does this skill help on L3/L4, or only on easy L1 tasks? |
| Security teams | Does the agent fix the issue without weakening controls? |
| Customers with incidents | Can our past outages become private agent regression tests? |

## What Bench Measures

Bench checks the outcome and the path the agent took:

| Metric | What it shows |
|---|---|
| Pass rate | Whether final infrastructure checks passed |
| Turns | How many agent/tool iterations were needed |
| Tokens and cost | Whether a change saves or burns budget |
| Duration | Wall-clock runtime for the scenario |
| Tool calls | What the agent inspected or changed |
| Timeline | Discovery, diagnosis, action, and verification phases |
| Failure patterns | Loops, premature success, missed diagnostics, unsafe actions |

The next product layer is agent failure autopsy: a report that explains where
the agent got stuck, what it missed, and which behavior caused the regression.
See [Agent Failure Autopsy](docs/AGENT_FAILURE_AUTOPSY.md).

## Scenario Catalog

The catalog is organized by operational domain and difficulty:

| Track | What it tests |
|---|---|
| `workloads` | Deployments, pods, scheduling, resources |
| `troubleshooting` | Diagnosis, correlation, cascading failures |
| `networking` | Services, DNS, ingress, network policies |
| `storage` | PVCs, StorageClass behavior, volume expansion |
| `pod-security` | RBAC, capabilities, PSA, CSR, AWS SG/S3 |
| `runtime-security` | Runtime disruptions and chaos resilience |
| `release-ops` | Helm, Argo CD, rollbacks, GitOps |
| `platform-eng` | Terraform state, drift, import, refactoring |

Difficulty levels:

| Level | Name | What it tests |
|---|---|---|
| L1 | Fix | One clear problem, one fix |
| L2 | Diagnose | The agent must investigate before fixing |
| L3 | Judge | The fix has traps, trade-offs, or safety constraints |
| L4 | Investigate | Multi-step forensics and root-cause tracing |

Infrastructure categories:

| Category | Runtime |
|---|---|
| Kubernetes | kind or k3d cluster |
| Helm | kind or k3d cluster |
| Argo CD | kind or k3d cluster |
| Terraform | local state |
| AWS | LocalStack, no cloud account required |

## Execution Adapters

Bench keeps scenario setup and verification local, then swaps how the agent is
executed:

| Adapter | Example | What it tests |
|---|---|---|
| Built-in provider loop | `--provider bifrost --model ...` | Raw model behavior with Bench-owned tools |
| MCP server | `--mcp-server "..."` | Model behavior through a tool server |
| A2A agent | `--adapter a2a --a2a-agent-url ...` | Remote agent behavior with local verification |
| CLI process | `--adapter cli` | External agent process compatibility |
| Skill prompt | `--skill-file ...` | Prompt/skill impact under fixed scenarios |

Any MCP tool server can be tested by passing its command to `--mcp-server`:

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION"
```

For a private report deliverable, use the report pack workflow. It runs a
direct baseline and the selected MCP server over the same scenario slice, sends
both sides to Bench, then prints live report URLs:

```bash
bench-cli report-pack \
  --model sonnet \
  --provider bifrost \
  --bench-url https://api.evidra.cc \
  --bench-api-key "$BENCH_API_KEY" \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION"
```

See [Private Report Pack](docs/PRIVATE_REPORT_PACK.md) for the reporting
workflow. The first public multi-server report is tracked in
[Kubernetes MCP Readiness 2026-05](https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05).
The launch article is
[Kubernetes MCP Servers Passed. That Was Not Enough.](https://bench.evidra.cc/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough).

## How It Works

```text
acquire lease
  -> provision workspace
  -> bootstrap healthy baseline
  -> inject failure
  -> execute agent through adapter
  -> collect artifacts and timeline
  -> verify infrastructure outcome
  -> store result
  -> report leaderboard/private regression data
```

Scenario checks are declarative. The agent can fix the problem any way it wants
as long as the final infrastructure state satisfies the checks.

## Quick Start

Prerequisites: Go 1.25.10+, kind or k3d, kubectl, helm.

```bash
# Build
make build

# List scenarios
bench-cli scenario list

# Validate a scenario without a cluster run
bench-cli run --scenario kubernetes/broken-deployment --dry-run

# Run one scenario
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model gemini-2.5-flash \
  --reuse-cluster

# Certify on one track
bench-cli certify --track workloads --model sonnet --provider bifrost

# Run a full benchmark
bench-cli bench --provider bifrost --model sonnet --reuse-cluster

# Open the local TUI
bench-cli lab
```

## Provider Setup

Route model requests directly or through a unified Bifrost gateway:

```bash
# Direct OpenAI-compatible endpoint
export INFRA_BENCH_BIFROST_URL=https://generativelanguage.googleapis.com/v1beta/openai
export INFRA_BENCH_BIFROST_AUTH_BEARER=$GEMINI_API_KEY
bench-cli run --provider bifrost --model gemini-2.5-flash --scenario ...

# Bifrost gateway
source .env
./scripts/bifrost-start.sh
export INFRA_BENCH_BIFROST_URL=http://localhost:9090/v1
bench-cli run --provider bifrost --model google/gemini-2.5-flash --scenario ...
bench-cli run --provider bifrost --model deepseek/deepseek-chat --scenario ...
bench-cli run --provider bifrost --model openai/gpt-4.1 --scenario ...
```

Claude CLI is also supported:

```bash
bench-cli run --provider claude --model sonnet --scenario ...
```

## Multi-Stage Scenarios

Scenarios can inject failures sequentially while the agent stays in one
session:

```yaml
stages:
  - name: wrong-image
    break:
      apply: fixtures/wrong-image.yaml
    verify:
      - deployment-ready: bench/web

  - name: missing-secret
    break:
      apply: fixtures/delete-secret.yaml
      memory: compact
    agent_goal: "New issue: the API is returning database errors."
    verify:
      - resource-exists: bench/db-credentials
```

`memory: compact` summarizes prior context. `memory: reset` clears it.
`agent_goal` sends a new user message mid-run.

## Bench API And Runners

This repo owns the private bench control plane:

- `/v1/bench/*` for runs, artifacts, analytics, trigger jobs, and scenario sync
- `/v1/runners/*` for poll-based runner registration, job claim, and completion
- `/v1/certify` for the direct executor contract used by `bench-cli serve`

Run the service locally:

```bash
BENCH_DATABASE_URL=postgres://bench:bench@localhost:5432/bench?sslmode=disable \
BENCH_API_KEY=dev-secret \
BENCH_SERVICE_ADDR=:8090 \
bench-cli serve
```

Hosted control-plane deployments that rely on remote runners should disable
the direct executor so the API process does not provision a local cluster:

```bash
BENCH_CONTROL_PLANE_ONLY=true bench-cli serve --control-plane-only
```

Production deployment is intentionally out of scope for this repository. Keep
environment-specific manifests, secrets, and hosted topology in a separate
private infrastructure repository. This repo stays focused on code, local
execution, API contracts, scenarios, and tests.

## Documentation

- [Docs Home](docs/README.md)
- [Quickstart](docs/QUICKSTART.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Runner Architecture](docs/RUNNER_ARCHITECTURE.md)
- [Testing Guide](docs/TESTING.md)
- [Testing Methodology](docs/TESTING_METHODOLOGY.md)
- [Scoring](docs/SCORING.md)
- [Reproducibility](docs/REPRODUCIBILITY.md)
- [Agent Failure Autopsy](docs/AGENT_FAILURE_AUTOPSY.md)
- [Sample Bench Report](docs/SAMPLE_BENCH_REPORT.md)
- [Scenario Authoring Guide](docs/SCENARIO_AUTHORING_GUIDE.md)
- [Tool Server Integration](docs/TOOL_SERVER_INTEGRATION.md)
- [Bench API Reference](docs/BENCH_API_REFERENCE.md)
- [Bench Service Setup](docs/guides/bench-service-setup.md)
- [Executor Contract v1.0.0](docs/contracts/EXECUTOR_CONTRACT_V1.md)
- [Runner Control Plane Contract v1](docs/contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
- [Open Source Boundary](docs/OPEN_SOURCE.md)
- [Roadmap](docs/ROADMAP.md)
- [Threat Model](docs/THREAT_MODEL.md)

## Development

```bash
make test           # Go unit tests
make test-race      # with race detector
make fmt            # gofmt
make lint           # golangci-lint
make vuln           # govulncheck
make smoke          # dry-run all scenarios
make public-smoke   # live public API smoke, requires BENCH_API_URL
make private-review-smoke # live private review write smoke
make ui-dev         # Vite dev server for local UI
make ui-build       # production UI build
```

See [Testing Guide](docs/TESTING.md) for the full testing guide.

## License

[Apache License 2.0](LICENSE)
