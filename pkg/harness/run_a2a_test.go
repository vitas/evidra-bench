package harness

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func TestExecuteSingleAgent_A2APrecedesProvider(t *testing.T) {
	t.Parallel()

	server := newA2ATestServer(t, func(method string, _ map[string]any) map[string]any {
		if method != "message/send" {
			t.Fatalf("method = %q, want message/send", method)
		}
		return map[string]any{
			"id": "task-1",
			"status": map[string]any{
				"state": "completed",
			},
			"artifacts": []map[string]any{
				{"parts": []map[string]any{{"text": "done"}}},
			},
		}
	})
	defer server.Close()

	h := New(Deps{})
	cfg := config.Default()
	cfg.Adapter = "a2a"
	cfg.Provider = "not-a-real-provider"
	cfg.A2AAgentURL = server.URL

	got, err := h.executeSingleAgent(context.Background(), RunRequest{Config: cfg}, &scenario.Scenario{ID: "broken-deployment"}, "/tmp/kubeconfig", "fix it", time.Second, "")
	if err != nil {
		t.Fatalf("executeSingleAgent() error = %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	if got.Stdout != "done" {
		t.Fatalf("Stdout = %q, want done", got.Stdout)
	}
}

func TestRunWithA2A_NormalizesCompletedTask(t *testing.T) {
	t.Parallel()

	server := newA2ATestServer(t, func(method string, _ map[string]any) map[string]any {
		if method != "message/send" {
			t.Fatalf("method = %q, want message/send", method)
		}
		return map[string]any{
			"id":        "task-1",
			"contextId": "ctx-1",
			"status": map[string]any{
				"state": "completed",
			},
			"artifacts": []map[string]any{
				{"parts": []map[string]any{{"text": "done"}}},
			},
		}
	})
	defer server.Close()

	h := New(Deps{})
	cfg := config.Default()
	cfg.Adapter = "a2a"
	cfg.A2AAgentURL = server.URL

	got, err := h.runWithA2A(context.Background(), RunRequest{Config: cfg}, &scenario.Scenario{ID: "broken-deployment"}, "fix it", time.Second)
	if err != nil {
		t.Fatalf("runWithA2A() error = %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	if got.Stdout != "done" {
		t.Fatalf("Stdout = %q, want done", got.Stdout)
	}
	if got.Transcript != "done" {
		t.Fatalf("Transcript = %q, want done", got.Transcript)
	}
	if got.Metadata["adapter"] != "a2a" {
		t.Fatalf("adapter metadata = %q, want a2a", got.Metadata["adapter"])
	}
	if got.Metadata["a2a_task_id"] != "task-1" {
		t.Fatalf("a2a_task_id = %q, want task-1", got.Metadata["a2a_task_id"])
	}
	if got.Metadata["a2a_context_id"] != "ctx-1" {
		t.Fatalf("a2a_context_id = %q, want ctx-1", got.Metadata["a2a_context_id"])
	}
}

func TestRunWithA2A_NormalizesFailedTask(t *testing.T) {
	t.Parallel()

	server := newA2ATestServer(t, func(method string, _ map[string]any) map[string]any {
		if method != "message/send" {
			t.Fatalf("method = %q, want message/send", method)
		}
		return map[string]any{
			"id": "task-1",
			"status": map[string]any{
				"state": "failed",
			},
		}
	})
	defer server.Close()

	h := New(Deps{})
	cfg := config.Default()
	cfg.Adapter = "a2a"
	cfg.A2AAgentURL = server.URL

	got, err := h.runWithA2A(context.Background(), RunRequest{Config: cfg}, &scenario.Scenario{ID: "broken-deployment"}, "fix it", time.Second)
	if err != nil {
		t.Fatalf("runWithA2A() error = %v", err)
	}
	if got.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", got.ExitCode)
	}
	if got.Metadata["a2a_state"] != "failed" {
		t.Fatalf("a2a_state = %q, want failed", got.Metadata["a2a_state"])
	}
}

func TestRunWithA2A_WrapsTransportErrorsAsInfraErrors(t *testing.T) {
	t.Parallel()

	h := New(Deps{})
	cfg := config.Default()
	cfg.Adapter = "a2a"
	cfg.A2AAgentURL = "http://127.0.0.1:1"

	_, err := h.runWithA2A(context.Background(), RunRequest{Config: cfg}, &scenario.Scenario{ID: "broken-deployment"}, "fix it", time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	var infraErr *InfraError
	if !errors.As(err, &infraErr) {
		t.Fatalf("expected InfraError, got %T", err)
	}
}

func TestShouldUseProviderEvidenceDir_SkipsA2A(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Adapter = "a2a"
	cfg.Provider = "bifrost"
	if shouldUseProviderEvidenceDir(cfg) {
		t.Fatal("shouldUseProviderEvidenceDir() = true, want false for a2a")
	}
}

func newA2ATestServer(t *testing.T, result func(method string, params map[string]any) map[string]any) *httptest.Server {
	t.Helper()

	type rpcRequest struct {
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/agent-card.json":
			writeJSONResponse(t, w, map[string]any{
				"name": "demo-agent",
				"url":  server.URL,
			})
		case "":
			t.Fatalf("unexpected empty path")
		default:
			var req rpcRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			writeJSONResponse(t, w, map[string]any{
				"jsonrpc": "2.0",
				"id":      "req-1",
				"result":  result(req.Method, req.Params),
			})
		}
	}))

	return server
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
