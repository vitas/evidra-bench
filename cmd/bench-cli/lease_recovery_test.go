package main

import (
	"context"
	"errors"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

type fakeLeaseRecoveryProvisioner struct {
	recreateCalls int
	lastReq       environment.ProvisionRequest
	lease         *environment.Lease
	err           error
}

func (f *fakeLeaseRecoveryProvisioner) Recreate(_ context.Context, req environment.ProvisionRequest) (*environment.Lease, error) {
	f.recreateCalls++
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.lease, nil
}

func TestRunWithBatchLeaseRecovery_RecreatesAndRetriesOnInfraError(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.EnvironmentProvider = "kind"
	cfg.ClusterName = "bench-cli"
	cfg.ReuseCluster = true

	s := &scenario.Scenario{
		ID: "argocd/scenario",
		Environment: scenario.EnvironmentConfig{
			Profile: scenario.ProfileArgocd,
		},
	}

	initial := &environment.Lease{KubeconfigPath: "/tmp/old"}
	replacement := &environment.Lease{KubeconfigPath: "/tmp/new"}
	provisioner := &fakeLeaseRecoveryProvisioner{lease: replacement}

	calls := 0
	result, lease, err := runWithBatchLeaseRecovery(context.Background(), cfg, s, initial, provisioner, func(l *environment.Lease) (*harness.RunResult, error) {
		calls++
		if calls == 1 {
			if l != initial {
				t.Fatalf("first run used wrong lease: got %#v want %#v", l, initial)
			}
			return nil, &harness.InfraError{Err: errors.New("canary failed")}
		}
		if l != replacement {
			t.Fatalf("retry used wrong lease: got %#v want %#v", l, replacement)
		}
		return &harness.RunResult{ScenarioID: s.ID, Passed: true}, nil
	}, "bench")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.Passed {
		t.Fatalf("expected passing retry result, got %#v", result)
	}
	if lease != replacement {
		t.Fatalf("expected replacement lease, got %#v", lease)
	}
	if calls != 2 {
		t.Fatalf("expected 2 run attempts, got %d", calls)
	}
	if provisioner.recreateCalls != 1 {
		t.Fatalf("expected one recreate call, got %d", provisioner.recreateCalls)
	}
	if provisioner.lastReq.Profile != scenario.ProfileArgocd {
		t.Fatalf("expected argocd profile, got %q", provisioner.lastReq.Profile)
	}
	if provisioner.lastReq.ClusterName != cfg.ClusterName {
		t.Fatalf("expected cluster %q, got %q", cfg.ClusterName, provisioner.lastReq.ClusterName)
	}
	if !provisioner.lastReq.ReuseCluster {
		t.Fatal("expected recreate request to preserve reuse-cluster semantics")
	}
}
