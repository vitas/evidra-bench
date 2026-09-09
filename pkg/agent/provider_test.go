package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveProviderWithConfigUsesSharedOpenAICompatiblePath(t *testing.T) {
	client := &http.Client{}
	provider, err := ResolveProviderWithConfig("openai", OpenAICompatibleConfig{
		BaseURL:    "https://api.openai.test/v1",
		APIKey:     "secret",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("ResolveProviderWithConfig() error = %v", err)
	}
	openAI, ok := provider.(*OpenAICompatibleProvider)
	if !ok {
		t.Fatalf("provider type = %T", provider)
	}
	if openAI.Name() != "openai" || openAI.baseURL != "https://api.openai.test/v1" || openAI.httpClient != client {
		t.Fatalf("provider = %+v", openAI)
	}
}

func TestResolveProviderOllamaSharesOpenAICompatibleInference(t *testing.T) {
	var sawModel string
	var sawTools int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload struct {
			Model string          `json:"model"`
			Tools json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		sawModel = payload.Model
		if len(payload.Tools) > 0 && string(payload.Tools) != "null" {
			var tools []any
			if err := json.Unmarshal(payload.Tools, &tools); err == nil {
				sawTools = len(tools)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"pong"}}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	}))
	defer server.Close()

	provider, err := ResolveProviderWithConfig("ollama", OpenAICompatibleConfig{
		BaseURL:    server.URL + "/v1",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("ResolveProviderWithConfig(ollama) error = %v", err)
	}
	shared, ok := provider.(*OpenAICompatibleProvider)
	if !ok {
		t.Fatalf("Ollama bypassed shared OpenAI-compatible provider: %T", provider)
	}
	if shared.Name() != "ollama" {
		t.Fatalf("provider name = %q, want ollama", shared.Name())
	}

	response, err := provider.Chat(context.Background(), ChatRequest{
		Model:    "qwen3:8b",
		Messages: []Message{{Role: "user", Content: "ping"}},
		Tools:    []ToolDef{{Name: "probe_tool", Description: "probe", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if response.Content != "pong" {
		t.Fatalf("response = %+v", response)
	}
	if sawModel != "qwen3:8b" || sawTools != 1 {
		t.Fatalf("server saw model=%q tools=%d, want qwen3:8b and 1 tool", sawModel, sawTools)
	}
}

func TestResolveProvider_Bifrost(t *testing.T) {
	t.Parallel()
	p, err := ResolveProvider("bifrost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "bifrost" {
		t.Fatalf("expected bifrost, got %s", p.Name())
	}
}

func TestResolveProvider_Claude(t *testing.T) {
	t.Parallel()
	p, err := ResolveProvider("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "claude" {
		t.Fatalf("expected claude, got %s", p.Name())
	}
}

func TestResolveProvider_Unknown(t *testing.T) {
	t.Parallel()
	_, err := ResolveProvider("unknown")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestChatResponse_Done(t *testing.T) {
	t.Parallel()
	resp := &ChatResponse{Content: "hello"}
	if !resp.Done() {
		t.Fatal("expected Done=true with no tool calls")
	}
	resp.ToolCalls = []ToolCall{{ID: "1", Name: "test"}}
	if resp.Done() {
		t.Fatal("expected Done=false with tool calls")
	}
}

func TestBenchTools_Count(t *testing.T) {
	t.Parallel()
	tools := BenchTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
		if strings.HasPrefix(tool.Name, "evidra_") {
			t.Fatalf("default bench tools must stay MCP-server agnostic, got %q", tool.Name)
		}
	}
	for _, expected := range []string{"run_command", "write_file"} {
		if !names[expected] {
			t.Fatalf("missing tool: %s", expected)
		}
	}
}

func TestResolveClaudeModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"", "sonnet"},
		{"sonnet", "sonnet"},
		{"opus", "opus"},
		{"claude/haiku", "haiku"},
		{"custom-model", "custom-model"},
	}
	for _, tt := range tests {
		if got := resolveClaudeModel(tt.input); got != tt.expected {
			t.Errorf("resolveClaudeModel(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseOpenAIResponse_Basic(t *testing.T) {
	t.Parallel()
	body := `{
		"choices": [{"message": {"content": "hello"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5}
	}`
	resp, err := parseOpenAIResponse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello" {
		t.Fatalf("expected hello, got %s", resp.Content)
	}
	if !resp.Done() {
		t.Fatal("expected Done")
	}
	if resp.Usage.PromptTokens != 10 {
		t.Fatalf("expected 10 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
}

func TestParseOpenAIResponse_ToolCalls(t *testing.T) {
	t.Parallel()
	body := `{
		"choices": [{
			"message": {
				"content": "",
				"tool_calls": [
					{"id": "tc1", "type": "function", "function": {"name": "run_command", "arguments": "{\"command\":\"kubectl get pods\"}"}}
				]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 20, "completion_tokens": 10}
	}`
	resp, err := parseOpenAIResponse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Done() {
		t.Fatal("expected not Done with tool calls")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "run_command" {
		t.Fatalf("expected run_command, got %s", resp.ToolCalls[0].Name)
	}
}

func TestParseOpenAIResponse_DeepSeekPromptCacheUsage(t *testing.T) {
	t.Parallel()
	body := `{
		"choices": [{"message": {"content": "done"}, "finish_reason": "stop"}],
		"usage": {
			"prompt_tokens": 3000,
			"prompt_cache_hit_tokens": 2000,
			"prompt_cache_miss_tokens": 1000,
			"completion_tokens": 500
		}
	}`
	resp, err := parseOpenAIResponse([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Usage.PromptCacheHitTokens != 2000 {
		t.Fatalf("prompt cache hit tokens = %d, want 2000", resp.Usage.PromptCacheHitTokens)
	}
	if resp.Usage.PromptCacheMissTokens != 1000 {
		t.Fatalf("prompt cache miss tokens = %d, want 1000", resp.Usage.PromptCacheMissTokens)
	}
}

func TestBuildOpenAIPayload_DeepSeekV4FlashEnablesThinking(t *testing.T) {
	t.Parallel()
	payload := buildOpenAIPayload(ChatRequest{
		Model:    "deepseek-v4-flash",
		Messages: []Message{{Role: "user", Content: "fix the cluster"}},
		Tools:    BenchTools(),
	})
	thinking, ok := payload["thinking"].(map[string]string)
	if !ok {
		t.Fatalf("thinking payload = %#v, want object", payload["thinking"])
	}
	if thinking["type"] != "enabled" {
		t.Fatalf("thinking.type = %q, want enabled", thinking["type"])
	}
	if payload["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort = %v, want high", payload["reasoning_effort"])
	}
}

func TestParseClaudeStream_Text(t *testing.T) {
	t.Parallel()
	stream := `{"type":"text","text":"The deployment"}
{"type":"text","text":" is fixed."}
`
	resp, err := parseClaudeStream(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "The deployment is fixed." {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if !resp.Done() {
		t.Fatal("expected Done")
	}
}

func TestParseClaudeStream_ToolUse(t *testing.T) {
	t.Parallel()
	stream := `{"type":"tool_use","id":"tu1","name":"run_command","input":{"command":"kubectl get pods"}}
`
	resp, err := parseClaudeStream(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "run_command" {
		t.Fatalf("expected run_command, got %s", resp.ToolCalls[0].Name)
	}
}

func TestParseClaudeStream_StructuredToolCall(t *testing.T) {
	t.Parallel()
	tools := []ToolDef{{Name: "run_command", Description: "run a command"}}
	stream := "{\"type\":\"text\",\"text\":\"I'll check the pods.\\n```json\\n{\\\"tool\\\": \\\"run_command\\\", \\\"arguments\\\": {\\\"command\\\": \\\"kubectl get pods\\\"}}\\n```\"}\n"
	resp, err := parseClaudeStream(stream, tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "run_command" {
		t.Fatalf("expected run_command, got %s", resp.ToolCalls[0].Name)
	}
}

func TestBuildToolPrompt(t *testing.T) {
	t.Parallel()
	tools := BenchTools()
	prompt := buildToolPrompt(tools)
	if !strings.Contains(prompt, "run_command") {
		t.Fatal("missing run_command in tool prompt")
	}
	if strings.Contains(prompt, "evidra_") {
		t.Fatal("default provider prompt must not include evidra-specific tools")
	}
	if !strings.Contains(prompt, "```json") {
		t.Fatal("missing JSON format instruction")
	}
}
