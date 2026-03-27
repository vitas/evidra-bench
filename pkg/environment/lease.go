package environment

import (
	"context"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// Lease represents an acquired environment ready for scenario execution.
// Call Release when done to clean up the underlying resources (destroy cluster,
// stop LocalStack, etc.).
type Lease struct {
	Profile        scenario.ExecutionProfile
	KubeconfigPath string
	ExtraEnv       []string
	Provider       ClusterLifecycle
	Shared         bool
	release        func(context.Context) error
}

// Release frees all resources held by this lease.
func (l *Lease) Release(ctx context.Context) error {
	if l == nil || l.release == nil {
		return nil
	}
	return l.release(ctx)
}

// ProvisionRequest describes what the caller needs from the provisioner.
type ProvisionRequest struct {
	Scenario           *scenario.Scenario
	Profile            scenario.ExecutionProfile
	ProviderName       string
	ClusterName        string
	ReuseCluster       bool
	ExistingKubeconfig string
	Shared             bool
}

// Provisioner acquires environment leases for scenario execution.
type Provisioner interface {
	Acquire(ctx context.Context, req ProvisionRequest) (*Lease, error)
}
