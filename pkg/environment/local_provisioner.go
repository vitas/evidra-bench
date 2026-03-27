package environment

import (
	"context"
	"fmt"
	"log"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// LocalProvisioner creates cluster leases using local ClusterLifecycle providers
// and installs profile-specific addons via the bootstrapper.
type LocalProvisioner struct {
	Providers map[string]ClusterLifecycle
	Runner    CommandRunner
}

// NewLocalProvisioner returns a LocalProvisioner with the given providers and runner.
func NewLocalProvisioner(providers map[string]ClusterLifecycle, runner CommandRunner) *LocalProvisioner {
	return &LocalProvisioner{
		Providers: providers,
		Runner:    runner,
	}
}

// Acquire provisions an environment matching the requested profile and returns
// a lease. The caller must call Lease.Release when done.
func (p *LocalProvisioner) Acquire(ctx context.Context, req ProvisionRequest) (*Lease, error) {
	// External kubeconfig: return a lease with the existing path, no create/destroy.
	if req.ExistingKubeconfig != "" {
		return &Lease{
			Profile:        req.Profile,
			KubeconfigPath: req.ExistingKubeconfig,
			Shared:         req.Shared,
		}, nil
	}

	provider, ok := p.Providers[req.ProviderName]
	if !ok {
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: unknown provider %q", req.ProviderName)
	}

	switch req.Profile {
	case scenario.ProfileDefault:
		return p.acquireDefault(ctx, req, provider)
	case scenario.ProfileArgocd:
		return p.acquireArgocd(ctx, req, provider)
	case scenario.ProfileAWSLocalStack:
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: profile %q not yet implemented", req.Profile)
	default:
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: unsupported profile %q", req.Profile)
	}
}

// acquireDefault creates a plain cluster with no extra addons.
func (p *LocalProvisioner) acquireDefault(ctx context.Context, req ProvisionRequest, provider ClusterLifecycle) (*Lease, error) {
	handle, err := p.createCluster(ctx, req, provider)
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

// acquireArgocd creates a cluster and installs the ArgoCD addon.
func (p *LocalProvisioner) acquireArgocd(ctx context.Context, req ProvisionRequest, provider ClusterLifecycle) (*Lease, error) {
	handle, err := p.createCluster(ctx, req, provider)
	if err != nil {
		return nil, err
	}

	// Install ArgoCD via addon registry.
	addon, ok := AddonRegistry["argocd"]
	if !ok {
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: argocd addon not found in registry")
	}

	plan := &BootstrapPlan{Steps: addon.Steps}
	bootstrapper := NewBootstrapper(p.Runner)
	if err := bootstrapper.Execute(ctx, plan, handle.KubeconfigPath); err != nil {
		// Best-effort cleanup on failure.
		if !req.ReuseCluster {
			_ = provider.Destroy(ctx, handle)
		}
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: install argocd: %w", err)
	}

	log.Printf("[provisioner] argocd addon installed on cluster %s", handle.ClusterName)

	release := p.releaseFunc(req, provider, handle)

	return &Lease{
		Profile:        req.Profile,
		KubeconfigPath: handle.KubeconfigPath,
		Provider:       provider,
		Shared:         req.Shared,
		release:        release,
	}, nil
}

// createCluster provisions a cluster using the provider, respecting reuse settings.
func (p *LocalProvisioner) createCluster(ctx context.Context, req ProvisionRequest, provider ClusterLifecycle) (*Handle, error) {
	var k8s scenario.KubernetesConfig
	if req.Scenario != nil {
		k8s = req.Scenario.Environment.Kubernetes
	}
	handle, err := provider.Create(ctx, req.ClusterName, k8s)
	if err != nil {
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: create cluster: %w", err)
	}
	return handle, nil
}

// releaseFunc returns the cleanup function for a lease. When ReuseCluster is
// set, the cluster is kept alive; otherwise it is destroyed.
func (p *LocalProvisioner) releaseFunc(req ProvisionRequest, provider ClusterLifecycle, handle *Handle) func(context.Context) error {
	if req.ReuseCluster {
		return nil
	}
	return func(ctx context.Context) error {
		return provider.Destroy(ctx, handle)
	}
}
