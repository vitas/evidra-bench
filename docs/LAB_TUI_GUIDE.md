# Lab TUI User Guide

`infra-bench lab` is an interactive terminal UI for browsing, running, and reviewing benchmark scenarios.

## Quick Start

```bash
# Build
make build

# Launch in dry-run mode (default)
./bin/infra-bench lab

# Launch with agent configured
./bin/infra-bench lab --agent-command /path/to/agent --adapter cli

# Launch pointing at a custom scenarios directory
./bin/infra-bench lab --scenarios-dir ./scenarios
```

## Views

### Catalog View (default)

The main screen shows all scenarios in a filterable list.

```
infra-bench lab  [all]  [dry-run]

  P E kubernetes    broken-deployment           (2/3)
>   E kubernetes    crashloop-backoff
    E helm          helm-failed-upgrade         (1/1)
      argocd        argocd-out-of-sync
  F   kubernetes    impossible-scheduling       (0/2)

Fix a pod stuck in CrashLoopBackOff
  category: kubernetes  timeout: 3m0s
  tags: pod, crashloop, container
  checks: deployment-ready/app
  evidra: enabled

j/k:nav  /:filter  t:category  h:history  d:dry-run  e:config  enter:run  ?:help  q:quit
```

**Columns:**
- First badge: `P` (pass, green) / `F` (fail, red) / blank (never run)
- Second badge: `E` (evidra protocol checks enabled, blue)
- Category and scenario ID
- Run count in parentheses: `(pass/total)`

**Navigation:**
- `j` / `k` or arrow keys — move cursor up/down
- `g` / `G` — jump to first/last
- `/` — start text search (matches ID, title, tags, category)
- `t` — cycle category filter: all → kubernetes → helm → argocd → all
- `Enter` — run the selected scenario
- `h` — show run history for selected scenario
- `d` — toggle dry-run mode
- `e` — edit run configuration
- `?` — show help
- `q` — quit

### Run Result View

After a scenario completes, shows pass/fail verdict with individual check results:

```
PASS  scenario=broken-deployment  duration=45.2s

  ok deployment-ready/bench/web
  ok evidra-protocol/prescribe-count-min
  ok evidra-protocol/report-count-min
  ok evidra-protocol/orphaned-prescriptions

Press any key to return
```

Failed checks show details:

```
FAIL  scenario=impossible-scheduling  duration=5m0s

  !! deployment-ready/bench/scheduler-test — ready replicas: 0/1
  ok deployment-ready/bench/web
  ok evidra-protocol/expected-signal/thrashing

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

Toggle adapter and dry-run mode:

```
Run Configuration

  [1] Adapter:       cli
  [2] Dry-run:       true
      Agent command: /path/to/agent
      Timeout:       5m

1/2: toggle  esc: back
```

Press `1` to toggle between `cli` and `mcp` adapter.
Press `2` to toggle dry-run.

Configuration persists across sessions in `.infra-bench-lab.yaml`.

## Configuration

### CLI Flags

```
--scenarios-dir    base directory for scenarios (default: scenarios)
--runs-dir         output directory for run artifacts (default: runs)
--adapter          agent adapter type: cli or mcp (default: cli)
--agent-command    command to invoke the agent
--dry-run          start in dry-run mode
```

CLI flags override saved configuration. Without flags, the TUI loads
the last-used config from `.infra-bench-lab.yaml`.

### Persistent Config

The TUI saves your last-used settings to `.infra-bench-lab.yaml`:

```yaml
adapter: cli
agent_command: /path/to/agent
timeout: 5m
dry_run: false
evidra_evidence_dir: ""
```

This is created automatically on first use. Edit with `e` inside the TUI
or modify the file directly.

## Typical Workflows

### Agent Developer: Test your agent

```bash
# First time — configure and dry-run
./bin/infra-bench lab --agent-command "./my-agent" --dry-run

# Browse scenarios, pick one, press Enter to dry-run
# Press 'd' to disable dry-run when ready
# Press Enter to run for real
# Press 'h' to see history after multiple runs
```

### Maintainer: Verify new scenario

```bash
# After adding a new scenario YAML
./bin/infra-bench lab

# Filter with '/' and type the scenario name
# Press Enter to dry-run — verifies it loads without error
# Check detail pane for correct checks and evidra expectations
```

### Iterative Debugging

```bash
./bin/infra-bench lab --agent-command "./my-agent"

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
```

The history view reads these artifacts to show past results.

## Results Database

Every non-dry-run is also stored in `runs/bench.db` (SQLite). Use the CLI
to query results outside the TUI:

```bash
infra-bench db stats                               # aggregate stats
infra-bench db query --model haiku                 # filter by model
infra-bench db query --scenario broken-deployment  # by scenario
infra-bench db query --failed --limit 5            # recent failures
```

The JSONL backup at `runs/results.jsonl` is committable to git for
tracking progression over time.
