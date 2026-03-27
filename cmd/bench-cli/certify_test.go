package main

import (
	"io"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func TestRunCertifySingle_ExcludesIncompatibleScenariosFromTotals(t *testing.T) {
	t.Parallel()

	// runCertifySingle loads from disk, so we need real scenario files.
	// Instead, we test the filtering logic directly: given a set of scenarios
	// with mixed provider compatibility, verify that filterRunnableScenarios
	// (used by runCertifySingle) produces correct totals and skipped count.

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

	// Verify the Skipped field would be set correctly in CertResult.
	cert := CertResult{
		Total:   len(runnable),
		Skipped: skipped,
	}
	if cert.Total != 2 {
		t.Fatalf("Total = %d, want 2", cert.Total)
	}
	if cert.Skipped != 2 {
		t.Fatalf("Skipped = %d, want 2", cert.Skipped)
	}
}
