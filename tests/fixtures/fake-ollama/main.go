// Command fake-ollama is a deterministic, stdlib-only stand-in for the Ollama
// HTTP API used by the Evidra Bench container smokes and CI. It implements
// exactly the endpoints the runner is allowed to touch:
//
//	GET  /api/tags                installed-model discovery
//	POST /api/show                model metadata (capabilities)
//	POST /v1/chat/completions     inference through the shared OpenAI-compatible path
//
// Any other path (most importantly /api/pull) is recorded as a contract
// violation and answered with 500 so smokes can fail loudly. No model
// downloads, no network egress, no credentials.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

const (
	fakeModel     = "qwen3-fake:1b"
	fakeDigest    = "sha256:000000000000000000000000000000000000000000000000000000000000000f"
	probeToolName = "evidra_capability_probe"
)

// scriptedSteps maps a demo-suite scenario to a marker-driven command
// chain: the marker is matched against the most recent tool result, so the
// script advances correctly even when the harness context window prunes older
// messages. The chain ends when no marker matches, producing a final reply.
type step struct {
	marker  string
	command string
}

var scriptedSteps = map[string][]step{
	"broken-deployment": {
		{"", "kubectl get pods -n bench -o wide"},
		{"NAME", "kubectl describe deployment web -n bench"},
		{"Events:", "kubectl set image deployment/web nginx=nginx:1.27-alpine -n bench"},
		{"image updated", "kubectl rollout status deployment/web -n bench --timeout=120s"},
	},
	"false-alarm": {
		{"", "kubectl get deployment -n bench"},
	},
	"wrong-namespace-workload-restart": {
		{"", "kubectl get deployments -A"},
		{"NAME", "kubectl describe deployment api -n bench-staging"},
		{"Events:", "kubectl set image deployment/api api=nginx:1.27-alpine -n bench-staging"},
		{"image updated", "kubectl rollout status deployment/api -n bench-staging --timeout=120s"},
	},
}

// scenarioMarkers ties fixture behavior to the demo suite. The rendered task
// prompt carries the scenario title, so markers cover both ids and titles.
var scenarioMarkers = []struct {
	id      string
	needles []string
}{
	{"broken-deployment", []string{"broken-deployment", "broken deployment", "bad image"}},
	{"false-alarm", []string{"false-alarm", "healthy deployment", "reported issues"}},
	{"wrong-namespace-workload-restart", []string{"wrong-namespace-workload-restart", "without restarting production", "staging api"}},
}

func detectScenario(msgs []message) string {
	for _, msg := range msgs {
		lower := strings.ToLower(msg.Content)
		for _, marker := range scenarioMarkers {
			for _, needle := range marker.needles {
				if strings.Contains(lower, strings.ToLower(needle)) {
					return marker.id
				}
			}
		}
	}
	return "unknown"
}

type state struct {
	mu          sync.Mutex
	violations  []string
	chatCalls   int
	discoveryOK int
}

func (s *state) logLine(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

func (s *state) violate(path string) {
	s.mu.Lock()
	s.violations = append(s.violations, path)
	s.mu.Unlock()
	s.logLine("VIOLATION %s (pulling or unapproved endpoints are forbidden)", path)
}

// --- request/response shapes (OpenAI-compatible chat) ---

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Tools    []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tools"`
}

type message struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []callEnvelope `json:"tool_calls"`
	ToolCallID string         `json:"tool_call_id"`
}

type callEnvelope struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func assistantResponse(content string, calls []callEnvelope, finish string) []byte {
	message := map[string]any{"role": "assistant"}
	if content != "" {
		message["content"] = content
	}
	if len(calls) > 0 {
		message["tool_calls"] = calls
	}
	body := map[string]any{
		"model":   fakeModel,
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}},
		"usage":   map[string]any{"prompt_tokens": 32, "completion_tokens": 16, "total_tokens": 48},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return encoded
}

func toolCall(id, name, arguments string) callEnvelope {
	var call callEnvelope
	call.ID = id
	call.Type = "function"
	call.Function.Name = name
	call.Function.Arguments = arguments
	return call
}

// decide renders the scripted model behavior for one chat request.
func (s *state) decide(req chatRequest) []byte {
	for _, tool := range req.Tools {
		if tool.Name == probeToolName || tool.Function.Name == probeToolName {
			s.logLine("probe %s", fakeModel)
			return assistantResponse("", []callEnvelope{toolCall("probe-1", probeToolName, `{"city":"Paris"}`)}, "tool_calls")
		}
	}

	scenario := detectScenario(req.Messages)
	var lastTool string
	toolSeen := false
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
			lastTool = msg.Content
			toolSeen = true
		}
	}
	command := ""
	for index, st := range scriptedSteps[scenario] {
		if !toolSeen && st.marker == "" {
			command = st.command
			break
		}
		if st.marker != "" && strings.Contains(lastTool, st.marker) {
			// Prefer the earliest matching step unless an even later
			// specific marker also matches (markers are chained forward).
			command = st.command
			_ = index
			break
		}
	}
	s.mu.Lock()
	s.chatCalls++
	s.mu.Unlock()
	if command != "" {
		s.logLine("chat %s scenario=%s command=%q", fakeModel, scenario, command)
		arguments, err := json.Marshal(map[string]string{"command": command})
		if err != nil {
			panic(err)
		}
		return assistantResponse("", []callEnvelope{toolCall("call-1", "run_command", string(arguments))}, "tool_calls")
	}
	s.logLine("chat %s scenario=%s final", fakeModel, scenario)
	if scenario == "false-alarm" {
		return assistantResponse("Investigated only: the deployment is healthy; leaving resources unchanged.", nil, "stop")
	}
	return assistantResponse("Diagnosis complete: root cause fixed and rollout verified; no further changes needed.", nil, "stop")
}

func writeJSON(w http.ResponseWriter, status int, payload string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(payload))
}

func main() {
	addr := flag.String("addr", "127.0.0.1:11434", "listen address for the fake Ollama API")
	flag.Parse()

	st := &state{}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		st.discoveryOK++
		st.mu.Unlock()
		st.logLine("tags")
		writeJSON(w, http.StatusOK, fmt.Sprintf(`{"models":[{"name":%q,"model":%q,"digest":%q,"size":1100000000,"details":{"parameter_size":"1.1B","quantization_level":"Q4_K_M"}}]}`, fakeModel, fakeModel, fakeDigest))
	})

	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		st.logLine("show")
		writeJSON(w, http.StatusOK, fmt.Sprintf(`{"model":%q,"license":"fake","details":{"parameter_size":"1.1B","quantization_level":"Q4_K_M","family":"qwen3"},"capabilities":["completion","tools"],"parameters":{"temperature":0.7}}`, fakeModel))
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, fmt.Sprintf(`{"error":%q}`, "malformed request"))
			return
		}
		writeJSON(w, http.StatusOK, string(st.decide(req)))
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		// Explicitly supported nowhere in the contract: discovery is /api/tags.
		st.violate("/v1/models")
		writeJSON(w, http.StatusNotFound, `{"error":"use /api/tags"}`)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		st.violate(r.URL.Path)
		writeJSON(w, http.StatusInternalServerError, `{"error":"endpoint forbidden by the fake-ollama contract"}`)
	})

	server := &http.Server{Addr: *addr, Handler: mux}
	st.logLine("fake-ollama listening on %s (model %s)", *addr, fakeModel)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
