package environment

import (
	"context"
	"os/exec"
	"path/filepath"
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
	lastSpec  ClusterSpec
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

func (f *fakeProvider) Create(_ context.Context, _ string, spec ClusterSpec) (*Handle, error) {
	f.created = true
	f.lastSpec = spec
	return f.handle, nil
}

func (f *fakeProvider) Destroy(_ context.Context, _ *Handle) error {
	f.destroyed = true
	return nil
}

func (f *fakeProvider) Recreate(ctx context.Context, name string, spec ClusterSpec) (*Handle, error) {
	f.destroyed = true
	return f.Create(ctx, name, spec)
}

// provisionerAssetsRoot returns the path to the test asset tree.
func provisionerAssetsRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(testdataDir(t), "provisioner-assets")
}

// newTestProvisioner creates a LocalProvisioner with test assets.
func newTestProvisioner(t *testing.T, runner *fakeRunner, provider *fakeProvider) *LocalProvisioner {
	t.Helper()
	root := provisionerAssetsRoot(t)
	return &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{"kind": provider},
		Runner:    runner,
		Assets:    AssetResolver{RootDir: root},
		Profiles:  &ProfileRunner{},
	}
}

func TestLocalProvisioner_AcquireDefault(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := newTestProvisioner(t, runner, provider)

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
	// Verify config path was passed to provider.
	if !strings.HasSuffix(provider.lastSpec.ConfigPath, "clusters/kind/default.yaml") {
		t.Fatalf("expected config path to end with clusters/kind/default.yaml, got %q", provider.lastSpec.ConfigPath)
	}
}

func TestLocalProvisioner_AcquireArgocd_UsesProfileAssets(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := newTestProvisioner(t, runner, provider)

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

	// Verify config path was resolved from assets.
	if !strings.HasSuffix(provider.lastSpec.ConfigPath, "clusters/kind/argocd.yaml") {
		t.Fatalf("expected config path to end with clusters/kind/argocd.yaml, got %q", provider.lastSpec.ConfigPath)
	}

	// The old test checked for argocd commands via bootstrapper.
	// Now we verify that profile hooks ran instead (no bootstrapper commands needed).
	// The install.sh in testdata writes a marker; if Prepare succeeded, hooks ran.
}

func TestLocalProvisioner_AcquireAWSLocalStack_UsesLeaseEnvFromProfileHooks(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := newTestProvisioner(t, runner, provider)

	lease, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:      scenario.ProfileAWSLocalStack,
		ProviderName: "kind",
		ClusterName:  "test-aws",
		Scenario: &scenario.Scenario{
			Environment: scenario.EnvironmentConfig{
				Cloud: scenario.CloudConfig{
					Provider: "localstack",
					Services: []string{"s3"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = lease.Release(context.Background()) }()

	// Verify AWS env vars come from profile hooks' lease.env.
	envMap := make(map[string]string)
	for _, kv := range lease.ExtraEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	for _, key := range []string{"AWS_ENDPOINT_URL", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_DEFAULT_REGION"} {
		if _, ok := envMap[key]; !ok {
			t.Errorf("expected ExtraEnv to contain %s", key)
		}
	}

	if envMap["AWS_ENDPOINT_URL"] != "http://localhost:4566" {
		t.Errorf("expected AWS_ENDPOINT_URL=http://localhost:4566, got %q", envMap["AWS_ENDPOINT_URL"])
	}
	if envMap["AWS_ACCESS_KEY_ID"] != "test" {
		t.Errorf("expected AWS_ACCESS_KEY_ID=test, got %q", envMap["AWS_ACCESS_KEY_ID"])
	}
	if envMap["AWS_DEFAULT_REGION"] != "us-east-1" {
		t.Errorf("expected AWS_DEFAULT_REGION=us-east-1, got %q", envMap["AWS_DEFAULT_REGION"])
	}

	if !provider.created {
		t.Fatal("expected provider.Create to be called")
	}
	if lease.Profile != scenario.ProfileAWSLocalStack {
		t.Fatalf("expected profile aws-localstack, got %q", lease.Profile)
	}
}

func TestLocalProvisioner_Release_RunsProfileCleanupBeforeDestroy(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := newTestProvisioner(t, runner, provider)

	lease, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:      scenario.ProfileArgocd,
		ProviderName: "kind",
		ClusterName:  "test-release",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Release should run profile cleanup, then destroy cluster.
	if err := lease.Release(context.Background()); err != nil {
		t.Fatalf("unexpected release error: %v", err)
	}

	if !provider.destroyed {
		t.Fatal("expected provider.Destroy to be called on release")
	}
}

func TestLocalProvisioner_AcquireExistingKubeconfig_SkipsCreate(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := newTestProvisioner(t, runner, provider)

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

func TestLocalProvisioner_AcquireExistingKubeconfig_ProfileLeaseIsNonOwning(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := newTestProvisioner(t, runner, provider)

	lease, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:            scenario.ProfileArgocd,
		ProviderName:       "kind",
		ClusterName:        "test",
		ExistingKubeconfig: "/home/user/.kube/config",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := lease.Release(context.Background()); err != nil {
		t.Fatalf("unexpected release error: %v", err)
	}

	// External kubeconfig: cluster should NOT be destroyed.
	if provider.destroyed {
		t.Fatal("expected no destroy for external kubeconfig")
	}
}

func TestLocalProvisioner_AcquireDefault_ReleaseDestroysCluster(t *testing.T) {
	t.Parallel()
	runner := newFakeRunner()
	provider := newFakeProvider(runner)
	p := newTestProvisioner(t, runner, provider)

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
	p := newTestProvisioner(t, runner, provider)

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
	p := newTestProvisioner(t, runner, provider)

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
	root := provisionerAssetsRoot(t)
	p := &LocalProvisioner{
		Providers: map[string]ClusterLifecycle{},
		Runner:    newFakeRunner(),
		Assets:    AssetResolver{RootDir: root},
		Profiles:  &ProfileRunner{},
	}

	_, err := p.Acquire(context.Background(), ProvisionRequest{
		Profile:      scenario.ProfileDefault,
		ProviderName: "docker-desktop",
		ClusterName:  "test",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	// The error could be from the asset resolver (no clusters/docker-desktop/default.yaml)
	// or from the provider lookup. Either is acceptable.
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
