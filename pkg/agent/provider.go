// Package agent provides pluggable LLM providers and a multi-turn tool-use agent loop.
package agent

import (
	"context"
	"fmt"
)

// Provider sends messages to an LLM and gets responses.
type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// ChatRequest is a single turn in a conversation.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []ToolDef `json:"tools,omitempty"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// Message is a single message in the conversation.
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"` // DeepSeek Reasoner thinking tokens
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

// ToolDef defines a tool the model can call.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall is a function call requested by the model.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatResponse is the model's response to a ChatRequest.
type ChatResponse struct {
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"` // DeepSeek Reasoner
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	Usage            Usage      `json:"usage"`
}

// Usage tracks token consumption including prompt cache tokens.
type Usage struct {
	PromptTokens             int `json:"prompt_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// Done returns true if the model is finished (no more tool calls).
func (r *ChatResponse) Done() bool {
	return len(r.ToolCalls) == 0
}

// ResolveProvider returns a Provider by name.
func ResolveProvider(name string) (Provider, error) {
	switch name {
	case "bifrost":
		return NewBifrostProvider(), nil
	case "claude":
		return NewClaudeProvider(), nil
	case "anthropic":
		return NewAnthropicProvider(), nil
	default:
		return nil, fmt.Errorf("agent.ResolveProvider: unknown provider %q (available: bifrost, claude, anthropic)", name)
	}
}
