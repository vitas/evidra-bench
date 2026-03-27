package environment

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// LocalStackStartFunc starts a LocalStack container and returns a handle.
type LocalStackStartFunc func(ctx context.Context, name string, services []string) (*LocalStackHandle, error)

// LocalStackStopFunc stops a running LocalStack container.
type LocalStackStopFunc func(ctx context.Context, handle *LocalStackHandle) error

// LocalProvisioner creates cluster leases using local ClusterLifecycle providers
// and installs profile-specific addons via the bootstrapper.
type LocalProvisioner struct {
	Providers       map[string]ClusterLifecycle
	Runner          CommandRunner
	StartLocalStack LocalStackStartFunc // defaults to StartLocalStack
	StopLocalStack  LocalStackStopFunc  // defaults to StopLocalStack
}

// NewLocalProvisioner returns a LocalProvisioner with the given providers and runner.
func NewLocalProvisioner(providers map[string]ClusterLifecycle, runner CommandRunner) *LocalProvisioner {
	return &LocalProvisioner{
		Providers:       providers,
		Runner:          runner,
		StartLocalStack: StartLocalStack,
		StopLocalStack:  StopLocalStack,
	}
}

// Acquire provisions an environment matching the requested profile and returns
// a lease. The caller must call Lease.Release when done.
func (p *LocalProvisioner) Acquire(ctx context.Context, req ProvisionRequest) (*Lease, error) {
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
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: unknown provider %q", req.ProviderName)
	}

	switch req.Profile {
	case scenario.ProfileDefault:
		return p.acquireDefault(ctx, req, provider)
	case scenario.ProfileArgocd:
		return p.acquireArgocd(ctx, req, provider)
	case scenario.ProfileAWSLocalStack:
		return p.acquireAWSLocalStack(ctx, req, provider)
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
	var handle *Handle
	var err error
	if req.ExistingKubeconfig != "" {
		handle = &Handle{ClusterName: req.ClusterName, KubeconfigPath: req.ExistingKubeconfig}
	} else {
		handle, err = p.createCluster(ctx, req, provider)
		if err != nil {
			return nil, err
		}
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

// acquireAWSLocalStack creates a cluster, starts a LocalStack container, and
// builds AWS environment variables including a wrapper script for the aws CLI.
func (p *LocalProvisioner) acquireAWSLocalStack(ctx context.Context, req ProvisionRequest, provider ClusterLifecycle) (*Lease, error) {
	var handle *Handle
	var err error
	if req.ExistingKubeconfig != "" {
		handle = &Handle{ClusterName: req.ClusterName, KubeconfigPath: req.ExistingKubeconfig}
	} else {
		handle, err = p.createCluster(ctx, req, provider)
		if err != nil {
			return nil, err
		}
	}

	// Determine which services to start.
	var services []string
	if req.Scenario != nil {
		services = req.Scenario.Environment.Cloud.Services
	}

	startLS := p.StartLocalStack
	if startLS == nil {
		startLS = StartLocalStack
	}
	stopLS := p.StopLocalStack
	if stopLS == nil {
		stopLS = StopLocalStack
	}

	lsHandle, err := startLS(ctx, req.ClusterName, services)
	if err != nil {
		if !req.ReuseCluster {
			_ = provider.Destroy(ctx, handle)
		}
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: start localstack: %w", err)
	}

	// Build AWS env vars.
	awsEnv := map[string]string{
		"AWS_ENDPOINT_URL":      lsHandle.EndpointURL,
		"AWS_ACCESS_KEY_ID":     "test",
		"AWS_SECRET_ACCESS_KEY": "test",
		"AWS_DEFAULT_REGION":    "us-east-1",
	}

	// Create a wrapper script for `aws` that injects --endpoint-url.
	// This works with all AWS CLI versions (env var requires 2.13+).
	awsBinDir, err := os.MkdirTemp("", "evidra-aws-bin-*")
	if err != nil {
		_ = stopLS(ctx, lsHandle)
		if !req.ReuseCluster {
			_ = provider.Destroy(ctx, handle)
		}
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: create aws wrapper dir: %w", err)
	}

	awsWrapper := filepath.Join(awsBinDir, "aws")
	if err := os.WriteFile(awsWrapper, []byte(fmt.Sprintf(
		"#!/bin/sh\nexec /usr/local/bin/aws --endpoint-url %s \"$@\"\n", lsHandle.EndpointURL,
	)), 0755); err != nil {
		_ = os.RemoveAll(awsBinDir)
		_ = stopLS(ctx, lsHandle)
		if !req.ReuseCluster {
			_ = provider.Destroy(ctx, handle)
		}
		return nil, fmt.Errorf("environment.LocalProvisioner.Acquire: write aws wrapper: %w", err)
	}

	awsEnv["PATH"] = awsBinDir + ":" + os.Getenv("PATH")

	// Convert to slice for ExtraEnv.
	var extraEnv []string
	for k, v := range awsEnv {
		extraEnv = append(extraEnv, fmt.Sprintf("%s=%s", k, v))
	}

	log.Printf("[provisioner] localstack started for cluster %s (services: %v)", handle.ClusterName, services)

	// Release stops LocalStack and removes the wrapper directory, then
	// optionally destroys the cluster.
	clusterRelease := p.releaseFunc(req, provider, handle)
	release := func(ctx context.Context) error {
		_ = os.RemoveAll(awsBinDir)
		if stopErr := stopLS(ctx, lsHandle); stopErr != nil {
			log.Printf("[provisioner] warning: stop localstack: %v", stopErr)
		}
		if clusterRelease != nil {
			return clusterRelease(ctx)
		}
		return nil
	}

	return &Lease{
		Profile:        req.Profile,
		KubeconfigPath: handle.KubeconfigPath,
		ExtraEnv:       extraEnv,
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
