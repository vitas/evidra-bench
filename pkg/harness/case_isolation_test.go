package harness

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

type isolationRunner struct {
	argvs []string
	err   error
}

func (r *isolationRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	r.argvs = append(r.argvs, strings.Join(cmd.Args, " "))
	return nil, r.err
}

func TestIsolationNamespacesOnlyAllowsDisposableBenchScopes(t *testing.T) {
	t.Parallel()
	s := &scenario.Scenario{ID: "x", Scope: scenario.Scope{Namespaces: []string{"bench-staging", "bench", "bench"}}}
	got, err := isolationNamespaces(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "bench,bench-staging" {
		t.Fatalf("got %v, want [bench bench-staging] stable order", got)
	}
	for _, bad := range []scenario.Scenario{
		{ID: "d", Scope: scenario.Scope{Namespaces: []string{"default"}}},
		{ID: "k", Scope: scenario.Scope{Namespaces: []string{"kube-system"}}},
		{ID: "p", Scope: scenario.Scope{Namespaces: []string{"kube-public"}}},
		{ID: "n", Scope: scenario.Scope{Namespaces: []string{"kube-node-lease"}}},
		{ID: "e", Scope: scenario.Scope{Namespaces: []string{" "}}},
		{ID: "f", Scope: scenario.Scope{Namespaces: []string{"frontpage"}}},
		{ID: "b", Scope: scenario.Scope{Namespaces: []string{"benching"}}}, // prefix match is NOT enough
	} {
		if _, err := isolationNamespaces(&bad); err == nil {
			t.Fatalf("%s: namespace %v must be refused", bad.ID, bad.Scope.Namespaces)
		}
	}
}

func TestResetDeletesOnlyWhenFlagged(t *testing.T) {
	rr := &isolationRunner{}
	h := &Harness{deps: Deps{Runner: rr}}
	s := &scenario.Scenario{ID: "x", Scope: scenario.Scope{Namespaces: []string{"bench", "bench-staging"}}}
	req := RunRequest{Config: config.Default()}
	handle := &environment.Handle{KubeconfigPath: "/kc"}

	// Flag off (bench/run, external kubeconfig, reused cluster): nothing runs.
	if err := h.resetScenarioNamespaces(context.Background(), req, handle, s, "bench"); err != nil {
		t.Fatal(err)
	}
	if len(rr.argvs) != 0 {
		t.Fatalf("unflagged run must not delete anything: %v", rr.argvs)
	}

	// Flag on (owned disposable test cluster): one waited delete per scope,
	// through the ADMIN lease kubeconfig.
	req.Config.ResetNamespacesBeforeCase = true
	if err := h.resetScenarioNamespaces(context.Background(), req, handle, s, "bench"); err != nil {
		t.Fatal(err)
	}
	if len(rr.argvs) != 2 {
		t.Fatalf("argvs = %v", rr.argvs)
	}
	if !strings.Contains(rr.argvs[0], "--kubeconfig /kc delete namespace bench --ignore-not-found --wait=true --timeout=90s") ||
		!strings.Contains(rr.argvs[1], "delete namespace bench-staging") {
		t.Fatalf("unexpected deletes: %v", rr.argvs)
	}

	// Deletion failure is infra (INCOMPLETE), never a behavioral FAIL.
	rr2 := &isolationRunner{err: exec.ErrNotFound}
	h2 := &Harness{deps: Deps{Runner: rr2}}
	if got := h2.resetScenarioNamespaces(context.Background(), req, handle, s, "bench"); got == nil {
		t.Fatal("delete failure must error")
	} else {
		var infra *InfraError
		if !errors.As(got, &infra) {
			t.Fatalf("delete failure must wrap as InfraError, got %T: %v", got, got)
		}
	}

	// Out-of-policy scope is also infra (loud), not a silent skip.
	bad := &scenario.Scenario{ID: "y", Scope: scenario.Scope{Namespaces: []string{"kube-system"}}}
	if got := h.resetScenarioNamespaces(context.Background(), req, handle, bad, "bench"); got == nil {
		t.Fatal("protected scope must error when the flag is on")
	}
}

// recorder orders every namespace operation the single isolation stage
// performs; the invariant is EXACTLY ONE lifecycle: delete-all ->
// create-target -> canary (run_environment must not add its own pass).
type fakeEnv struct {
	environment.ClusterLifecycle
	seq       []string
	canaryErr error
}

func (f *fakeEnv) CreateNamespace(_ context.Context, _, ns string) error {
	f.seq = append(f.seq, "create:"+ns)
	return nil
}

func (f *fakeEnv) RunCanary(_ context.Context, _, ns string) error {
	f.seq = append(f.seq, "canary:"+ns)
	return f.canaryErr
}

func TestResetIsTheSingleNamespaceLifecycleStage(t *testing.T) {
	rr := &isolationRunner{}
	env := &fakeEnv{}
	h := &Harness{deps: Deps{Runner: rr, EnvProvider: env}}
	s := &scenario.Scenario{ID: "x", Scope: scenario.Scope{Namespaces: []string{"bench", "bench-staging"}}}
	req := RunRequest{Config: config.Default()}
	req.Config.ResetNamespacesBeforeCase = true
	handle := &environment.Handle{KubeconfigPath: "/kc"}

	if err := h.resetScenarioNamespaces(context.Background(), req, handle, s, "bench"); err != nil {
		t.Fatal(err)
	}
	got := append([]string{}, env.seq...)
	if strings.Join(got, ",") != "create:bench,canary:bench" {
		t.Fatalf("suite mode must recreate the target and run the canary exactly once: %v", got)
	}
	if len(rr.argvs) != 2 {
		t.Fatalf("expected one waited delete per declared scope, got %v", rr.argvs)
	}

	// A canary failure after reset is infra (INCOMPLETE), loudly.
	env2 := &fakeEnv{canaryErr: errors.New("no schedulable node")}
	h2 := &Harness{deps: Deps{Runner: &isolationRunner{}, EnvProvider: env2}}
	if got := h2.resetScenarioNamespaces(context.Background(), req, handle, s, "bench"); got == nil {
		t.Fatal("canary failure must error")
	} else {
		var infra *InfraError
		if !errors.As(got, &infra) {
			t.Fatalf("canary failure must wrap as InfraError, got %T", got)
		}
	}
}
