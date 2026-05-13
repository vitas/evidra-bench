package harness

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
)

func targetNamespace(req RunRequest) string {
	if req.TargetNamespace != "" {
		return req.TargetNamespace
	}
	return config.DefaultNamespace
}

func (h *Harness) prepareRunEnvironment(ctx context.Context, req RunRequest, handle *environment.Handle, ns string) (func(), error) {
	s := req.Scenario

	// Step 1b: Run cloud setup script if the scenario provides one.
	// The profile provisioner (profiles/<profile>/install.sh) already ran and
	// wrote lease.env. ExtraEnv carries the resulting vars (AWS_ENDPOINT_URL,
	// credentials, wrapper PATH) from the lease into the harness process.
	if s.Environment.Cloud.Setup != "" {
		setupCmd := exec.CommandContext(ctx, "bash", s.Environment.Cloud.Setup)
		setupCmd.Env = append(os.Environ(), req.ExtraEnv...)
		if out, setupErr := setupCmd.CombinedOutput(); setupErr != nil {
			return nil, fmt.Errorf("harness.Run: cloud setup: %s: %w", string(out), setupErr)
		}
	}

	cleanupExtraEnv, err := applyRunExtraEnv(req.ExtraEnv)
	if err != nil {
		return nil, err
	}

	kubeconfigExists := handle.KubeconfigPath != ""
	if kubeconfigExists {
		if _, statErr := os.Stat(handle.KubeconfigPath); statErr != nil {
			kubeconfigExists = false
		}
	}

	// Step 2: Force-clean stale namespace before bootstrap.
	// Namespace deletion is async and can leave finalizers hanging (kubernetes#53327),
	// so we force-delete the namespace entirely and wait for actual removal before recreating.
	if kubeconfigExists && h.deps.EnvProvider != nil {
		h.cleanupCluster(ctx, handle.KubeconfigPath, ns)
	}

	// Step 2b: Recreate target namespace.
	if handle.KubeconfigPath != "" && h.deps.EnvProvider != nil {
		if err := h.deps.EnvProvider.CreateNamespace(ctx, handle.KubeconfigPath, ns); err != nil {
			log.Printf("[harness] namespace create (non-fatal): %v", err)
		}
	}

	// Canary pod — verify scheduling before bootstrap.
	if kubeconfigExists && h.deps.EnvProvider != nil {
		if err := h.deps.EnvProvider.RunCanary(ctx, handle.KubeconfigPath, ns); err != nil {
			cleanupExtraEnv()
			return nil, &InfraError{Err: fmt.Errorf("harness.Run: canary failed on leased cluster: %w", err)}
		}
	}

	// Health check — informational after canary.
	if h.deps.EnvProvider != nil {
		if err := h.deps.EnvProvider.HealthCheck(ctx, handle.KubeconfigPath); err != nil {
			log.Printf("[harness] health check warning: %v", err)
		}
	}

	return cleanupExtraEnv, nil
}

func applyRunExtraEnv(extraEnv []string) (func(), error) {
	// Propagate lease env vars to the process so verifier checks and agent
	// subprocesses can inherit them (e.g. AWS credentials from a profile hook).
	for _, kv := range extraEnv {
		if parts := strings.SplitN(kv, "=", 2); len(parts) == 2 {
			if err := os.Setenv(parts[0], parts[1]); err != nil {
				return nil, fmt.Errorf("harness.Run: set %s: %w", parts[0], err)
			}
		}
	}
	return func() {
		for _, kv := range extraEnv {
			if parts := strings.SplitN(kv, "=", 2); len(parts) == 2 {
				_ = os.Unsetenv(parts[0])
			}
		}
	}, nil
}

func (h *Harness) cleanupCluster(ctx context.Context, kubeconfigPath, ns string) {
	if err := h.deps.EnvProvider.ForceDeleteNamespace(ctx, kubeconfigPath, ns); err != nil {
		log.Printf("[harness] namespace cleanup %s (non-fatal): %v", ns, err)
	}

	// Also clean cluster-scoped resources that scenarios may create.
	for _, res := range []string{"pv", "storageclass", "validatingwebhookconfiguration", "mutatingwebhookconfiguration"} {
		cleanCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
			"delete", res, "--all", "--ignore-not-found", "--timeout=10s")
		if out, err := cleanCmd.CombinedOutput(); err != nil {
			log.Printf("[harness] cluster cleanup %s (non-fatal): %s %v", res, string(out), err)
		}
	}
	// Clean scenario-created namespaces (webhook-system, etc.).
	for _, extraNS := range []string{"webhook-system"} {
		_ = h.deps.EnvProvider.ForceDeleteNamespace(ctx, kubeconfigPath, extraNS)
	}
	// Clean ArgoCD applications (if ArgoCD is installed).
	cleanCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"delete", "application", "--all", "-n", "argocd", "--ignore-not-found", "--timeout=15s")
	if out, err := cleanCmd.CombinedOutput(); err != nil {
		log.Printf("[harness] argocd application cleanup (non-fatal): %s %v", string(out), err)
	}
}
