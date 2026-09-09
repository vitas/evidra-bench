package modelconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Ollama discovery/preflight limits. Discovery is configuration, not
// inference: requests are bounded and non-mutating by contract.
const (
	ollamaRequestTimeout = 5 * time.Second
	ollamaMaxResponse    = 1 << 20
)

// LocalModel is the stable internal view of one installed Ollama model. Only
// non-secret identity metadata from the documented API is exposed.
type LocalModel struct {
	Name          string
	Digest        string
	ParameterSize string
	Quantization  string
	Capabilities  []string
}

// OllamaClient reads installed-model metadata from the native Ollama API.
// Inference never goes through this client; it uses the shared
// OpenAI-compatible provider via the resolved /v1 endpoint.
type OllamaClient struct {
	// BaseURL is the native API root, e.g. http://127.0.0.1:11434/api.
	BaseURL    string
	HTTPClient *http.Client
}

func (c OllamaClient) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: ollamaRequestTimeout}
}

func (c OllamaClient) baseURL() string {
	return strings.TrimRight(c.BaseURL, "/")
}

// Discover lists every installed model exactly as reported by /api/tags.
func (c OllamaClient) Discover(ctx context.Context) ([]LocalModel, error) {
	var payload struct {
		Models []struct {
			Name    string `json:"name"`
			Model   string `json:"model"`
			Digest  string `json:"digest"`
			Details struct {
				ParameterSize     string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := c.getJSON(ctx, "/tags", nil, &payload); err != nil {
		return nil, fmt.Errorf("ollama model discovery: %w", err)
	}
	models := make([]LocalModel, 0, len(payload.Models))
	for _, m := range payload.Models {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = strings.TrimSpace(m.Model)
		}
		if name == "" {
			continue
		}
		models = append(models, LocalModel{
			Name:          name,
			Digest:        strings.TrimSpace(m.Digest),
			ParameterSize: strings.TrimSpace(m.Details.ParameterSize),
			Quantization:  strings.TrimSpace(m.Details.QuantizationLevel),
		})
	}
	return models, nil
}

// Show reads identity and capability metadata for one model from /api/show.
// Older runtimes may omit capabilities entirely; that yields an empty list,
// not an error, so callers can fall back to a behavioral probe.
func (c OllamaClient) Show(ctx context.Context, name string) (LocalModel, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return LocalModel{}, fmt.Errorf("ollama model details: model name is required")
	}
	var payload struct {
		Details struct {
			ParameterSize     string `json:"parameter_size"`
			QuantizationLevel string `json:"quantization_level"`
		} `json:"details"`
		Capabilities []string `json:"capabilities"`
	}
	requestBody, err := json.Marshal(map[string]string{"model": name})
	if err != nil {
		return LocalModel{}, fmt.Errorf("ollama model details: encode request: %w", err)
	}
	if err := c.getJSON(ctx, "/show", requestBody, &payload); err != nil {
		return LocalModel{}, fmt.Errorf("ollama model details: %w", err)
	}
	capabilities := make([]string, 0, len(payload.Capabilities))
	for _, capability := range payload.Capabilities {
		if trimmed := strings.TrimSpace(capability); trimmed != "" {
			capabilities = append(capabilities, trimmed)
		}
	}
	return LocalModel{
		Name:          name,
		ParameterSize: strings.TrimSpace(payload.Details.ParameterSize),
		Quantization:  strings.TrimSpace(payload.Details.QuantizationLevel),
		Capabilities:  capabilities,
	}, nil
}

// FindInstalled returns the model with the exact requested tag. It never
// fuzzy-matches: a bare family name is not an installed model.
func (c OllamaClient) FindInstalled(ctx context.Context, name string) (LocalModel, error) {
	models, err := c.Discover(ctx)
	if err != nil {
		return LocalModel{}, err
	}
	for _, m := range models {
		if m.Name == name {
			return m, nil
		}
	}
	return LocalModel{}, fmt.Errorf("ollama model %q is not installed", name)
}

// CompatibleModels lists installed models that declare every required
// capability. Without requirements it is Discover. Models whose metadata
// cannot be read are skipped rather than failing the whole listing; selection
// of a specific model uses FindInstalled plus Show and surfaces errors.
func (c OllamaClient) CompatibleModels(ctx context.Context, required []string) ([]LocalModel, error) {
	models, err := c.Discover(ctx)
	if err != nil {
		return nil, err
	}
	if len(required) == 0 {
		return models, nil
	}
	compatible := make([]LocalModel, 0, len(models))
	for _, m := range models {
		details, err := c.Show(ctx, m.Name)
		if err != nil {
			continue
		}
		if !hasAllCapabilities(details.Capabilities, required) {
			continue
		}
		m.ParameterSize = details.ParameterSize
		m.Quantization = details.Quantization
		m.Capabilities = details.Capabilities
		compatible = append(compatible, m)
	}
	return compatible, nil
}

func hasAllCapabilities(declared, required []string) bool {
	for _, want := range required {
		found := false
		for _, have := range declared {
			if have == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// getJSON performs one bounded, non-mutating API call. Request bodies are
// only used by documented POST metadata endpoints such as /api/show.
func (c OllamaClient) getJSON(ctx context.Context, path string, requestBody []byte, out any) error {
	endpoint := c.baseURL() + path
	var body io.Reader
	if requestBody != nil {
		body = bytes.NewReader(requestBody)
	}
	method := http.MethodGet
	if requestBody != nil {
		method = http.MethodPost
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("ollama runtime unreachable: %w", err)
	}
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client().Do(request)
	if err != nil {
		return fmt.Errorf("ollama runtime unreachable: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("ollama runtime unreachable: %s returned HTTP %d", path, response.StatusCode)
	}
	limited := io.LimitReader(response.Body, ollamaMaxResponse+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("ollama runtime unreachable: read %s: %w", path, err)
	}
	if len(data) > ollamaMaxResponse {
		return fmt.Errorf("ollama response from %s is malformed: exceeds %d bytes", path, ollamaMaxResponse)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("ollama response from %s is malformed: %w", path, err)
	}
	return nil
}
