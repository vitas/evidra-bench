package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeProvider uses the Claude CLI (`claude -p`) as an LLM backend.
//
// Claude CLI doesn't support arbitrary tool definitions via flags like the
// OpenAI API does. Instead, we embed tool schemas in the system prompt and
// instruct Claude to output structured JSON tool calls. The stream-json
// output format captures tool_use events when Claude invokes its built-in
// tools, and we parse structured JSON blocks for our custom tools.
type ClaudeProvider struct{}

// NewClaudeProvider creates a new ClaudeProvider.
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{}
}

func (p *ClaudeProvider) Name() string { return "claude" }

// Chat sends a prompt to Claude CLI and parses the response.
// Tools from req.Tools are embedded in the system prompt as structured
// descriptions so Claude knows what's available.
func (p *ClaudeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := resolveClaudeModel(req.Model)

	// Build system prompt with embedded tool definitions
	systemPrompt := ""
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			break
		}
	}
	if len(req.Tools) > 0 {
		systemPrompt = systemPrompt + "\n\n" + buildToolPrompt(req.Tools)
	}

	// Build user prompt from message history
	var prompt strings.Builder
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			continue
		case "user":
			prompt.WriteString(m.Content)
			prompt.WriteString("\n")
		case "tool":
			fmt.Fprintf(&prompt, "[Tool result for %s]: %s\n", m.ToolCallID, m.Content)
		case "assistant":
			if m.Content != "" {
				fmt.Fprintf(&prompt, "[Assistant]: %s\n", m.Content)
			}
			for _, tc := range m.ToolCalls {
				fmt.Fprintf(&prompt, "[Tool call]: %s(%s)\n", tc.Name, tc.Arguments)
			}
		}
	}

	args := []string{
		"-p", prompt.String(),
		"--output-format", "stream-json",
		"--verbose",
		"--model", model,
		"--max-turns", "1",
	}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("claude: command failed: %w\noutput: %s", err, truncate(string(out), 500))
	}

	return parseClaudeStream(string(out), req.Tools)
}

// buildToolPrompt creates a structured tool description for the system prompt.
func buildToolPrompt(tools []ToolDef) string {
	var b strings.Builder
	b.WriteString("CRITICAL: You ONLY have access to the tools listed below.\n")
	b.WriteString("Do NOT use Bash, Skill, ToolSearch, Agent, or any mcp__ tools — they do not exist in this environment.\n")
	b.WriteString("To use a tool, respond with a JSON block:\n")
	b.WriteString("```json\n{\"tool\": \"<tool_name>\", \"arguments\": {<args>}}\n```\n\n")
	b.WriteString("Available tools (ONLY these):\n\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "### %s\n%s\n", t.Name, t.Description)
		if props, ok := t.Parameters["properties"].(map[string]any); ok {
			b.WriteString("Parameters:\n")
			for name, schema := range props {
				desc := ""
				if s, ok := schema.(map[string]any); ok {
					desc, _ = s["description"].(string)
				}
				fmt.Fprintf(&b, "  - %s: %s\n", name, desc)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("After receiving a tool result, continue working toward the goal.\n")
	b.WriteString("When done, respond with your final answer as plain text (no JSON block).\n")
	return b.String()
}

// parseClaudeStream parses Claude CLI stream-json output.
// It handles both native tool_use events and structured JSON tool calls
// embedded in text output.
func parseClaudeStream(stream string, tools []ToolDef) (*ChatResponse, error) {
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
		case "assistant":
			// Parse content blocks from assistant message
			if msg, ok := event["message"].(map[string]any); ok {
				if blocks, ok := msg["content"].([]any); ok {
					for _, block := range blocks {
						b, ok := block.(map[string]any)
						if !ok {
							continue
						}
						switch b["type"] {
						case "text":
							if text, ok := b["text"].(string); ok {
								content.WriteString(text)
							}
						case "tool_use":
							id, _ := b["id"].(string)
							name, _ := b["name"].(string)
							input, _ := b["input"].(map[string]any)
							argsJSON, _ := json.Marshal(input)
							toolCalls = append(toolCalls, ToolCall{ID: id, Name: name, Arguments: string(argsJSON)})
						}
					}
				}
			}
		case "result":
			if result, ok := event["result"].(string); ok {
				if content.Len() == 0 {
					content.WriteString(result)
				}
			} else if result, ok := event["result"].(map[string]any); ok {
				b, _ := json.Marshal(result)
				if content.Len() == 0 {
					content.WriteString(string(b))
				}
			}
			// Extract usage from result event
			if u, ok := event["usage"].(map[string]any); ok {
				if pt, ok := u["input_tokens"].(float64); ok {
					usage.PromptTokens = int(pt)
				}
				if ct, ok := u["output_tokens"].(float64); ok {
					usage.CompletionTokens = int(ct)
				}
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

	// If no native tool_use events, check for structured JSON tool calls in text
	text := content.String()
	if len(toolCalls) == 0 && len(tools) > 0 {
		if tc, remaining := extractToolCallFromText(text, tools); tc != nil {
			toolCalls = append(toolCalls, *tc)
			text = strings.TrimSpace(remaining)
		}
	}

	return &ChatResponse{
		Content:   strings.TrimSpace(text),
		ToolCalls: toolCalls,
		Usage:     usage,
	}, nil
}

// extractToolCallFromText looks for a JSON tool call block in the text.
func extractToolCallFromText(text string, tools []ToolDef) (*ToolCall, string) {
	toolNames := make(map[string]bool)
	for _, t := range tools {
		toolNames[t.Name] = true
	}

	// Look for ```json blocks or bare JSON objects with "tool" key
	candidates := extractJSONBlocks(text)
	for _, candidate := range candidates {
		var call struct {
			Tool      string         `json:"tool"`
			Arguments map[string]any `json:"arguments"`
		}
		if json.Unmarshal([]byte(candidate.json), &call) == nil && toolNames[call.Tool] {
			argsJSON, _ := json.Marshal(call.Arguments)
			tc := &ToolCall{
				ID:        fmt.Sprintf("tc-%s", call.Tool),
				Name:      call.Tool,
				Arguments: string(argsJSON),
			}
			remaining := text[:candidate.start] + text[candidate.end:]
			return tc, remaining
		}
	}
	return nil, text
}

type jsonBlock struct {
	json       string
	start, end int
}

// extractJSONBlocks finds JSON objects in text, both in code fences and bare.
func extractJSONBlocks(text string) []jsonBlock {
	var blocks []jsonBlock

	// Check code fences first
	rest := text
	offset := 0
	for {
		fenceStart := strings.Index(rest, "```")
		if fenceStart < 0 {
			break
		}
		afterStart := rest[fenceStart+3:]
		fenceEnd := strings.Index(afterStart, "```")
		if fenceEnd < 0 {
			break
		}
		inner := afterStart[:fenceEnd]
		// Strip optional language tag
		if nl := strings.Index(inner, "\n"); nl >= 0 {
			tag := strings.TrimSpace(inner[:nl])
			if tag == "json" || tag == "" {
				inner = inner[nl+1:]
			}
		}
		inner = strings.TrimSpace(inner)
		if strings.HasPrefix(inner, "{") {
			blocks = append(blocks, jsonBlock{
				json:  inner,
				start: offset + fenceStart,
				end:   offset + fenceStart + 3 + fenceEnd + 3,
			})
		}
		advance := fenceStart + 3 + fenceEnd + 3
		offset += advance
		rest = rest[advance:]
	}

	// Check for bare JSON objects if no fenced blocks found
	if len(blocks) == 0 {
		for i := 0; i < len(text); i++ {
			if text[i] == '{' {
				depth := 0
				for j := i; j < len(text); j++ {
					if text[j] == '{' {
						depth++
					} else if text[j] == '}' {
						depth--
						if depth == 0 {
							candidate := text[i : j+1]
							blocks = append(blocks, jsonBlock{json: candidate, start: i, end: j + 1})
							break
						}
					}
				}
				break // only first bare object
			}
		}
	}

	return blocks
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
