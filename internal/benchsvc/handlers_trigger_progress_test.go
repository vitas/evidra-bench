package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

func TestHandleTriggerStatus_RestoresPersistedRunnerJobWhenTriggerStoreIsEmpty(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	store := NewTriggerStore()
	repo := &handlerRepo{
		persistedJob: &BenchJob{
			ID:         "job-persisted-1",
			TenantID:   "t1",
			Model:      "sonnet",
			Provider:   "bifrost",
			Status:     "running",
			Total:      2,
			Completed:  1,
			Passed:     1,
			Failed:     0,
			ConfigJSON: json.RawMessage(`{"scenarios":["s1","s2"],"execution_mode":"a2a","mcp_server":"npx mcp"}`),
			CreatedAt:  createdAt,
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/bench/trigger/job-persisted-1", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got TriggerJob
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "job-persisted-1" || got.Status != "running" || got.Completed != 1 || got.Passed != 1 || got.Failed != 0 {
		t.Fatalf("restored job = id:%q status:%q completed:%d passed:%d failed:%d, want persisted counters", got.ID, got.Status, got.Completed, got.Passed, got.Failed)
	}
	if got.ExecutionMode != "a2a" || got.MCPServer != "npx mcp" {
		t.Fatalf("restored config = execution_mode:%q mcp_server:%q, want a2a/npx mcp", got.ExecutionMode, got.MCPServer)
	}
	if len(got.Progress) != 2 || got.Progress[0].Scenario != "s1" || got.Progress[1].Scenario != "s2" {
		t.Fatalf("progress = %#v, want scenarios from persisted config", got.Progress)
	}
	if repo.lastTenant != "t1" {
		t.Fatalf("tenant = %q, want t1", repo.lastTenant)
	}
	if stored := store.Get("job-persisted-1"); stored == nil {
		t.Fatal("expected restored job to be cached in trigger store")
	}
}

func TestHandleTriggerProgress_RestoresPersistedRunnerJobWhenTriggerStoreIsEmpty(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	repo := &handlerRepo{
		persistedJob: &BenchJob{
			ID:         "job-persisted-progress",
			TenantID:   "t1",
			Model:      "sonnet",
			Provider:   "bifrost",
			Status:     "claimed",
			Total:      2,
			ConfigJSON: json.RawMessage(`{"scenarios":["s1","s2"]}`),
			CreatedAt:  time.Now(),
		},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"scenario":"s1","status":"passed","run_id":"run-1","completed":1,"total":2}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger/job-persisted-progress/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := store.Get("job-persisted-progress")
	if got == nil {
		t.Fatal("expected progress handler to restore trigger store snapshot")
	}
	if got.Status != "running" || got.Completed != 1 || got.Passed != 1 || got.Failed != 0 {
		t.Fatalf("stored job = status:%q completed:%d passed:%d failed:%d, want running 1/1/0", got.Status, got.Completed, got.Passed, got.Failed)
	}
	if repo.lastProgressJob != "job-persisted-progress" || repo.lastProgressDone != 1 || repo.lastProgressPass != 1 || repo.lastProgressFail != 0 {
		t.Fatalf("persisted progress = job:%q done:%d passed:%d failed:%d, want job-persisted-progress 1/1/0", repo.lastProgressJob, repo.lastProgressDone, repo.lastProgressPass, repo.lastProgressFail)
	}
}

func TestHandleTriggerProgress_DuplicateCallbackPersistsIdempotentCounters(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	store.Create(&TriggerJob{
		ID:     "job-duplicate-callback",
		Status: "pending",
		Total:  1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
		},
		CreatedAt: time.Now(),
	})
	repo := &handlerRepo{}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	for range 2 {
		rec := httptest.NewRecorder()
		body := `{"scenario":"s1","status":"passed","run_id":"run-1","completed":1,"total":1}`
		req := httptest.NewRequest("POST", "/v1/bench/trigger/job-duplicate-callback/progress", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	}

	if repo.lastProgressJob != "job-duplicate-callback" || repo.lastProgressDone != 1 || repo.lastProgressPass != 1 || repo.lastProgressFail != 0 {
		t.Fatalf("persisted progress = job:%q done:%d passed:%d failed:%d, want job-duplicate-callback 1/1/0", repo.lastProgressJob, repo.lastProgressDone, repo.lastProgressPass, repo.lastProgressFail)
	}
}

// ---------- Runner Registration ----------
