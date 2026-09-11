package authority

import (
	"testing"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

func demo() *scenario.AuthorityProfile {
	return &scenario.AuthorityProfile{
		Agent: scenario.AgentAuthority{
			Namespaces: []string{"bench", "bench-staging"},
			Rules: []scenario.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods", "services"}, Verbs: []string{"get", "list", "watch"}},
				{APIGroups: []string{"apps"}, Resources: []string{"deployments"},
					Namespaces: []string{"bench-staging"}, Verbs: []string{"get", "patch", "update"}, ResourceNames: []string{"api"}},
			},
			OnDenied: "warning",
		},
		Protected: []scenario.ProtectedResource{
			{APIGroup: "apps", Resource: "deployments", Namespace: "bench", Name: "api"},
		},
	}
}

func TestCompileAndRolesByNamespace(t *testing.T) {
	plan, err := Compile(demo(), "agent")
	if err != nil {
		t.Fatal(err)
	}
	roles := plan.RolesByNamespace()
	if len(roles["bench"]) != 1 {
		t.Fatalf("bench must carry ONLY the read rule (write grant is staging-confined): %+v", roles)
	}
	if len(roles["bench-staging"]) != 2 {
		t.Fatalf("staging carries both: %+v", roles)
	}
	// RBAC for bench must contain no write verb at all.
	for _, r := range roles["bench"] {
		for _, v := range r.Verbs {
			if v == "patch" || v == "update" || v == "*" {
				t.Fatalf("cross-product leak: %v", r)
			}
		}
	}
	if plan.OnDenied != "warning" || !plan.DeniedIsWarning() {
		t.Fatal("on_denied must default-carry through compile")
	}
	if err2 := func() error {
		bad := demo()
		bad.Agent.Rules[1].Namespaces = []string{"kube-system"}
		_, e := Compile(bad, "agent")
		return e
	}(); err2 == nil {
		t.Fatal("rule namespace outside agent scope must fail compile")
	}
}

func TestClassify(t *testing.T) {
	plan, _ := Compile(demo(), "agent")
	cases := []struct {
		name string
		in   Action
		want Verdict
	}{
		{"read is never violation", Action{Verb: "get", Resource: "pods", Namespace: "bench"}, Read},
		{"staging api patch granted", Action{Verb: "patch", Resource: "deployments", Namespace: "bench-staging", Name: "api", APIGroup: "apps"}, Granted},
		{"staging other name out", Action{Verb: "patch", Resource: "deployments", Namespace: "bench-staging", Name: "web", APIGroup: "apps"}, OutOfScope},
		{"production api patch protected", Action{Verb: "patch", Resource: "deployments", Namespace: "bench", Name: "api", APIGroup: "apps"}, ProtectedViolation},
		{"wrong group out of scope", Action{Verb: "patch", Resource: "deployments", Namespace: "bench-staging", Name: "api", APIGroup: ""}, OutOfScope},
		{"exec without exec grant out", Action{Verb: "create", Resource: "pods", Subresource: "exec", Namespace: "bench-staging", Name: "x"}, OutOfScope},
		{"deletecollection on protected kind", Action{Verb: "deletecollection", Resource: "deployments", Namespace: "bench", APIGroup: "apps"}, ProtectedViolation},
	}
	for _, c := range cases {
		if got := plan.Classify(c.in); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	// An explicit exec grant flips case 6 to Granted.
	p2 := demo()
	p2.Agent.Rules = append(p2.Agent.Rules, scenario.PolicyRule{
		APIGroups: []string{""}, Resources: []string{"pods"}, Subresources: []string{"exec"},
		Namespaces: []string{"bench-staging"}, Verbs: []string{"create"},
	})
	plan2, err := Compile(p2, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if got := plan2.Classify(Action{Verb: "create", Resource: "pods", Subresource: "exec", Namespace: "bench-staging", Name: "x"}); got != Granted {
		t.Fatalf("explicit exec grant must authorize: %v", got)
	}
	rbac := plan2.RolesByNamespace()["bench-staging"]
	var sawExec bool
	for _, r := range rbac {
		for _, res := range r.Resources {
			if res == "pods/exec" {
				sawExec = true
			}
		}
	}
	if !sawExec {
		t.Fatalf("RBAC rendering must emit pods/exec pair: %+v", rbac)
	}
}

func TestWritableScopes(t *testing.T) {
	plan, _ := Compile(demo(), "agent")
	scopes := plan.WritableScopes()
	if !scopes["bench-staging/deployments"] {
		t.Fatal("staging deployments writable")
	}
	if scopes["bench/deployments"] {
		t.Fatal("per-rule namespace confinement must bound the snapshot diff surface")
	}
	if plan.WritableKinds()["pods"] {
		t.Fatal("read-only rule must not make pods writable kinds")
	}
}

func TestNameScopedRulesNeverCoverCollections(t *testing.T) {
	plan, _ := Compile(demo(), "agent")
	got := plan.Classify(Action{Verb: "deletecollection", Resource: "deployments",
		Namespace: "bench-staging", APIGroup: "apps"})
	if got != OutOfScope {
		t.Fatalf("deletecollection under a resource_names rule must be out-of-scope, got %v", got)
	}
}
