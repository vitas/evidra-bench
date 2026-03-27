package orchestrator

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

type fakeProvider struct {
	destroyCalls   int
	recreateCalls  int
	recreateHandle *environment.Handle
	recreateErr    error
}

func (f *fakeProvider) Create(_ context.Context, clusterName string, _ scenario.KubernetesConfig) (*environment.Handle, error) {
	return &environment.Handle{ClusterName: clusterName, KubeconfigPath: "/tmp/fake-kubeconfig"}, nil
}

func (f *fakeProvider) Destroy(_ context.Context, _ *environment.Handle) error {
	f.destroyCalls++
	return nil
}

func (f *fakeProvider) Recreate(_ context.Context, clusterName string, _ scenario.KubernetesConfig) (*environment.Handle, error) {
	f.recreateCalls++
	if f.recreateErr != nil {
		return nil, f.recreateErr
	}
	if f.recreateHandle != nil {
		return f.recreateHandle, nil
	}
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

func TestSelectWorkerKubeconfigPath_RecreateUpdatesClusterHandle(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{
		recreateHandle: &environment.Handle{
			ClusterName:    "bench",
			KubeconfigPath: "/tmp/recreated-kubeconfig",
		},
	}
	orch := &Orchestrator{
		cluster: &environment.Handle{
			ClusterName:    "bench",
			KubeconfigPath: "/tmp/original-kubeconfig",
		},
		provider: fp,
	}

	var consecutiveInfraFailures int64 = 2
	var recreateMu sync.Mutex

	got := orch.selectWorkerKubeconfigPath(
		context.Background(),
		1,
		&consecutiveInfraFailures,
		2,
		&recreateMu,
	)

	if got != "/tmp/recreated-kubeconfig" {
		t.Fatalf("selectWorkerKubeconfigPath() = %q, want recreated kubeconfig", got)
	}
	if orch.cluster.KubeconfigPath != "/tmp/recreated-kubeconfig" {
		t.Fatalf("cluster kubeconfig = %q, want recreated kubeconfig", orch.cluster.KubeconfigPath)
	}
	if fp.recreateCalls != 1 {
		t.Fatalf("Recreate() calls = %d, want 1", fp.recreateCalls)
	}
	if gotFailures := atomic.LoadInt64(&consecutiveInfraFailures); gotFailures != 0 {
		t.Fatalf("consecutiveInfraFailures = %d, want 0 after successful recreate", gotFailures)
	}
}
