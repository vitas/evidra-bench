package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
