package environment

import (
	"context"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/scenario"
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

	handle, err := p.Create(context.Background(), "bench-cli", scenario.KubernetesConfig{})
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
