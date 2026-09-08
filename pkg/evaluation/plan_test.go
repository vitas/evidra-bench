package evaluation

import (
	"strings"
	"testing"
	"time"
)

func TestPlanValidateRequiresRunnableCases(t *testing.T) {
	plan := Plan{
		Suite:       SuitePlan{ID: "kubernetes-demo@1", Digest: "sha256:abc"},
		Environment: EnvironmentPlan{Provider: "kind", Profile: "default"},
		Target:      TargetPlan{Kind: TargetModel, Provider: "openai", Model: "gpt-test"},
		Limits:      Limits{CaseTimeout: time.Minute},
		Attempts:    1,
	}

	err := plan.Validate()
	if err == nil || !strings.Contains(err.Error(), "case") {
		t.Fatalf("Validate() error = %v, want missing case error", err)
	}
}

func TestPlanFingerprintIsStableAndContainsNoCredentials(t *testing.T) {
	plan := Plan{
		Suite: SuitePlan{
			ID:     "kubernetes-demo@1",
			Digest: "sha256:abc",
			Cases:  []CasePlan{{ID: "broken-deployment"}},
		},
		Environment: EnvironmentPlan{Provider: "kind", Profile: "default"},
		Target: TargetPlan{
			Kind:             TargetModel,
			Provider:         "openai",
			Model:            "gpt-test",
			EndpointClass:    "official",
			CredentialSource: "OPENAI_API_KEY",
		},
		Limits:   Limits{CaseTimeout: time.Minute, MaxTurns: 25, MaxTokens: 4096},
		Attempts: 1,
		Output:   OutputPolicy{Directory: "results", Formats: []OutputFormat{OutputJSON, OutputHTML}},
	}

	first, err := plan.Fingerprint()
	if err != nil {
		t.Fatalf("Fingerprint() error = %v", err)
	}
	second, err := plan.Fingerprint()
	if err != nil {
		t.Fatalf("Fingerprint() second error = %v", err)
	}
	if first != second {
		t.Fatalf("Fingerprint() unstable: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "sha256:") {
		t.Fatalf("Fingerprint() = %q, want sha256 prefix", first)
	}

	encoded, err := plan.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	for _, forbidden := range []string{`"api_key":`, `"token":`, `"password":`, `"secret":`} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("serialized plan contains forbidden credential field %q: %s", forbidden, encoded)
		}
	}
}
