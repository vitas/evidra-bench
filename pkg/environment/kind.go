package environment

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	Runner CommandRunner
}

// NewKindProvider returns a KindProvider with the default command runner.
func NewKindProvider() *KindProvider {
	return &KindProvider{Runner: &ExecRunner{}}
}

func (p *KindProvider) createCommand(clusterName string) *exec.Cmd {
	return exec.Command("kind", "create", "cluster",
		"--name", clusterName,
		"--wait", "60s",
	)
}

func (p *KindProvider) deleteCommand(clusterName string) *exec.Cmd {
	return exec.Command("kind", "delete", "cluster", "--name", clusterName)
}

func (p *KindProvider) kubeconfigCommand(clusterName string) *exec.Cmd {
	return exec.Command("kind", "get", "kubeconfig", "--name", clusterName)
}

// Create provisions a new kind cluster and writes a kubeconfig file.
func (p *KindProvider) Create(ctx context.Context, clusterName string) (*Handle, error) {
	cmd := p.createCommand(clusterName)
	if _, err := p.Runner.Run(ctx, cmd); err != nil {
		return nil, fmt.Errorf("environment.KindProvider.Create: %w", err)
	}

	kubeconfigCmd := p.kubeconfigCommand(clusterName)
	out, err := p.Runner.Run(ctx, kubeconfigCmd)
	if err != nil {
		return nil, fmt.Errorf("environment.KindProvider.Create: get kubeconfig: %w", err)
	}

	kubeconfigPath := filepath.Join(os.TempDir(), fmt.Sprintf("infra-bench-%s-kubeconfig", clusterName))
	if err := os.WriteFile(kubeconfigPath, out, 0600); err != nil {
		return nil, fmt.Errorf("environment.KindProvider.Create: write kubeconfig: %w", err)
	}

	return &Handle{
		ClusterName:    clusterName,
		KubeconfigPath: kubeconfigPath,
	}, nil
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
