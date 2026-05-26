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
		return
	}
	if got.Status != "completed" {
		t.Fatalf("trigger status = %q, want completed", got.Status)
	}
	if got.Completed != 2 || got.Passed != 2 || got.Failed != 0 {
		t.Fatalf("trigger counters = completed:%d passed:%d failed:%d, want 2/2/0", got.Completed, got.Passed, got.Failed)
	}
}

func TestHandlePollJob_ReturnsToolServerConfig(t *testing.T) {
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
				"execution_mode":"a2a",
				"mcp_server":"npx -y @vendor/kubernetes-mcp --stdio",
				"tool_server":"kubernetes-mcp",
				"tool_server_version":"1.2.3"
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
	if resp["execution_mode"] != "a2a" {
		t.Fatalf("execution_mode = %v, want a2a", resp["execution_mode"])
	}
	if resp["mcp_server"] != "npx -y @vendor/kubernetes-mcp --stdio" {
		t.Fatalf("mcp_server = %v, want command", resp["mcp_server"])
	}
	if resp["tool_server"] != "kubernetes-mcp" {
		t.Fatalf("tool_server = %v, want kubernetes-mcp", resp["tool_server"])
	}
	if resp["tool_server_version"] != "1.2.3" {
		t.Fatalf("tool_server_version = %v, want 1.2.3", resp["tool_server_version"])
	}
}

func TestHandlePollJob_ReturnsSkillConfig(t *testing.T) {
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
			ID:       "job-q-skill",
			TenantID: "pub",
			Model:    "sonnet",
			Provider: "bifrost",
			Status:   "queued",
			ConfigJSON: json.RawMessage(`{
				"scenarios":["s1"],
				"runner_id":"runner-1",
				"skill_file":"/tmp/skill.md",
				"skill_id":"k8s-admin",
				"skill_version":"2026-05-13",
				"skill_source":"local-temp",
				"skill_sha256":"abc123"
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
	if resp["skill_file"] != "/tmp/skill.md" {
		t.Fatalf("skill_file = %v, want /tmp/skill.md", resp["skill_file"])
	}
	if resp["skill_id"] != "k8s-admin" {
		t.Fatalf("skill_id = %v, want k8s-admin", resp["skill_id"])
	}
	if resp["skill_version"] != "2026-05-13" {
		t.Fatalf("skill_version = %v, want 2026-05-13", resp["skill_version"])
	}
	if resp["skill_source"] != "local-temp" {
		t.Fatalf("skill_source = %v, want local-temp", resp["skill_source"])
	}
	if resp["skill_sha256"] != "abc123" {
		t.Fatalf("skill_sha256 = %v, want abc123", resp["skill_sha256"])
	}
}

func TestHandlePollJob_DefaultsExecutionModeForMinimalJobs(t *testing.T) {
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
			configJSON: `{"scenarios":["s1"],"execution_mode":`,
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
