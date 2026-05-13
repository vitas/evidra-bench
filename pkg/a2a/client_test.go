package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient_DefaultHTTPClientHasNoTimeout(t *testing.T) {
	t.Parallel()

	client := NewClient("https://agent.example", nil)
	if client.HTTPClient == nil {
		t.Fatal("HTTPClient = nil")
	}
	if client.HTTPClient.Timeout != 0 {
		t.Fatalf("HTTPClient.Timeout = %v, want 0 to rely on request context", client.HTTPClient.Timeout)
	}
}

func TestClient_RunTextTask_UsesAgentCardURL(t *testing.T) {
	t.Parallel()

	type rpcRequest struct {
		Method string `json:"method"`
	}

	var rpcHits int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/agent-card.json":
			writeJSON(t, w, map[string]any{
				"name": "demo-agent",
				"url":  server.URL + "/rpc",
			})
		case "/rpc":
			rpcHits++
			var req rpcRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if req.Method != "message/send" {
				t.Fatalf("method = %q, want message/send", req.Method)
			}
			writeJSON(t, w, map[string]any{
				"jsonrpc": "2.0",
				"id":      "req-1",
				"result": map[string]any{
					"id": "task-1",
					"status": map[string]any{
						"state": "completed",
					},
					"artifacts": []map[string]any{
						{"parts": []map[string]any{{"text": "done"}}},
					},
				},
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	got, err := client.RunTextTask(context.Background(), "req-1", "hello")
	if err != nil {
		t.Fatalf("RunTextTask() error = %v", err)
	}
	if rpcHits != 1 {
		t.Fatalf("rpcHits = %d, want 1", rpcHits)
	}
	if got.RPCURL != server.URL+"/rpc" {
		t.Fatalf("RPCURL = %q, want %q", got.RPCURL, server.URL+"/rpc")
	}
}

func TestClient_RunTextTask_CompletesImmediately(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/agent-card.json":
			writeJSON(t, w, map[string]any{
				"name": "demo-agent",
				"url":  server.URL + "/rpc",
			})
		case "/rpc":
			writeJSON(t, w, map[string]any{
				"jsonrpc": "2.0",
				"id":      "req-1",
				"result": map[string]any{
					"id":        "task-1",
					"contextId": "ctx-1",
					"status": map[string]any{
						"state": "TASK_STATE_COMPLETED",
					},
					"artifacts": []map[string]any{
						{"parts": []map[string]any{{"text": "first line"}, {"text": "second line"}}},
					},
				},
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	got, err := client.RunTextTask(context.Background(), "req-1", "hello")
	if err != nil {
		t.Fatalf("RunTextTask() error = %v", err)
	}
	if !got.Completed {
		t.Fatal("Completed = false, want true")
	}
	if got.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want task-1", got.TaskID)
	}
	if got.ContextID != "ctx-1" {
		t.Fatalf("ContextID = %q, want ctx-1", got.ContextID)
	}
	if got.Output != "first line\nsecond line" {
		t.Fatalf("Output = %q", got.Output)
	}
}

func TestClient_RunTextTask_PollsNonTerminalTask(t *testing.T) {
	t.Parallel()

	type rpcRequest struct {
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}

	var calls []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/agent-card.json":
			writeJSON(t, w, map[string]any{
				"name": "demo-agent",
				"url":  server.URL + "/rpc",
			})
		case "/rpc":
			var req rpcRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			calls = append(calls, req.Method)
			switch req.Method {
			case "message/send":
				writeJSON(t, w, map[string]any{
					"jsonrpc": "2.0",
					"id":      "req-1",
					"result": map[string]any{
						"id": "task-1",
						"status": map[string]any{
							"state": "TASK_STATE_WORKING",
						},
					},
				})
			case "tasks/get":
				if req.Params["id"] != "task-1" {
					t.Fatalf("tasks/get id = %#v, want task-1", req.Params["id"])
				}
				writeJSON(t, w, map[string]any{
					"jsonrpc": "2.0",
					"id":      "req-1",
					"result": map[string]any{
						"id": "task-1",
						"status": map[string]any{
							"state": "completed",
						},
						"artifacts": []map[string]any{
							{"parts": []map[string]any{{"text": "done"}}},
						},
					},
				})
			default:
				t.Fatalf("unexpected method %q", req.Method)
			}
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	client.PollInterval = time.Millisecond

	got, err := client.RunTextTask(context.Background(), "req-1", "hello")
	if err != nil {
		t.Fatalf("RunTextTask() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v, want message/send then tasks/get", calls)
	}
	if calls[0] != "message/send" || calls[1] != "tasks/get" {
		t.Fatalf("calls = %v, want [message/send tasks/get]", calls)
	}
	if !got.Completed {
		t.Fatal("Completed = false, want true")
	}
	if got.Output != "done" {
		t.Fatalf("Output = %q, want done", got.Output)
	}
}

func TestClient_RunTextTask_PropagatesRPCError(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/agent-card.json":
			writeJSON(t, w, map[string]any{
				"name": "demo-agent",
				"url":  server.URL + "/rpc",
			})
		case "/rpc":
			writeJSON(t, w, map[string]any{
				"jsonrpc": "2.0",
				"id":      "req-1",
				"error": map[string]any{
					"code":    -32001,
					"message": "task rejected",
				},
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, err := client.RunTextTask(context.Background(), "req-1", "hello")
	if err == nil || err.Error() != "a2a: rpc error -32001: task rejected" {
		t.Fatalf("RunTextTask() error = %v, want rpc error", err)
	}
}

func TestClient_ParseSendResult_AcceptsDirectMessage(t *testing.T) {
	t.Parallel()

	client := NewClient("https://agent.example", nil)
	got, err := client.parseSendResult("demo-agent", "https://agent.example/rpc", json.RawMessage(`{
		"messageId":"msg-1",
		"role":"agent",
		"parts":[{"kind":"text","text":"done"}]
	}`))
	if err != nil {
		t.Fatalf("parseSendResult() error = %v", err)
	}
	if !got.Completed {
		t.Fatal("Completed = false, want true")
	}
	if got.Output != "done" {
		t.Fatalf("Output = %q, want done", got.Output)
	}
}

func TestClient_ParseSendResult_RejectsTaskWithoutState(t *testing.T) {
	t.Parallel()

	client := NewClient("https://agent.example", nil)
	_, err := client.parseSendResult("demo-agent", "https://agent.example/rpc", json.RawMessage(`{"id":"task-1"}`))
	if err == nil {
		t.Fatal("parseSendResult() error = nil, want validation error")
	}
	if got := err.Error(); !containsAll(got, "unrecognized send result", `{"id":"task-1"}`) {
		t.Fatalf("parseSendResult() error = %q, want raw payload context", got)
	}
}

func TestEndpointForCard_PrefersSupportedInterfacesJSONRPC(t *testing.T) {
	t.Parallel()

	card := &AgentCard{
		URL: "https://agent.example/card-url",
		SupportedInterfaces: []AgentInterface{
			{URL: "https://agent.example/http", ProtocolBinding: "HTTP"},
			{URL: "https://agent.example/rpc", ProtocolBinding: "JSON-RPC", ProtocolVersion: "0.5"},
		},
	}

	rpcURL, version, err := endpointForCard("https://agent.example/base", card)
	if err != nil {
		t.Fatalf("endpointForCard() error = %v", err)
	}
	if rpcURL != "https://agent.example/rpc" {
		t.Fatalf("rpcURL = %q, want supported interface URL", rpcURL)
	}
	if version != "0.5" {
		t.Fatalf("version = %q, want 0.5", version)
	}
}

func TestEndpointForCard_FallsBackToBaseURL(t *testing.T) {
	t.Parallel()

	card := &AgentCard{}
	rpcURL, version, err := endpointForCard("https://agent.example/base", card)
	if err != nil {
		t.Fatalf("endpointForCard() error = %v", err)
	}
	if rpcURL != "https://agent.example/base" {
		t.Fatalf("rpcURL = %q, want base URL", rpcURL)
	}
	if version != defaultProtocolVersion {
		t.Fatalf("version = %q, want default protocol version", version)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
