package benchsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTriggerStore_CreateAndGet(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()

	job := &TriggerJob{
		ID:        "job-001",
		Status:    "pending",
		Model:     "sonnet-4",
		Provider:  "anthropic",
		Total:     2,
		CreatedAt: time.Now(),
		Progress: []ScenarioProgress{
			{Scenario: "cka-01", Status: "pending"},
			{Scenario: "cka-02", Status: "pending"},
		},
	}
	store.Create(job)

	got := store.Get("job-001")
	if got == nil { //nolint:staticcheck // t.Fatal stops execution
		t.Fatal("expected job, got nil")
		return
	}
	if got.Model != "sonnet-4" {
		t.Errorf("model = %q, want %q", got.Model, "sonnet-4")
	}
	if got.Total != 2 {
		t.Errorf("total = %d, want 2", got.Total)
	}
	if len(got.Progress) != 2 {
		t.Errorf("progress len = %d, want 2", len(got.Progress))
	}

	// Get returns nil for unknown IDs.
	if store.Get("unknown") != nil {
		t.Error("expected nil for unknown job")
	}
}

func TestNewRemoteExecutorUsesDedicatedTransport(t *testing.T) {
	t.Parallel()

	exec := NewRemoteExecutor("https://bench.example")

	if exec.HTTPClient == nil {
		t.Fatal("HTTPClient is nil")
	}
	if exec.HTTPClient.Transport == nil {
		t.Fatal("HTTPClient.Transport is nil; expected dedicated transport")
	}
	if exec.HTTPClient.Transport == http.DefaultTransport {
		t.Fatal("HTTPClient.Transport uses http.DefaultTransport")
	}
}

func TestTriggerStore_UpdateNotifiesSubscriber(t *testing.T) {
	t.Parallel()

	store := NewTriggerStore()

	job := &TriggerJob{
		ID:     "job-002",
		Status: "pending",
		Model:  "opus-4",
		Total:  1,
		Progress: []ScenarioProgress{
			{Scenario: "cka-05", Status: "pending"},
		},
		CreatedAt: time.Now(),
	}
	store.Create(job)

	ch := store.Subscribe("job-002")

	update := ProgressUpdate{
		JobID:     "job-002",
		Scenario:  "cka-05",
		Status:    "passed",
		RunID:     "run-abc",
		Completed: 1,
		Total:     1,
	}
	ok := store.Update(update)
	if !ok {
		t.Fatal("Update returned false, expected true")
	}

	// Subscriber should receive the update.
	select {
	case got := <-ch:
		if got.Scenario != "cka-05" {
			t.Errorf("scenario = %q, want %q", got.Scenario, "cka-05")
		}
		if got.RunID != "run-abc" {
			t.Errorf("run_id = %q, want %q", got.RunID, "run-abc")
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive update within 1s")
	}

	// Job should be completed.
	got := store.Get("job-002")
	if got.Status != "completed" {
		t.Errorf("status = %q, want %q", got.Status, "completed")
	}
	if got.Passed != 1 {
		t.Errorf("passed = %d, want 1", got.Passed)
	}

	store.Unsubscribe("job-002", ch)

	// Update for unknown job returns false.
	if store.Update(ProgressUpdate{JobID: "unknown"}) {
		t.Error("expected false for unknown job update")
	}
}

func TestRemoteExecutor_StartSendsToolServerConfig(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/certify" {
			t.Fatalf("path = %s, want /v1/certify", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	exec := NewRemoteExecutor(srv.URL)
	job := &TriggerJob{
		ID:                "job-123",
		Status:            "pending",
		Model:             "sonnet",
		Provider:          "bifrost",
		MCPServer:         "npx -y @vendor/kubernetes-mcp --stdio",
		ToolServer:        "kubernetes-mcp",
		ToolServerVersion: "1.2.3",
		Total:             1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
		},
		CreatedAt: time.Now(),
	}

	if err := exec.Start(t.Context(), job, "https://bench.example", "Bearer token"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	cfg, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("config missing or wrong type: %#v", payload["config"])
	}
	legacyModeKey := "evidence" + "_mode"
	if _, ok := cfg[legacyModeKey]; ok {
		t.Fatalf("unexpected legacy mode in config: %#v", cfg)
	}
	if got := cfg["mcp_server"]; got != "npx -y @vendor/kubernetes-mcp --stdio" {
		t.Fatalf("mcp_server = %v, want command", got)
	}
	if got := cfg["tool_server"]; got != "kubernetes-mcp" {
		t.Fatalf("tool_server = %v, want kubernetes-mcp", got)
	}
	if got := cfg["tool_server_version"]; got != "1.2.3" {
		t.Fatalf("tool_server_version = %v, want 1.2.3", got)
	}
	callback, ok := payload["callback"].(map[string]any)
	if !ok {
		t.Fatalf("callback missing or wrong type: %#v", payload["callback"])
	}
	if got := callback["bench_url"]; got != "https://bench.example" {
		t.Fatalf("bench_url = %v, want https://bench.example", got)
	}
	if _, ok := callback["evidra_url"]; ok {
		t.Fatalf("evidra_url should not be present: %#v", callback)
	}
}

func TestRemoteExecutor_StartSendsA2AAdapterForA2AExecutionMode(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	exec := NewRemoteExecutor(srv.URL)
	job := &TriggerJob{
		ID:            "job-123",
		Status:        "pending",
		Model:         "sonnet",
		Provider:      "bifrost",
		ExecutionMode: "a2a",
		Total:         1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
		},
		CreatedAt: time.Now(),
	}

	if err := exec.Start(t.Context(), job, "https://bench.example", "Bearer token"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	cfg, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("config missing or wrong type: %#v", payload["config"])
	}
	if got := cfg["adapter"]; got != "a2a" {
		t.Fatalf("adapter = %v, want a2a", got)
	}
}

func TestRemoteExecutor_StartOmitsAdapterForProviderExecutionMode(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	exec := NewRemoteExecutor(srv.URL)
	job := &TriggerJob{
		ID:            "job-123",
		Status:        "pending",
		Model:         "sonnet",
		Provider:      "bifrost",
		ExecutionMode: "provider",
		Total:         1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
		},
		CreatedAt: time.Now(),
	}

	if err := exec.Start(t.Context(), job, "https://bench.example", "Bearer token"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	cfg, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("config missing or wrong type: %#v", payload["config"])
	}
	if _, ok := cfg["adapter"]; ok {
		t.Fatalf("adapter override should be absent, got %v", cfg["adapter"])
	}
}
