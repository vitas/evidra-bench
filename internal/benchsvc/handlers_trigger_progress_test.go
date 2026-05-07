package benchsvc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

// ---------- Runner Registration ----------
