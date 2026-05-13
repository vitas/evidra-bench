package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/orchestrator"
)

func TestBenchReporter_SubmitBenchRunOmitsLegacyMode(t *testing.T) {
	t.Parallel()

	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/bench/runs" {
			t.Fatalf("path = %q, want /v1/bench/runs", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	reporter := &benchReporter{
		benchURL: server.URL,
	}
	reporter.submitBenchRun(orchestratorScenarioEventForTest())

	legacyModeKey := "evidence" + "_mode"
	if _, ok := got[legacyModeKey]; ok {
		t.Fatalf("unexpected legacy mode in payload: %v", got[legacyModeKey])
	}
}

func TestBenchReporter_SubmitBenchRunUsesA2AAdapter(t *testing.T) {
	t.Parallel()

	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/bench/runs" {
			t.Fatalf("path = %q, want /v1/bench/runs", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	reporter := &benchReporter{
		benchURL: server.URL,
		adapter:  "a2a",
	}
	reporter.submitBenchRun(orchestratorScenarioEventForTest())

	if got["adapter"] != "a2a" {
		t.Fatalf("adapter = %v, want a2a", got["adapter"])
	}
}

func TestBenchReporter_SubmitBenchRunIncludesToolServerIdentity(t *testing.T) {
	t.Parallel()

	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	reporter := &benchReporter{
		benchURL:          server.URL,
		toolServer:        "kubernetes-mcp",
		toolServerVersion: "1.2.3",
	}
	reporter.submitBenchRun(orchestratorScenarioEventForTest())

	if got["tool_server"] != "kubernetes-mcp" {
		t.Fatalf("tool_server = %v, want kubernetes-mcp", got["tool_server"])
	}
	if got["tool_server_version"] != "1.2.3" {
		t.Fatalf("tool_server_version = %v, want 1.2.3", got["tool_server_version"])
	}
}

func orchestratorScenarioEventForTest() orchestrator.ScenarioEvent {
	return orchestrator.ScenarioEvent{
		JobID:      "job-1",
		ScenarioID: "scenario-1",
		Model:      "sonnet",
		Provider:   "claude",
		RunID:      "run-1",
		Duration:   5 * time.Second,
		Passed:     true,
	}
}
