# Testing Guide — evidra-infra-bench

How to run tests, what they cover, and how to add new ones.

## Quick Start

```bash
make test          # all unit tests (go test ./... -v -count=1)
make test-race     # with race detector (CI-required)
make smoke         # dry-run all scenarios (build + validate)
make lint          # golangci-lint
make fmt           # gofmt -w .
```

## Test Matrix

| Package | Tests | What it covers |
|---------|-------|----------------|
| `cmd/bench-cli` | CLI tests | CLI commands: run --dry-run, scenario list, skill-delta |
| `pkg/adapter` | 20 | CLI and MCP agent adapters, env vars, exit codes |
| `pkg/agent` | 20+ | Provider loop, context window, tool schemas, message injection, memory reset |
| `pkg/artifact` | 18 | Run bundle writing, transcript/tool-call export |
| `pkg/config` | 12 | Config defaults, validation, version metadata |
| `pkg/environment` | 20 | Kind cluster provider, bootstrap step execution |
| `pkg/harness` | 25+ | Run orchestration, chaos injection, rollout waiting |
| `pkg/report` | 10 | Evidra JSONL reporting, offline mode |
| `pkg/scenario` | 58 | YAML loading, runtime contract validation, multi-stage |
| `pkg/skilldelta` | 15 | Paired benchmark aggregation, markdown rendering |
| `pkg/store` | 30 | SQLite CRUD, JSONL backup/rebuild, filtering |
| `pkg/tui` | 35 | Lab TUI config, catalog filtering, run history |
| `pkg/verifier` | 12 | Kubernetes checks, PollChecks retry loop |

The suite covers the CLI plus the core runtime packages listed above.

## CI Pipeline

`.github/workflows/ci.yml` runs on every push to `main` and every PR:

1. **Format check** — `gofmt -l .` fails if any file is unformatted
2. **Unit tests** — `go test ./... -count=1`
3. **Race detector** — `go test -race ./... -count=1`

CI requires checkout of the parent `evidra-benchmark` repo (for contract schema validation).

## Test Categories

### 1. Scenario Runtime Contracts

**File:** `pkg/scenario/runtime_contract_test.go`

Validates all scenario YAML files at load time — no cluster needed. Catches:

- Break patches referencing containers not in the baseline deployment
- Checks referencing resources that don't exist in bootstrap
- Kubectl wait steps targeting unknown resources
- Chaos steps with unsupported types
- Missing Evidra protocol fields on evidra-enabled scenarios
- Multi-stage scenarios: validates each stage's break and checks independently

```bash
go test -run TestImplementedScenarios ./pkg/scenario/ -v
```

**When to update:** Every new scenario must pass runtime contracts. If you add a new check type or resource kind, update `canonicalKind()` and `validateChecks()`.

### 2. Agent Loop Tests

**File:** `pkg/agent/loop_test.go`

Tests the multi-turn tool-use loop with mock provider and executor.

Key tests:
- **Context window** — `TestBuildContextWindow_*`: verifies memory windowing (full, stateless, window=N)
- **Message injection** — `TestRunLoop_InjectMessage`: mid-run user message delivery via `InjectChan`
- **Memory reset** — `TestRunLoop_MemoryReset`: conversation compaction via `MemoryResetChan`

```bash
go test -run TestBuildContextWindow ./pkg/agent/ -v
go test -run TestRunLoop ./pkg/agent/ -v
```

**Test doubles:**
- `mockProvider` — records received messages, returns canned tool calls
- `mockExecutor` — returns "ok" for any tool call

### 3. Verifier Polling

**File:** `pkg/verifier/poll_test.go`

Tests `PollChecks` — the retry loop used by multi-stage verification.

- `TestPollChecks_PassesAfterRetries` — checker passes on Nth call
- `TestPollChecks_TimeoutFails` — context deadline stops polling
- `TestPollChecks_ImmediatePass` — no retry needed

```bash
go test -run TestPollChecks ./pkg/verifier/ -v
```

**Test double:** `countingChecker` — tracks call count, passes after N calls.

### 4. Harness Orchestration

**File:** `pkg/harness/run_test.go`

Tests the full run lifecycle with fake providers and adapters. Covers:

- Dry-run validation
- Cluster provisioning and reuse
- Break injection (kubectl-apply, kubectl, shell)
- After-break step execution
- Chaos timing (concurrent with agent, repeat mode)
- Artifact writing (chaos timeline, verifier output)
- Evidence directory isolation per run

```bash
go test ./pkg/harness/ -v
```

**Test doubles:**
- `fakeProvider` — implements `environment.Provider` without real clusters
- `fakeAdapter` — implements `adapter.Adapter` with configurable exit codes

### 5. Store (SQLite)

**File:** `pkg/store/store_test.go`

Tests the results database with isolated temp databases per test.

- CRUD operations on RunRecord
- Filtering by model, provider, scenario, passed status
- Pagination (limit/offset)
- Aggregate statistics
- JSONL backup export and database rebuild from JSONL

```bash
go test ./pkg/store/ -v
```

### 6. Smoke Tests

**File:** `tests/smoke/run_local_smoke.sh`

Dry-run validation of the built binary against real scenarios:

```bash
make smoke
```

Validates:
- Binary builds and runs
- `scenario list` discovers all scenarios
- `run --dry-run` succeeds for kubernetes, helm, and argocd scenarios
- Scenario resolution works by path and by ID

## Adding a New Scenario

1. Create `scenarios/{category}/{name}/scenario.yaml`, `prompts/task.md`, `fixtures/`
2. Run runtime contract validation:
   ```bash
   go test -run TestImplementedScenarios ./pkg/scenario/ -v
   ```
3. Run smoke test:
   ```bash
   make smoke
   ```

For multi-stage scenarios, ensure each stage has `name`, `break`, and at least one `verify` check. The runtime contract validator checks stages independently.

## Adding a New Check Type

1. Implement the `Checker` interface in `pkg/verifier/`
2. Add to `BuildCheckers()` switch in `pkg/verifier/common.go`
3. Add to `validateChecks()` in `pkg/scenario/runtime_contract_test.go`
4. Write unit tests with mock kubectl output

## Adding Agent Features

1. Add fields to `LoopConfig` in `pkg/agent/loop.go`
2. Write tests using `mockProvider` and `mockExecutor` in `loop_test.go`
3. Wire through `runWithProvider()` in `pkg/harness/run.go`
4. Run full suite: `make test`

## Test Patterns

### Parallel by default

Every test uses `t.Parallel()`:

```go
func TestFoo_ValidInput(t *testing.T) {
    t.Parallel()
    // ...
}
```

### Table-driven tests

Functions with multiple input/output cases use table-driven:

```go
func TestParse_Scenarios(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name  string
        input string
        want  int
    }{
        {"empty", "", 0},
        {"one", "a", 1},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got := Parse(tt.input)
            if got != tt.want {
                t.Errorf("Parse(%q) = %d, want %d", tt.input, got, tt.want)
            }
        })
    }
}
```

### Temp directories

Filesystem tests use `t.TempDir()` — automatically cleaned up:

```go
dir := t.TempDir()
store, _ := store.Open(filepath.Join(dir, "test.db"))
```

### No mocking libraries

All test doubles are hand-written structs implementing the relevant interface. No testify, gomock, or similar.

## What's Not Tested

- **No integration tests** — no real cluster tests (would need `//go:build integration` tag)
- **No benchmarks** — no `BenchmarkXxx` functions
- **No UI tests** — lab.evidra.cc has no automated browser tests
- **No real Evidra API tests** — reporting tested with offline JSONL only
- **No cross-provider comparison tests** — each provider tested in isolation
