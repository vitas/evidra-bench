package agent

import (
	"strings"
	"testing"
)

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
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, expected := range []string{"run_command", "evidra_prescribe", "evidra_report"} {
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
	if !strings.Contains(prompt, "evidra_prescribe") {
		t.Fatal("missing evidra_prescribe in tool prompt")
	}
	if !strings.Contains(prompt, "```json") {
		t.Fatal("missing JSON format instruction")
	}
}
