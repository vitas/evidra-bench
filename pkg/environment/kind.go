package environment

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// CommandRunner executes shell commands. Extracted for testing.
type CommandRunner interface {
	Run(ctx context.Context, cmd *exec.Cmd) ([]byte, error)
}

// ExecRunner runs commands via os/exec.
type ExecRunner struct{}

// Run executes cmd and returns combined output.
func (r *ExecRunner) Run(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	return cmd.CombinedOutput()
}

// KindProvider manages kind cluster lifecycles.
type KindProvider struct {
	kubectlOps
	ReuseExisting bool
}

// NewKindProvider returns a KindProvider with the default command runner.
func NewKindProvider() *KindProvider {
	runner := &ExecRunner{}
	return &KindProvider{
		kubectlOps: kubectlOps{Runner: runner},
	}
}

func (p *KindProvider) createCommand(clusterName string) *exec.Cmd {
	return exec.Command("kind", "create", "cluster",
		"--name", clusterName,
		"--wait", "60s",
	)
}

// createCommandWithConfig builds a kind create command that uses a config file
// when the scenario declares Kubernetes infrastructure requirements.
func (p *KindProvider) createCommandWithConfig(clusterName string, k8s scenario.KubernetesConfig) (*exec.Cmd, func(), error) {
	configYAML := BuildKindConfig(k8s)
	if configYAML == "" {
		return p.createCommand(clusterName), func() {}, nil
	}

	tmpFile, err := os.CreateTemp("", "kind-config-*.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("write kind config: %w", err)
	}
	if _, err := tmpFile.WriteString(configYAML); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("write kind config: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("write kind config: %w", err)
	}

	cleanup := func() { _ = os.Remove(tmpFile.Name()) }

	cmd := exec.Command("kind", "create", "cluster",
		"--name", clusterName,
		"--config", tmpFile.Name(),
		"--wait", "60s",
	)
	return cmd, cleanup, nil
}

func (p *KindProvider) deleteCommand(clusterName string) *exec.Cmd {
	return exec.Command("kind", "delete", "cluster", "--name", clusterName)
}

func (p *KindProvider) listClustersCommand() *exec.Cmd {
	return exec.Command("kind", "get", "clusters")
}

func (p *KindProvider) kubeconfigCommand(clusterName string) *exec.Cmd {
	return exec.Command("kind", "get", "kubeconfig", "--name", clusterName)
}

// Create provisions a kind cluster with Kubernetes infrastructure
// requirements applied. When k8s is zero-valued, creates a plain cluster.
func (p *KindProvider) Create(ctx context.Context, clusterName string, k8s scenario.KubernetesConfig) (*Handle, error) {
	exists, err := p.clusterExists(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("environment.KindProvider.Create: check existing cluster: %w", err)
	}
	if !exists || !p.ReuseExisting {
		cmd, cleanup, err := p.createCommandWithConfig(clusterName, k8s)
		if err != nil {
			return nil, fmt.Errorf("environment.KindProvider.Create: %w", err)
		}
		defer cleanup()
		if _, err := p.Runner.Run(ctx, cmd); err != nil {
			return nil, fmt.Errorf("environment.KindProvider.Create: %w", err)
		}
	}

	kubeconfigCmd := p.kubeconfigCommand(clusterName)
	out, err := p.Runner.Run(ctx, kubeconfigCmd)
	if err != nil {
		return nil, fmt.Errorf("environment.KindProvider.Create: get kubeconfig: %w", err)
	}

	kubeconfigPath := filepath.Join(os.TempDir(), fmt.Sprintf("bench-cli-%s-kubeconfig", clusterName))
	if err := os.WriteFile(kubeconfigPath, out, 0600); err != nil {
		return nil, fmt.Errorf("environment.KindProvider.Create: write kubeconfig: %w", err)
	}

	return &Handle{
		ClusterName:    clusterName,
		KubeconfigPath: kubeconfigPath,
	}, nil
}

func (p *KindProvider) clusterExists(ctx context.Context, clusterName string) (bool, error) {
	cmd := p.listClustersCommand()
	out, err := p.Runner.Run(ctx, cmd)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == clusterName {
			return true, nil
		}
	}
	return false, nil
}

// Destroy tears down the kind cluster and removes the kubeconfig file.
func (p *KindProvider) Destroy(ctx context.Context, handle *Handle) error {
	cmd := p.deleteCommand(handle.ClusterName)
	if _, err := p.Runner.Run(ctx, cmd); err != nil {
		return fmt.Errorf("environment.KindProvider.Destroy: %w", err)
	}
	_ = os.Remove(handle.KubeconfigPath)
	return nil
}

// Recreate destroys and recreates a kind cluster from scratch.
func (p *KindProvider) Recreate(ctx context.Context, clusterName string, k8s scenario.KubernetesConfig) (*Handle, error) {
	log.Printf("[kind] recreating cluster %s (delete + create)", clusterName)
	delCmd := p.deleteCommand(clusterName)
	if _, err := p.Runner.Run(ctx, delCmd); err != nil {
		log.Printf("[kind] delete during recreate (non-fatal): %v", err)
	}
	return p.Create(ctx, clusterName, k8s)
}

// BuildKindConfig generates a kind cluster config YAML from KubernetesConfig.
// Returns empty string when no special configuration is needed.
func BuildKindConfig(k8s scenario.KubernetesConfig) string {
	if k8s.CNI == "" && len(k8s.Runtimes) == 0 && len(k8s.Features) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\n")

	// Networking: disable default CNI for Cilium/Calico.
	if k8s.CNI != "" {
		b.WriteString("networking:\n")
		b.WriteString("  disableDefaultCNI: true\n")
		if k8s.CNI == "cilium" {
			// Cilium requires no kube-proxy when using its replacement.
			b.WriteString("  kubeProxyMode: none\n")
		}
	}

	// Node configuration.
	needsNodeConfig := len(k8s.Runtimes) > 0 || hasFeature(k8s.Features, "audit-logging")
	if needsNodeConfig {
		b.WriteString("nodes:\n")
		b.WriteString("  - role: control-plane\n")

		if hasFeature(k8s.Features, "audit-logging") {
			b.WriteString("    kubeadmConfigPatches:\n")
			b.WriteString("      - |\n")
			b.WriteString("        kind: ClusterConfiguration\n")
			b.WriteString("        apiServer:\n")
			b.WriteString("          extraArgs:\n")
			b.WriteString("            audit-log-path: /var/log/kubernetes/audit.log\n")
			b.WriteString("            audit-policy-file: /etc/kubernetes/audit-policy.yaml\n")
			b.WriteString("          extraVolumes:\n")
			b.WriteString("            - name: audit-policy\n")
			b.WriteString("              hostPath: /etc/kubernetes/audit-policy.yaml\n")
			b.WriteString("              mountPath: /etc/kubernetes/audit-policy.yaml\n")
			b.WriteString("              readOnly: true\n")
			b.WriteString("            - name: audit-logs\n")
			b.WriteString("              hostPath: /var/log/kubernetes\n")
			b.WriteString("              mountPath: /var/log/kubernetes\n")
		}

		if len(k8s.Runtimes) > 0 {
			b.WriteString("    extraMounts:\n")
			for _, rt := range k8s.Runtimes {
				if rt.Name == "gvisor" {
					b.WriteString("      - hostPath: /usr/local/bin/runsc\n")
					b.WriteString("        containerPath: /usr/local/bin/runsc\n")
				}
			}
		}

		b.WriteString("  - role: worker\n")
		if len(k8s.Runtimes) > 0 {
			b.WriteString("    extraMounts:\n")
			for _, rt := range k8s.Runtimes {
				if rt.Name == "gvisor" {
					b.WriteString("      - hostPath: /usr/local/bin/runsc\n")
					b.WriteString("        containerPath: /usr/local/bin/runsc\n")
				}
			}
		}
	}

	return b.String()
}

func hasFeature(features []string, name string) bool {
	for _, f := range features {
		if f == name {
			return true
		}
	}
	return false
}
