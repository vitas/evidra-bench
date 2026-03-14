package environment

import (
	"context"
	"os/exec"
	"strings"
	"testing"
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
			"kind get clusters":                      []byte("infra-bench\n"),
			"kind get kubeconfig --name infra-bench": []byte("apiVersion: v1\nkind: Config\n"),
		},
	}
	p := &KindProvider{Runner: runner, ReuseExisting: true}

	handle, err := p.Create(context.Background(), "infra-bench")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if handle.ClusterName != "infra-bench" {
		t.Fatalf("unexpected cluster: %s", handle.ClusterName)
	}
	for _, cmd := range runner.seen {
		if strings.Contains(cmd, "kind create cluster") {
			t.Fatalf("unexpected create command when reusing cluster: %s", cmd)
		}
	}
}
