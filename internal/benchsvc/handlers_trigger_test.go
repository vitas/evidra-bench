package benchsvc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------- Trigger ----------

// spyExecutor records whether Start was called.
type spyExecutor struct {
	started   bool
	job       *TriggerJob
	startedCh chan struct{}
}

func (e *spyExecutor) Start(_ context.Context, job *TriggerJob, _, _ string) error {
	e.started = true
	e.job = job
	if e.startedCh != nil {
		close(e.startedCh)
		e.startedCh = nil
	}
	return nil
}

func TestHandleTrigger_NoExecutor_Returns501(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     nil, // no executor
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","evidence_mode":"mcp","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
}

func TestHandleTrigger_RequiresEvidenceMode(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     &spyExecutor{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleTrigger_RejectsInvalidEvidenceMode(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     &spyExecutor{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","evidence_mode":"legacy","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleTrigger_RejectsInvalidExecutionMode(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     &spyExecutor{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","execution_mode":"wat","evidence_mode":"mcp","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleTrigger_ValidRequest_Returns202(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	startedCh := make(chan struct{})
	spy := &spyExecutor{startedCh: startedCh}
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","evidence_mode":"mcp","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["id"]; !ok {
		t.Fatal("response missing 'id' key")
	}
	select {
	case <-startedCh:
	case <-time.After(time.Second):
		t.Fatal("executor Start was not called")
	}
	stored := store.Get(resp["id"].(string))
	if stored == nil {
		t.Fatal("stored trigger job missing")
	}
	if stored.EvidenceMode != "mcp" {
		t.Fatalf("stored evidence mode = %q, want mcp", stored.EvidenceMode)
	}
	if stored.ExecutionMode != "provider" {
		t.Fatalf("stored execution mode = %q, want provider", stored.ExecutionMode)
	}
	if spy.job == nil {
		t.Fatal("executor job missing")
	}
	if spy.job.EvidenceMode != "mcp" {
		t.Fatalf("job evidence mode = %q, want mcp", spy.job.EvidenceMode)
	}
	if spy.job.ExecutionMode != "provider" {
		t.Fatalf("job execution mode = %q, want provider", spy.job.ExecutionMode)
	}
}

func TestHandleTrigger_ValidRequest_Returns202_WithEvidenceModeNone(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	spy := &spyExecutor{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","evidence_mode":"none","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stored := store.Get(resp["id"].(string))
	if stored == nil {
		t.Fatal("stored trigger job missing")
	}
	if stored.EvidenceMode != "none" {
		t.Fatalf("stored evidence mode = %q, want none", stored.EvidenceMode)
	}
	if stored.ExecutionMode != "provider" {
		t.Fatalf("stored execution mode = %q, want provider", stored.ExecutionMode)
	}
}

func TestHandleTrigger_ValidRequest_Returns202_WithExecutionModeA2A(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	startedCh := make(chan struct{})
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	spy := &spyExecutor{startedCh: startedCh}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","execution_mode":"a2a","evidence_mode":"mcp","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stored := store.Get(resp["id"].(string))
	if stored == nil {
		t.Fatal("stored trigger job missing")
	}
	if stored.ExecutionMode != "a2a" {
		t.Fatalf("stored execution mode = %q, want a2a", stored.ExecutionMode)
	}
	select {
	case <-startedCh:
	case <-time.After(time.Second):
		t.Fatal("executor Start was not called")
	}
	if spy.job == nil {
		t.Fatal("executor job missing")
	}
	if spy.job.ExecutionMode != "a2a" {
		t.Fatalf("job execution mode = %q, want a2a", spy.job.ExecutionMode)
	}
}

func TestHandleTriggerProgress_UnknownJob_Returns404(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"scenario":"s1","status":"passed","completed":1,"total":1}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger/nonexistent/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandleTriggerProgress_InvalidVersion_Returns400(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	// Create a job first so the 404 check passes.
	job := &TriggerJob{
		ID:     "job-1",
		Status: "running",
		Total:  1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "running"},
		},
	}
	store.Create(job)

	rec := httptest.NewRecorder()
	body := `{"contract_version":"v2.0.0","scenario":"s1","status":"passed","completed":1,"total":1}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger/job-1/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// ---------- Runner Registration ----------

func TestHandleRegisterRunner_ValidModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		registeredRunner: &Runner{
			ID:     "runner-1",
			Status: "healthy",
			Config: RunnerConfig{Models: []string{"sonnet"}, PollInterval: 5},
		},
	}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	reqBody := `{"name":"my-runner","models":["sonnet"]}`
	req := httptest.NewRequest("POST", "/v1/runners/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["runner_id"] != "runner-1" {
		t.Fatalf("runner_id = %v, want runner-1", resp["runner_id"])
	}
	if resp["poll_interval"] != float64(5) {
		t.Fatalf("poll_interval = %v, want 5", resp["poll_interval"])
	}
}

func TestHandleRegisterRunner_MissingModels(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{}
	mux := setupMux(repo, ServiceConfig{PublicTenant: "pub"}, "t1")

	rec := httptest.NewRecorder()
	reqBody := `{"name":"my-runner"}`
	req := httptest.NewRequest("POST", "/v1/runners/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// ---------- Trigger with Runner ----------

func TestHandleTrigger_WithRunner_QueuesJob(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
		foundRunner: &Runner{
			ID:     "runner-1",
			Status: "healthy",
			Config: RunnerConfig{Models: []string{"sonnet"}},
		},
		enqueuedJob: &BenchJob{
			ID:       "job-q-1",
			Status:   "queued",
			Model:    "sonnet",
			Provider: "bifrost",
		},
		claimedJob: &BenchJob{
			ID:         "job-q-1",
			TenantID:   "pub",
			Model:      "sonnet",
			Provider:   "bifrost",
			Status:     "queued",
			ConfigJSON: json.RawMessage(`{"scenarios":["s1"],"evidence_mode":"mcp","execution_mode":"a2a"}`),
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Dispatcher:   &PoolDispatcher{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","execution_mode":"a2a","evidence_mode":"mcp","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["id"] != "job-q-1" {
		t.Fatalf("id = %v, want job-q-1", resp["id"])
	}
	if resp["mode"] != "runner" {
		t.Fatalf("mode = %v, want runner", resp["mode"])
	}
	if repo.lastEnqueueCfg.EvidenceMode != "mcp" {
		t.Fatalf("enqueue evidence mode = %q, want mcp", repo.lastEnqueueCfg.EvidenceMode)
	}
	if repo.lastEnqueueCfg.ExecutionMode != "a2a" {
		t.Fatalf("enqueue execution mode = %q, want a2a", repo.lastEnqueueCfg.ExecutionMode)
	}
	stored := store.Get("job-q-1")
	if stored == nil {
		t.Fatal("stored runner trigger job missing")
	}
	if stored.EvidenceMode != "mcp" {
		t.Fatalf("stored evidence mode = %q, want mcp", stored.EvidenceMode)
	}
	if stored.ExecutionMode != "a2a" {
		t.Fatalf("stored execution mode = %q, want a2a", stored.ExecutionMode)
	}
}

func TestHandleTrigger_WithRunner_AllowsProviderSuppliedModelAlias(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		foundRunner: &Runner{
			ID:     "runner-1",
			Status: "healthy",
			Config: RunnerConfig{Models: []string{"sonnet"}, Provider: "claude"},
		},
		enqueuedJob: &BenchJob{
			ID:       "job-alias-1",
			Status:   "queued",
			Model:    "sonnet",
			Provider: "claude",
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Dispatcher:   &PoolDispatcher{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","provider":"claude","evidence_mode":"mcp","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if repo.lastEnqueueCfg.EvidenceMode != "mcp" {
		t.Fatalf("enqueue evidence mode = %q, want mcp", repo.lastEnqueueCfg.EvidenceMode)
	}
	stored := store.Get("job-alias-1")
	if stored == nil {
		t.Fatal("stored runner trigger job missing")
	}
	if stored.Provider != "claude" {
		t.Fatalf("stored provider = %q, want claude", stored.Provider)
	}
}

func TestHandleTrigger_WithRunner_DoesNotRequireControlPlaneModelAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	store := NewTriggerStore()
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "anthropic", APIKeyEnv: "ANTHROPIC_API_KEY"},
		foundRunner: &Runner{
			ID:     "runner-1",
			Status: "healthy",
			Config: RunnerConfig{Models: []string{"claude-sonnet-4-20250514"}, Provider: "anthropic"},
		},
		enqueuedJob: &BenchJob{
			ID:       "job-runner-key-1",
			Status:   "queued",
			Model:    "claude-sonnet-4-20250514",
			Provider: "anthropic",
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Dispatcher:   &PoolDispatcher{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"claude-sonnet-4-20250514","evidence_mode":"mcp","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if stored := store.Get("job-runner-key-1"); stored == nil {
		t.Fatal("stored runner trigger job missing")
	}
}

func TestHandleTrigger_WithPinnedRunnerUnavailable_Returns400AndSkipsExecutor(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	spy := &spyExecutor{}
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
		// No healthy runner for this model. This used to fall through to V1.
		runners: []Runner{
			{
				ID:     "runner-other",
				Status: "healthy",
				Config: RunnerConfig{Models: []string{"other-model"}},
			},
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     spy,
		Dispatcher:   &PoolDispatcher{},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"sonnet","runner_id":"runner-missing","evidence_mode":"mcp","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if spy.started {
		t.Fatal("executor should not start when pinned runner is unavailable")
	}
}

func TestHandleCompleteJob_UpdatesTriggerStoreSnapshot(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	store.Create(&TriggerJob{
		ID:       "job-complete-1",
		Status:   "pending",
		Model:    "sonnet",
		Provider: "claude",
		Total:    2,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
			{Scenario: "s2", Status: "pending"},
		},
		CreatedAt: time.Now(),
	})

	svc := NewService(&handlerRepo{}, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"runner_id":"runner-1","status":"completed","passed":2,"failed":0}`
	req := httptest.NewRequest("POST", "/v1/runners/jobs/job-complete-1/complete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	got := store.Get("job-complete-1")
	if got == nil {
		t.Fatal("expected trigger job snapshot")
	}
	if got.Status != "completed" {
		t.Fatalf("trigger status = %q, want completed", got.Status)
	}
	if got.Completed != 2 || got.Passed != 2 || got.Failed != 0 {
		t.Fatalf("trigger counters = completed:%d passed:%d failed:%d, want 2/2/0", got.Completed, got.Passed, got.Failed)
	}
}

func TestHandlePollJob_ReturnsEvidenceMode(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runners: []Runner{
			{
				ID:     "runner-1",
				Status: "healthy",
				Config: RunnerConfig{Models: []string{"sonnet"}},
			},
		},
		claimedJob: &BenchJob{
			ID:       "job-q-2",
			TenantID: "pub",
			Model:    "sonnet",
			Provider: "bifrost",
			Status:   "queued",
			ConfigJSON: json.RawMessage(`{
				"scenarios":["s1"],
				"runner_id":"runner-1",
				"evidence_mode":"mcp",
				"execution_mode":"a2a"
			}`),
		},
	}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/runners/jobs?runner_id=runner-1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["provider"] != "bifrost" {
		t.Fatalf("provider = %v, want bifrost", resp["provider"])
	}
	if resp["evidence_mode"] != "mcp" {
		t.Fatalf("evidence_mode = %v, want mcp", resp["evidence_mode"])
	}
	if resp["execution_mode"] != "a2a" {
		t.Fatalf("execution_mode = %v, want a2a", resp["execution_mode"])
	}
}

func TestHandlePollJob_DefaultsEvidenceModeForLegacyJobs(t *testing.T) {
	t.Parallel()

	repo := &handlerRepo{
		runners: []Runner{
			{ID: "runner-1", Status: "healthy", Config: RunnerConfig{Models: []string{"sonnet"}}},
		},
		claimedJob: &BenchJob{
			ID: "job-legacy", TenantID: "pub", Model: "sonnet", Provider: "bifrost",
			Status: "queued", ConfigJSON: json.RawMessage(`{"scenarios":["s1"]}`),
		},
	}
	svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/runners/jobs?runner_id=runner-1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["evidence_mode"] != "none" {
		t.Fatalf("evidence_mode = %v, want none", resp["evidence_mode"])
	}
	if resp["execution_mode"] != "provider" {
		t.Fatalf("execution_mode = %v, want provider", resp["execution_mode"])
	}
}

func TestHandlePollJob_RejectsMalformedConfigJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configJSON string
	}{
		{
			name:       "malformed config json",
			configJSON: `{"scenarios":["s1"],"evidence_mode":`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &handlerRepo{
				runners: []Runner{
					{
						ID:     "runner-1",
						Status: "healthy",
						Config: RunnerConfig{Models: []string{"sonnet"}},
					},
				},
				claimedJob: &BenchJob{
					ID:         "job-q-2",
					TenantID:   "pub",
					Model:      "sonnet",
					Provider:   "bifrost",
					Status:     "queued",
					ConfigJSON: json.RawMessage(tt.configJSON),
				},
			}
			svc := NewService(repo, ServiceConfig{PublicTenant: "pub"})
			mux := http.NewServeMux()
			RegisterRoutes(mux, svc, passthroughAuth("t1"))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/runners/jobs?runner_id=runner-1", nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}
