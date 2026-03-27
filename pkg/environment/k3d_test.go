package environment

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestK3dProvider_CreateCommand(t *testing.T) {
	t.Parallel()
	p := NewK3dProvider()
	cmd := p.createCommand("bench-test")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "k3d cluster create") {
		t.Fatalf("unexpected command: %s", got)
	}
	if !strings.Contains(got, "bench-test") {
		t.Fatalf("missing cluster name: %s", got)
	}
}

func TestK3dProvider_DeleteCommand(t *testing.T) {
	t.Parallel()
	p := NewK3dProvider()
	cmd := p.deleteCommand("bench-test")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "k3d cluster delete") {
		t.Fatalf("unexpected command: %s", got)
	}
	if !strings.Contains(got, "bench-test") {
		t.Fatalf("missing cluster name: %s", got)
	}
}

func TestK3dProvider_KubeconfigCommand(t *testing.T) {
	t.Parallel()
	p := NewK3dProvider()
	cmd := p.kubeconfigCommand("bench-test")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "k3d kubeconfig get") {
		t.Fatalf("unexpected command: %s", got)
	}
}

func TestK3dProvider_ImplementsProvider(t *testing.T) {
	t.Parallel()
	var _ ClusterLifecycle = (*K3dProvider)(nil)
}

func TestK3dProvider_Create_ReusesExistingCluster(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{
		outputs: map[string][]byte{
			"k3d cluster list --no-headers": []byte("bench-cli   1/1   0/0   true\n"),
			"k3d kubeconfig get bench-cli":  []byte("apiVersion: v1\nkind: Config\n"),
		},
	}
	p := &K3dProvider{
		kubectlOps:    kubectlOps{Runner: runner},
		ReuseExisting: true,
	}

	handle, err := p.Create(context.Background(), "bench-cli", ClusterSpec{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if handle.ClusterName != "bench-cli" {
		t.Fatalf("unexpected cluster: %s", handle.ClusterName)
	}
	for _, cmd := range runner.seen {
		if strings.Contains(cmd, "k3d cluster create") {
			t.Fatalf("unexpected create command when reusing cluster: %s", cmd)
		}
	}
}

type k3dRunner struct {
	results map[string]struct {
		out []byte
		err error
	}
	seen []string
}

func (r *k3dRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	key := strings.Join(cmd.Args, " ")
	r.seen = append(r.seen, key)
	if result, ok := r.results[key]; ok {
		return result.out, result.err
	}
	return nil, nil
}

func TestK3dProvider_Create_UsesExplicitConfigPath(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{
		outputs: map[string][]byte{
			"k3d cluster list --no-headers":                                        []byte(""),
			"k3d cluster create bench-cli --config /repo/clusters/k3d/argocd.yaml": []byte(""),
			"k3d kubeconfig get bench-cli":                                         []byte("apiVersion: v1\nkind: Config\n"),
		},
	}
	p := &K3dProvider{
		kubectlOps: kubectlOps{Runner: runner},
	}

	spec := ClusterSpec{ConfigPath: "/repo/clusters/k3d/argocd.yaml"}
	handle, err := p.Create(context.Background(), "bench-cli", spec)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if handle.ClusterName != "bench-cli" {
		t.Fatalf("unexpected cluster: %s", handle.ClusterName)
	}

	found := false
	for _, cmd := range runner.seen {
		if strings.Contains(cmd, "--config /repo/clusters/k3d/argocd.yaml") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --config flag with explicit path, got commands: %v", runner.seen)
	}
}

func TestK3dProvider_Create_FallsBackToPlainClusterWhenSpecEmpty(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{
		outputs: map[string][]byte{
			"k3d cluster list --no-headers":               []byte(""),
			"k3d cluster create bench-cli --no-lb --wait": []byte(""),
			"k3d kubeconfig get bench-cli":                []byte("apiVersion: v1\nkind: Config\n"),
		},
	}
	p := &K3dProvider{
		kubectlOps: kubectlOps{Runner: runner},
	}

	handle, err := p.Create(context.Background(), "bench-cli", ClusterSpec{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if handle.ClusterName != "bench-cli" {
		t.Fatalf("unexpected cluster: %s", handle.ClusterName)
	}

	for _, cmd := range runner.seen {
		if strings.Contains(cmd, "--config") {
			t.Fatalf("unexpected --config flag for empty spec: %s", cmd)
		}
	}
}

func TestK3dProvider_Recreate_DeleteFailureIsFatal(t *testing.T) {
	t.Parallel()

	runner := &k3dRunner{
		results: map[string]struct {
			out []byte
			err error
		}{
			"k3d cluster delete bench-cli": {
				out: []byte("delete failed"),
				err: errors.New("exit status 1"),
			},
		},
	}
	p := &K3dProvider{
		kubectlOps:    kubectlOps{Runner: runner},
		ReuseExisting: true,
	}

	_, err := p.Recreate(context.Background(), "bench-cli", ClusterSpec{})
	if err == nil {
		t.Fatal("Recreate() error = nil, want delete failure")
	}
	if !strings.Contains(err.Error(), "delete existing cluster") && !strings.Contains(err.Error(), "delete during recreate") {
		t.Fatalf("Recreate() error = %v, want delete context", err)
	}
	for _, cmd := range runner.seen {
		if strings.Contains(cmd, "k3d cluster create") {
			t.Fatalf("unexpected create command after delete failure: %s", cmd)
		}
	}
}
