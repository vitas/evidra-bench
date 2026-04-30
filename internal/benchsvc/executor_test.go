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
		ID:           "job-001",
		Status:       "pending",
		Model:        "sonnet-4",
		Provider:     "anthropic",
		EvidenceMode: "smart",
		Total:        2,
		CreatedAt:    time.Now(),
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
	if got.EvidenceMode != "smart" {
		t.Errorf("evidence_mode = %q, want smart", got.EvidenceMode)
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

func TestRemoteExecutor_StartSendsEvidenceMode(t *testing.T) {
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
		ID:           "job-123",
		Status:       "pending",
		Model:        "sonnet",
		Provider:     "bifrost",
		EvidenceMode: "smart",
		Total:        1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
		},
		CreatedAt: time.Now(),
	}

	if err := exec.Start(t.Context(), job, "https://evidra.example", "Bearer token"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	cfg, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("config missing or wrong type: %#v", payload["config"])
	}
	if got := cfg["evidence_mode"]; got != "smart" {
		t.Fatalf("evidence_mode = %v, want smart", got)
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
		EvidenceMode:  "smart",
		ExecutionMode: "a2a",
		Total:         1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
		},
		CreatedAt: time.Now(),
	}

	if err := exec.Start(t.Context(), job, "https://evidra.example", "Bearer token"); err != nil {
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
		EvidenceMode:  "smart",
		ExecutionMode: "provider",
		Total:         1,
		Progress: []ScenarioProgress{
			{Scenario: "s1", Status: "pending"},
		},
		CreatedAt: time.Now(),
	}

	if err := exec.Start(t.Context(), job, "https://evidra.example", "Bearer token"); err != nil {
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
