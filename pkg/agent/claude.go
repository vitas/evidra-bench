package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeProvider uses the Claude CLI (`claude -p`) as an LLM backend.
// It adapts the CLI's stream-json output to the ChatResponse format.
//
// Note: Claude CLI handles tool use internally via --allowedTools.
// For the agent loop, we use Claude's --output-format stream-json to
// capture the conversation and parse tool calls from the stream.
type ClaudeProvider struct{}

// NewClaudeProvider creates a new ClaudeProvider.
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{}
}

func (p *ClaudeProvider) Name() string { return "claude" }

// Chat sends a prompt to Claude CLI and parses the response.
// For multi-turn tool use, Claude CLI manages its own tool loop internally.
// This provider sends the full conversation as a single prompt and parses
// the final output.
func (p *ClaudeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := resolveClaudeModel(req.Model)

	// Build prompt from messages
	var prompt strings.Builder
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			// System prompt goes via --append-system-prompt
			continue
		case "user":
			prompt.WriteString(m.Content)
			prompt.WriteString("\n")
		case "tool":
			prompt.WriteString(fmt.Sprintf("[Tool result for %s]: %s\n", m.ToolCallID, m.Content))
		case "assistant":
			if m.Content != "" {
				prompt.WriteString(fmt.Sprintf("[Assistant]: %s\n", m.Content))
			}
		}
	}

	// Extract system prompt
	systemPrompt := ""
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			break
		}
	}

	args := []string{
		"-p", prompt.String(),
		"--output-format", "stream-json",
		"--model", model,
		"--max-turns", "1",
	}
	if systemPrompt != "" {
		args = append(args, "--append-system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("claude: command failed: %w\noutput: %s", err, truncate(string(out), 500))
	}

	return parseClaudeStream(string(out))
}

func parseClaudeStream(stream string) (*ChatResponse, error) {
	var content strings.Builder
	var toolCalls []ToolCall
	var usage Usage

	for _, line := range strings.Split(stream, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)
		switch eventType {
		case "text":
			if text, ok := event["text"].(string); ok {
				content.WriteString(text)
			}
		case "result":
			if result, ok := event["result"].(string); ok {
				content.WriteString(result)
			} else if result, ok := event["result"].(map[string]any); ok {
				b, _ := json.Marshal(result)
				content.WriteString(string(b))
			}
		case "tool_use":
			id, _ := event["id"].(string)
			name, _ := event["name"].(string)
			input, _ := event["input"].(map[string]any)
			argsJSON, _ := json.Marshal(input)
			toolCalls = append(toolCalls, ToolCall{
				ID:        id,
				Name:      name,
				Arguments: string(argsJSON),
			})
		case "usage":
			if u, ok := event["usage"].(map[string]any); ok {
				if pt, ok := u["input_tokens"].(float64); ok {
					usage.PromptTokens = int(pt)
				}
				if ct, ok := u["output_tokens"].(float64); ok {
					usage.CompletionTokens = int(ct)
				}
			}
		}
	}

	return &ChatResponse{
		Content:   strings.TrimSpace(content.String()),
		ToolCalls: toolCalls,
		Usage:     usage,
	}, nil
}

func resolveClaudeModel(model string) string {
	if model == "" {
		return "sonnet"
	}
	if strings.HasPrefix(model, "claude/") {
		return strings.TrimPrefix(model, "claude/")
	}
	return model
}
