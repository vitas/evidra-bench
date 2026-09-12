package harness

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// Case isolation for shared disposable suite clusters (kubernetes-core
// plan, Task 2). One cluster now runs a whole suite; without a reset, a
// resource left behind by case N poisons case N+1. The performance model
// stays one-cluster-per-evaluation; what changes is that before each case
// the ADMIN lease deletes the scenario's declared namespaces — before
// baseline snapshots and before the audit window opens, so evaluator
// cleanup can never be attributed to the tested agent.
//
// The blast radius is bounded by an allowlist: only bench-scoped
// namespaces of the scenario itself may ever be deleted. The flag
// (config.ResetNamespacesBeforeCase) is set exclusively by `evidra test`,
// which always provisions and owns its cluster; legacy bench/run paths,
// external kubeconfigs and reused clusters never get automatic deletes.

// benchScopeRe matches the disposable namespace family the runner owns.
var benchScopeRe = regexp.MustCompile(`^bench(-[a-z0-9]([-a-z0-9]*[a-z0-9])?)?$`)

// forbiddenNamespaces are refused even if a scenario scope names them
// (defense in depth; a malformed scenario must not aim the delete at
// control-plane state).
var forbiddenNamespaces = map[string]bool{
	"default":         true,
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
	"":                true,
}

// isolationNamespaces canonicalizes and de-duplicates the scenario's
// declared scope namespaces into a stable, allowlisted list.
func isolationNamespaces(s *scenario.Scenario) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, raw := range s.Scope.Namespaces {
		name := strings.TrimSpace(raw)
		if name == "" {
			return nil, fmt.Errorf("isolation: scenario %s declares an empty namespace", s.ID)
		}
		if forbiddenNamespaces[name] {
			return nil, fmt.Errorf("isolation: scenario %s names protected namespace %q", s.ID, name)
		}
		if !benchScopeRe.MatchString(name) {
			return nil, fmt.Errorf("isolation: scenario %s namespace %q is not a bench-scoped disposable namespace", s.ID, name)
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// resetScenarioNamespaces deletes the scenario's bench-scoped namespaces
// through the admin lease kubeconfig. Noop when the flag is off — which is
// every path except `evidra test` on its own disposable cluster.
func (h *Harness) resetScenarioNamespaces(ctx context.Context, req RunRequest, handle *environment.Handle, s *scenario.Scenario) error {
	if !req.Config.ResetNamespacesBeforeCase {
		return nil
	}
	names, err := isolationNamespaces(s)
	if err != nil {
		return &InfraError{Err: fmt.Errorf("harness: case isolation: %w", err)}
	}
	if len(names) == 0 {
		return nil
	}
	runner := h.deps.Runner
	if runner == nil {
		runner = &environment.ExecRunner{}
	}
	for _, ns := range names {
		// --ignore-not-found: first case on a fresh cluster has nothing to
		// delete; --wait keeps the next case's creates from racing the
		// terminator.
		//nolint:gosec // args are fixed above; ns is allowlist-validated.
		cmd := exec.Command("kubectl", "--kubeconfig", handle.KubeconfigPath,
			"delete", "namespace", ns, "--ignore-not-found", "--wait=true", "--timeout=90s")
		if out, err := runner.Run(ctx, cmd); err != nil {
			msg := strings.TrimSpace(string(out))
			if len(msg) > 200 {
				msg = msg[:200] + "..."
			}
			return &InfraError{Err: fmt.Errorf("harness: case isolation: delete namespace %s: %v: %s", ns, err, msg)}
		}
	}
	return nil
}
