package harness

import (
	"context"
	"fmt"
	"log"
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

// resetScenarioNamespaces is the ONE namespace lifecycle stage in suite
// mode: delete every declared scope once (through the admin lease
// kubeconfig), then recreate the target namespace and prove scheduling
// with the canary before bootstrap. Noop when the flag is off — which is
// every path except `evidra test` on its own disposable cluster.
func (h *Harness) resetScenarioNamespaces(ctx context.Context, req RunRequest, handle *environment.Handle, s *scenario.Scenario, ns string) error {
	if !req.Config.ResetNamespacesBeforeCase {
		return nil
	}
	// Delete every declared scope exactly once.
	names, err := isolationNamespaces(s)
	if err != nil {
		return &InfraError{Err: fmt.Errorf("harness: case isolation: %w", err)}
	}
	runner := h.deps.Runner
	if runner == nil {
		runner = &environment.ExecRunner{}
	}
	for _, name := range names {
		// --ignore-not-found: first case on a fresh cluster has nothing to
		// delete; --wait keeps the next case's creates from racing the
		// terminator.
		//nolint:gosec // args are fixed above; ns is allowlist-validated.
		cmd := exec.Command("kubectl", "--kubeconfig", handle.KubeconfigPath,
			"delete", "namespace", name, "--ignore-not-found", "--wait=true", "--timeout=90s")
		if out, err := runner.Run(ctx, cmd); err != nil {
			msg := strings.TrimSpace(string(out))
			if len(msg) > 200 {
				msg = msg[:200] + "..."
			}
			return &InfraError{Err: fmt.Errorf("harness: case isolation: delete namespace %s: %v: %s", name, err, msg)}
		}
	}
	if h.deps.EnvProvider == nil {
		return nil
	}
	// Recreate the target namespace after the sweep, then prove the
	// cluster can schedule — the two steps prepareRunEnvironment used to
	// duplicate on every suite case.
	if err := h.deps.EnvProvider.CreateNamespace(ctx, handle.KubeconfigPath, ns); err != nil {
		log.Printf("[harness] namespace create (non-fatal): %v", err)
	}
	if err := h.deps.EnvProvider.RunCanary(ctx, handle.KubeconfigPath, ns); err != nil {
		return &InfraError{Err: fmt.Errorf("harness: case isolation: canary failed after namespace reset: %w", err)}
	}
	return nil
}
