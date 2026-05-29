package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

type fakeProvider struct {
	destroyCalls   int
	recreateCalls  int
	recreateHandle *environment.Handle
	recreateErr    error
}

func (f *fakeProvider) Create(_ context.Context, clusterName string, _ environment.ClusterSpec) (*environment.Handle, error) {
	return &environment.Handle{ClusterName: clusterName, KubeconfigPath: "/tmp/fake-kubeconfig"}, nil
}

func (f *fakeProvider) Destroy(_ context.Context, _ *environment.Handle) error {
	f.destroyCalls++
	return nil
}

func (f *fakeProvider) Recreate(_ context.Context, clusterName string, _ environment.ClusterSpec) (*environment.Handle, error) {
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

func TestClassifyScenarioResult_FailedResultIsFailed(t *testing.T) {
	t.Parallel()

	o := classifyScenarioResult(&harness.RunResult{
		ScenarioID: "kubernetes/broken-deployment",
		Passed:     false,
		ExitCode:   0,
	}, nil)

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

func TestClassifyScenarioResult_UsesNonZeroResultExitCode(t *testing.T) {
	t.Parallel()

	o := classifyScenarioResult(&harness.RunResult{
		ScenarioID: "kubernetes/broken-deployment",
		Passed:     false,
		ExitCode:   42,
	}, nil)

	if o.status != "failed" {
		t.Fatalf("status = %q, want failed", o.status)
	}
	if o.exitCode != 42 {
		t.Fatalf("exitCode = %d, want 42", o.exitCode)
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

func TestValidateParallelProfiles_DefaultOnly(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{ID: "s1", Environment: scenario.EnvironmentConfig{}},
		{ID: "s2", Environment: scenario.EnvironmentConfig{Profile: scenario.ProfileDefault}},
	}

	if err := ValidateParallelProfiles(scenarios); err != nil {
		t.Fatalf("expected no error for default profiles, got: %v", err)
	}
}

func TestValidateParallelProfiles_RejectsArgocd(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{ID: "s1", Environment: scenario.EnvironmentConfig{}},
		{ID: "s2", Environment: scenario.EnvironmentConfig{Profile: scenario.ProfileArgocd}},
	}

	err := ValidateParallelProfiles(scenarios)
	if err == nil {
		t.Fatal("expected error for argocd profile")
	}
	if !strings.Contains(err.Error(), "argocd") {
		t.Fatalf("error should mention argocd, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not isolated") {
		t.Fatalf("error should explain isolation issue, got: %v", err)
	}
	if !strings.Contains(err.Error(), "s2") {
		t.Fatalf("error should mention scenario ID, got: %v", err)
	}
}

func TestValidateParallelProfiles_RejectsAWSLocalStack(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{ID: "s1", Environment: scenario.EnvironmentConfig{Profile: scenario.ProfileAWSLocalStack}},
	}

	err := ValidateParallelProfiles(scenarios)
	if err == nil {
		t.Fatal("expected error for aws-localstack profile")
	}
	if !strings.Contains(err.Error(), "aws-localstack") {
		t.Fatalf("error should mention aws-localstack, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not isolated") {
		t.Fatalf("error should explain isolation issue, got: %v", err)
	}
}

func TestValidateParallelProfiles_EmptyList(t *testing.T) {
	t.Parallel()

	if err := ValidateParallelProfiles(nil); err != nil {
		t.Fatalf("expected no error for empty list, got: %v", err)
	}
}

func TestValidateParallelProfiles_LegacyLocalStackInference(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{
			ID: "aws-scenario",
			Environment: scenario.EnvironmentConfig{
				Cloud: scenario.CloudConfig{Provider: "localstack"},
			},
		},
	}

	err := ValidateParallelProfiles(scenarios)
	if err == nil {
		t.Fatal("expected error for inferred aws-localstack profile")
	}
	if !strings.Contains(err.Error(), "aws-localstack") {
		t.Fatalf("error should mention aws-localstack, got: %v", err)
	}
}
