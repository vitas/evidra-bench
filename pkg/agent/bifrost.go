package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// BifrostProvider talks to any LLM via an OpenAI-compatible API proxy.
type BifrostProvider struct {
	*OpenAICompatibleProvider
}

// NewBifrostProvider creates a BifrostProvider from environment variables.
// Bifrost is an OpenAI-compatible gateway that handles multi-provider routing.
// Point INFRA_BENCH_BIFROST_URL at your Bifrost instance.
// Set INFRA_BENCH_BIFROST_RPM to throttle requests (e.g. "10" for 10 req/min).
func NewBifrostProvider() *BifrostProvider {
	baseURL := os.Getenv("INFRA_BENCH_BIFROST_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080/v1"
	}

	var minInterval time.Duration
	if rpmStr := os.Getenv("INFRA_BENCH_BIFROST_RPM"); rpmStr != "" {
		if rpm, err := strconv.Atoi(rpmStr); err == nil && rpm > 0 {
			minInterval = time.Minute / time.Duration(rpm)
			log.Printf("[bifrost] throttle: %d RPM (min interval %s)", rpm, minInterval)
		}
	}

	headers := make(http.Header)
	applyBifrostEnvHeaders(headers)
	return &BifrostProvider{OpenAICompatibleProvider: NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		Name:        "bifrost",
		BaseURL:     baseURL,
		Headers:     headers,
		HTTPClient:  &http.Client{Timeout: 5 * time.Minute},
		Retry:       DefaultRetryConfig(),
		MinInterval: minInterval,
	})}
}

func buildOpenAIPayload(req ChatRequest) map[string]any {
	messages := make([]map[string]any, 0, len(req.Messages))
	isDeepSeek := strings.Contains(req.Model, "deepseek")

	for _, m := range req.Messages {
		msg := map[string]any{"role": m.Role}
		if m.Content != "" {
			msg["content"] = m.Content
		} else if m.Role == "tool" || (isDeepSeek && m.Role == "assistant") {
			// Tool messages require content field even when empty (Anthropic API).
			// DeepSeek requires content field on all assistant messages.
			msg["content"] = ""
		}
		// DeepSeek Reasoner requires reasoning_content on assistant messages.
		if isDeepSeek && m.Role == "assistant" && m.ReasoningContent != "" {
			msg["reasoning_content"] = m.ReasoningContent
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
	if strings.HasPrefix(req.Model, "deepseek-v4-") {
		payload["thinking"] = map[string]string{"type": "enabled"}
		payload["reasoning_effort"] = "high"
	}
	if req.MaxTokens > 0 {
		// GPT-5+ models use max_completion_tokens instead of max_tokens.
		if strings.HasPrefix(req.Model, "gpt-5") || strings.HasPrefix(req.Model, "o3") || strings.HasPrefix(req.Model, "o4") {
			payload["max_completion_tokens"] = req.MaxTokens
		} else {
			payload["max_tokens"] = req.MaxTokens
		}
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
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
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
			PromptTokens             int `json:"prompt_tokens"`
			CompletionTokens         int `json:"completion_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			PromptCacheHitTokens     int `json:"prompt_cache_hit_tokens"`
			PromptCacheMissTokens    int `json:"prompt_cache_miss_tokens"`
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
		Content:          choice.Message.Content,
		ReasoningContent: choice.Message.ReasoningContent,
		ToolCalls:        toolCalls,
		Usage: Usage{
			PromptTokens:             raw.Usage.PromptTokens,
			CompletionTokens:         raw.Usage.CompletionTokens,
			CacheCreationInputTokens: raw.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     raw.Usage.CacheReadInputTokens,
			PromptCacheHitTokens:     raw.Usage.PromptCacheHitTokens,
			PromptCacheMissTokens:    raw.Usage.PromptCacheMissTokens,
		},
	}, nil
}

func applyBifrostEnvHeaders(header http.Header) {
	if vk := strings.TrimSpace(os.Getenv("INFRA_BENCH_BIFROST_VK")); vk != "" {
		header.Set("x-bf-vk", vk)
	}
	if bearer := strings.TrimSpace(os.Getenv("INFRA_BENCH_BIFROST_AUTH_BEARER")); bearer != "" {
		header.Set("Authorization", "Bearer "+bearer)
	}
}
