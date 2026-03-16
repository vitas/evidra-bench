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
	"strconv"
	"strings"
	"time"
)

// BifrostProvider talks to any LLM via an OpenAI-compatible API proxy.
type BifrostProvider struct {
	BaseURL     string
	HTTPClient  *http.Client
	Retry       RetryConfig
	minInterval time.Duration // minimum delay between requests (anti-throttle)
	lastRequest time.Time
}

// NewBifrostProvider creates a BifrostProvider from environment variables.
// Set INFRA_BENCH_BIFROST_RPM to throttle requests (e.g. "10" for 10 req/min).
func NewBifrostProvider() *BifrostProvider {
	baseURL := os.Getenv("INFRA_BENCH_BIFROST_URL")
	if baseURL == "" {
		baseURL = os.Getenv("EVIDRA_BIFROST_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080/openai"
	}

	var minInterval time.Duration
	if rpmStr := os.Getenv("INFRA_BENCH_BIFROST_RPM"); rpmStr != "" {
		if rpm, err := strconv.Atoi(rpmStr); err == nil && rpm > 0 {
			minInterval = time.Minute / time.Duration(rpm)
			log.Printf("[bifrost] throttle: %d RPM (min interval %s)", rpm, minInterval)
		}
	}

	return &BifrostProvider{
		BaseURL:     strings.TrimRight(baseURL, "/"),
		HTTPClient:  &http.Client{Timeout: 5 * time.Minute},
		Retry:       DefaultRetryConfig(),
		minInterval: minInterval,
	}
}

func (p *BifrostProvider) Name() string { return "bifrost" }

// Chat sends a chat completion request to the Bifrost proxy with adaptive retry.
func (p *BifrostProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Anti-throttle: wait if needed to respect RPM limit.
	if p.minInterval > 0 && !p.lastRequest.IsZero() {
		elapsed := time.Since(p.lastRequest)
		if wait := p.minInterval - elapsed; wait > 0 {
			log.Printf("[bifrost] throttle: waiting %s", wait.Round(time.Millisecond))
			if err := SleepWithContext(ctx, wait); err != nil {
				return nil, err
			}
		}
	}
	p.lastRequest = time.Now()

	payload := buildOpenAIPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("bifrost: marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= p.Retry.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[bifrost] retry attempt %d/%d after: %v", attempt, p.Retry.MaxRetries, lastErr)
		}

		resp, respBytes, err := p.doRequest(ctx, body)
		if err != nil {
			lastErr = err
			if attempt < p.Retry.MaxRetries {
				backoff := BackoffDuration(p.Retry, attempt, http.Header{})
				log.Printf("[bifrost] connection error, backing off %s", backoff)
				if sleepErr := SleepWithContext(ctx, backoff); sleepErr != nil {
					return nil, sleepErr
				}
			}
			continue
		}

		if resp.StatusCode < 400 {
			return parseOpenAIResponse(respBytes)
		}

		if IsRetryable(resp.StatusCode) && attempt < p.Retry.MaxRetries {
			backoff := BackoffDuration(p.Retry, attempt, resp.Header)
			log.Printf("[bifrost] HTTP %d, backing off %s", resp.StatusCode, backoff)
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

		return nil, fmt.Errorf("bifrost: HTTP %d: %s", resp.StatusCode, truncate(string(respBytes), 300))
	}

	return nil, fmt.Errorf("bifrost: exhausted %d retries: %w", p.Retry.MaxRetries, lastErr)
}

func (p *BifrostProvider) doRequest(ctx context.Context, body []byte) (*http.Response, []byte, error) {
	url := p.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	applyBifrostEnvHeaders(httpReq.Header)

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}
	return &http.Response{StatusCode: resp.StatusCode, Header: resp.Header}, respBytes, nil
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
