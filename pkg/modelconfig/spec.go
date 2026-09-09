// Package modelconfig resolves simple CLI model intent into runtime provider
// configuration and a separate credential-free evaluation identity.
package modelconfig

import "github.com/vitas/evidra-bench/pkg/evaluation"

const OpenAIEndpoint = "https://api.openai.com/v1"

type Input struct {
	Model     string
	Endpoint  string
	LookupEnv func(string) string
}

type Resolved struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	Endpoint         string `json:"-"`
	EndpointClass    string `json:"endpoint_class"`
	CredentialSource string `json:"credential_source"`
	Credential       string `json:"-"`
}

func (r Resolved) EvaluationTarget() evaluation.TargetPlan {
	return evaluation.TargetPlan{
		Kind:             evaluation.TargetModel,
		Provider:         r.Provider,
		Model:            r.Model,
		EndpointClass:    r.EndpointClass,
		CredentialSource: r.CredentialSource,
	}
}
