package main

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

func TestCertifyFiltering_ExcludesIncompatibleAndSkipped(t *testing.T) {
	t.Parallel()

	// Tests the filtering logic used by runCertifyTrack and executeCertifySingle.
	// Full integration test of runCertifyTrack requires real scenario files
	// on disk; the provider guard in runScenarioOnce is the backstop.

	scenarios := []*scenario.Scenario{
		{
			ID: "s1", Track: "workloads", Level: "L1",
			Environment: scenario.EnvironmentConfig{},
		},
		{
			ID: "s2", Track: "workloads", Level: "L2",
			Environment: scenario.EnvironmentConfig{Providers: []string{"k3d"}},
		},
		{
			ID: "s3", Track: "workloads", Level: "L2",
			Environment: scenario.EnvironmentConfig{Providers: []string{"kind"}},
		},
		{
			ID: "s4", Track: "workloads", Level: "L1",
			Skip: true, SkipReason: "not ready",
		},
	}

	cfg := config.Default()
	cfg.EnvironmentProvider = "kind"

	runnable, skipped := filterRunnableScenarios(scenarios, cfg.EnvironmentProvider, io.Discard)

	// s1 (no providers = all), s3 (kind) should be runnable.
	// s2 (k3d only), s4 (skip) should be skipped.
	if len(runnable) != 2 {
		t.Fatalf("expected 2 runnable, got %d", len(runnable))
	}
	if runnable[0].ID != "s1" || runnable[1].ID != "s3" {
		t.Fatalf("unexpected runnable: %s, %s", runnable[0].ID, runnable[1].ID)
	}
	if skipped != 2 {
		t.Fatalf("expected 2 skipped, got %d", skipped)
	}
}

func TestCertifySequential_ReuseClusterMixedProfiles_FailsFast(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{
			ID:    "s1",
			Track: "workloads",
			Level: "L1",
			Environment: scenario.EnvironmentConfig{
				Profile: scenario.ProfileDefault,
			},
		},
		{
			ID:    "s2",
			Track: "workloads",
			Level: "L2",
			Environment: scenario.EnvironmentConfig{
				Profile: scenario.ProfileArgocd,
			},
		},
	}

	err := validateSingleProfile(scenarios)
	if err == nil {
		t.Fatal("expected error for mixed profiles in certify")
	}
	if !strings.Contains(err.Error(), "--reuse-cluster") {
		t.Fatalf("error should mention --reuse-cluster, got: %v", err)
	}
}

func TestCertifySequential_ReuseClusterSingleProfile_Passes(t *testing.T) {
	t.Parallel()

	scenarios := []*scenario.Scenario{
		{ID: "s1", Track: "workloads", Level: "L1", Environment: scenario.EnvironmentConfig{}},
		{ID: "s2", Track: "workloads", Level: "L2", Environment: scenario.EnvironmentConfig{}},
	}

	if err := validateSingleProfile(scenarios); err != nil {
		t.Fatalf("expected no error for same profile, got: %v", err)
	}
}

func TestProviderGuardInSharedRunPaths(t *testing.T) {
	t.Parallel()

	// Verify that runScenarioOnce rejects incompatible scenarios directly,
	// not just through the caller-level filter. This is the backstop that
	// catches callers (like skill-delta run) that bypass filterRunnableScenarios.

	s := &scenario.Scenario{
		ID:          "k3d-only-scenario",
		Environment: scenario.EnvironmentConfig{Providers: []string{"k3d"}},
	}

	cfg := config.Default()
	cfg.EnvironmentProvider = "kind"
	cfg.DryRun = true

	_, err := runScenarioOnce(t.Context(), cfg, s)
	if err == nil {
		t.Fatal("expected provider incompatibility error")
	}

	var incompatible *scenario.IncompatibleProviderError
	if !errors.As(err, &incompatible) {
		t.Fatalf("expected IncompatibleProviderError, got: %T: %v", err, err)
	}
}
