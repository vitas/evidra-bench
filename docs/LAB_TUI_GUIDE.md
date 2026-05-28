---
title: Lab TUI User Guide
type: guide
status: active
tags:
  - bench
  - tui
  - local
---

# Lab TUI User Guide

`bench-cli lab` is an interactive terminal UI for browsing, running, and reviewing benchmark scenarios. It is intended to cover the day-to-day single-scenario workflow of `bench-cli run` while keeping batch, certification, service, and report-pack workflows in the regular CLI.

## Quick Start

```bash
# Build
make build

# Launch in dry-run mode (default)
./bin/bench-cli lab

# Launch with agent configured
./bin/bench-cli lab --agent-command /path/to/agent --adapter cli

# Launch pointing at a custom scenarios directory
./bin/bench-cli lab --scenarios-dir ./scenarios
```

## Views

### Catalog View (default)

The main screen shows all scenarios in a filterable list.

```
bench-cli lab  [all]  [dry-run]

  P   kubernetes    broken-deployment           (2/3)
>     kubernetes    crashloop-backoff
      helm          helm-failed-upgrade         (1/1)
      argocd        argocd-out-of-sync
  F   kubernetes    impossible-scheduling       (0/2)

Fix a pod stuck in CrashLoopBackOff
  category: kubernetes  timeout: 3m0s
  tags: pod, crashloop, container
  checks: deployment-ready/app

j/k:nav  /:filter  t:category  h:history  a:artifacts  d:dry-run  e:config  enter:run  ?:help  q:quit
```

**Columns:**
- First badge: `P` (pass, green) / `F` (fail, red) / blank (never run)
- Category and scenario ID
- Run count in parentheses: `(pass/total)`

**Navigation:**
- `j` / `k` or arrow keys — move cursor up/down
- `g` / `G` — jump to first/last
- `/` — start text search (matches ID, title, tags, category)
- `t` — cycle category filter: all → kubernetes → helm → argocd → all
- `Enter` — run the selected scenario
- `h` — show run history for selected scenario
- `a` — show latest local artifacts for selected scenario
- `d` — toggle dry-run mode
- `e` — edit run configuration
- `?` — show help
- `q` — quit

### Run Result View

After a scenario completes, shows pass/fail verdict with individual check results:

```
PASS  scenario=broken-deployment  duration=45.2s

  ok deployment-ready/bench/web

Press any key to return
```

When a run writes artifacts, press `a` from the result view to inspect them without leaving the TUI.

### Artifact View (`a`)

Shows local run evidence for the latest selected run, or for the just-completed run when opened from the result view.

```
Run Artifacts  runs/20260518-120000-broken-deployment-cli
[summary] [review*] [autopsy] [timeline] [transcript] [tool-calls] [scorecard*]

Autopsy
  outcome: fail
  primary: missed_diagnostic_step
  confidence: medium

Run failed with primary failure missed_diagnostic_step.

left/right:tab  r:review  esc/q:back  * missing artifact
```

Tabs:
- `summary` — which artifact files are available
- `review` — parsed `run_review.json` human review when present
- `autopsy` — parsed `failure-autopsy.json`
- `timeline` — decision timeline derived from `tool-calls.json`
- `transcript` — raw `transcript.txt`
- `tool-calls` — formatted `tool-calls.json`
- `scorecard` — formatted `scorecard.json`

The review tab uses the same `run_review.v1` schema as the hosted API. It
shows verdict, visibility, reviewer, labels, notes, evidence snippets, and
suggested scenario rules. See [Human Review](guides/human-review.md).

Press `r` from the artifact view to create or replace `run_review.json` for
the loaded run. The editor starts with a `Review Focus` block that surfaces
autopsy primary failure, the strongest finding, the first failed verifier check,
and timeline counts. It selects the strongest autopsy evidence step when
available, otherwise falls back to the first mutation step. It also fills an
evidence snippet from the timeline step and creates a reviewer note so
warning-or-higher labels are valid by default.

Review editor keys:

- `j`/`k` — choose the timeline step used as evidence
- `v` — cycle run verdict
- `l` — cycle label kind
- `s` — cycle severity
- `p` — cycle review visibility
- `n` — edit the reviewer note
- `w` — save local `run_review.json`
- `u` — save locally and upload with `BENCH_API_URL`/`BENCH_API_KEY` or the
  lab config `bench_url`/`bench_api_key`

Failed checks show details:

```
FAIL  scenario=impossible-scheduling  duration=5m0s

  !! deployment-ready/bench/scheduler-test — ready replicas: 0/1
  ok deployment-ready/bench/web

Press any key to return
```

### History View (`h`)

Shows the last 10 runs for the selected scenario with check diffs:

```
Run History: broken-deployment

  Total runs: 3   Pass: 2   Fail: 1

  PASS  2026-03-14 18:30:12  45.2s  checks: 4/4
  FAIL  2026-03-14 17:15:08  5m0s   checks: 3/4
    ~ deployment-ready/bench/web: fail -> pass
  PASS  2026-03-14 16:00:00  38.1s  checks: 4/4

Press any key to return
```

The diff lines between runs show which checks changed between consecutive runs:
- `~` — check verdict changed (e.g., fail → pass)
- `+` — new check appeared

### Config View (`e`)

Toggle core run settings and review the full run configuration:

```
Run Configuration

  [1] Adapter:       cli
  [2] Dry-run:       true
  [3] Model:         sonnet
  [4] Provider:      bifrost
      Agent command: /path/to/agent
      Timeout:       5m
      MCP server:    npx -y kubernetes-mcp-server
      Tool server:   kubernetes-mcp @ 1.2.3
      Skill:         k8s-admin @ 2026.05
      Report ID:     public-report
      Contract:      v1.2.0
      Cluster:       bench-cli
      Environment:   kind
      Memory window: -1
      Reuse cluster: true

1:adapter  2:dry-run  3:model  4:provider  esc:back
```

Press `1` to cycle `cli`, `mcp`, and `a2a` adapter.
Press `2` to toggle dry-run.
Press `3` to cycle common model aliases.
Press `4` to cycle provider mode.

Configuration persists across sessions in `.bench-cli-lab.yaml`.

## Configuration

### CLI Flags

```
--scenarios-dir    base directory for scenarios (default: scenarios)
--runs-dir         output directory for run artifacts (default: runs)
--adapter          agent adapter type: cli, mcp, or a2a (default: cli)
--a2a-agent-url    A2A agent URL
--agent-command    command to invoke the agent
--model            model for agent
--provider         provider for Bench-owned tool-use loop
--environment      environment provider: kind or k3d
--timeout          agent execution timeout
--reuse-cluster    reuse an existing local cluster
--cluster-name     local cluster name
--dry-run          start in dry-run mode
--bench-url        Bench API URL for reporting
--bench-api-key    Bench API key
--evidence-dir     verifier evidence directory
--memory-window    agent memory window (-1=full, 0=stateless, N=last N)
--system-prompt-file
--skill-file
--skill-id
--skill-version
--skill-source
--skill-sha256
--mcp-server
--tool-server-id
--tool-server-version
--report-id
--contract-version
--parallel
--database-url
```

CLI flags override saved configuration. Without flags, the TUI loads
the last-used config from `.bench-cli-lab.yaml`.

### Persistent Config

The TUI saves your last-used settings to `.bench-cli-lab.yaml`:

```yaml
adapter: cli
environment_provider: kind
agent_command: /path/to/agent
model: sonnet
provider: bifrost
runs_dir: runs
cluster_name: bench-cli
timeout: 5m
dry_run: false
memory_window: -1
reuse_cluster: true
mcp_server: npx -y kubernetes-mcp-server
tool_server_id: kubernetes-mcp
tool_server_version: 1.2.3
skill_id: k8s-admin
skill_version: "2026.05"
report_id: public-report
contract_version: v1.2.0
evidence_dir: ""
```

This is created automatically on first use. Edit with `e` inside the TUI
or modify the file directly.

## Typical Workflows

### Agent Developer: Test your agent

```bash
# First time — configure and dry-run
./bin/bench-cli lab --agent-command "./my-agent" --dry-run

# Browse scenarios, pick one, press Enter to dry-run
# Press 'd' to disable dry-run when ready
# Press Enter to run for real
# Press 'h' to see history after multiple runs
```

### Maintainer: Verify new scenario

```bash
# After adding a new scenario YAML
./bin/bench-cli lab

# Filter with '/' and type the scenario name
# Press Enter to dry-run — verifies it loads without error
# Check detail pane for correct checks and scenario metadata
```

### Iterative Debugging

```bash
./bin/bench-cli lab --agent-command "./my-agent"

# Run a scenario, see which checks fail
# Fix your agent, run again
# Press 'h' to see the diff — which checks improved
# Repeat until all checks pass
```

## Run Artifacts

Each run (non-dry-run) writes artifacts to `runs/<timestamp>-<scenario>-<adapter>/`:

```
run.json            # Run metadata (scenario, pass/fail, timing)
verifier.json       # Individual check results
prompt.txt          # Exact prompt given to agent
transcript.txt      # Agent transcript
stdout.txt          # Agent stdout
stderr.txt          # Agent stderr
tool-calls.json     # Tool call log
failure-autopsy.json # Deterministic failure or unsafe-pass analysis
scorecard.json      # Signal scorecard when available
```

The history view reads these artifacts to show past results.

## Results Database

Every non-dry-run is also stored in `runs/bench.db` (SQLite). Use the CLI
to query results outside the TUI:

```bash
bench-cli db stats                               # aggregate stats
bench-cli db query --model haiku                 # filter by model
bench-cli db query --scenario broken-deployment  # by scenario
bench-cli db query --failed --limit 5            # recent failures
```

The JSONL backup at `runs/results.jsonl` is committable to git for
tracking progression over time.
