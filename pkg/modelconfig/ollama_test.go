package modelconfig

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaClientDiscoversInstalledModelsTolerantly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/tags" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Includes unknown fields and partially missing optional metadata:
		// parsing must tolerate both.
		_, _ = w.Write([]byte(`{
			"models": [
				{"name":"qwen3:8b","model":"qwen3:8b","digest":"sha256:aaa","size":9000000,
				 "details":{"family":"qwen3","parameter_size":"8.4B","quantization_level":"Q4_K_M"},
				 "future_field":{"nested":true}},
				{"name":"llama3.2:latest","digest":"sha256:bbb"},
				{"name":"no-metadata:tag"}
			],
			"unknown_root":"ignored"}`))
	}))
	defer server.Close()

	client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()}
	models, err := client.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("len(models) = %d, want 3", len(models))
	}
	if models[0].Name != "qwen3:8b" || models[0].Digest != "sha256:aaa" ||
		models[0].ParameterSize != "8.4B" || models[0].Quantization != "Q4_K_M" {
		t.Fatalf("first model = %+v", models[0])
	}
	if models[1].Name != "llama3.2:latest" || models[1].ParameterSize != "" {
		t.Fatalf("second model = %+v", models[1])
	}
	if models[2].Digest != "" {
		t.Fatalf("missing digest should stay empty: %+v", models[2])
	}
}

func TestOllamaClientShowReadsCapabilitiesAndDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/show" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "qwen3:8b" {
			t.Errorf("show request body = %+v, err = %v", body, err)
		}
		_, _ = w.Write([]byte(`{"details":{"parameter_size":"8.4B","quantization_level":"Q4_K_M"},
			"capabilities":["completion","tools","insert"],"licenses":{"licence":"MIT"},"extra":1}`))
	}))
	defer server.Close()

	client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()}
	details, err := client.Show(context.Background(), "qwen3:8b")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if details.Name != "qwen3:8b" || details.ParameterSize != "8.4B" || details.Quantization != "Q4_K_M" {
		t.Fatalf("details = %+v", details)
	}
	if !containsString(details.Capabilities, "tools") || len(details.Capabilities) != 3 {
		t.Fatalf("capabilities = %v", details.Capabilities)
	}
}

func TestOllamaClientShowWithoutCapabilitiesField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"details":{"family":"llama"}}`))
	}))
	defer server.Close()

	client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()}
	details, err := client.Show(context.Background(), "old-model:1")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if len(details.Capabilities) != 0 {
		t.Fatalf("capabilities = %v, want empty for old runtimes", details.Capabilities)
	}
}

func TestOllamaClientErrorsNameTheFailedBoundary(t *testing.T) {
	t.Run("non-2xx", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()
		client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()}
		if _, err := client.Discover(context.Background()); err == nil || !strings.Contains(err.Error(), "unreachable") {
			t.Fatalf("Discover() error = %v, want unreachable", err)
		}
		if _, err := client.Show(context.Background(), "m"); err == nil || !strings.Contains(err.Error(), "unreachable") {
			t.Fatalf("Show() error = %v, want unreachable", err)
		}
	})
	t.Run("malformed json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"models": [`))
		}))
		defer server.Close()
		client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()}
		if _, err := client.Discover(context.Background()); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("Discover() error = %v, want malformed", err)
		}
	})
	t.Run("connect failure", func(t *testing.T) {
		client := OllamaClient{BaseURL: "http://127.0.0.1:1/api", HTTPClient: &http.Client{Timeout: time.Second}}
		if _, err := client.Discover(context.Background()); err == nil || !strings.Contains(err.Error(), "unreachable") {
			t.Fatalf("Discover() error = %v, want unreachable", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()
		client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: &http.Client{Timeout: 50 * time.Millisecond}}
		if _, err := client.Discover(context.Background()); err == nil {
			t.Fatal("Discover() expected timeout error")
		}
	})
}

func TestOllamaClientFiltersModelsByRequiredCapabilities(t *testing.T) {
	var showCalls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_, _ = w.Write([]byte(`{"models":[
				{"name":"tool-model:1","digest":"sha256:t"},
				{"name":"plain-model:1","digest":"sha256:p"},
				{"name":"other-tools:2","digest":"sha256:o"}]}`))
		case "/api/show":
			var body struct {
				Model string `json:"model"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			showCalls = append(showCalls, body.Model)
			capabilities := []string{"completion"}
			if strings.HasPrefix(body.Model, "tool") || strings.HasPrefix(body.Model, "other") {
				capabilities = append(capabilities, "tools")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"details":      map[string]string{"parameter_size": "8B", "quantization_level": "Q4_K_M"},
				"capabilities": capabilities,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()}
	compatible, err := client.CompatibleModels(context.Background(), []string{"tools"})
	if err != nil {
		t.Fatalf("CompatibleModels() error = %v", err)
	}
	if len(compatible) != 2 {
		t.Fatalf("compatible = %+v, want two models", compatible)
	}
	if compatible[0].Name != "tool-model:1" || compatible[1].Name != "other-tools:2" {
		t.Fatalf("compatible order = %v", []string{compatible[0].Name, compatible[1].Name})
	}
	if compatible[0].ParameterSize != "8B" || compatible[0].Quantization != "Q4_K_M" {
		t.Fatalf("metadata not enriched: %+v", compatible[0])
	}
	if strings.Join(showCalls, ",") != "tool-model:1,plain-model:1,other-tools:2" {
		t.Fatalf("show calls = %v, want one per installed model", showCalls)
	}
}

func TestOllamaClientCompatibleModelsWithoutRequirementsSkipsShow(t *testing.T) {
	showCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show" {
			showCalled = true
		}
		if r.URL.Path == "/api/tags" {
			_, _ = w.Write([]byte(`{"models":[{"name":"a:1"},{"name":"b:2"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()}
	models, err := client.CompatibleModels(context.Background(), nil)
	if err != nil {
		t.Fatalf("CompatibleModels() error = %v", err)
	}
	if len(models) != 2 || showCalled {
		t.Fatalf("models = %+v, showCalled = %v; want no Show without requirements", models, showCalled)
	}
}

func TestOllamaClientExactTagLookup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"qwen3:8b","digest":"sha256:x"},{"name":"qwen3:latest"}]}`))
	}))
	defer server.Close()

	client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()}
	model, err := client.FindInstalled(context.Background(), "qwen3:8b")
	if err != nil || model.Name != "qwen3:8b" {
		t.Fatalf("FindInstalled() = %+v, %v", model, err)
	}
	// "qwen3" alone is not an installed tag: exact match only, no guessing.
	if _, err := client.FindInstalled(context.Background(), "qwen3"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("FindInstalled(qwen3) error = %v, want not-installed", err)
	}
}

func TestOllamaClientNeverCallsPull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "pull") || strings.Contains(r.URL.Path, "create") || strings.Contains(r.URL.Path, "push") {
			t.Errorf("client used a model-mutation endpoint: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := OllamaClient{BaseURL: server.URL + "/api", HTTPClient: server.Client()}
	if _, err := client.Discover(context.Background()); err == nil {
		t.Fatal("expected 404 error")
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
