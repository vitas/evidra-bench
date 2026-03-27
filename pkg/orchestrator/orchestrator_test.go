package orchestrator

import (
	"context"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

type fakeProvider struct {
	destroyCalls int
}

func (f *fakeProvider) Create(_ context.Context, clusterName string, _ scenario.KubernetesConfig) (*environment.Handle, error) {
	return &environment.Handle{ClusterName: clusterName, KubeconfigPath: "/tmp/fake-kubeconfig"}, nil
}

func (f *fakeProvider) Destroy(_ context.Context, _ *environment.Handle) error {
	f.destroyCalls++
	return nil
}

func (f *fakeProvider) Recreate(_ context.Context, clusterName string, _ scenario.KubernetesConfig) (*environment.Handle, error) {
	return &environment.Handle{ClusterName: clusterName, KubeconfigPath: "/tmp/fake-kubeconfig"}, nil
}

func (f *fakeProvider) HealthCheck(_ context.Context, _ string) error { return nil }

func (f *fakeProvider) ForceDeleteNamespace(_ context.Context, _, _ string) error { return nil }

func (f *fakeProvider) CreateNamespace(_ context.Context, _, _ string) error { return nil }

func (f *fakeProvider) RunCanary(_ context.Context, _, _ string) error { return nil }

func TestTeardownSkipsExternalKubeconfig(t *testing.T) {
	t.Parallel()

	orch := New(config.Config{
		ClusterName:    "external",
		KubeconfigPath: "/tmp/external-kubeconfig",
	}, nil)
	if _, err := orch.Provision(context.Background()); err != nil {
		t.Fatalf("Provision() error = %v", err)
	}

	fp := &fakeProvider{}
	orch.provider = fp

	orch.Teardown(context.Background())

	if fp.destroyCalls != 0 {
		t.Fatalf("Destroy() calls = %d, want 0 for external kubeconfig", fp.destroyCalls)
	}
}
