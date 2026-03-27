// Package environment manages disposable cluster lifecycles.
package environment

import (
	"context"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// Handle holds references to a provisioned environment.
type Handle struct {
	ClusterName    string
	KubeconfigPath string
}

// ClusterLifecycle is the full contract for cluster management.
// Kind, k3d, remote clusters — all implement this interface.
type ClusterLifecycle interface {
	// Provisioning — k8s config allows providers to apply CNI, addons, runtimes, features.
	// When k8s is zero-valued, creates a plain cluster.
	Create(ctx context.Context, clusterName string, k8s scenario.KubernetesConfig) (*Handle, error)
	Destroy(ctx context.Context, handle *Handle) error
	Recreate(ctx context.Context, clusterName string, k8s scenario.KubernetesConfig) (*Handle, error)

	// Health
	HealthCheck(ctx context.Context, kubeconfigPath string) error

	// Namespace lifecycle
	ForceDeleteNamespace(ctx context.Context, kubeconfigPath, ns string) error
	CreateNamespace(ctx context.Context, kubeconfigPath, ns string) error

	// Scheduling verification
	RunCanary(ctx context.Context, kubeconfigPath, ns string) error
}
