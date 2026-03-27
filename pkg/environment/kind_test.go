package environment

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func TestKindProvider_CreateCommand(t *testing.T) {
	t.Parallel()
	p := NewKindProvider()
	cmd := p.createCommand("bench-test")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "kind create cluster") {
		t.Fatalf("unexpected command: %s", got)
	}
	if !strings.Contains(got, "--name bench-test") {
		t.Fatalf("missing cluster name: %s", got)
	}
}

func TestKindProvider_DeleteCommand(t *testing.T) {
	t.Parallel()
	p := NewKindProvider()
	cmd := p.deleteCommand("bench-test")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "kind delete cluster") {
		t.Fatalf("unexpected command: %s", got)
	}
	if !strings.Contains(got, "--name bench-test") {
		t.Fatalf("missing cluster name: %s", got)
	}
}

func TestKindProvider_KubeconfigCommand(t *testing.T) {
	t.Parallel()
	p := NewKindProvider()
	cmd := p.kubeconfigCommand("bench-test")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "kind get kubeconfig") {
		t.Fatalf("unexpected command: %s", got)
	}
}

func TestKindProvider_ImplementsProvider(t *testing.T) {
	t.Parallel()
	var _ Provider = (*KindProvider)(nil)
}

type stubRunner struct {
	outputs map[string][]byte
	seen    []string
}

func (s *stubRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	key := strings.Join(cmd.Args, " ")
	s.seen = append(s.seen, key)
	if out, ok := s.outputs[key]; ok {
		return out, nil
	}
	return nil, nil
}

func TestKindProvider_Create_ReusesExistingCluster(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{
		outputs: map[string][]byte{
			"kind get clusters":                    []byte("bench-cli\n"),
			"kind get kubeconfig --name bench-cli": []byte("apiVersion: v1\nkind: Config\n"),
		},
	}
	p := &KindProvider{
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
		if strings.Contains(cmd, "kind create cluster") {
			t.Fatalf("unexpected create command when reusing cluster: %s", cmd)
		}
	}
}

func TestBuildKindConfig_Empty(t *testing.T) {
	t.Parallel()
	cfg := BuildKindConfig(scenario.KubernetesConfig{})
	if cfg != "" {
		t.Fatalf("expected empty config for zero-valued KubernetesConfig, got:\n%s", cfg)
	}
}

func TestBuildKindConfig_Cilium(t *testing.T) {
	t.Parallel()
	cfg := BuildKindConfig(scenario.KubernetesConfig{CNI: "cilium"})
	if !strings.Contains(cfg, "disableDefaultCNI: true") {
		t.Fatalf("expected disableDefaultCNI for cilium, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "kubeProxyMode: none") {
		t.Fatalf("expected kubeProxyMode: none for cilium, got:\n%s", cfg)
	}
}

func TestBuildKindConfig_Calico(t *testing.T) {
	t.Parallel()
	cfg := BuildKindConfig(scenario.KubernetesConfig{CNI: "calico"})
	if !strings.Contains(cfg, "disableDefaultCNI: true") {
		t.Fatalf("expected disableDefaultCNI for calico, got:\n%s", cfg)
	}
	if strings.Contains(cfg, "kubeProxyMode") {
		t.Fatalf("calico should not set kubeProxyMode, got:\n%s", cfg)
	}
}

func TestBuildKindConfig_GVisor(t *testing.T) {
	t.Parallel()
	cfg := BuildKindConfig(scenario.KubernetesConfig{
		Runtimes: []scenario.RuntimeConfig{{Name: "gvisor", Handler: "runsc"}},
	})
	if !strings.Contains(cfg, "runsc") {
		t.Fatalf("expected gvisor mount config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "extraMounts") {
		t.Fatalf("expected extraMounts for gvisor, got:\n%s", cfg)
	}
}

func TestBuildKindConfig_AuditLogging(t *testing.T) {
	t.Parallel()
	cfg := BuildKindConfig(scenario.KubernetesConfig{
		Features: []string{"audit-logging"},
	})
	if !strings.Contains(cfg, "audit-log-path") {
		t.Fatalf("expected audit-log-path config, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "audit-policy-file") {
		t.Fatalf("expected audit-policy-file config, got:\n%s", cfg)
	}
}
