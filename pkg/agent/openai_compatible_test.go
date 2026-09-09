package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleProviderUsesConfiguredEndpointAndBearerToken(t *testing.T) {
	var gotAuth string
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		gotModel, _ = payload["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"done"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		Name:       "openai",
		BaseURL:    server.URL + "/v1/",
		APIKey:     "secret-token",
		HTTPClient: server.Client(),
		Retry:      RetryConfig{},
	})
	response, err := provider.Chat(context.Background(), ChatRequest{Model: "gpt-test", Messages: []Message{{Role: "user", Content: "test"}}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if provider.Name() != "openai" || gotAuth != "Bearer secret-token" || gotModel != "gpt-test" {
		t.Fatalf("provider/auth/model = %q/%q/%q", provider.Name(), gotAuth, gotModel)
	}
	if response.Content != "done" || response.Usage.PromptTokens != 3 {
		t.Fatalf("response = %+v", response)
	}
}

func TestOpenAICompatibleProviderSupportsConfiguredHeadersWithoutBearer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Gateway-Key"); got != "gateway-secret" {
			t.Errorf("X-Gateway-Key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"done"}}]}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		Name:       "custom",
		BaseURL:    server.URL,
		Headers:    http.Header{"X-Gateway-Key": []string{"gateway-secret"}},
		HTTPClient: server.Client(),
		Retry:      RetryConfig{},
	})
	if _, err := provider.Chat(context.Background(), ChatRequest{Model: "local"}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}

func TestNewBifrostProviderNormalizesIntoOpenAICompatibleClient(t *testing.T) {
	t.Setenv("INFRA_BENCH_BIFROST_URL", "http://bifrost.test/v1/")
	t.Setenv("INFRA_BENCH_BIFROST_URL_API_KEY", "gateway-key")
	t.Setenv("INFRA_BENCH_BIFROST_AUTH_BEARER", "gateway-bearer")
	provider := NewBifrostProvider()
	if provider.OpenAICompatibleProvider == nil {
		t.Fatal("Bifrost provider does not use shared OpenAI-compatible client")
	}
	if provider.Name() != "bifrost" || provider.baseURL != "http://bifrost.test/v1" {
		t.Fatalf("provider name/base URL = %q/%q", provider.Name(), provider.baseURL)
	}
	if got := provider.headers.Get("Authorization"); got != "Bearer gateway-bearer" {
		t.Fatalf("Authorization = %q", got)
	}
}
