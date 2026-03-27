package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
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

func TestClassifyScenarioError_NilIsPassed(t *testing.T) {
	t.Parallel()
	o := classifyScenarioError(nil)
	if o.status != "passed" {
		t.Fatalf("status = %q, want passed", o.status)
	}
	if o.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", o.exitCode)
	}
	if !o.passed {
		t.Fatal("passed = false, want true")
	}
	if o.failed || o.skipped || o.infra {
		t.Fatalf("unexpected flags: failed=%v skipped=%v infra=%v", o.failed, o.skipped, o.infra)
	}
}

func TestClassifyScenarioError_IncompatibleProviderIsSkipped(t *testing.T) {
	t.Parallel()
	err := &scenario.IncompatibleProviderError{
		ScenarioID: "kubernetes/broken-deployment",
		Required:   []string{"k3d"},
		Running:    "kind",
	}
	o := classifyScenarioError(err)
	if o.status != "skipped" {
		t.Fatalf("status = %q, want skipped", o.status)
	}
	if o.exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", o.exitCode)
	}
	if !o.skipped {
		t.Fatal("skipped = false, want true")
	}
	if o.passed || o.failed || o.infra {
		t.Fatalf("unexpected flags: passed=%v failed=%v infra=%v", o.passed, o.failed, o.infra)
	}
}

func TestClassifyScenarioError_InfraErrorIsError(t *testing.T) {
	t.Parallel()
	err := &harness.InfraError{Err: fmt.Errorf("cluster degraded")}
	o := classifyScenarioError(err)
	if o.status != "error" {
		t.Fatalf("status = %q, want error", o.status)
	}
	if o.exitCode != -1 {
		t.Fatalf("exitCode = %d, want -1", o.exitCode)
	}
	if !o.failed {
		t.Fatal("failed = false, want true")
	}
	if !o.infra {
		t.Fatal("infra = false, want true")
	}
	if o.passed || o.skipped {
		t.Fatalf("unexpected flags: passed=%v skipped=%v", o.passed, o.skipped)
	}
}

func TestClassifyScenarioError_RegularErrorIsFailed(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("agent verification failed")
	o := classifyScenarioError(err)
	if o.status != "failed" {
		t.Fatalf("status = %q, want failed", o.status)
	}
	if o.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", o.exitCode)
	}
	if !o.failed {
		t.Fatal("failed = false, want true")
	}
	if o.passed || o.skipped || o.infra {
		t.Fatalf("unexpected flags: passed=%v skipped=%v infra=%v", o.passed, o.skipped, o.infra)
	}
}
