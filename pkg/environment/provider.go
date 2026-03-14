// Package environment manages disposable cluster lifecycles.
package environment

import "context"

// Handle holds references to a provisioned environment.
type Handle struct {
	ClusterName    string
	KubeconfigPath string
}

// Provider defines the contract for environment lifecycle management.
type Provider interface {
	Create(ctx context.Context, clusterName string) (*Handle, error)
	Destroy(ctx context.Context, handle *Handle) error
}
