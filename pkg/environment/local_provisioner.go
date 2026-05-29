package environment

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

// LocalProvisioner creates cluster leases using local ClusterLifecycle providers
// and runs profile hooks (install, healthcheck, cleanup) from checked-in assets.
type LocalProvisioner struct {
	Providers map[string]ClusterLifecycle
	Runner    CommandRunner
	Assets    AssetResolver
	Profiles  *ProfileRunner
}

// NewLocalProvisioner returns a LocalProvisioner with the given providers, runner,
// and asset root directory. The assetsRoot is the repository root containing the
// clusters/ and profiles/ directories.
func NewLocalProvisioner(providers map[string]ClusterLifecycle, runner CommandRunner, assetsRoot string) *LocalProvisioner {
	return &LocalProvisioner{
		Providers: providers,
		Runner:    runner,
		Assets:    AssetResolver{RootDir: assetsRoot},
		Profiles:  &ProfileRunner{},
	}
}

// Acquire provisions an environment matching the requested profile and returns
// a lease. The caller must call Lease.Release when done.
func (p *LocalProvisioner) Acquire(ctx context.Context, req ProvisionRequest) (*Lease, error) {
	return p.acquire(ctx, req, false)
}

// Recreate refreshes the requested environment. Owned clusters are recreated
// through the provider; external kubeconfigs keep the same cluster and rerun
// profile setup only.
func (p *LocalProvisioner) Recreate(ctx context.Context, req ProvisionRequest) (*Lease, error) {
	return p.acquire(ctx, req, true)
}

func (p *LocalProvisioner) acquire(ctx context.Context, req ProvisionRequest, recreate bool) (*Lease, error) {
	// Resolve provider — nil is valid for external kubeconfig with default profile.
	provider := p.Providers[req.ProviderName]

	// External kubeconfig: skip cluster create/destroy but still run profile-specific setup.
	if req.ExistingKubeconfig != "" {
		if req.Profile == scenario.ProfileDefault {
			return &Lease{
				Profile:        req.Profile,
				KubeconfigPath: req.ExistingKubeconfig,
				Provider:       provider,
				Shared:         req.Shared,
			}, nil
		}
		// Non-default profiles still need setup (ArgoCD addon, LocalStack, etc.)
		// Fall through to profile switch with the external kubeconfig.
	} else if provider == nil {
		return nil, fmt.Errorf("environment.LocalProvisioner.acquire: unknown provider %q", req.ProviderName)
	}

	switch req.Profile {
	case scenario.ProfileDefault:
		return p.acquireDefault(ctx, req, provider, recreate)
	case scenario.ProfileMultiNode:
		return p.acquireDefault(ctx, req, provider, recreate)
	case scenario.ProfileArgocd:
		return p.acquireArgocd(ctx, req, provider, recreate)
	case scenario.ProfileAWSLocalStack:
		return p.acquireAWSLocalStack(ctx, req, provider, recreate)
	default:
		return nil, fmt.Errorf("environment.LocalProvisioner.acquire: unsupported profile %q", req.Profile)
	}
}

// acquireDefault creates a plain cluster with no extra addons.
func (p *LocalProvisioner) acquireDefault(ctx context.Context, req ProvisionRequest, provider ClusterLifecycle, recreate bool) (*Lease, error) {
	assets, err := p.Assets.Resolve(req.ProviderName, req.Profile)
	if err != nil {
		return nil, fmt.Errorf("environment.LocalProvisioner.acquireDefault: %w", err)
	}

	handle, err := p.provisionCluster(ctx, req, provider, assets, recreate)
	if err != nil {
		return nil, err
	}

	release := p.releaseFunc(req, provider, handle)

	return &Lease{
		Profile:        req.Profile,
		KubeconfigPath: handle.KubeconfigPath,
		Provider:       provider,
		Shared:         req.Shared,
		release:        release,
	}, nil
}

// acquireArgocd creates a cluster and runs the argocd profile hooks.
func (p *LocalProvisioner) acquireArgocd(ctx context.Context, req ProvisionRequest, provider ClusterLifecycle, recreate bool) (*Lease, error) {
	assets, err := p.Assets.Resolve(req.ProviderName, req.Profile)
	if err != nil {
		return nil, fmt.Errorf("environment.LocalProvisioner.acquireArgocd: %w", err)
	}

	handle, err := p.provisionCluster(ctx, req, provider, assets, recreate)
	if err != nil {
		return nil, err
	}

	// Run profile hooks (install.sh, healthcheck.sh).
	profileResult, err := p.Profiles.Prepare(ctx, ProfileRunRequest{
		Assets:      assets,
		Profile:     req.Profile,
		Provider:    req.ProviderName,
		ClusterName: handle.ClusterName,
		Kubeconfig:  handle.KubeconfigPath,
	})
	if err != nil {
		if p.ownsCluster(req) && !req.ReuseCluster {
			_ = provider.Destroy(ctx, handle)
		}
		return nil, fmt.Errorf("environment.LocalProvisioner.acquireArgocd: %w", err)
	}

	log.Printf("[provisioner] argocd profile installed on cluster %s", handle.ClusterName)

	// Compose release: profile cleanup first, then cluster destroy.
	clusterRelease := p.releaseFunc(req, provider, handle)
	release := p.composeRelease(profileResult.Release, clusterRelease)

	return &Lease{
		Profile:        req.Profile,
		KubeconfigPath: handle.KubeconfigPath,
		Provider:       provider,
		Shared:         req.Shared,
		release:        release,
	}, nil
}

// acquireAWSLocalStack creates a cluster and runs the aws-localstack profile
// hooks, which start LocalStack and produce lease.env with AWS credentials.
func (p *LocalProvisioner) acquireAWSLocalStack(ctx context.Context, req ProvisionRequest, provider ClusterLifecycle, recreate bool) (*Lease, error) {
	assets, err := p.Assets.Resolve(req.ProviderName, req.Profile)
	if err != nil {
		return nil, fmt.Errorf("environment.LocalProvisioner.acquireAWSLocalStack: %w", err)
	}

	handle, err := p.provisionCluster(ctx, req, provider, assets, recreate)
	if err != nil {
		return nil, err
	}

	// Build extra env for the profile hooks.
	extraEnv := make(map[string]string)
	if req.Scenario != nil && len(req.Scenario.Environment.Cloud.Services) > 0 {
		extraEnv["BENCH_LOCALSTACK_SERVICES"] = strings.Join(req.Scenario.Environment.Cloud.Services, ",")
	}

	// Run profile hooks (install.sh starts LocalStack, writes lease.env).
	profileResult, err := p.Profiles.Prepare(ctx, ProfileRunRequest{
		Assets:      assets,
		Profile:     req.Profile,
		Provider:    req.ProviderName,
		ClusterName: handle.ClusterName,
		Kubeconfig:  handle.KubeconfigPath,
		ExtraEnv:    extraEnv,
	})
	if err != nil {
		if p.ownsCluster(req) && !req.ReuseCluster {
			_ = provider.Destroy(ctx, handle)
		}
		return nil, fmt.Errorf("environment.LocalProvisioner.acquireAWSLocalStack: %w", err)
	}

	log.Printf("[provisioner] aws-localstack profile installed on cluster %s", handle.ClusterName)

	// Compose release: profile cleanup first, then cluster destroy.
	clusterRelease := p.releaseFunc(req, provider, handle)
	release := p.composeRelease(profileResult.Release, clusterRelease)

	return &Lease{
		Profile:        req.Profile,
		KubeconfigPath: handle.KubeconfigPath,
		ExtraEnv:       profileResult.ExtraEnv,
		Provider:       provider,
		Shared:         req.Shared,
		release:        release,
	}, nil
}

func (p *LocalProvisioner) provisionCluster(ctx context.Context, req ProvisionRequest, provider ClusterLifecycle, assets ProfileAssets, recreate bool) (*Handle, error) {
	if !p.ownsCluster(req) {
		return &Handle{ClusterName: req.ClusterName, KubeconfigPath: req.ExistingKubeconfig}, nil
	}

	spec := ClusterSpec{
		ConfigPath: assets.ClusterConfigPath,
	}
	if req.Scenario != nil {
		spec.LegacyKubernetes = req.Scenario.Environment.Kubernetes
	}

	var (
		handle *Handle
		err    error
	)
	if recreate {
		handle, err = provider.Recreate(ctx, req.ClusterName, spec)
		if err != nil {
			return nil, fmt.Errorf("environment.LocalProvisioner.Recreate: recreate cluster: %w", err)
		}
		return handle, nil
	}

	handle, err = provider.Create(ctx, req.ClusterName, spec)
	if err != nil {
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: create cluster: %w", err)
	}
	return handle, nil
}

func (p *LocalProvisioner) ownsCluster(req ProvisionRequest) bool {
	return req.ExistingKubeconfig == ""
}

// releaseFunc returns the cleanup function for a lease. When ReuseCluster is
// set, the cluster is kept alive; otherwise it is destroyed.
func (p *LocalProvisioner) releaseFunc(req ProvisionRequest, provider ClusterLifecycle, handle *Handle) func(context.Context) error {
	if !p.ownsCluster(req) || req.ReuseCluster || provider == nil {
		return nil
	}
	return func(ctx context.Context) error {
		return provider.Destroy(ctx, handle)
	}
}

// composeRelease returns a function that runs profileRelease first, then
// clusterRelease. Either may be nil.
func (p *LocalProvisioner) composeRelease(profileRelease, clusterRelease func(context.Context) error) func(context.Context) error {
	if profileRelease == nil && clusterRelease == nil {
		return nil
	}
	return func(ctx context.Context) error {
		if profileRelease != nil {
			if err := profileRelease(ctx); err != nil {
				log.Printf("[provisioner] warning: profile cleanup: %v", err)
			}
		}
		if clusterRelease != nil {
			return clusterRelease(ctx)
		}
		return nil
	}
}
