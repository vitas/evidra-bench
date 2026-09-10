package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScenario(t *testing.T, body string) *Scenario {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

const authorityBase = `id: auth-test
title: authority profile fixture
category: kubernetes
prompt: do the thing
break:
  type: kubectl
  args: [scale, deployment/web, --replicas=0, -n, bench]
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`

func TestAuthorityProfileValid(t *testing.T) {
	s := writeScenario(t, authorityBase+`authority_profile:
  agent:
    namespaces: [bench]
    rules:
      - apiGroups: [apps]
        resources: [deployments]
        verbs: [get, list, watch, patch, update]
        resource_names: [web]
      - apiGroups: [""]
        resources: [pods, services]
        verbs: [get, list, watch]
    cluster_scoped_rules:
      - apiGroups: [""]
        resources: [namespaces]
        verbs: [get, list]
    on_denied: warning
  protected:
    - {apiGroup: "", resource: services, name: web, namespace: bench}
  evidence_reader:
    namespaces: [bench]
    resources: [deployments, pods]
    extra: ["pods/exec:create"]
`)
	if s.AuthorityProfile == nil {
		t.Fatal("authority_profile did not parse")
	}
	p := s.AuthorityProfile
	if len(p.Agent.Rules) != 2 || len(p.Agent.ClusterScopedRules) != 1 {
		t.Fatalf("rule sets not parsed: %+v", p.Agent)
	}
	if p.Agent.OnDenied != OnDeniedWarning {
		t.Fatalf("on_denied = %q, want %q", p.Agent.OnDenied, OnDeniedWarning)
	}
	if p.Agent.effectiveOnDenied() != OnDeniedWarning {
		t.Fatal("effectiveOnDenied wrong for explicit value")
	}
}

func TestAuthorityProfileAbsentIsLoadable(t *testing.T) {
	s := writeScenario(t, authorityBase)
	if s.AuthorityProfile != nil {
		t.Fatal("absent profile must stay nil")
	}
}

func TestAuthorityProfileDefaultOnDenied(t *testing.T) {
	s := writeScenario(t, authorityBase+`authority_profile:
  agent:
    rules:
      - apiGroups: [apps]
        resources: [deployments]
        verbs: [get]
`)
	if got := s.AuthorityProfile.Agent.effectiveOnDenied(); got != OnDeniedUnsafe {
		t.Fatalf("default on_denied = %q, want %q", got, OnDeniedUnsafe)
	}
}

func TestAuthorityProfileRejectsNonCanonicalVerbs(t *testing.T) {
	// The full set of strings that have shown up in reviews as fake verbs:
	// describe first — it is what kubectl does as get/list, never a verb.
	for _, verb := range []string{"describe", "Describe", "log", "exec", "read", "write", "restart", "scale"} {
		dir := t.TempDir()
		body := authorityBase + `authority_profile:
  agent:
    rules:
      - apiGroups: [""]
        resources: [pods]
        verbs: [` + verb + `]
`
		if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(dir)
		if err == nil {
			t.Fatalf("verb %q must be rejected", verb)
		}
		if !strings.Contains(err.Error(), "not a canonical Kubernetes RBAC verb") {
			t.Fatalf("verb %q: unexpected error %v", verb, err)
		}
		if verb == "describe" && !strings.Contains(err.Error(), "get/list") {
			t.Fatalf("describe rejection must explain get/list, got %v", err)
		}
	}
}

func TestAuthorityProfileAcceptsCanonicalVerbUniverse(t *testing.T) {
	verbs := `"*", get, list, watch, create, update, patch, delete, deletecollection, bind, escalate, impersonate, use`
	dir := t.TempDir()
	body := authorityBase + "authority_profile:\n  agent:\n    rules:\n      - apiGroups: [\"\"]\n        resources: [pods]\n        verbs: [" + verbs + "]\n"
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err != nil {
		t.Fatalf("canonical verbs must all load: %v", err)
	}
}

func TestAuthorityProfileValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"bad on_denied", `authority_profile:
  agent:
    on_denied: ignore
`, "on_denied"},
		{"rule without verbs", `authority_profile:
  agent:
    rules:
      - apiGroups: [apps]
        resources: [deployments]
`, "verbs is required"},
		{"protected missing name", `authority_profile:
  protected:
    - {apiGroup: apps, resource: deployments}
`, "must set resource and name"},
		{"extra bad verb", `authority_profile:
  evidence_reader:
    extra: ["pods/exec:describe"]
`, "not a canonical RBAC verb"},
		{"impersonation without rule", `authority_profile:
  allow_impersonation: true
  agent:
    rules:
      - apiGroups: [apps]
        resources: [deployments]
        verbs: [get]
`, "no rule grants the impersonate verb"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(authorityBase+c.yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(dir)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: want error containing %q, got %v", c.name, c.want, err)
		}
	}
}

func TestImpersonationWithExplicitRuleLoads(t *testing.T) {
	s := writeScenario(t, authorityBase+`authority_profile:
  allow_impersonation: true
  agent:
    rules:
      - apiGroups: [""]
        resources: [serviceaccounts]
        verbs: [impersonate]
`)
	if !s.AuthorityProfile.AllowImpersonate {
		t.Fatal("allow_impersonation not parsed")
	}
}

func TestAssertV2CheckParsesAtSchemaLevel(t *testing.T) {
	s := writeScenario(t, authorityBase2AssertV2)
	if len(s.Checks) != 1 || s.Checks[0].Type != CheckTypeAssertV2 {
		t.Fatalf("assert-v2 check not parsed: %+v", s.Checks)
	}
	if s.Checks[0].Name != "web-image-repaired" || s.Checks[0].Condition != "fixtures/assert_web.sh" {
		t.Fatalf("assert-v2 fields not carried: %+v", s.Checks[0])
	}
}

const authorityBase2AssertV2 = `id: assertv2-test
title: assert-v2 parse fixture
category: kubernetes
prompt: do the thing
break:
  type: kubectl
  args: [scale, deployment/web, --replicas=0, -n, bench]
checks:
  - type: assert-v2
    name: web-image-repaired
    condition: fixtures/assert_web.sh
`

func TestDemoScenariosCarryMigrationProfiles(t *testing.T) {
	for _, id := range []string{"broken-deployment", "false-alarm", "wrong-namespace-workload-restart"} {
		s, err := Load(filepath.Join("..", "..", "scenarios", "kubernetes", id))
		if err != nil {
			t.Fatalf("%s: load: %v", id, err)
		}
		if s.AuthorityProfile == nil {
			t.Fatalf("%s: migration authority_profile missing", id)
		}
		if got := s.AuthorityProfile.Agent.effectiveOnDenied(); got != OnDeniedUnsafe {
			t.Fatalf("%s: on_denied = %q, want unsafe", id, got)
		}
		if len(s.AuthorityProfile.Agent.Rules) == 0 {
			t.Fatalf("%s: agent rules empty", id)
		}
	}
}
