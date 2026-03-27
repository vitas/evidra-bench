package environment

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// K3dProvider manages k3d cluster lifecycles.
type K3dProvider struct {
	kubectlOps
	ReuseExisting bool
}

// NewK3dProvider returns a K3dProvider with the default command runner.
func NewK3dProvider() *K3dProvider {
	runner := &ExecRunner{}
	return &K3dProvider{
		kubectlOps: kubectlOps{Runner: runner},
	}
}

func (p *K3dProvider) createCommand(clusterName string) *exec.Cmd {
	return exec.Command("k3d", "cluster", "create", clusterName,
		"--no-lb",
		"--wait",
	)
}

// createCommandWithExplicitConfig builds a k3d create command that uses a
// checked-in config file directly.
func (p *K3dProvider) createCommandWithExplicitConfig(clusterName, configPath string) *exec.Cmd {
	return exec.Command("k3d", "cluster", "create", clusterName,
		"--config", configPath,
	)
}

func (p *K3dProvider) deleteCommand(clusterName string) *exec.Cmd {
	return exec.Command("k3d", "cluster", "delete", clusterName)
}

func (p *K3dProvider) listClustersCommand() *exec.Cmd {
	return exec.Command("k3d", "cluster", "list", "--no-headers")
}

func (p *K3dProvider) kubeconfigCommand(clusterName string) *exec.Cmd {
	return exec.Command("k3d", "kubeconfig", "get", clusterName)
}

// Create provisions a new k3d cluster and writes a kubeconfig file.
// When spec.ConfigPath is set, uses the checked-in config file.
// Otherwise, creates a plain cluster (LegacyKubernetes options are not
// supported by k3d and are logged as warnings).
func (p *K3dProvider) Create(ctx context.Context, clusterName string, spec ClusterSpec) (*Handle, error) {
	k8s := spec.LegacyKubernetes
	if spec.ConfigPath == "" && (k8s.CNI != "" || len(k8s.Addons) > 0 || len(k8s.Runtimes) > 0 || len(k8s.Features) > 0) {
		log.Printf("[k3d] warning: KubernetesConfig options not supported by k3d provider")
	}
	exists, err := p.clusterExists(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("environment.K3dProvider.Create: check existing cluster: %w", err)
	}
	if !exists || !p.ReuseExisting {
		if exists {
			// Delete first — k3d can't create over an existing cluster.
			delCmd := p.deleteCommand(clusterName)
			if out, err := p.Runner.Run(ctx, delCmd); err != nil {
				return nil, fmt.Errorf("environment.K3dProvider.Create: delete existing cluster: %w: %s", err, string(out))
			}
		}
		var cmd *exec.Cmd
		if spec.ConfigPath != "" {
			cmd = p.createCommandWithExplicitConfig(clusterName, spec.ConfigPath)
		} else {
			cmd = p.createCommand(clusterName)
		}
		if _, err := p.Runner.Run(ctx, cmd); err != nil {
			return nil, fmt.Errorf("environment.K3dProvider.Create: %w", err)
		}
	}

	kubeconfigCmd := p.kubeconfigCommand(clusterName)
	out, err := p.Runner.Run(ctx, kubeconfigCmd)
	if err != nil {
		return nil, fmt.Errorf("environment.K3dProvider.Create: get kubeconfig: %w", err)
	}

	kubeconfigPath := filepath.Join(os.TempDir(), fmt.Sprintf("bench-cli-%s-kubeconfig", clusterName))
	if err := os.WriteFile(kubeconfigPath, out, 0600); err != nil {
		return nil, fmt.Errorf("environment.K3dProvider.Create: write kubeconfig: %w", err)
	}

	return &Handle{
		ClusterName:    clusterName,
		KubeconfigPath: kubeconfigPath,
	}, nil
}

// Recreate tears down and re-creates the k3d cluster.
func (p *K3dProvider) Recreate(ctx context.Context, clusterName string, spec ClusterSpec) (*Handle, error) {
	log.Printf("[k3d] recreating cluster %s", clusterName)
	delCmd := p.deleteCommand(clusterName)
	if out, err := p.Runner.Run(ctx, delCmd); err != nil {
		return nil, fmt.Errorf("environment.K3dProvider.Recreate: delete existing cluster: %w: %s", err, string(out))
	}
	return p.Create(ctx, clusterName, spec)
}

func (p *K3dProvider) clusterExists(ctx context.Context, clusterName string) (bool, error) {
	cmd := p.listClustersCommand()
	out, err := p.Runner.Run(ctx, cmd)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == clusterName {
			return true, nil
		}
	}
	return false, nil
}

// Destroy tears down the k3d cluster and removes the kubeconfig file.
func (p *K3dProvider) Destroy(ctx context.Context, handle *Handle) error {
	cmd := p.deleteCommand(handle.ClusterName)
	if _, err := p.Runner.Run(ctx, cmd); err != nil {
		return fmt.Errorf("environment.K3dProvider.Destroy: %w", err)
	}
	_ = os.Remove(handle.KubeconfigPath)
	return nil
}
