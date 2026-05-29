package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/orchestrator"
)

func TestBenchReporter_OnScenarioOnlySendsProgress(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	progressRequests := 0
	benchRequests := 0
	progressAuth := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/progress":
			progressRequests++
			progressAuth = r.Header.Get("Authorization")
		case "/v1/bench/runs":
			benchRequests++
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)

	reporter := &benchReporter{
		progressURL: server.URL + "/progress",
		authToken:   "raw-key",
	}
	ev := orchestratorScenarioEventForTest()
	ev.Status = "passed"
	reporter.OnScenario(context.Background(), ev)

	mu.Lock()
	defer mu.Unlock()
	if progressRequests != 1 {
		t.Fatalf("progress requests = %d, want 1", progressRequests)
	}
	if benchRequests != 0 {
		t.Fatalf("bench run requests = %d, want 0", benchRequests)
	}
	if progressAuth != "Bearer raw-key" {
		t.Fatalf("progress Authorization = %q, want Bearer raw-key", progressAuth)
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
