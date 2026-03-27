package environment

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// fakeRunner records commands and returns canned output.
type fakeRunner struct {
	commands []string
	outputs  map[string][]byte
	err      error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{outputs: make(map[string][]byte)}
}

func (f *fakeRunner) Run(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	key := strings.Join(cmd.Args, " ")
	f.commands = append(f.commands, key)
	if f.err != nil {
		return nil, f.err
	}
	if out, ok := f.outputs[key]; ok {
		return out, nil
	}
	return []byte("ok"), nil
}

func (f *fakeRunner) hasCommandContaining(sub string) bool {
	for _, c := range f.commands {
		if strings.Contains(c, sub) {
			return true
		}
	}
	return false
}

// fakeProvider is a minimal ClusterLifecycle for testing.
type fakeProvider struct {
	kubectlOps
	created   bool
	destroyed bool
	handle    *Handle
}

func newFakeProvider(runner CommandRunner) *fakeProvider {
	return &fakeProvider{
		kubectlOps: kubectlOps{Runner: runner},
		handle: &Handle{
			ClusterName:    "test-cluster",
			KubeconfigPath: "/tmp/test-kubeconfig",
		},
	}
}

func (f *fakeProvider) Create(_ context.Context, _ string, _ scenario.KubernetesConfig) (*Handle, error) {
	f.created = true
	return f.handle, nil
}

func (f *fakeProvider) Destroy(_ context.Context, _ *Handle) error {
	f.destroyed = true
	return nil
}

func (f *fakeProvider) Recreate(ctx context.Context, name string, k8s scenario.KubernetesConfig) (*Handle, error) {
	f.destroyed = true
	return f.Create(ctx, name, k8s)
}

func TestLocalProvisioner_AcquireDefault(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{"kind": provider},
		Runner:    runner,
	}

	lease, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:      scenario.ProfileDefault,
		ProviderName: "kind",
		ClusterName:  "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = lease.Release(context.Background()) }()

	if !provider.created {
		t.Fatal("expected provider.Create to be called")
	}
	if lease.Profile != scenario.ProfileDefault {
		t.Fatalf("expected profile default, got %q", lease.Profile)
	}
	if lease.KubeconfigPath != "/tmp/test-kubeconfig" {
		t.Fatalf("expected kubeconfig /tmp/test-kubeconfig, got %q", lease.KubeconfigPath)
	}
	if len(lease.ExtraEnv) != 0 {
		t.Fatalf("expected no extra env, got %v", lease.ExtraEnv)
	}
}

func TestLocalProvisioner_AcquireArgocd_InstallsAddon(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{"kind": provider},
		Runner:    runner,
	}

	lease, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:      scenario.ProfileArgocd,
		ProviderName: "kind",
		ClusterName:  "test-argocd",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = lease.Release(context.Background()) }()

	if !provider.created {
		t.Fatal("expected provider.Create to be called")
	}
	if lease.Profile != scenario.ProfileArgocd {
		t.Fatalf("expected profile argocd, got %q", lease.Profile)
	}

	// Verify ArgoCD addon steps were executed via the bootstrapper.
	if !runner.hasCommandContaining("argocd") {
		t.Fatal("expected argocd addon steps to be executed")
	}
}

func TestLocalProvisioner_AcquireAWSLocalStack_NotYetImplemented(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{"kind": provider},
		Runner:    runner,
	}

	_, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:      scenario.ProfileAWSLocalStack,
		ProviderName: "kind",
		ClusterName:  "test-aws",
	})
	if err == nil {
		t.Fatal("expected error for aws-localstack profile")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("expected 'not yet implemented' error, got: %v", err)
	}
}

func TestLocalProvisioner_AcquireExistingKubeconfig_SkipsCreate(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{"kind": provider},
		Runner:    runner,
	}

	lease, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:            scenario.ProfileDefault,
		ProviderName:       "kind",
		ClusterName:        "test",
		ExistingKubeconfig: "/home/user/.kube/config",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = lease.Release(context.Background()) }()

	if provider.created {
		t.Fatal("expected provider.Create NOT to be called for existing kubeconfig")
	}
	if lease.KubeconfigPath != "/home/user/.kube/config" {
		t.Fatalf("expected existing kubeconfig path, got %q", lease.KubeconfigPath)
	}
}

func TestLocalProvisioner_AcquireDefault_ReleaseDestroysCluster(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{"kind": provider},
		Runner:    runner,
	}

	lease, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:      scenario.ProfileDefault,
		ProviderName: "kind",
		ClusterName:  "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := lease.Release(context.Background()); err != nil {
		t.Fatalf("unexpected release error: %v", err)
	}
	if !provider.destroyed {
		t.Fatal("expected provider.Destroy to be called on release")
	}
}

func TestLocalProvisioner_AcquireDefault_ReuseCluster_ReleaseSkipsDestroy(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{"kind": provider},
		Runner:    runner,
	}

	lease, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:      scenario.ProfileDefault,
		ProviderName: "kind",
		ClusterName:  "test",
		ReuseCluster: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := lease.Release(context.Background()); err != nil {
		t.Fatalf("unexpected release error: %v", err)
	}
	if provider.destroyed {
		t.Fatal("expected provider.Destroy NOT to be called when reusing cluster")
	}
}

func TestLocalProvisioner_AcquireExistingKubeconfig_ReleaseIsNoop(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{"kind": provider},
		Runner:    runner,
	}

	lease, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:            scenario.ProfileDefault,
		ProviderName:       "kind",
		ExistingKubeconfig: "/home/user/.kube/config",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := lease.Release(context.Background()); err != nil {
		t.Fatalf("unexpected release error: %v", err)
	}
	if provider.destroyed {
		t.Fatal("expected no destroy for external kubeconfig")
	}
}

func TestLocalProvisioner_UnknownProvider_ReturnsError(t *testing.T) {
	t.Parallel()
	p := &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{},
		Runner:    newFakeRunner(),
	}

	_, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:      scenario.ProfileDefault,
		ProviderName: "docker-desktop",
		ClusterName:  "test",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "docker-desktop") {
		t.Fatalf("expected error to mention provider name, got: %v", err)
	}
}

func TestLease_Release_NilIsNoop(t *testing.T) {
	t.Parallel()
	var l *Lease
	if err := l.Release(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLease_Release_NilFuncIsNoop(t *testing.T) {
	t.Parallel()
	l := &Lease{}
	if err := l.Release(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
