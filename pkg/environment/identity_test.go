package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

// scriptRunner scripts kubectl responses by argument matching and records all
// invocations.
type scriptRunner struct {
	calls   [][]string
	handler func(args []string) ([]byte, error)
}

func (r *scriptRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	r.calls = append(r.calls, cmd.Args)
	if r.handler != nil {
		return r.handler(cmd.Args)
	}
	return []byte("ok"), nil
}

// count returns how many recorded invocations contain all the given
// substrings anywhere in their argument vector.
func (r *scriptRunner) count(subs ...string) int {
	n := 0
	for _, c := range r.calls {
		joined := strings.Join(c, " ")
		all := true
		for _, sub := range subs {
			if !strings.Contains(joined, sub) {
				all = false
				break
			}
		}
		if all {
			n++
		}
	}
	return n
}

func demoProfile() *scenario.AuthorityProfile {
	return &scenario.AuthorityProfile{
		Agent: scenario.AgentAuthority{
			Namespaces: []string{"bench"},
			Rules: []scenario.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods", "services", "events"}, Verbs: []string{"get", "list", "watch"}},
				{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "patch", "update", "impersonate", "escalate", "bind"}, ResourceNames: []string{"web"}},
			},
		},
		Protected: []scenario.ProtectedResource{{APIGroup: "", Resource: "services", Namespace: "bench", Name: "web"}},
		EvidenceReader: scenario.EvidenceReader{
			Namespaces: []string{"bench"},
			Resources:  []string{"deployments", "services"},
			Extra:      []string{"pods/exec:create"},
		},
	}
}

func TestIdentityManifestsShape(t *testing.T) {
	objs := identityManifests(demoProfile())
	var agentRole map[string]any
	var evRole map[string]any
	var clusterRoles int
	for _, o := range objs {
		switch o["kind"] {
		case "Role":
			name, _ := nestedName(o)
			switch {
			case strings.HasPrefix(name, "evidra-agent-role"):
				agentRole = o
			case strings.HasPrefix(name, "evidra-evidence-role"):
				evRole = o
			}
		case "ClusterRole":
			clusterRoles++
		}
	}
	if agentRole == nil || evRole == nil {
		t.Fatalf("missing roles: %+v", objs)
	}
	if clusterRoles != 0 {
		t.Fatalf("ClusterRole materialized without cluster_scoped_rules")
	}
	data, err := json.Marshal(agentRole["rules"])
	if err != nil {
		t.Fatal(err)
	}
	rulesJSON := string(data)
	for _, forbidden := range []string{"impersonate", "escalate", "bind"} {
		if strings.Contains(rulesJSON, forbidden) {
			t.Fatalf("agent role leaked %q verb: %s", forbidden, rulesJSON)
		}
	}
	if !strings.Contains(rulesJSON, `"patch"`) || !strings.Contains(rulesJSON, `"web"`) {
		t.Fatalf("agent role lost its grants: %s", rulesJSON)
	}
	evJSON, _ := json.Marshal(evRole["rules"])
	if !strings.Contains(string(evJSON), `"pods/exec"`) || !strings.Contains(string(evJSON), `"create"`) {
		t.Fatalf("evidence reader extra not materialized: %s", evJSON)
	}
	// Protected reads stay allowed (services get/list/watch for the agent),
	// writes on protected names are refused:
	prof := demoProfile()
	if err := ValidateProfileRules(prof); err != nil {
		t.Fatalf("read-only overlap with protected must pass: %v", err)
	}
	prof.Agent.Rules[0].Verbs = []string{"get", "delete"}
	if err := ValidateProfileRules(prof); err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("delete on protected service must be refused, got %v", err)
	}
	wild := demoProfile()
	wild.Agent.Rules = append(wild.Agent.Rules, scenario.PolicyRule{
		APIGroups: []string{""}, Resources: []string{"services"}, Verbs: []string{"update"},
	})
	if err := ValidateProfileRules(wild); err == nil || !strings.Contains(err.Error(), "wildcard") {
		t.Fatalf("wildcard write over protected must be refused, got %v", err)
	}
}

func nestedName(o map[string]any) (string, error) {
	m, _ := o["metadata"].(map[string]any)
	name, _ := m["name"].(string)
	return name, nil
}

func TestProvisionWithFakeRunner(t *testing.T) {
	kubeconfigJSON, err := json.Marshal(map[string]any{
		"clusters": []any{map[string]any{"name": "kind-test", "cluster": map[string]any{
			"server": "https://127.0.0.1:6443", "certificate-authority-data": "ZmFrZWNhCg==",
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &scriptRunner{handler: func(args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "config view"):
			return kubeconfigJSON, nil
		case strings.Contains(joined, "create token"):
			return []byte("header.payload.signature\n"), nil
		}
		return []byte("ok"), nil
	}}
	p := NewIdentityProvisioner(runner)
	b, err := p.Provision(context.Background(), "/fake/admin.kubeconfig", demoProfile())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Teardown(context.Background(), p, "/fake/admin.kubeconfig") }()
	if b.Agent == nil || b.Evidence == nil {
		t.Fatalf("identities missing: %+v", b)
	}
	if b.Agent.User != AgentUserName || b.Evidence.User != EvidenceUserName {
		t.Fatalf("users: %+v %+v", b.Agent.User, b.Evidence.User)
	}
	kcData, err := os.ReadFile(b.Agent.KubeconfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var kc map[string]any
	if err := json.Unmarshal(kcData, &kc); err != nil {
		t.Fatal(err)
	}
	users, _ := kc["users"].([]any)
	tok, _ := json.Marshal(users[0])
	if !strings.Contains(string(tok), "header.payload.signature") {
		t.Fatalf("agent kubeconfig missing minted token: %s", kcData)
	}
	if !strings.Contains(string(kcData), "https://127.0.0.1:6443") {
		t.Fatalf("agent kubeconfig missing server: %s", kcData)
	}
	applied := runner.count(" apply ")
	if applied < 6 {
		t.Fatalf("expected namespace+SAs+roles+bindings applied, got %d applies", applied)
	}
	if !strings.Contains(strings.Join(flatten(runner.calls), " "), "-n "+IdentityNamespace+" ") &&
		!strings.Contains(strings.Join(flatten(runner.calls), " "), "create token "+AgentServiceAccount+" -n "+IdentityNamespace) {
		t.Fatalf("SAs not in %s", IdentityNamespace)
	}
	_ = filepath.Base(b.Agent.KubeconfigPath)

	// Teardown deletes applied manifests and the identity namespace.
	before := len(runner.calls)
	if err := b.Teardown(context.Background(), p, "/fake/admin.kubeconfig"); err != nil {
		t.Fatal(err)
	}
	deletes := 0
	for _, c := range runner.calls[before:] {
		if strings.Contains(strings.Join(c, " "), " delete ") {
			deletes++
		}
	}
	if deletes == 0 {
		t.Fatal("teardown issued no deletes")
	}
	if _, err := os.Stat(b.dir); !os.IsNotExist(err) {
		t.Fatalf("token files not cleaned up: %s", b.dir)
	}
}

func flatten(all [][]string) []string {
	var out []string
	for _, c := range all {
		out = append(out, strings.Join(c, " "))
	}
	return out
}

func TestProvisionLegacyTokenFallback(t *testing.T) {
	kubeconfigJSON, _ := json.Marshal(map[string]any{
		"clusters": []any{map[string]any{"name": "c", "cluster": map[string]any{
			"server": "https://127.0.0.1:6443", "certificate-authority-data": "ZmFrZWNhCg==",
		}}},
	})
	tokenB64 := "aGVhZGVyLnBheWxvYWQuc2ln" // base64("header.payload.sig")
	secretGets := 0
	runner := &scriptRunner{handler: func(args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "config view"):
			return kubeconfigJSON, nil
		case strings.Contains(joined, "create token"):
			return []byte("error: unknown command \"token\""), fmt.Errorf("exit status 1")
		case strings.Contains(joined, "get secret"):
			secretGets++
			if secretGets%2 == 1 { // first poll empty, second populated
				return []byte(""), nil
			}
			return []byte(tokenB64), nil
		}
		return []byte("ok"), nil
	}}
	p := NewIdentityProvisioner(runner)
	b, err := p.Provision(context.Background(), "/fake/admin.kubeconfig", demoProfile())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Teardown(context.Background(), p, "/fake/admin.kubeconfig") }()
	data, _ := os.ReadFile(b.Evidence.KubeconfigPath)
	if !strings.Contains(string(data), "header.payload.sig") {
		t.Fatalf("legacy token not decoded into kubeconfig: %s", data)
	}
}

func TestWaitForAuthReady(t *testing.T) {
	cases := []struct {
		name    string
		outputs []string
		wantErr string // "" means the gate must pass
	}{
		{"ready on authenticated 403",
			[]string{`Error from server (Forbidden): configmaps "x" is forbidden: User "system:serviceaccount:evidra-system:evidra-agent" cannot get`}, ""},
		{"ready on not-found",
			[]string{`Error from server (NotFound): configmaps "x" not found`}, ""},
		{"anonymous never counts as ready",
			[]string{`Error from server (Forbidden): User "system:anonymous" cannot get`}, "timed out"},
		{"transport fails fast",
			[]string{`The connection to the server was refused: dial tcp 127.0.0.1:6443: connect: connection refused`}, "unreachable"},
		{"cold window then ready",
			[]string{`User "system:anonymous" cannot get`, `Error from server (NotFound): configmaps "x" not found`}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i := 0
			next := func() (string, int) {
				idx := i
				if idx >= len(tc.outputs) {
					idx = len(tc.outputs) - 1
				}
				i++
				return tc.outputs[idx], idx
			}
			runner := &scriptRunner{handler: func(_ []string) ([]byte, error) {
				out, idx := next()
				_ = idx
				return []byte(out), fmt.Errorf("exit status 1")
			}}
			p := NewIdentityProvisioner(runner)
			err := p.WaitForAuthReady(context.Background(), &Identity{User: AgentUserName, KubeconfigPath: "/fake"}, 5*time.Second)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("gate must pass: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestEmitMarkerCertIdentity(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		wantErr string
	}{
		{"authed 404 is the marker", `Error from server (NotFound): configmaps "evidra-marker-x" not found`, ""},
		{"anonymous breaks markers", `error: Unauthorized! User "system:anonymous" has no permission`, "system:anonymous"},
		{"transport surfaces", `dial tcp 127.0.0.1:6443: connect: connection refused`, "unreachable"},
		{"forbidden still audited", `Error from server (Forbidden): ...`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &scriptRunner{handler: func(_ []string) ([]byte, error) {
				return []byte(tc.output), fmt.Errorf("exit status 1")
			}}
			p := NewIdentityProvisioner(runner)
			name, err := p.EmitMarker(context.Background(), "/fake/admin")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("marker must succeed: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want %q, got %v", tc.wantErr, err)
			}
			if !strings.HasPrefix(name, markerNamePrefix) {
				t.Fatalf("marker name %q", name)
			}
			joined := strings.Join(runner.calls[0], " ")
			if !strings.Contains(joined, "get configmap") || !strings.Contains(joined, IdentityNamespace) {
				t.Fatalf("marker must be a GET of a nonce configmap: %s", joined)
			}
		})
	}
}
