package environment

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// K3dProvider manages k3d cluster lifecycles.
type K3dProvider struct {
	Runner        CommandRunner
	ReuseExisting bool
}

// NewK3dProvider returns a K3dProvider with the default command runner.
func NewK3dProvider() *K3dProvider {
	return &K3dProvider{Runner: &ExecRunner{}}
}

func (p *K3dProvider) createCommand(clusterName string) *exec.Cmd {
	return exec.Command("k3d", "cluster", "create", clusterName,
		"--no-lb",
		"--wait",
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
func (p *K3dProvider) Create(ctx context.Context, clusterName string) (*Handle, error) {
	exists, err := p.clusterExists(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("environment.K3dProvider.Create: check existing cluster: %w", err)
	}
	if !exists || !p.ReuseExisting {
		cmd := p.createCommand(clusterName)
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
