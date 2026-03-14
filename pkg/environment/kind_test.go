package environment

import (
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
