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

func TestPlanFingerprintIncludesLocalModelIdentityFields(t *testing.T) {
	base := Plan{
		Suite: SuitePlan{
			ID:     "kubernetes-demo@1",
			Digest: "sha256:abc",
			Cases:  []CasePlan{{ID: "broken-deployment"}},
		},
		Environment: EnvironmentPlan{Provider: "kind", Profile: "default"},
		Target: TargetPlan{
			Kind:     TargetModel,
			Provider: "ollama",
			Model:    "qwen3:8b",
		},
		Limits:   Limits{CaseTimeout: time.Minute},
		Attempts: 1,
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	baseFP, err := base.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}

	withIdentity := base
	withIdentity.Target.ModelDigest = "sha256:deadbeef"
	withIdentity.Target.ParameterSize = "8B"
	withIdentity.Target.Quantization = "Q4_K_M"
	withIdentity.Target.CapabilityCheck = "behavioral_tool_call"
	identityFP, err := withIdentity.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if identityFP == baseFP {
		t.Fatal("fingerprint must change when local model identity fields change")
	}

	encoded, err := withIdentity.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"model_digest":"sha256:deadbeef"`, `"parameter_size":"8B"`, `"quantization":"Q4_K_M"`, `"capability_check":"behavioral_tool_call"`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("serialized plan missing %s: %s", want, encoded)
		}
	}

	omitted, err := base.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(omitted), `"model_digest"`) {
		t.Fatalf("identity fields must be omitted when unset: %s", omitted)
	}
}
