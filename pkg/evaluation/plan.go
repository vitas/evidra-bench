// Package evaluation defines the application-level contract for executing and
// reporting a suite of infrastructure-agent scenarios.
package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const PlanVersion = "evaluation-plan.v1"

type TargetKind string

const (
	TargetModel TargetKind = "model"
	TargetAgent TargetKind = "agent"
)

type OutputFormat string

const (
	OutputTerminal OutputFormat = "terminal"
	OutputHTML     OutputFormat = "html"
	OutputJSON     OutputFormat = "json"
	OutputJUnit    OutputFormat = "junit"
)

// Plan is the fully resolved, credential-free input to an evaluation.
type Plan struct {
	Version     string          `json:"version"`
	Suite       SuitePlan       `json:"suite"`
	Environment EnvironmentPlan `json:"environment"`
	Target      TargetPlan      `json:"target"`
	Limits      Limits          `json:"limits"`
	Attempts    int             `json:"attempts"`
	Output      OutputPolicy    `json:"output"`
}

type SuitePlan struct {
	ID     string     `json:"id"`
	Digest string     `json:"digest"`
	Cases  []CasePlan `json:"cases"`
	// RequiredModelCapabilities lists model capabilities the suite declares
	// (see suite.SupportedModelCapabilities). Preflight enforces them.
	RequiredModelCapabilities []string `json:"required_model_capabilities,omitempty"`
}

type CasePlan struct {
	ID string `json:"id"`
}

type EnvironmentPlan struct {
	Provider string `json:"provider"`
	Profile  string `json:"profile"`
}

type TargetPlan struct {
	Kind     TargetKind `json:"kind"`
	Provider string     `json:"provider,omitempty"`
	Model    string     `json:"model,omitempty"`
	Adapter  string     `json:"adapter,omitempty"`
	CommandIdentity  string     `json:"command_identity,omitempty"`
	EndpointClass    string     `json:"endpoint_class,omitempty"`
	CredentialSource string     `json:"credential_source,omitempty"`
	// Local model identity captured by discovery preflight. All optional and
	// credential-free; they participate in the tested-configuration
	// fingerprint because different weights or quantizations of the same tag
	// are different configurations.
	ModelDigest     string `json:"model_digest,omitempty"`
	ParameterSize   string `json:"parameter_size,omitempty"`
	Quantization    string `json:"quantization,omitempty"`
	CapabilityCheck string `json:"capability_check,omitempty"`
}

type Limits struct {
	CaseTimeout time.Duration `json:"case_timeout_ns"`
	MaxTurns    int           `json:"max_turns,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type OutputPolicy struct {
	Directory string         `json:"directory"`
	Formats   []OutputFormat `json:"formats"`
}

func (p Plan) Validate() error {
	if strings.TrimSpace(p.Suite.ID) == "" {
		return fmt.Errorf("evaluation plan: suite id is required")
	}
	if strings.TrimSpace(p.Suite.Digest) == "" {
		return fmt.Errorf("evaluation plan: suite digest is required")
	}
	if len(p.Suite.Cases) == 0 {
		return fmt.Errorf("evaluation plan: at least one case is required")
	}
	seen := make(map[string]struct{}, len(p.Suite.Cases))
	for i, c := range p.Suite.Cases {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			return fmt.Errorf("evaluation plan: case %d id is required", i)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("evaluation plan: duplicate case %q", id)
		}
		seen[id] = struct{}{}
	}
	if strings.TrimSpace(p.Environment.Provider) == "" {
		return fmt.Errorf("evaluation plan: environment provider is required")
	}
	if p.Target.Kind != TargetModel && p.Target.Kind != TargetAgent {
		return fmt.Errorf("evaluation plan: target kind must be %q or %q", TargetModel, TargetAgent)
	}
	if p.Target.Kind == TargetModel && (strings.TrimSpace(p.Target.Provider) == "" || strings.TrimSpace(p.Target.Model) == "") {
		return fmt.Errorf("evaluation plan: model target requires provider and model")
	}
	if p.Target.Kind == TargetAgent && strings.TrimSpace(p.Target.Adapter) == "" {
		return fmt.Errorf("evaluation plan: agent target requires adapter")
	}
	if p.Limits.CaseTimeout <= 0 {
		return fmt.Errorf("evaluation plan: positive case timeout is required")
	}
	if p.Attempts != 1 {
		return fmt.Errorf("evaluation plan: attempts must be 1 for the initial runner")
	}
	return nil
}

func (p Plan) MarshalJSON() ([]byte, error) {
	type planAlias Plan
	if p.Version == "" {
		p.Version = PlanVersion
	}
	return json.Marshal(planAlias(p))
}

// Fingerprint identifies execution-affecting configuration. Output paths and
// formats are deliberately excluded because rerendering is not a new test.
func (p Plan) Fingerprint() (string, error) {
	identity := struct {
		Version     string          `json:"version"`
		Suite       SuitePlan       `json:"suite"`
		Environment EnvironmentPlan `json:"environment"`
		Target      TargetPlan      `json:"target"`
		Limits      Limits          `json:"limits"`
		Attempts    int             `json:"attempts"`
	}{
		Version:     PlanVersion,
		Suite:       p.Suite,
		Environment: p.Environment,
		Target:      p.Target,
		Limits:      p.Limits,
		Attempts:    p.Attempts,
	}
	b, err := json.Marshal(identity)
	if err != nil {
		return "", fmt.Errorf("evaluation plan fingerprint: %w", err)
	}
	digest := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
