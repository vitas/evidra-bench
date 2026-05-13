# Parallel Bench Runner — Design Doc

## Problem

Benchmark runs execute sequentially — one scenario at a time. A full suite
(62 scenarios × 1 model) takes ~1 hour. Multiple models multiply linearly.
The current `serve.go` is a single goroutine with a mutex — no parallelism,
no persistence, no crash recovery.

## Goal

Run N scenarios in parallel on a single Mac (or Linux host), reducing
wall-clock time by ~4x. Lay the groundwork for the River-based SaaS
job engine without over-engineering the local path.

## Architecture

```
┌─────────────────────────────────────────────────┐
│  CLI / API                                       │
│  infra-bench bench --parallel 4 --model sonnet   │
└──────────────┬──────────────────────────────────┘
               │ enqueue N jobs
               ▼
┌─────────────────────────────────────────────────┐
│  Job Queue (in-memory channel, later River/PG)   │
│  [scenario-1] [scenario-2] ... [scenario-62]     │
└──────────────┬──────────────────────────────────┘
               │ workers pick jobs
               ▼
┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
│Worker 1│ │Worker 2│ │Worker 3│ │Worker 4│
│ns:run-1│ │ns:run-2│ │ns:run-3│ │ns:run-4│
└───┬────┘ └───┬────┘ └───┬────┘ └───┬────┘
    │          │          │          │
    └──────────┴──────────┴──────────┘
               │
     shared kind cluster
```

## Design

### River from day one

No throwaway in-memory queue. River (github.com/riverqueue/river) is a Go
library, not a service — adding it is a dependency + ~100 lines of wiring.
It uses the existing PostgreSQL, runs workers as goroutines, and gives us
persistence, retries, and SaaS foundation from the start.

**Job type:**
```go
type BenchJobArgs struct {
    TenantID   string `json:"tenant_id"`
    ScenarioID string `json:"scenario_id"`
    Model      string `json:"model"`
    Provider   string `json:"provider"`
    MCPServer  string `json:"mcp_server,omitempty"`
    WorkerID   int    `json:"worker_id"`
    Namespace  string `json:"namespace"`
}

func (BenchJobArgs) Kind() string { return "bench_scenario" }

type BenchWorker struct {
    river.WorkerDefaults[BenchJobArgs]
}

func (w *BenchWorker) Work(ctx context.Context, job *river.Job[BenchJobArgs]) error {
    ns := fmt.Sprintf("bench-w%d", job.Args.WorkerID)
    // bootstrap → break → agent → verify → store results
    return runScenarioInNamespace(ctx, job.Args, ns)
}
```

**Enqueue (CLI or API):**
```go
// CLI: infra-bench bench --parallel 4 --model sonnet
for _, s := range scenarios {
    client.Insert(ctx, BenchJobArgs{
        ScenarioID: s.ID,
        Model:      model,
        Provider:   provider,
    }, nil)
}

// API: POST /v1/certify
for _, sid := range req.Scenarios {
    client.Insert(ctx, BenchJobArgs{
        TenantID:   tenantID,
        ScenarioID: sid,
        Model:      req.Model,
    }, &river.InsertOpts{Queue: tenantID})
}
```

**Worker pool:**
```go
riverClient, _ := river.NewClient(riverpgxv5.New(pool), &river.Config{
    Queues: map[string]river.QueueConfig{
        river.QueueDefault: {MaxWorkers: parallel}, // --parallel flag
    },
    Workers: workers,
})
```

**Namespace per worker:**
Each worker gets a dedicated namespace (`bench-w0`, `bench-w1`, ..., `bench-wN`).
The scenario's `scope.namespaces` is rewritten from `bench` to the worker's
namespace before execution. All kubectl/helm commands receive the worker's
namespace. Cleanup happens per-namespace after each scenario completes.

**Shared kind cluster:**
One `kind create cluster` at startup. All workers share it. Kind handles
the Docker layer; workers only manage namespaces. On Mac, each kind cluster
costs ~1GB RAM, so one shared cluster is efficient.

**CLI flag:**
```
infra-bench bench --parallel 4 --model sonnet --provider bifrost
```

Default `--parallel 1` preserves current behavior.

**Concurrency limits:**
- Mac recommended: `--parallel 3-4` (8GB+ RAM)
- Linux/CI: `--parallel 8-12` depending on node size
- Each worker: ~200MB overhead (namespace resources)

**What River gives us immediately:**
- Job persistence — crash mid-run, restart, jobs resume
- Retry with backoff — transient failures don't lose runs
- Job status — query progress from PostgreSQL
- Priority queues — per-tenant queue priority for SaaS
- Completion hooks — trigger progress webhooks on job done
- Unique jobs — prevent duplicate scenario+model runs

### serve.go refactor

Current `serve.go` (goroutine + mutex) becomes:

```go
func handleCertifyAPI(riverClient *river.Client) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // ... validate request ...
        for _, sid := range req.Scenarios {
            riverClient.Insert(ctx, BenchJobArgs{
                TenantID:   tenantID,
                ScenarioID: sid,
                Model:      req.Model,
            }, nil)
        }
        writeJSON(w, http.StatusAccepted, map[string]string{
            "job_id": jobID, "status": "accepted",
        })
    }
}
```

No mutex, no single-goroutine limit. Multiple certify requests can
run in parallel, each scenario is an independent River job.

### Later: K8s pods (cloud)

Workers become Kubernetes pods instead of goroutines:
- DinD sidecar for kind clusters (Linux)
- Or connect to tenant's vCluster (enterprise)
- River still manages the queue; K8s manages compute

## Job workspace isolation

Each job runs in its own temporary workspace. No writes to the repo.
No shared state between jobs. No untracked files.

**Job lifecycle:**
```
1. River picks job
2. Create workspace:  /tmp/bench-jobs/<job-id>/
3. Copy scenarios:    cp -r scenarios/ → workspace/scenarios/
4. Create dirs:       workspace/runs/, workspace/evidence/
5. Set agent cwd:     workspace/ (agent writes here, not repo root)
6. Run scenario:      all paths relative to workspace
7. Store results:     PostgreSQL (via River) + Bench API
8. Cleanup:           rm -rf workspace/
```

**What this fixes:**
- Terraform fixtures overwritten by agents (current bug)
- Agent-generated YAML files in repo root (current bug)
- Race conditions between parallel workers writing same files
- Untracked files accumulating in git status

**Implementation:**
```go
type JobWorkspace struct {
    Root        string // /tmp/bench-jobs/<job-id>
    ScenariosDir string // Root/scenarios
    RunsDir     string // Root/runs
    EvidenceDir string // Root/evidence
}

func NewJobWorkspace(jobID string, srcScenariosDir string) (*JobWorkspace, error) {
    root := filepath.Join(os.TempDir(), "bench-jobs", jobID)
    ws := &JobWorkspace{
        Root:         root,
        ScenariosDir: filepath.Join(root, "scenarios"),
        RunsDir:      filepath.Join(root, "runs"),
        EvidenceDir:  filepath.Join(root, "evidence"),
    }
    // Copy scenarios to workspace (writable copy)
    if err := copyDir(srcScenariosDir, ws.ScenariosDir); err != nil {
        return nil, fmt.Errorf("workspace: copy scenarios: %w", err)
    }
    os.MkdirAll(ws.RunsDir, 0755)
    os.MkdirAll(ws.EvidenceDir, 0755)
    return ws, nil
}

func (ws *JobWorkspace) Cleanup() {
    os.RemoveAll(ws.Root)
}
```

**Agent environment:**
```go
env := []string{
    "BENCH_WORKSPACE=" + ws.Root,
    "KUBECONFIG=" + kubeconfigPath,
    "BENCH_NAMESPACE=" + workerNamespace,
}
// Agent cwd = workspace root, all writes stay isolated
cmd.Dir = ws.Root
```

**Results flow:**
- No JSONL file writes — results go directly to PostgreSQL via River job metadata
- Bench API reporting happens from the worker, same as today
- Run artifacts (transcript, tool-calls) written to workspace, uploaded to object storage or Bench API, then workspace deleted
- Local CLI mode can optionally still write to `runs/` for backward compatibility

## Namespace isolation — what changes

Currently the harness hardcodes `bench` as the target namespace. Changes needed:

1. **`pkg/harness/run.go`**: Accept `targetNamespace` parameter, pass to all
   kubectl commands, bootstrap steps, verifier checks.

2. **`pkg/scenario/types.go`**: `scope.namespaces` becomes a template — worker
   substitutes its namespace at runtime.

3. **`pkg/verifier/`**: All check types already accept `namespace` parameter —
   no change needed.

4. **`pkg/agent/tools.go`**: `BENCH_NAMESPACE` env var injected into agent
   process so MCP tools use the correct namespace.

5. **Bootstrap steps**: `kubectl-apply` with `-n` flag set to worker namespace.
   Fixtures that hardcode `namespace: bench` need a sed/envsubst pass.

## Fixture namespace rewriting

Most fixtures contain `namespace: bench` in YAML. Two approaches:

**Option A: sed at runtime (simple)**
```go
content := os.ReadFile(fixturePath)
content = bytes.ReplaceAll(content, []byte("namespace: bench"),
    []byte(fmt.Sprintf("namespace: %s", workerNS)))
tmpFile := writeTempFile(content)
kubectl apply -f tmpFile
```

**Option B: kustomize overlay (clean but heavier)**
- Generate kustomization.yaml per worker with namespace transformer
- Requires kustomize binary

Recommend Option A — simple, no dependencies, covers all cases.

## Conflict scenarios

Some scenarios may conflict when running in parallel on a shared cluster:

| Resource | Conflict? | Mitigation |
|---|---|---|
| Namespaced resources | No | Each worker has its own namespace |
| ClusterRoles / ClusterRoleBindings | Yes | Prefix with worker ID |
| StorageClasses | Yes | Prefix with worker ID |
| ValidatingWebhooks | Yes | Prefix with worker ID |
| CRDs | Yes | Shared — run CRD scenarios sequentially |
| Node-level changes | Yes | Not applicable (single-node kind) |

Cluster-scoped resources need worker-ID prefixing. ~5 scenarios use cluster-scoped
resources and need special handling. The rest (~57) are namespace-scoped and safe
to parallelize immediately.

## Results collection

Workers write to a thread-safe results collector:

```go
type ResultCollector struct {
    mu      sync.Mutex
    results []RunResult
}

func (rc *ResultCollector) Add(r RunResult) {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    rc.results = append(rc.results, r)
}
```

JSONL writes are serialized through the collector. SQLite writes use WAL mode
for concurrent readers.

## Progress reporting

Live TUI progress (Bubble Tea) showing worker status:

```
[w0] ██████████░░ kubernetes/broken-deployment     PASS  32s
[w1] ████████░░░░ kubernetes/wrong-probes          running...
[w2] ██████░░░░░░ helm/failed-upgrade              running...
[w3] ████░░░░░░░░ kubernetes/dns-resolution-failure running...

Progress: 28/62 scenarios  |  Pass: 24  Fail: 4  |  ETA: 12m
```

## Estimated impact

| Parallel | Wall time (62 scenarios) | Speedup |
|---|---|---|
| 1 (current) | ~60 min | 1x |
| 3 | ~22 min | 2.7x |
| 4 | ~17 min | 3.5x |
| 6 | ~12 min | 5x |

Diminishing returns above 4 on Mac due to CPU/memory contention.

## Dependencies

```
go get github.com/riverqueue/river
go get github.com/riverqueue/river/riverdriver/riverpgxv5
```

River migrations auto-create its tables in PostgreSQL (`river_job`, `river_leader`, etc.).

## Implementation order

1. Add River dependency, connect to PostgreSQL, register BenchWorker
2. Namespace rewriting (sed approach) for fixtures and bootstrap
3. `--parallel` flag wires River worker pool size
4. Refactor `serve.go` to enqueue River jobs instead of goroutine+mutex
5. Cluster-scoped resource prefixing for ~5 scenarios
6. TUI progress display (reads River job status)
7. (Later) Per-tenant queues with priority for SaaS
8. (Later) K8s pod workers for cloud execution
