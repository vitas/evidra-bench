package config

import (
	"testing"
	"time"
)

func TestDefault_EnvironmentProvider(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if cfg.EnvironmentProvider != "kind" {
		t.Fatalf("expected kind, got %s", cfg.EnvironmentProvider)
	}
}

func TestDefault_Adapter(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if cfg.Adapter != "cli" {
		t.Fatalf("expected cli, got %s", cfg.Adapter)
	}
}

func TestDefault_RunsDir(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if cfg.RunsDir != "runs" {
		t.Fatalf("expected runs, got %s", cfg.RunsDir)
	}
}

func TestDefault_Timeout(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if cfg.Timeout != 5*time.Minute {
		t.Fatalf("expected 5m, got %s", cfg.Timeout)
	}
}

func TestValidate_MissingScenario(t *testing.T) {
	t.Parallel()
	cfg := Default()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing scenario")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.Scenario = "kubernetes/broken-deployment"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
