package main

import (
	"context"
	"errors"
	"log"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

type batchLeaseProvisioner interface {
	Recreate(ctx context.Context, req environment.ProvisionRequest) (*environment.Lease, error)
}

func runWithBatchLeaseRecovery(
	ctx context.Context,
	cfg config.Config,
	s *scenario.Scenario,
	lease *environment.Lease,
	provisioner batchLeaseProvisioner,
	run func(*environment.Lease) (*harness.RunResult, error),
	logPrefix string,
) (*harness.RunResult, *environment.Lease, error) {
	result, err := run(lease)
	if err == nil || lease == nil || provisioner == nil {
		return result, lease, err
	}

	var infraErr *harness.InfraError
	if !errors.As(err, &infraErr) {
		return result, lease, err
	}

	log.Printf("[%s] infra error on reused cluster, recreating lease: %v", logPrefix, err)
	newLease, recreateErr := provisioner.Recreate(ctx, environment.ProvisionRequest{
		Scenario:           s,
		Profile:            s.ResolvedProfile(),
		ProviderName:       cfg.EnvironmentProvider,
		ClusterName:        cfg.ClusterName,
		ReuseCluster:       cfg.ReuseCluster,
		ExistingKubeconfig: cfg.KubeconfigPath,
		Shared:             true,
	})
	if recreateErr != nil {
		log.Printf("[%s] recreate failed: %v", logPrefix, recreateErr)
		return result, lease, err
	}

	if releaseErr := lease.Release(ctx); releaseErr != nil {
		log.Printf("[%s] warning: release failed lease: %v", logPrefix, releaseErr)
	}

	result, err = run(newLease)
	return result, newLease, err
}
