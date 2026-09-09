package modelconfig

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/agent"
)

type scriptedProbeProvider struct {
	name     string
	response *agent.ChatResponse
	err      error
	calls    int
}

func (p *scriptedProbeProvider) Name() string { return p.name }

func (p *scriptedProbeProvider) Chat(_ context.Context, _ agent.ChatRequest) (*agent.ChatResponse, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return p.response, nil
}

func toolCallArguments(name, arguments string) *agent.ChatResponse {
	return &agent.ChatResponse{
		ToolCalls: []agent.ToolCall{{ID: "call_1", Name: name, Arguments: arguments}},
		Usage:     agent.Usage{PromptTokens: 7, CompletionTokens: 3},
	}
}

func TestProbeToolCallingSuccessReportsMethodAndUsage(t *testing.T) {
	provider := &scriptedProbeProvider{response: toolCallArguments(probeToolName, `{"city":"Paris"}`)}
	result, err := ProbeToolCalling(context.Background(), provider, "qwen3:8b")
	if err != nil {
		t.Fatalf("ProbeToolCalling() error = %v", err)
	}
	if result.Method != CapabilityCheckBehavioralProbe {
		t.Fatalf("method = %q, want %q", result.Method, CapabilityCheckBehavioralProbe)
	}
	if result.Usage.PromptTokens != 7 || provider.calls != 1 {
		t.Fatalf("usage = %+v, calls = %d", result.Usage, provider.calls)
	}
}

func TestProbeToolCallingFailureModes(t *testing.T) {
	tests := []struct {
		name     string
		provider *scriptedProbeProvider
		want     string
	}{
		{
			name:     "no tool call",
			provider: &scriptedProbeProvider{response: &agent.ChatResponse{Content: "I cannot help"}},
			want:     "did not call",
		},
		{
			name:     "wrong tool",
			provider: &scriptedProbeProvider{response: toolCallArguments("delete_everything", `{}`)},
			want:     "wrong tool",
		},
		{
			name:     "invalid json arguments",
			provider: &scriptedProbeProvider{response: toolCallArguments(probeToolName, `{"city":`)},
			want:     "malformed",
		},
		{
			name:     "provider error",
			provider: &scriptedProbeProvider{err: errors.New("connection reset")},
			want:     "connection reset",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ProbeToolCalling(context.Background(), tt.provider, "qwen3:8b"); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ProbeToolCalling() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestProbeToolCallingRespectsContextDeadline(t *testing.T) {
	blocking := &blockingProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ProbeToolCalling(ctx, blocking, "qwen3:8b"); err == nil {
		t.Fatal("expected context error")
	}
}

type blockingProvider struct{}

func (blockingProvider) Name() string { return "blocking" }
func (blockingProvider) Chat(ctx context.Context, _ agent.ChatRequest) (*agent.ChatResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func newFakeOllamaServer(t *testing.T, showCapabilities []string, showHasCapabilitiesField bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"name":"qwen3:8b","digest":"sha256:abc","details":{"parameter_size":"8.4B","quantization_level":"Q4_K_M"}}]}`))
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"details": map[string]string{"parameter_size": "8.4B", "quantization_level": "Q4_K_M"},
		}
		if showHasCapabilitiesField {
			response["capabilities"] = showCapabilities
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestPrepareOllamaModelAcceptsDeclaredToolCapabilityWithoutInference(t *testing.T) {
	server := newFakeOllamaServer(t, []string{"completion", "tools"}, true)
	provider := &scriptedProbeProvider{}
	prepared, err := prepareOllamaModel(context.Background(),
		OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()},
		func() (agent.Provider, error) { return provider, nil },
		"qwen3:8b", []string{"tools"})
	if err != nil {
		t.Fatalf("prepareOllamaModel() error = %v", err)
	}
	if prepared.CapabilityCheck != CapabilityCheckOllamaShow {
		t.Fatalf("capability check = %q, want %q", prepared.CapabilityCheck, CapabilityCheckOllamaShow)
	}
	if prepared.Name != "qwen3:8b" || prepared.Digest != "sha256:abc" ||
		prepared.ParameterSize != "8.4B" || prepared.Quantization != "Q4_K_M" {
		t.Fatalf("prepared = %+v", prepared)
	}
	if provider.calls != 0 {
		t.Fatal("metadata match must not run inference")
	}
}

func TestPrepareOllamaModelFailsDeclaredAbsentCapabilityWithoutInference(t *testing.T) {
	server := newFakeOllamaServer(t, []string{"completion", "insert"}, true)
	provider := &scriptedProbeProvider{response: toolCallArguments(probeToolName, `{"city":"Paris"}`)}
	_, err := prepareOllamaModel(context.Background(),
		OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()},
		func() (agent.Provider, error) { return provider, nil },
		"qwen3:8b", []string{"tools"})
	if err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("prepareOllamaModel() error = %v, want capability error", err)
	}
	if provider.calls != 0 {
		t.Fatal("explicitly missing capability must not trigger inference")
	}
}

func TestPrepareOllamaModelFallsBackToBehavioralProbe(t *testing.T) {
	server := newFakeOllamaServer(t, nil, false)
	provider := &scriptedProbeProvider{response: toolCallArguments(probeToolName, `{"city":"Paris"}`)}
	prepared, err := prepareOllamaModel(context.Background(),
		OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()},
		func() (agent.Provider, error) { return provider, nil },
		"qwen3:8b", []string{"tools"})
	if err != nil {
		t.Fatalf("prepareOllamaModel() error = %v", err)
	}
	if prepared.CapabilityCheck != CapabilityCheckBehavioralProbe {
		t.Fatalf("capability check = %q, want probe", prepared.CapabilityCheck)
	}
	if provider.calls != 1 {
		t.Fatalf("probe calls = %d, want 1", provider.calls)
	}
	if prepared.Digest != "sha256:abc" {
		t.Fatalf("identity lost during probe: %+v", prepared)
	}
}

func TestPrepareOllamaModelProbeFailureIsActionable(t *testing.T) {
	server := newFakeOllamaServer(t, []string{"completion"}, true)
	serverWithAmbiguous := newFakeOllamaServer(t, nil, false)
	provider := &scriptedProbeProvider{response: &agent.ChatResponse{Content: "no tools here"}}
	_, err := prepareOllamaModel(context.Background(),
		OllamaClient{BaseURL: serverWithAmbiguous.URL + "/api", HTTPClient: serverWithAmbiguous.Client()},
		func() (agent.Provider, error) { return provider, nil },
		"qwen3:8b", []string{"tools"})
	if err == nil || !strings.Contains(err.Error(), "does not support tool calling") {
		t.Fatalf("prepareOllamaModel() error = %v, want does-not-support error", err)
	}
	_ = server
}

func TestPrepareOllamaModelMissingModelNeverProbes(t *testing.T) {
	server := newFakeOllamaServer(t, []string{"tools"}, true)
	provider := &scriptedProbeProvider{}
	_, err := prepareOllamaModel(context.Background(),
		OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()},
		func() (agent.Provider, error) { return provider, nil },
		"ghost:9b", []string{"tools"})
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("prepareOllamaModel() error = %v, want not-installed", err)
	}
	if provider.calls != 0 {
		t.Fatal("probe must not run for a missing model")
	}
}

func TestPrepareOllamaModelWithoutRequirementsSkipsValidation(t *testing.T) {
	server := newFakeOllamaServer(t, []string{"tools"}, true)
	prepared, err := prepareOllamaModel(context.Background(),
		OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()},
		func() (agent.Provider, error) { return nil, errors.New("must not be called") },
		"qwen3:8b", nil)
	if err != nil {
		t.Fatalf("prepareOllamaModel() error = %v", err)
	}
	if prepared.CapabilityCheck != "" || prepared.Digest != "sha256:abc" {
		t.Fatalf("prepared = %+v", prepared)
	}
}

func TestPrepareOllamaUsesResolvedConfigurationAndSharedProvider(t *testing.T) {
	server := newFakeOllamaServer(t, []string{"completion", "tools"}, true)
	chatSeen := 0
	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chatSeen++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"evidra_capability_probe","arguments":"{\"city\":\"Paris\"}"}}]}}]}`))
	}))
	defer chatServer.Close()

	resolved := Resolved{
		Provider:          "ollama",
		Model:             "qwen3:8b",
		Endpoint:          chatServer.URL + "/v1",
		EndpointClass:     "local",
		CredentialSource:  "none",
		DiscoveryEndpoint: server.URL + "/api",
	}
	prepared, err := PrepareOllamaModel(context.Background(), resolved, []string{"tools"})
	if err != nil {
		t.Fatalf("PrepareOllamaModel() error = %v", err)
	}
	if prepared.CapabilityCheck != CapabilityCheckOllamaShow {
		t.Fatalf("prepared = %+v", prepared)
	}
	if chatSeen != 0 {
		t.Fatal("metadata path must not touch the inference endpoint")
	}

	ambiguousServer := newFakeOllamaServer(t, nil, false)
	resolved.DiscoveryEndpoint = ambiguousServer.URL + "/api"
	prepared, err = PrepareOllamaModel(context.Background(), resolved, []string{"tools"})
	if err != nil {
		t.Fatalf("PrepareOllamaModel() with probe error = %v", err)
	}
	if prepared.CapabilityCheck != CapabilityCheckBehavioralProbe || chatSeen != 1 {
		t.Fatalf("prepared = %+v, chatSeen = %d; probe must use the shared /v1 endpoint", prepared, chatSeen)
	}
}
