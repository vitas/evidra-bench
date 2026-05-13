package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// AnthropicProvider talks to the Anthropic Messages API directly.
type AnthropicProvider struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Retry      RetryConfig
}

// NewAnthropicProvider creates an AnthropicProvider from environment variables.
func NewAnthropicProvider() *AnthropicProvider {
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	return &AnthropicProvider{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
		Retry:      DefaultRetryConfig(),
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

// Chat sends a chat completion request to the Anthropic Messages API.
func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	payload := buildAnthropicPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= p.Retry.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[anthropic] retry attempt %d/%d after: %v", attempt, p.Retry.MaxRetries, lastErr)
		}

		resp, respBytes, err := p.doRequest(ctx, body)
		if err != nil {
			lastErr = err
			if attempt < p.Retry.MaxRetries {
				backoff := BackoffDuration(p.Retry, attempt, http.Header{})
				if sleepErr := SleepWithContext(ctx, backoff); sleepErr != nil {
					return nil, sleepErr
				}
			}
			continue
		}

		if resp.StatusCode < 400 {
			return parseAnthropicResponse(respBytes)
		}

		if IsRetryable(resp.StatusCode) && attempt < p.Retry.MaxRetries {
			backoff := BackoffDuration(p.Retry, attempt, resp.Header)
			log.Printf("[anthropic] HTTP %d, backing off %s", resp.StatusCode, backoff)
			lastErr = &RateLimitError{
				StatusCode: resp.StatusCode,
				Body:       truncate(string(respBytes), 200),
				RetryAfter: backoff,
			}
			if sleepErr := SleepWithContext(ctx, backoff); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		return nil, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, truncate(string(respBytes), 300))
	}

	return nil, fmt.Errorf("anthropic: exhausted %d retries: %w", p.Retry.MaxRetries, lastErr)
}

func (p *AnthropicProvider) doRequest(ctx context.Context, body []byte) (*http.Response, []byte, error) {
	url := p.BaseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}
	return &http.Response{StatusCode: resp.StatusCode, Header: resp.Header}, respBytes, nil
}

func buildAnthropicPayload(req ChatRequest) map[string]any {
	// Anthropic uses a separate system field, not a system message.
	var system string
	var messages []map[string]any

	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}

		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			// Assistant message with tool calls uses content blocks.
			var content []map[string]any
			if m.Content != "" {
				content = append(content, map[string]any{"type": "text", "text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				var input any
				if err := json.Unmarshal([]byte(tc.Arguments), &input); err != nil {
					input = tc.Arguments
				}
				content = append(content, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": input,
				})
			}
			messages = append(messages, map[string]any{"role": "assistant", "content": content})
			continue
		}

		if m.Role == "tool" {
			messages = append(messages, map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
			continue
		}

		messages = append(messages, map[string]any{"role": m.Role, "content": m.Content})
	}

	payload := map[string]any{
		"model":      req.Model,
		"messages":   messages,
		"max_tokens": req.MaxTokens,
	}
	if system != "" {
		payload["system"] = system
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.Parameters,
			}
		}
		payload["tools"] = tools
	}

	return payload
}

func parseAnthropicResponse(body []byte) (*ChatResponse, error) {
	var raw struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("anthropic: parse response: %w", err)
	}

	var content string
	var toolCalls []ToolCall

	for _, block := range raw.Content {
		switch block.Type {
		case "text":
			content = block.Text
		case "tool_use":
			args := string(block.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	return &ChatResponse{
		Content:   content,
		ToolCalls: toolCalls,
		Usage: Usage{
			PromptTokens:     raw.Usage.InputTokens,
			CompletionTokens: raw.Usage.OutputTokens,
		},
	}, nil
}
