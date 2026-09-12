package scenario

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestAuthorityRuleShadowMirrorsSchema pins the strict loader mirror to the
// real structs: a new schema field silently unreachable to
// validateAuthorityRuleKeys would let key typos widen grants again.
func TestAuthorityRuleShadowMirrorsSchema(t *testing.T) {
	yamlKeys := func(rt reflect.Type) map[string]bool {
		out := map[string]bool{}
		for i := 0; i < rt.NumField(); i++ {
			tag := rt.Field(i).Tag.Get("yaml")
			name := strings.Split(tag, ",")[0]
			if name != "" && name != "-" {
				out[name] = true
			}
		}
		return out
	}
	pairs := []struct{ real, mirror reflect.Type }{
		{reflect.TypeOf(PolicyRule{}), reflect.TypeOf(authorityRuleShape{})},
		{reflect.TypeOf(AgentAuthority{}), reflect.TypeOf(authorityAgentShape{})},
		{reflect.TypeOf(ProtectedResource{}), reflect.TypeOf(authorityProtectedShape{})},
		{reflect.TypeOf(EvidenceReader{}), reflect.TypeOf(authorityReaderShape{})},
		{reflect.TypeOf(AuthorityProfile{}), reflect.TypeOf(authorityProfileShape{})},
	}
	for _, pr := range pairs {
		want, got := yamlKeys(pr.real), yamlKeys(pr.mirror)
		for k := range want {
			if !got[k] {
				t.Errorf("%s yaml key %q missing from loader strict mirror", pr.real.Name(), k)
			}
		}
		for k := range got {
			if !want[k] {
				t.Errorf("loader strict mirror has unknown key %q for %s", k, pr.real.Name())
			}
		}
	}
}

func TestStrictAuthorityKeysRejectSilentWidening(t *testing.T) {
	dir := t.TempDir()
	write := func(profile string) {
		t.Helper()
		doc := "id: strict-probe\ntitle: t\ncategory: kubernetes\nprompt: p.md\ntimeout: \"2m\"\n" +
			"bootstrap:\n  - name: b\n    type: sleep\n    duration: \"1s\"\n" +
			"break:\n  type: sleep\n  duration: \"1s\"\n" +
			"checks:\n  - type: deployment-ready\n    namespace: bench\n    name: web\n" +
			"scope:\n  namespaces: [bench]\n" + profile
		if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The exact bug this guard exists for: camelCase resourceNames parses
	// into NOTHING, widening a narrow grant to a wildcard — the identity
	// materializer then refuses the profile (INCOMPLETE) or, worse where
	// protections do not collide, silently grants cluster-wide writes.
	write(`authority_profile:
  agent:
    namespaces: [bench]
    rules:
      - apiGroups: [apps]
        resources: [deployments]
        resourceNames: [web]
        verbs: [patch]
    on_denied: unsafe
  evidence_reader:
    namespaces: [bench]
    resources: [deployments]
`)
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "authority_profile") {
		t.Fatalf("typo'd rule key must be a hard load error, got %v", err)
	}
	// The correct spelling loads.
	write(`authority_profile:
  agent:
    namespaces: [bench]
    rules:
      - apiGroups: [apps]
        resources: [deployments]
        resource_names: [web]
        verbs: [patch]
    on_denied: unsafe
  evidence_reader:
    namespaces: [bench]
    resources: [deployments]
  allow_impersonation: false
`)
	if _, err := Load(dir); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
}
