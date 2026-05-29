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
	body := `{"model":"test-model","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
}

func TestHandleTrigger_DefaultsExecutionMode(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	startedCh := make(chan struct{})
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	svc := NewService(repo, ServiceConfig{
		PublicTenant: "pub",
		TriggerStore: store,
		Executor:     &spyExecutor{startedCh: startedCh},
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","scenarios":["s1","s2"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	select {
	case <-startedCh:
	case <-time.After(time.Second):
		t.Fatal("executor Start was not called")
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stored := store.Get(resp["id"].(string))
	if stored == nil {
		t.Fatal("stored trigger job missing")
		return
	}
	if stored.ExecutionMode != "provider" {
		t.Fatalf("stored execution mode = %q, want provider", stored.ExecutionMode)
	}
}

func TestNormalizeTriggerExecutionMode_UsesExecutionModeConstants(t *testing.T) {
	t.Parallel()

	got, ok := normalizeTriggerExecutionMode("")
	if !ok {
		t.Fatal("normalizeTriggerExecutionMode(\"\") ok = false, want true")
	}
	if got != ExecutionModeProvider {
		t.Fatalf("default mode = %q, want %q", got, ExecutionModeProvider)
	}

	got, ok = normalizeTriggerExecutionMode(ExecutionModeA2A)
	if !ok {
		t.Fatalf("normalizeTriggerExecutionMode(%q) ok = false, want true", ExecutionModeA2A)
	}
	if got != ExecutionModeA2A {
		t.Fatalf("mode = %q, want %q", got, ExecutionModeA2A)
	}
}

func TestHandleTrigger_UsesServiceBackgroundContext(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()
	startedCh := make(chan struct{})
	spy := &spyExecutor{startedCh: startedCh}
	repo := &handlerRepo{
		modelProvider: &ModelProviderInfo{Provider: "bifrost"},
	}
	bgCtx := context.WithValue(context.Background(), testContextKey{}, "service-context")
	svc := NewService(repo, ServiceConfig{
		PublicTenant:      "pub",
		TriggerStore:      store,
		Executor:          spy,
		BackgroundContext: bgCtx,
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc, passthroughAuth("t1"))

	rec := httptest.NewRecorder()
	body := `{"model":"test-model","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	select {
	case <-startedCh:
	case <-time.After(time.Second):
		t.Fatal("executor Start was not called")
	}
	if spy.ctxValue != "service-context" {
		t.Fatalf("executor context value = %v, want service-context", spy.ctxValue)
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
	body := `{"model":"test-model","execution_mode":"wat","scenarios":["s1"]}`
	req := httptest.NewRequest("POST", "/v1/bench/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
