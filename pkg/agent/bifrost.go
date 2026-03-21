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

// modelRoutes maps model name prefixes to their provider's OpenAI-compatible base URL
// and the env var holding the API key. The provider auto-routes based on model name.
var modelRoutes = []struct {
	prefix  string
	urlEnv  string
	keyEnv  string
	baseURL string // fallback if env not set
}{
	{"gpt-", "OPENAI_API_URL", "OPENAI_API_KEY", "https://api.openai.com/v1"},
	{"o1-", "OPENAI_API_URL", "OPENAI_API_KEY", "https://api.openai.com/v1"},
	{"o3-", "OPENAI_API_URL", "OPENAI_API_KEY", "https://api.openai.com/v1"},
	{"claude-", "ANTHROPIC_API_URL", "ANTHROPIC_API_KEY", "https://api.anthropic.com/v1"},
	{"gemini-", "GEMINI_API_URL", "GEMINI_API_KEY", "https://generativelanguage.googleapis.com/v1beta/openai"},
	{"deepseek-", "DEEPSEEK_API_URL", "DEEPSEEK_API_KEY", "https://api.deepseek.com/v1"},
	{"qwen", "DASHSCOPE_API_URL", "DASHSCOPE_API_KEY", "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"},
}

// resolveModelRoute returns the base URL and auth bearer for a given model name.
// Priority: explicit EVIDRA_BIFROST_BASE_URL env > auto-route by model prefix.
func resolveModelRoute(model string) (baseURL, authBearer string) {
	// Explicit override wins (backwards compatible).
	if url := os.Getenv("EVIDRA_BIFROST_BASE_URL"); url != "" {
		bearer := os.Getenv("EVIDRA_BIFROST_AUTH_BEARER")
		return strings.TrimRight(url, "/"), bearer
	}
	if url := os.Getenv("INFRA_BENCH_BIFROST_URL"); url != "" {
		bearer := os.Getenv("EVIDRA_BIFROST_AUTH_BEARER")
		return strings.TrimRight(url, "/"), bearer
	}

	// Auto-route by model name prefix.
	lower := strings.ToLower(model)
	for _, r := range modelRoutes {
		if strings.HasPrefix(lower, r.prefix) {
			url := os.Getenv(r.urlEnv)
			if url == "" {
				url = r.baseURL
			}
			key := os.Getenv(r.keyEnv)
			return strings.TrimRight(url, "/"), key
		}
	}

	// Fallback to localhost proxy.
	return "http://localhost:8080/openai", ""
}

// NewBifrostProvider creates a BifrostProvider from environment variables.
// Set INFRA_BENCH_BIFROST_RPM to throttle requests (e.g. "10" for 10 req/min).
func NewBifrostProvider() *BifrostProvider {
	// Base URL is resolved per-request now (in Chat), but we still need a default
	// for backwards compatibility and logging.
	baseURL := os.Getenv("INFRA_BENCH_BIFROST_URL")
	if baseURL == "" {
		baseURL = os.Getenv("EVIDRA_BIFROST_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "auto-route"
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
// The target API is auto-resolved from the model name (e.g., gpt-4o → OpenAI,
// gemini-2.5-flash → Google). Override with EVIDRA_BIFROST_BASE_URL env var.
func (p *BifrostProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Resolve the target API for this model.
	baseURL, authBearer := resolveModelRoute(req.Model)
	log.Printf("[bifrost] model=%s → %s", req.Model, baseURL)

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

		resp, respBytes, err := p.doRequestWithRoute(ctx, body, baseURL, authBearer)
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

func (p *BifrostProvider) doRequestWithRoute(ctx context.Context, body []byte, baseURL, authBearer string) (*http.Response, []byte, error) {
	url := baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if authBearer != "" {
		httpReq.Header.Set("Authorization", "Bearer "+authBearer)
	}
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
		Content:          choice.Message.Content,
		ReasoningContent: choice.Message.ReasoningContent,
		ToolCalls:        toolCalls,
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
