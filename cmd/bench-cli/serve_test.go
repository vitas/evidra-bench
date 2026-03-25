package main

import (
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
)

func TestBuildCertifyRunConfig_UsesRequestOverrides(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"
	base.Adapter = "cli"
	base.Timeout = 5 * time.Minute

	req := CertifyRequest{
		Model:    "sonnet",
		Provider: "bifrost",
	}
	req.Config.Adapter = "mcp"
	req.Config.TimeoutPerScenario = 120

	got := buildCertifyRunConfig(base, req)

	if got.Provider != "bifrost" {
		t.Fatalf("Provider = %q, want bifrost", got.Provider)
	}
	if got.Adapter != "mcp" {
		t.Fatalf("Adapter = %q, want mcp", got.Adapter)
	}
	if got.Timeout != 120*time.Second {
		t.Fatalf("Timeout = %s, want 2m0s", got.Timeout)
	}
}

func TestBuildCertifyRunConfig_UsesFallbacksWhenRequestOmitted(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "anthropic"
	base.Adapter = "cli"
	base.Timeout = 3 * time.Minute

	got := buildCertifyRunConfig(base, CertifyRequest{Model: "haiku"})

	if got.Provider != "anthropic" {
		t.Fatalf("Provider = %q, want anthropic", got.Provider)
	}
	if got.Adapter != "cli" {
		t.Fatalf("Adapter = %q, want cli", got.Adapter)
	}
	if got.Timeout != 3*time.Minute {
		t.Fatalf("Timeout = %s, want 3m0s", got.Timeout)
	}
}
