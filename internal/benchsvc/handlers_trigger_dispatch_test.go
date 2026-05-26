package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
	body := `{"model":"test-model","scenarios":["s1","s2"]}`
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
		return
	}
	if stored.ExecutionMode != "provider" {
		t.Fatalf("stored execution mode = %q, want provider", stored.ExecutionMode)
	}
	if spy.job == nil {
		t.Fatal("executor job missing")
		return
	}
	if spy.job.ExecutionMode != "provider" {
		t.Fatalf("job execution mode = %q, want provider", spy.job.ExecutionMode)
	}
}

func TestHandleTrigger_ValidRequest_Returns202_WithDefaultExecutionMode(t *testing.T) {
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
	body := `{"model":"sonnet","scenarios":["s1","s2"]}`
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

func TestHandleTrigger_StoresToolServerIdentity(t *testing.T) {
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
	body := `{"model":"sonnet","tool_server":"kubernetes-mcp","scenarios":["s1"]}`
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
	if stored.ToolServer != "kubernetes-mcp" {
		t.Fatalf("stored tool server = %q, want kubernetes-mcp", stored.ToolServer)
	}
	if spy.job == nil || spy.job.ToolServer != "kubernetes-mcp" {
		t.Fatalf("executor job tool server = %v, want kubernetes-mcp", spy.job)
	}
}

func TestHandleTrigger_StoresSkillIdentity(t *testing.T) {
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
	body := `{"model":"sonnet","skill_id":"k8s-admin","skill_version":"2026-05-13","skill_source":"local-temp","skill_sha256":"abc123","skill_file":"/tmp/skill.md","scenarios":["s1"]}`
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
	if stored.SkillID != "k8s-admin" {
		t.Fatalf("stored skill id = %q, want k8s-admin", stored.SkillID)
	}
	if stored.SkillVersion != "2026-05-13" {
		t.Fatalf("stored skill version = %q, want 2026-05-13", stored.SkillVersion)
	}
	if spy.job == nil || spy.job.SkillID != "k8s-admin" {
		t.Fatalf("executor job skill id = %v, want k8s-admin", spy.job)
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
	body := `{"model":"sonnet","execution_mode":"a2a","scenarios":["s1","s2"]}`
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
		return
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
		return
	}
	if spy.job.ExecutionMode != "a2a" {
		t.Fatalf("job execution mode = %q, want a2a", spy.job.ExecutionMode)
	}
}
