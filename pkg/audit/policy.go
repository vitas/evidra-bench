package audit

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Policy is the subset of audit.k8s.io/v1 Policy we reason about.
type Policy struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Rules      []struct {
		Level string   `yaml:"level"`
		Verbs []string `yaml:"verbs,omitempty"`
	} `yaml:"rules"`
}

// MutationLevel reports the audit level applied to write verbs (create,
// update, patch, delete) in a provisioned policy. It drives the redaction
// decision and the health report (Metadata means no bodies were captured at
// all; Request/RequestResponse means bodies exist and MUST be stripped
// before persistence).
func MutationLevel(policyYAML string) (string, error) {
	var p Policy
	if err := yaml.Unmarshal([]byte(policyYAML), &p); err != nil {
		return "", fmt.Errorf("audit policy parse: %w", err)
	}
	if p.Kind != "Policy" {
		return "", fmt.Errorf("audit policy: unexpected kind %q", p.Kind)
	}
	for _, r := range p.Rules {
		joined := strings.Join(r.Verbs, ",")
		if strings.Contains(joined, "create") && strings.Contains(joined, "delete") {
			return r.Level, nil
		}
	}
	return "None", nil
}
