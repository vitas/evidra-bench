package modelconfig

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func Resolve(input Input) (Resolved, error) {
	modelSpec := strings.TrimSpace(input.Model)
	if modelSpec == "" {
		return Resolved{}, fmt.Errorf("model configuration: --model is required in non-interactive mode")
	}
	lookup := input.LookupEnv
	if lookup == nil {
		lookup = os.Getenv
	}

	if endpoint := strings.TrimSpace(input.Endpoint); endpoint != "" {
		if err := validateEndpoint(endpoint); err != nil {
			return Resolved{}, err
		}
		model := modelSpec
		if _, suffix, ok := strings.Cut(modelSpec, "/"); ok {
			model = suffix
		}
		if strings.TrimSpace(model) == "" {
			return Resolved{}, fmt.Errorf("model configuration: model name is required")
		}
		credential := strings.TrimSpace(lookup("OPENAI_API_KEY"))
		source := "none"
		if credential != "" {
			source = "OPENAI_API_KEY"
		}
		return Resolved{
			Provider:         "openai-compatible",
			Model:            model,
			Endpoint:         strings.TrimRight(endpoint, "/"),
			EndpointClass:    "custom",
			CredentialSource: source,
			Credential:       credential,
		}, nil
	}

	provider, model, ok := strings.Cut(modelSpec, "/")
	if !ok || strings.TrimSpace(model) == "" {
		return Resolved{}, fmt.Errorf("model configuration: use provider/model or supply --endpoint")
	}
	switch provider {
	case "openai":
		return resolveOfficial(provider, model, OpenAIEndpoint, "OPENAI_API_KEY", lookup)
	case "anthropic":
		return resolveOfficial(provider, model, "", "ANTHROPIC_API_KEY", lookup)
	default:
		return Resolved{}, fmt.Errorf("model configuration: unsupported model provider %q", provider)
	}
}

func resolveOfficial(provider, model, endpoint, credentialSource string, lookup func(string) string) (Resolved, error) {
	credential := strings.TrimSpace(lookup(credentialSource))
	if credential == "" {
		return Resolved{}, fmt.Errorf("model configuration: %s is required for %s", credentialSource, provider)
	}
	return Resolved{
		Provider:         provider,
		Model:            model,
		Endpoint:         endpoint,
		EndpointClass:    "official",
		CredentialSource: credentialSource,
		Credential:       credential,
	}, nil
}

func validateEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("model configuration: endpoint must be an absolute HTTP(S) URL")
	}
	return nil
}
