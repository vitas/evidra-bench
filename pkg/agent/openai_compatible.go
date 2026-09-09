package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatibleConfig struct {
	Name        string
	BaseURL     string
	APIKey      string
	Headers     http.Header
	HTTPClient  *http.Client
	Retry       RetryConfig
	MinInterval time.Duration
}

// OpenAICompatibleProvider sends chat-completion requests to a configurable
// endpoint. Named providers such as OpenAI and Bifrost normalize into this
// implementation rather than maintaining separate HTTP stacks.
type OpenAICompatibleProvider struct {
	name        string
	baseURL     string
	apiKey      string
	headers     http.Header
	httpClient  *http.Client
	retry       RetryConfig
	minInterval time.Duration
	lastRequest time.Time
}

func NewOpenAICompatibleProvider(cfg OpenAICompatibleConfig) *OpenAICompatibleProvider {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "openai-compatible"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	return &OpenAICompatibleProvider{
		name:        name,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:      cfg.APIKey,
		headers:     cfg.Headers.Clone(),
		httpClient:  client,
		retry:       cfg.Retry,
		minInterval: cfg.MinInterval,
	}
}

func (p *OpenAICompatibleProvider) Name() string { return p.name }

func (p *OpenAICompatibleProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if p.minInterval > 0 && !p.lastRequest.IsZero() {
		if wait := p.minInterval - time.Since(p.lastRequest); wait > 0 {
			if err := SleepWithContext(ctx, wait); err != nil {
				return nil, err
			}
		}
	}
	p.lastRequest = time.Now()

	body, err := json.Marshal(buildOpenAIPayload(req))
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", p.name, err)
	}
	var lastErr error
	for attempt := 0; attempt <= p.retry.MaxRetries; attempt++ {
		response, responseBody, requestErr := p.doRequest(ctx, body)
		if requestErr != nil {
			lastErr = requestErr
			if attempt < p.retry.MaxRetries {
				backoff := BackoffDuration(p.retry, attempt, http.Header{})
				log.Printf("[%s] connection error, backing off %s", p.name, backoff)
				if err := SleepWithContext(ctx, backoff); err != nil {
					return nil, err
				}
			}
			continue
		}
		if response.StatusCode < http.StatusBadRequest {
			return parseOpenAIResponse(responseBody)
		}
		if IsRetryable(response.StatusCode) && attempt < p.retry.MaxRetries {
			backoff := BackoffDuration(p.retry, attempt, response.Header)
			lastErr = &RateLimitError{StatusCode: response.StatusCode, Body: truncate(string(responseBody), 200), RetryAfter: backoff}
			if err := SleepWithContext(ctx, backoff); err != nil {
				return nil, err
			}
			continue
		}
		return nil, fmt.Errorf("%s: HTTP %d: %s", p.name, response.StatusCode, truncate(string(responseBody), 300))
	}
	return nil, fmt.Errorf("%s: exhausted %d retries: %w", p.name, p.retry.MaxRetries, lastErr)
}

func (p *OpenAICompatibleProvider) doRequest(ctx context.Context, body []byte) (*http.Response, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	for name, values := range p.headers {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	if p.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	response, err := p.httpClient.Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}
	return &http.Response{StatusCode: response.StatusCode, Header: response.Header}, responseBody, nil
}
