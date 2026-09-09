package modelconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/evaluation"
)

func TestResolveOpenAIUsesStandardCredentialWithoutSerializingSecret(t *testing.T) {
	resolved, err := Resolve(Input{
		Model: "openai/gpt-test",
		LookupEnv: func(name string) string {
			if name == "OPENAI_API_KEY" {
				return "super-secret"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Provider != "openai" || resolved.Model != "gpt-test" || resolved.Endpoint != OpenAIEndpoint {
		t.Fatalf("resolved = %+v", resolved)
	}
	if resolved.Credential != "super-secret" || resolved.CredentialSource != "OPENAI_API_KEY" {
		t.Fatalf("credential resolution = %+v", resolved)
	}
	target := resolved.EvaluationTarget()
	if target != (evaluation.TargetPlan{Kind: evaluation.TargetModel, Provider: "openai", Model: "gpt-test", EndpointClass: "official", CredentialSource: "OPENAI_API_KEY"}) {
		t.Fatalf("target = %+v", target)
	}
	encoded, err := json.Marshal(resolved)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-secret") {
		t.Fatalf("serialized resolved config leaked credential: %s", encoded)
	}
}

func TestResolveCustomOpenAICompatibleEndpointAllowsLocalCredentiallessUse(t *testing.T) {
	resolved, err := Resolve(Input{Model: "my-model", Endpoint: "http://127.0.0.1:8080/v1"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Provider != "openai-compatible" || resolved.EndpointClass != "custom" || resolved.CredentialSource != "none" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveAnthropicUsesExistingProviderPath(t *testing.T) {
	resolved, err := Resolve(Input{
		Model: "anthropic/claude-test",
		LookupEnv: func(name string) string {
			if name == "ANTHROPIC_API_KEY" {
				return "anthropic-secret"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Provider != "anthropic" || resolved.Model != "claude-test" || resolved.CredentialSource != "ANTHROPIC_API_KEY" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveFailsBeforeExecutionWhenRequiredConfigurationIsMissing(t *testing.T) {
	tests := []struct {
		name  string
		input Input
		want  string
	}{
		{name: "missing model", input: Input{}, want: "--model"},
		{name: "unknown provider", input: Input{Model: "mystery/model"}, want: "unsupported model provider"},
		{name: "missing OpenAI key", input: Input{Model: "openai/gpt-test"}, want: "OPENAI_API_KEY"},
		{name: "invalid endpoint", input: Input{Model: "my-model", Endpoint: "://bad"}, want: "endpoint"},
		{name: "empty ollama model", input: Input{Model: "ollama/"}, want: "model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Resolve() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestResolveOllamaUsesSharedOpenAIEndpointWithoutCredential(t *testing.T) {
	lookedUp := []string{}
	resolved, err := Resolve(Input{
		Model: "ollama/qwen3:8b",
		LookupEnv: func(name string) string {
			lookedUp = append(lookedUp, name)
			return "should-never-be-used"
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Provider != "ollama" || resolved.Model != "qwen3:8b" {
		t.Fatalf("resolved = %+v", resolved)
	}
	if resolved.Endpoint != OllamaOpenAIEndpoint || resolved.CredentialSource != "none" {
		t.Fatalf("runtime config = %+v", resolved)
	}
	if resolved.EndpointClass != "local" {
		t.Fatalf("EndpointClass = %q, want local", resolved.EndpointClass)
	}
	if resolved.Credential != "" {
		t.Fatal("ollama must not carry a credential")
	}
	if resolved.DiscoveryEndpoint != OllamaAPIEndpoint {
		t.Fatalf("DiscoveryEndpoint = %q, want %q", resolved.DiscoveryEndpoint, OllamaAPIEndpoint)
	}
	for _, name := range lookedUp {
		if name == "OPENAI_API_KEY" {
			t.Fatal("ollama resolution must not consult OPENAI_API_KEY")
		}
	}
}

func TestResolveOllamaEvaluationTargetIsSecretFree(t *testing.T) {
	resolved, err := Resolve(Input{Model: "ollama/qwen3:8b"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	target := resolved.EvaluationTarget()
	if target.Provider != "ollama" || target.Model != "qwen3:8b" {
		t.Fatalf("target = %+v", target)
	}
	if target.EndpointClass != "local" || target.CredentialSource != "none" {
		t.Fatalf("target classification = %+v", target)
	}
	encoded, err := json.Marshal(resolved)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"127.0.0.1", "11434", "http://"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("serialized resolved config leaks endpoint data %q: %s", forbidden, encoded)
		}
	}
}

func TestResolveExplicitEndpointWinsOverOllamaPrefix(t *testing.T) {
	resolved, err := Resolve(Input{
		Model:     "ollama/qwen3:8b",
		Endpoint:  "http://10.0.0.5:8000/v1",
		LookupEnv: func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Provider != "openai-compatible" || resolved.Model != "qwen3:8b" {
		t.Fatalf("resolved = %+v, want openai-compatible with suffix model", resolved)
	}
	if resolved.EndpointClass != "custom" {
		t.Fatalf("EndpointClass = %q, want custom", resolved.EndpointClass)
	}
	if resolved.DiscoveryEndpoint != "" {
		t.Fatal("custom endpoints get no runtime-specific discovery endpoint")
	}
}
