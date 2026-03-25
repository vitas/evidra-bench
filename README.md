# evidra-infra-bench

Development and testing framework for AI infrastructure agent skills. Run your agent, MCP tool, or skill prompt against real Kubernetes clusters, AWS resources, Helm charts, and Argo CD — measure what actually helps and what's just token waste.

**75 scenarios** | **8 CKA/CKS-aligned tracks** | **4 certification levels** | **5 infrastructure categories**

Puzzle Designer: [lab.evidra.cc](https://lab.evidra.cc) | Results: [lab.evidra.cc/results](https://lab.evidra.cc/results)

## Why

A 5-line troubleshooting skill cuts L1 scenario turns from 17 to 4 — **75% faster.**
The same skill on L2 makes the agent skip diagnosis and **fail.**

Skills aren't universally good or bad. You need to test them on real scenarios
to know which help and which hurt. That's what `bench-cli` does.

```
# Without skill: 17 turns, PASS
bench-cli run --scenario kubernetes/broken-deployment --provider bifrost --model gemini-2.5-flash

# With skill: 4 turns, PASS — 4x faster
bench-cli run --scenario kubernetes/broken-deployment --provider bifrost --model gemini-2.5-flash \
  --system-prompt-file my-skill.md

# Same skill on harder scenario: 4 turns, FAIL — skipped diagnosis
bench-cli run --scenario kubernetes/crashloop-backoff --provider bifrost --model gemini-2.5-flash \
  --system-prompt-file my-skill.md
```

**Use cases:**
- **Skill developers:** "Does my skill help on L3, or only on L1?"
- **MCP server builders:** "Does smart output reduce tokens without hurting pass rate?"
- **Agent vendors:** "How does my agent compare to GPT-4o on CKS security?"
- **Platform teams:** "Can this agent handle production incidents before we deploy it?"

## Certify Your Agent

```bash
# Build
make build

# Certify an agent on Kubernetes Admin scenarios
bench-cli certify --track workloads --model sonnet --provider bifrost

# Certify on Kubernetes Security (includes AWS scenarios via LocalStack)
bench-cli certify --track pod-security --model gpt-4o --provider bifrost
```

Output:

```
════════════════════════════════════════════════════
  EVIDRA AGENT CERTIFICATION
════════════════════════════════════════════════════
  Agent:    sonnet (bifrost)
  Track:    Workloads (workloads)

  Grade:    PROFICIENT (L3)

  L1 Fix:        8/8   v
  L2 Diagnose:   5/6   v
  L3 Judge:      4/4   v

  Overall:  17/18 (94.4%)
  Duration: 8m 12s

  Certified: 2026-03-21
════════════════════════════════════════════════════
```

## Execution Modes

| Mode | Command | What it tests |
|---|---|---|
| **Baseline** | `bench-cli run --scenario ...` | Raw model ability (direct exec) |
| **Via evidra-mcp** | `--mcp-server "evidra-mcp --signing-mode optional"` | Agent through evidra (smart output + auto-evidence) |
| **Via third-party** | `--mcp-server "npx -y @anthropic/mcp-server-kubernetes"` | Agent through any MCP server |
| **With role skill** | `--role k8s-admin` | Agent behavior with skill prompt (optional) |

Baseline is mandatory for every scenario. MCP server mode tests the real product experience.

## How It Works

```
provision cluster → bootstrap baseline → inject failure → execute agent → verify outcome → grade
```

1. Provisions a disposable `kind` cluster (+ LocalStack for AWS scenarios)
2. Bootstraps the healthy baseline declared by the scenario
3. Injects a known failure — wrong image, broken NetworkPolicy, open security group, etc.
4. Executes the AI agent via multi-turn tool-use loop
5. Verifies infrastructure outcome with declarative checks
6. Measures behavioral signals: blast radius, judgment, protocol compliance
7. Grades the agent: Novice → Competent → Proficient → Expert

## Classification System

### Tracks (what the agent manages)

Aligned with CKA/CKS exam domains:

| Track | Source | Scenarios | What it proves |
|---|---|---|---|
| `workloads` | CKA: Workloads & Scheduling (15%) | 14 | Deployments, pods, scheduling, resources |
| `troubleshooting` | CKA: Troubleshooting (30%) | 14 | Diagnosis, judgment, cascading failures |
| `networking` | CKA: Services & Networking (20%) | 7 | Services, DNS, ingress, network policies |
| `storage` | CKA: Storage (10%) | 4 | PVC, StorageClass, volume expansion |
| `pod-security` | CKS: Minimize Vulns (20%) | 16 | RBAC, capabilities, PSA, CSR, AWS SG/S3 |
| `runtime-security` | CKS: Monitoring (20%) | 4 | Chaos resilience, runtime disruptions |
| `release-ops` | Custom | 8 | Helm, Argo CD, rollbacks, GitOps |
| `platform-eng` | Custom | 7 | Terraform state, drift, import, refactoring |

### Levels (how the agent thinks)

| Level | Name | What it tests | Human analogy |
|---|---|---|---|
| **L1** | Fix | One clear problem, one fix | Junior — follows the runbook |
| **L2** | Diagnose | Must investigate before fixing | Mid — reads logs, correlates |
| **L3** | Judge | Fix has trade-offs, traps exist | Senior — knows what NOT to do |
| **L4** | Investigate | Multi-step forensics, root cause | Staff — traces across systems |

### Grades

| Grade | Requirements |
|---|---|
| Novice | Passes some L1 scenarios |
| Competent | ≥90% of L1 + L2 |
| Proficient | ≥85% of L1 + L2 + L3 |
| Expert | ≥80% of L1 through L4 |

## Behavioral Signals

We don't just check pass/fail — we analyze _how_ the agent works:

| Signal | What it detects |
|---|---|
| `blast_radius` | Agent modified resources outside the problem scope |
| `retry_loop` | Agent repeated the same failing action |
| `trap_triggered` | Agent took the obvious-but-wrong fix |
| `protocol_violation` | Agent skipped prescribe/report evidence protocol |
| `decision_quality` | Agent diagnosed before acting vs brute-forced |

Every scenario is designed to generate signals. The signals are the product.

## Infrastructure Categories

| Category | Tool | Runtime | Scenarios |
|---|---|---|---|
| Kubernetes | kubectl | kind cluster | 60 |
| Helm | helm | kind cluster | 4 |
| Argo CD | argocd | kind cluster | 4 |
| Terraform | terraform | local state | 5 |
| **AWS** | **aws CLI** | **LocalStack** | **2** |

AWS scenarios run against [LocalStack](https://localstack.cloud/) — no cloud account needed. The harness auto-provisions a LocalStack container, runs setup scripts, and injects AWS credentials into the agent's environment.

## Multi-Stage Puzzles

Scenarios can have multiple stages where breaks are injected sequentially as the agent fixes earlier problems:

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
      memory: compact    # compress agent's conversation history
    agent_goal: "New issue: the API is returning database errors."
    verify:
      - resource-exists: bench/db-credentials
```

The agent runs in one continuous session. `memory: compact` summarizes prior context. `memory: reset` clears it entirely. `agent_goal` sends a message to the agent mid-run.

## Quick Start

```bash
# Prerequisites: Go 1.25+, kind, kubectl, helm
make build

# List all available scenarios
bench-cli scenario list

# Dry-run (validate without cluster)
bench-cli run --scenario kubernetes/broken-deployment --dry-run

# Run a scenario with a model
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost --model gemini-2.5-flash \
  --reuse-cluster

# Run all scenarios in a track
bench-cli certify --track workloads --model sonnet --provider bifrost

# Full benchmark across all scenarios
bench-cli bench --provider bifrost --model sonnet --reuse-cluster

# Interactive TUI
bench-cli lab
```

## Pluggable Providers

```bash
# Any model via Bifrost proxy
bench-cli run --provider bifrost --model openai/gpt-4o --scenario ...
bench-cli run --provider bifrost --model anthropic/claude-sonnet-4 --scenario ...
bench-cli run --provider bifrost --model google/gemini-2.5-flash --scenario ...

# Claude CLI directly
bench-cli run --provider claude --model sonnet --scenario ...
```

## Visual Puzzle Designer

Design scenarios visually at [lab.evidra.cc](https://lab.evidra.cc):

- Drag-and-drop puzzle builder with React Flow
- 75-scenario catalog with track/level/category filters
- Multi-stage chain builder (+ Stage button)
- Export as YAML, generate CLI commands
- Run configurator with model picker

Source: `ui/` directory. Deploy: `make ui-docker`.

## Development

```bash
make test           # Go unit tests
make test-race      # with race detector
make fmt            # gofmt
make lint           # golangci-lint
make smoke          # dry-run all scenarios
make ui-dev         # Vite dev server for lab UI
make ui-build       # production build
```

See `docs/testing.md` for the full testing guide.

## License

Apache License 2.0
