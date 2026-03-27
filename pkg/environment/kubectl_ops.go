package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// kubectlOps provides kubectl-based cluster operations shared across providers.
// Embedded by providers that manage clusters accessible via kubeconfig.
type kubectlOps struct {
	Runner CommandRunner
}

// HealthCheck verifies node conditions (pressure, readiness) and checks for stuck pods.
func (k *kubectlOps) HealthCheck(ctx context.Context, kubeconfigPath string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "nodes", "-o", "json")
	out, err := k.Runner.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("health check: get nodes: %w: %s", err, string(out))
	}

	var nodeList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &nodeList); err != nil {
		return fmt.Errorf("health check: parse nodes: %w", err)
	}

	pressureConditions := map[string]bool{
		"MemoryPressure": true,
		"DiskPressure":   true,
		"PIDPressure":    true,
	}
	for _, node := range nodeList.Items {
		for _, cond := range node.Status.Conditions {
			if pressureConditions[cond.Type] && cond.Status == "True" {
				return fmt.Errorf("health check: node %s has %s", node.Metadata.Name, cond.Type)
			}
			if cond.Type == "Ready" && cond.Status != "True" {
				return fmt.Errorf("health check: node %s not Ready (status=%s)", node.Metadata.Name, cond.Status)
			}
		}
	}

	pendingCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "pods", "--all-namespaces", "--field-selector=status.phase=Pending",
		"-o", "jsonpath={.items[*].metadata.name}")
	pendingOut, err := k.Runner.Run(ctx, pendingCmd)
	if err != nil {
		log.Printf("[health] pending pod check failed (non-fatal): %v", err)
		return nil
	}
	pending := strings.TrimSpace(string(pendingOut))
	if pending != "" {
		return fmt.Errorf("health check: stuck pending pods: %s", pending)
	}
	return nil
}

// ForceDeleteNamespace force-deletes a namespace and waits for removal.
func (k *kubectlOps) ForceDeleteNamespace(ctx context.Context, kubeconfigPath, ns string) error {
	getCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "namespace", ns, "--ignore-not-found", "-o", "name")
	getOut, err := k.Runner.Run(ctx, getCmd)
	if err != nil || strings.TrimSpace(string(getOut)) == "" {
		return nil
	}

	log.Printf("[cluster] force-deleting namespace %s", ns)
	delCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"delete", "namespace", ns, "--force", "--grace-period=0", "--timeout=30s")
	if out, err := k.Runner.Run(ctx, delCmd); err != nil {
		log.Printf("[cluster] namespace force-delete %s (non-fatal): %s %v", ns, string(out), err)
	}

	for i := 0; i < 30; i++ {
		checkCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
			"get", "namespace", ns, "--ignore-not-found", "-o", "name")
		checkOut, _ := k.Runner.Run(ctx, checkCmd)
		if strings.TrimSpace(string(checkOut)) == "" {
			log.Printf("[cluster] namespace %s deleted", ns)
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("namespace %s still terminating after 30s", ns)
}

// CreateNamespace creates a namespace, ignoring "already exists" errors.
func (k *kubectlOps) CreateNamespace(ctx context.Context, kubeconfigPath, ns string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"create", "namespace", ns)
	out, err := k.Runner.Run(ctx, cmd)
	if err != nil && !strings.Contains(string(out), "already exists") {
		return fmt.Errorf("create namespace %s: %w: %s", ns, err, string(out))
	}
	return nil
}

// RunCanary runs a lightweight pod to verify scheduling works.
func (k *kubectlOps) RunCanary(ctx context.Context, kubeconfigPath, ns string) error {
	delCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"delete", "pod", "bench-canary", "-n", ns, "--ignore-not-found", "--timeout=10s")
	_, _ = k.Runner.Run(ctx, delCmd)

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"run", "bench-canary", "-n", ns,
		"--image=busybox:1.36", "--restart=Never",
		"--rm", "-i", "--timeout=30s", "--", "echo", "ok")
	out, err := k.Runner.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("canary pod failed: %w: %s", err, string(out))
	}
	return nil
}
