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
	if err := h.resetScenarioNamespaces(context.Background(), req, handle, s); err != nil {
		t.Fatal(err)
	}
	if len(rr.argvs) != 0 {
		t.Fatalf("unflagged run must not delete anything: %v", rr.argvs)
	}

	// Flag on (owned disposable test cluster): one waited delete per scope,
	// through the ADMIN lease kubeconfig.
	req.Config.ResetNamespacesBeforeCase = true
	if err := h.resetScenarioNamespaces(context.Background(), req, handle, s); err != nil {
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
	if got := h2.resetScenarioNamespaces(context.Background(), req, handle, s); got == nil {
		t.Fatal("delete failure must error")
	} else {
		var infra *InfraError
		if !errors.As(got, &infra) {
			t.Fatalf("delete failure must wrap as InfraError, got %T: %v", got, got)
		}
	}

	// Out-of-policy scope is also infra (loud), not a silent skip.
	bad := &scenario.Scenario{ID: "y", Scope: scenario.Scope{Namespaces: []string{"kube-system"}}}
	if got := h.resetScenarioNamespaces(context.Background(), req, handle, bad); got == nil {
		t.Fatal("protected scope must error when the flag is on")
	}
}
