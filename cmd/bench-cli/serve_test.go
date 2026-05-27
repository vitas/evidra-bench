package main

import (
	"testing"
)

func TestResolveServeTenants_UsesBenchPublicTenantAsFallback(t *testing.T) {
	t.Setenv("BENCH_DEFAULT_TENANT", "")
	t.Setenv("BENCH_PUBLIC_TENANT", "tnt-public")

	defaultTenant, publicTenant := resolveServeTenants()

	if defaultTenant != "tnt-public" {
		t.Fatalf("defaultTenant = %q, want tnt-public", defaultTenant)
	}
	if publicTenant != "tnt-public" {
		t.Fatalf("publicTenant = %q, want tnt-public", publicTenant)
	}
}

func TestResolveServeTenants_AllowsSeparateAuthenticatedTenant(t *testing.T) {
	t.Setenv("BENCH_DEFAULT_TENANT", "tenant-auth")
	t.Setenv("BENCH_PUBLIC_TENANT", "tenant-public")

	defaultTenant, publicTenant := resolveServeTenants()

	if defaultTenant != "tenant-auth" {
		t.Fatalf("defaultTenant = %q, want tenant-auth", defaultTenant)
	}
	if publicTenant != "tenant-public" {
		t.Fatalf("publicTenant = %q, want tenant-public", publicTenant)
	}
}

func TestApplyServeEnvOptions_HumanReviewDraftMode(t *testing.T) {
	t.Setenv("BENCH_REVIEW_DRAFT_MODE", "human")

	opts := applyServeEnvOptions(serveOptions{})

	if opts.ReviewDraftMode != "human" {
		t.Fatalf("ReviewDraftMode = %q, want human", opts.ReviewDraftMode)
	}
}
