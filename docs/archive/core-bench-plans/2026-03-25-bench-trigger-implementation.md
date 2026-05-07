# Bench Trigger Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a bench trigger system to Evidra that delegates scenario execution to a pluggable executor (local or remote) and streams progress to the UI via SSE.

**Architecture:** New `internal/benchsvc/trigger.go` handles job state and SSE. New handler `POST /v1/bench/trigger` starts a job, `GET /v1/bench/trigger/{id}` streams SSE, `POST /v1/bench/trigger/{id}/progress` receives webhooks. UI gets a "Run" button on the bench page.

**Tech Stack:** Go (stdlib net/http, SSE via flusher), React/TypeScript, existing benchsvc

---

### Task 1: RunExecutor Interface and Types

**Files:**
- Create: `internal/benchsvc/executor.go`
- Test: `internal/benchsvc/executor_test.go`

**Step 1: Write the types and interface**

```go
package benchsvc

import (
	"context"
	"sync"
	"time"
)

// TriggerRequest is sent by the UI to start a benchmark run.
type TriggerRequest struct {
	Model     string   `json:"model"`
	Provider  string   `json:"provider"`
	Scenarios []string `json:"scenarios"`
}

// TriggerJob tracks one benchmark trigger lifecycle.
type TriggerJob struct {
	ID        string          `json:"id"`
	Status    string          `json:"status"` // pending, running, completed, failed
	Model     string          `json:"model"`
	Provider  string          `json:"provider"`
	Total     int             `json:"total"`
	Completed int             `json:"completed"`
	Passed    int             `json:"passed"`
	Failed    int             `json:"failed"`
	Current   string          `json:"current_scenario,omitempty"`
	RunIDs    []string        `json:"run_ids,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	Progress  []ScenarioProgress `json:"progress,omitempty"`
}

// ScenarioProgress tracks one scenario within a job.
type ScenarioProgress struct {
	Scenario string `json:"scenario"`
	Status   string `json:"status"` // pending, running, passed, failed, error, skipped
	RunID    string `json:"run_id,omitempty"`
}

// ProgressUpdate is sent by the executor (or webhook) to update job state.
type ProgressUpdate struct {
	JobID     string `json:"job_id"`
	Scenario  string `json:"scenario"`
	Status    string `json:"status"`
	RunID     string `json:"run_id,omitempty"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

// RunExecutor executes benchmark scenarios and reports results.
type RunExecutor interface {
	Start(ctx context.Context, job *TriggerJob, evidraURL, apiKey string) error
}

// TriggerStore holds active jobs in memory (no DB needed for MVP).
type TriggerStore struct {
	mu       sync.RWMutex
	jobs     map[string]*TriggerJob
	channels map[string][]chan ProgressUpdate
}

func NewTriggerStore() *TriggerStore {
	return &TriggerStore{
		jobs:     make(map[string]*TriggerJob),
		channels: make(map[string][]chan ProgressUpdate),
	}
}

func (s *TriggerStore) Create(job *TriggerJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *TriggerStore) Get(id string) (*TriggerJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

func (s *TriggerStore) Update(update ProgressUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[update.JobID]
	if !ok {
		return
	}
	job.Completed = update.Completed
	job.Current = update.Scenario
	for i, p := range job.Progress {
		if p.Scenario == update.Scenario {
			job.Progress[i].Status = update.Status
			job.Progress[i].RunID = update.RunID
			break
		}
	}
	if update.Status == "passed" {
		job.Passed++
		job.RunIDs = append(job.RunIDs, update.RunID)
	} else if update.Status == "failed" {
		job.Failed++
		if update.RunID != "" {
			job.RunIDs = append(job.RunIDs, update.RunID)
		}
	}
	if update.Completed >= update.Total {
		job.Status = "completed"
	} else {
		job.Status = "running"
	}
	// Notify SSE listeners.
	for _, ch := range s.channels[update.JobID] {
		select {
		case ch <- update:
		default:
		}
	}
}

func (s *TriggerStore) Subscribe(jobID string) chan ProgressUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan ProgressUpdate, 10)
	s.channels[jobID] = append(s.channels[jobID], ch)
	return ch
}

func (s *TriggerStore) Unsubscribe(jobID string, ch chan ProgressUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	channels := s.channels[jobID]
	for i, c := range channels {
		if c == ch {
			s.channels[jobID] = append(channels[:i], channels[i+1:]...)
			close(ch)
			return
		}
	}
}
```

**Step 2: Write test**

```go
package benchsvc

import (
	"testing"
	"time"
)

func TestTriggerStore_CreateAndGet(t *testing.T) {
	t.Parallel()
	store := NewTriggerStore()
	job := &TriggerJob{
		ID:     "test-1",
		Status: "pending",
		Total:  2,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
			{Scenario: "s2", Status: "pending"},
		},
	}
	store.Create(job)
	got, ok := store.Get("test-1")
	if !ok || got.ID != "test-1" {
		t.Fatal("expected job")
	}
}

func TestTriggerStore_UpdateNotifiesSubscriber(t *testing.T) {
	t.Parallel()
	store := NewTriggerStore()
	job := &TriggerJob{
		ID:     "test-2",
		Status: "pending",
		Total:  1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
		},
	}
	store.Create(job)
	ch := store.Subscribe("test-2")

	store.Update(ProgressUpdate{
		JobID:     "test-2",
		Scenario:  "s1",
		Status:    "passed",
		RunID:     "run-1",
		Completed: 1,
		Total:     1,
	})

	select {
	case update := <-ch:
		if update.Status != "passed" {
			t.Fatalf("expected passed, got %s", update.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for update")
	}

	got, _ := store.Get("test-2")
	if got.Status != "completed" {
		t.Fatalf("expected completed, got %s", got.Status)
	}
}
```

**Step 3: Run tests**
```bash
go test ./internal/benchsvc/ -run TestTriggerStore -v -count=1
```

**Step 4: Commit**
```bash
git add internal/benchsvc/executor.go internal/benchsvc/executor_test.go
git commit -m "feat: add RunExecutor interface and TriggerStore for bench jobs

Signed-off-by: Vitas <vitas@users.noreply.github.com>"
```

---

### Task 2: Trigger HTTP Handlers (POST + SSE + webhook)

**Files:**
- Create: `internal/benchsvc/trigger_handler.go`
- Modify: `internal/benchsvc/handlers.go` (add routes)

**Step 1: Write the trigger handler**

```go
package benchsvc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"

	"samebits.com/evidra/internal/apiutil"
	"samebits.com/evidra/internal/auth"
)

func handleTrigger(svc *Service, store *TriggerStore, executor RunExecutor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if executor == nil {
			apiutil.WriteError(w, http.StatusNotImplemented, "bench executor not configured")
			return
		}
		tenantID := auth.TenantID(r.Context())

		var req TriggerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if req.Model == "" || len(req.Scenarios) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "model and scenarios required")
			return
		}

		job := &TriggerJob{
			ID:        "trigger-" + ulid.Make().String(),
			Status:    "pending",
			Model:     req.Model,
			Provider:  req.Provider,
			Total:     len(req.Scenarios),
			CreatedAt: time.Now().UTC(),
		}
		for _, s := range req.Scenarios {
			job.Progress = append(job.Progress, ScenarioProgress{
				Scenario: s,
				Status:   "pending",
			})
		}
		store.Create(job)

		// Resolve Evidra URL for callbacks.
		evidraURL := resolveEvidraURL(r)
		apiKey := r.Header.Get("Authorization")

		go func() {
			_ = executor.Start(r.Context(), job, evidraURL, apiKey)
		}()

		apiutil.WriteJSON(w, http.StatusAccepted, map[string]string{
			"id":     job.ID,
			"status": job.Status,
		})
	}
}

func handleTriggerStatus(store *TriggerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := r.PathValue("id")
		job, ok := store.Get(jobID)
		if !ok {
			apiutil.WriteError(w, http.StatusNotFound, "job not found")
			return
		}

		// SSE stream.
		flusher, ok := w.(http.Flusher)
		if !ok {
			// Fallback: return JSON snapshot.
			apiutil.WriteJSON(w, http.StatusOK, job)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send current state.
		data, _ := json.Marshal(job)
		fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
		flusher.Flush()

		if job.Status == "completed" || job.Status == "failed" {
			return
		}

		ch := store.Subscribe(jobID)
		defer store.Unsubscribe(jobID, ch)

		for {
			select {
			case update, open := <-ch:
				if !open {
					return
				}
				data, _ := json.Marshal(update)
				fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
				flusher.Flush()
				if update.Completed >= update.Total {
					j, _ := store.Get(jobID)
					final, _ := json.Marshal(j)
					fmt.Fprintf(w, "event: complete\ndata: %s\n\n", final)
					flusher.Flush()
					return
				}
			case <-r.Context().Done():
				return
			}
		}
	}
}

func handleTriggerProgress(store *TriggerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := r.PathValue("id")
		_, ok := store.Get(jobID)
		if !ok {
			apiutil.WriteError(w, http.StatusNotFound, "job not found")
			return
		}

		var update ProgressUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		update.JobID = jobID
		store.Update(update)
		w.WriteHeader(http.StatusOK)
	}
}

func resolveEvidraURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
```

**Step 2: Register routes in handlers.go**

Add to `RegisterRoutes` after existing routes:

```go
	// Bench trigger (remote/local executor).
	if cfg.TriggerStore != nil {
		mux.Handle("POST /v1/bench/trigger", authMw(http.HandlerFunc(handleTrigger(svc, cfg.TriggerStore, cfg.Executor))))
		mux.Handle("GET /v1/bench/trigger/{id}", authMw(http.HandlerFunc(handleTriggerStatus(cfg.TriggerStore))))
		mux.Handle("POST /v1/bench/trigger/{id}/progress", authMw(http.HandlerFunc(handleTriggerProgress(cfg.TriggerStore))))
	}
```

This requires adding `TriggerStore *TriggerStore` and `Executor RunExecutor` to the `RegisterRoutes` params. Currently it takes `(mux, svc, authMw)`. Change to accept a config struct or add params.

Simplest: add fields to `ServiceConfig`:

```go
type ServiceConfig struct {
	PublicTenant string
	TriggerStore *TriggerStore
	Executor     RunExecutor
}
```

Then `RegisterRoutes` reads from `svc.cfg`.

**Step 3: Run tests**
```bash
go build ./... && go test ./internal/benchsvc/ -v -count=1
```

**Step 4: Commit**
```bash
git add internal/benchsvc/trigger_handler.go internal/benchsvc/handlers.go internal/benchsvc/service.go
git commit -m "feat: add bench trigger handlers — POST trigger, SSE status, webhook progress

Signed-off-by: Vitas <vitas@users.noreply.github.com>"
```

---

### Task 3: RemoteExecutor Implementation

**Files:**
- Create: `internal/benchsvc/remote_executor.go`
- Test: `internal/benchsvc/remote_executor_test.go`

**Step 1: Write RemoteExecutor**

```go
package benchsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// RemoteExecutor delegates benchmark execution to an external REST service.
type RemoteExecutor struct {
	ServiceURL string
	HTTPClient *http.Client
}

func NewRemoteExecutor(serviceURL string) *RemoteExecutor {
	return &RemoteExecutor{
		ServiceURL: strings.TrimRight(serviceURL, "/"),
		HTTPClient: http.DefaultClient,
	}
}

func (e *RemoteExecutor) Start(ctx context.Context, job *TriggerJob, evidraURL, apiKey string) error {
	payload := map[string]any{
		"contract_version": "v1.0.0",
		"job_id":           job.ID,
		"model":            job.Model,
		"provider":         job.Provider,
		"scenarios":        scenarioIDs(job),
		"callback": map[string]string{
			"progress_url": evidraURL + "/v1/bench/trigger/" + job.ID + "/progress",
			"evidra_url":   evidraURL,
			"evidra_api_key": apiKey,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("remote executor: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.ServiceURL+"/v1/certify", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("remote executor: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("remote executor: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("remote executor: HTTP %d", resp.StatusCode)
	}
	return nil
}

func scenarioIDs(job *TriggerJob) []string {
	ids := make([]string, len(job.Progress))
	for i, p := range job.Progress {
		ids[i] = p.Scenario
	}
	return ids
}
```

**Step 2: Run tests**
```bash
go build ./... && go test ./internal/benchsvc/ -v -count=1
```

**Step 3: Commit**
```bash
git add internal/benchsvc/remote_executor.go
git commit -m "feat: add RemoteExecutor — delegates to bench service via REST

Signed-off-by: Vitas <vitas@users.noreply.github.com>"
```

---

### Task 4: Wire into cmd/evidra-api

**Files:**
- Modify: `cmd/evidra-api/main.go`

**Step 1: Add config and wiring**

In `configurePersistence` (or equivalent), add:

```go
	triggerStore := benchsvc.NewTriggerStore()

	var executor benchsvc.RunExecutor
	if benchServiceURL := os.Getenv("EVIDRA_BENCH_SERVICE_URL"); benchServiceURL != "" {
		executor = benchsvc.NewRemoteExecutor(benchServiceURL)
		deps.logf("bench executor: remote (%s)", benchServiceURL)
	}

	benchService := benchsvc.NewService(repo, benchsvc.ServiceConfig{
		PublicTenant: envOr("EVIDRA_BENCH_PUBLIC_TENANT", defaultTenant),
		TriggerStore: triggerStore,
		Executor:     executor,
	})
```

**Step 2: Build and test**
```bash
go build ./... && go test ./... -count=1 2>&1 | grep FAIL
```

**Step 3: Commit**
```bash
git add cmd/evidra-api/main.go
git commit -m "feat: wire TriggerStore and RemoteExecutor into evidra-api

EVIDRA_BENCH_SERVICE_URL enables remote executor. Without it,
trigger endpoint returns 501.

Signed-off-by: Vitas <vitas@users.noreply.github.com>"
```

---

### Task 5: UI — "Run" Button + Progress on Bench Page

**Files:**
- Modify: `ui/src/pages/bench/BenchDashboard.tsx`

**Step 1: Add trigger modal and SSE progress**

Add to BenchDashboard:
- "Run Benchmark" button in the header
- Modal: model input, scenario checkboxes, submit
- On submit: POST /v1/bench/trigger
- SSE listener: GET /v1/bench/trigger/{id} (EventSource)
- Progress overlay: scenario list with checkmarks
- On complete: close overlay, refresh data

Keep it minimal — just a button, a modal, and a progress list.

**Step 2: TypeScript check**
```bash
cd ui && npx tsc --noEmit
```

**Step 3: Commit**
```bash
git add ui/src/pages/bench/BenchDashboard.tsx
git commit -m "feat(ui): add Run Benchmark button with SSE progress

Triggers POST /v1/bench/trigger, streams progress via SSE,
shows scenario checkmarks, redirects to results on completion.

Signed-off-by: Vitas <vitas@users.noreply.github.com>"
```

---

### Task 6: Update Docs and Architecture

**Files:**
- Modify: `docs/ARCHITECTURE.md`
- Modify: `CLAUDE.md`

**Step 1: Add executor interface to ARCHITECTURE.md**

Add a "Bench Execution" section describing the trigger flow,
executor interface, and local/remote split.

**Step 2: Update CLAUDE.md**

Add `EVIDRA_BENCH_SERVICE_URL` to environment variables section.
Add trigger endpoints to API surface.

**Step 3: Commit**
```bash
git add docs/ARCHITECTURE.md CLAUDE.md
git commit -m "docs: add bench trigger and executor interface to architecture

Signed-off-by: Vitas <vitas@users.noreply.github.com>"
```

---

### Task 7: Final Verification

**Step 1: Build**
```bash
make build
```

**Step 2: Tests**
```bash
make test
```

**Step 3: Lint**
```bash
make fmt && make lint
```

**Step 4: Verify trigger endpoint works**

Start the stack, call:
```bash
curl -X POST http://localhost:28080/v1/bench/trigger \
  -H "Authorization: Bearer dev-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-chat","provider":"deepseek","scenarios":["broken-deployment"]}'
```

Without `EVIDRA_BENCH_SERVICE_URL`: expect 501.
With it: expect 202 + job ID.
