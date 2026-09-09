package modelconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vitas/evidra-bench/pkg/agent"
)

// Capability-check methods recorded in the evaluation target identity.
const (
	// CapabilityCheckOllamaShow means the runtime's own /api/show metadata
	// declared the required capability; no inference was performed.
	CapabilityCheckOllamaShow = "ollama_show"
	// CapabilityCheckBehavioralProbe means metadata was absent or ambiguous
	// and one bounded synthetic tool call confirmed the behavior. It is a
	// protocol check, not a quality score.
	CapabilityCheckBehavioralProbe = "behavioral_tool_call"
)

// probeToolName is synthetic and never executed; the probe validates that the
// model can emit a well-formed tool call, nothing more.
const probeToolName = "evidra_capability_probe"

const probePrompt = `You are in a capability validation step. No real systems are connected and this call is never executed. Reply by calling the ` + probeToolName + ` tool exactly once with the city set to Paris.`

// PreparedModel is the non-secret identity of a local model that has passed
// suite capability validation and can be copied into an evaluation target.
type PreparedModel struct {
	Name            string
	Digest          string
	ParameterSize   string
	Quantization    string
	CapabilityCheck string
	ProbeUsage      *agent.Usage
}

// ProbeResult records the outcome of one behavioral probe for evidence.
type ProbeResult struct {
	Method string
	Usage  agent.Usage
}

// ProbeToolCalling sends exactly one deterministic synthetic tool definition
// through the given provider. It never executes infrastructure tools; a
// successful probe proves protocol capability only, not quality.
func ProbeToolCalling(ctx context.Context, provider agent.Provider, model string) (ProbeResult, error) {
	if provider == nil {
		return ProbeResult{}, fmt.Errorf("capability probe: provider is required")
	}
	if err := ctx.Err(); err != nil {
		return ProbeResult{}, fmt.Errorf("capability probe: %w", err)
	}
	response, err := provider.Chat(ctx, agent.ChatRequest{
		Model: model,
		Messages: []agent.Message{
			{Role: "user", Content: probePrompt},
		},
		Tools: []agent.ToolDef{{
			Name:        probeToolName,
			Description: "Evidra capability validation probe. Never executed.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"city": map[string]any{"type": "string"}},
				"required":   []string{"city"},
			},
		}},
	})
	if err != nil {
		return ProbeResult{}, fmt.Errorf("capability probe: provider error: %w", err)
	}
	if response == nil {
		return ProbeResult{}, fmt.Errorf("capability probe: provider returned no response")
	}
	if len(response.ToolCalls) == 0 {
		return ProbeResult{}, fmt.Errorf("capability probe: model %q did not call the probe tool", model)
	}
	if len(response.ToolCalls) != 1 {
		return ProbeResult{}, fmt.Errorf("capability probe: model %q must call exactly one probe tool, got %d calls", model, len(response.ToolCalls))
	}
	probeCall := response.ToolCalls[0]
	if probeCall.Name != probeToolName {
		return ProbeResult{}, fmt.Errorf("capability probe: model %q returned the wrong tool (called %q)", model, probeCall.Name)
	}
	var arguments struct {
		City string `json:"city"`
	}
	if err := json.Unmarshal([]byte(probeCall.Arguments), &arguments); err != nil {
		return ProbeResult{}, fmt.Errorf("capability probe: model %q returned malformed tool arguments: %w", model, err)
	}
	if arguments.City != "Paris" {
		return ProbeResult{}, fmt.Errorf("capability probe: model %q must set city to Paris, got %q", model, arguments.City)
	}
	return ProbeResult{Method: CapabilityCheckBehavioralProbe, Usage: response.Usage}, nil
}

// PrepareOllamaModel validates a resolved Ollama model against the suite's
// required capabilities using metadata first and inference last. It is the
// production entry point used by preflight.
func PrepareOllamaModel(ctx context.Context, resolved Resolved, required []string) (PreparedModel, error) {
	if resolved.Provider != "ollama" {
		return PreparedModel{}, fmt.Errorf("prepare local model: provider %q is not ollama", resolved.Provider)
	}
	if resolved.DiscoveryEndpoint == "" || resolved.Endpoint == "" {
		return PreparedModel{}, fmt.Errorf("prepare local model: ollama configuration is missing its discovery or inference endpoint")
	}
	client := OllamaClient{BaseURL: resolved.DiscoveryEndpoint}
	newProvider := func() (agent.Provider, error) {
		return agent.ResolveProviderWithConfig(resolved.Provider, agent.OpenAICompatibleConfig{
			BaseURL: resolved.Endpoint,
		})
	}
	return prepareOllamaModel(ctx, client, newProvider, resolved.Model, required)
}

// prepareOllamaModel is the injectable core: tests script the client and the
// provider factory, production wires the real endpoints.
func prepareOllamaModel(ctx context.Context, client OllamaClient, newProvider func() (agent.Provider, error), name string, required []string) (PreparedModel, error) {
	installed, err := client.FindInstalled(ctx, name)
	if err != nil {
		return PreparedModel{}, err
	}
	prepared := PreparedModel{
		Name:          installed.Name,
		Digest:        installed.Digest,
		ParameterSize: installed.ParameterSize,
		Quantization:  installed.Quantization,
	}
	if len(required) == 0 {
		return prepared, nil
	}

	details, err := client.Show(ctx, name)
	if err != nil {
		return PreparedModel{}, fmt.Errorf("prepare local model: %w", err)
	}
	if details.ParameterSize != "" {
		prepared.ParameterSize = details.ParameterSize
	}
	if details.Quantization != "" {
		prepared.Quantization = details.Quantization
	}

	if details.CapabilitiesKnown {
		if hasAllCapabilities(details.Capabilities, required) {
			prepared.CapabilityCheck = CapabilityCheckOllamaShow
			return prepared, nil
		}
		for _, capability := range required {
			if !hasAllCapabilities(details.Capabilities, []string{capability}) {
				return PreparedModel{}, fmt.Errorf("ollama model %q does not declare required capability %q", name, capability)
			}
		}
	}

	// Capabilities metadata is absent or ambiguous: one bounded behavioral
	// probe through the same provider used for the evaluation decides.
	if newProvider == nil {
		return PreparedModel{}, fmt.Errorf("prepare local model: no provider factory available for the capability probe")
	}
	provider, err := newProvider()
	if err != nil {
		return PreparedModel{}, fmt.Errorf("prepare local model: %w", err)
	}
	result, err := ProbeToolCalling(ctx, provider, name)
	if err != nil {
		return PreparedModel{}, fmt.Errorf("ollama model %q does not support tool calling: %w", name, err)
	}
	prepared.CapabilityCheck = result.Method
	usage := result.Usage
	prepared.ProbeUsage = &usage
	return prepared, nil
}
