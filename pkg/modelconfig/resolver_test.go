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
