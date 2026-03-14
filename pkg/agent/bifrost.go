package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// BifrostProvider talks to any LLM via an OpenAI-compatible API proxy.
type BifrostProvider struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewBifrostProvider creates a BifrostProvider from environment variables.
func NewBifrostProvider() *BifrostProvider {
	baseURL := os.Getenv("INFRA_BENCH_BIFROST_URL")
	if baseURL == "" {
		baseURL = os.Getenv("EVIDRA_BIFROST_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080/openai"
	}
	return &BifrostProvider{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *BifrostProvider) Name() string { return "bifrost" }

// Chat sends a chat completion request to the Bifrost proxy.
func (p *BifrostProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	payload := buildOpenAIPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("bifrost: marshal request: %w", err)
	}

	url := p.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("bifrost: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	applyBifrostEnvHeaders(httpReq.Header)

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bifrost: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bifrost: read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("bifrost: HTTP %d: %s", resp.StatusCode, truncate(string(respBytes), 300))
	}

	return parseOpenAIResponse(respBytes)
}

func buildOpenAIPayload(req ChatRequest) map[string]any {
	messages := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		msg := map[string]any{"role": m.Role}
		if m.Content != "" {
			msg["content"] = m.Content
		}
		if len(m.ToolCalls) > 0 {
			tcs := make([]map[string]any, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				tcs[i] = map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": tc.Arguments,
					},
				}
			}
			msg["tool_calls"] = tcs
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		messages = append(messages, msg)
	}

	payload := map[string]any{
		"model":       req.Model,
		"messages":    messages,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.Parameters,
				},
			}
		}
		payload["tools"] = tools
	}

	return payload
}

func parseOpenAIResponse(body []byte) (*ChatResponse, error) {
	var raw struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("bifrost: parse response: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("bifrost: no choices in response")
	}

	choice := raw.Choices[0]
	var toolCalls []ToolCall
	for _, tc := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return &ChatResponse{
		Content:   choice.Message.Content,
		ToolCalls: toolCalls,
		Usage: Usage{
			PromptTokens:     raw.Usage.PromptTokens,
			CompletionTokens: raw.Usage.CompletionTokens,
		},
	}, nil
}

func applyBifrostEnvHeaders(header http.Header) {
	if vk := strings.TrimSpace(os.Getenv("EVIDRA_BIFROST_VK")); vk != "" {
		header.Set("x-bf-vk", vk)
	}
	if bearer := strings.TrimSpace(os.Getenv("EVIDRA_BIFROST_AUTH_BEARER")); bearer != "" {
		header.Set("Authorization", "Bearer "+bearer)
	}
}
