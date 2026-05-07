package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
