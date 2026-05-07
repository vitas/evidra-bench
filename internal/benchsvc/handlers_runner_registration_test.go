package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
