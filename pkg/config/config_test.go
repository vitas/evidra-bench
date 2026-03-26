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

func TestValidate_MissingAgentCommand(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.Scenario = "kubernetes/broken-deployment"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing agent-command")
	}
}

func TestValidate_DryRunSkipsAgentCommand(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.Scenario = "kubernetes/broken-deployment"
	cfg.DryRun = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.Scenario = "kubernetes/broken-deployment"
	cfg.AgentCommand = "my-agent"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfig_ResolveDatabaseURL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		envVal   string
		expected string
	}{
		{"flag wins", Config{DatabaseURL: "postgres://flag"}, "postgres://env", "postgres://flag"},
		{"env fallback", Config{}, "postgres://env", "postgres://env"},
		{"empty", Config{}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				t.Setenv("BENCH_DATABASE_URL", tt.envVal)
			} else {
				t.Setenv("BENCH_DATABASE_URL", "")
			}
			got := tt.cfg.ResolveDatabaseURL()
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestConfig_Parallel_Default(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	if cfg.Parallel != 0 {
		t.Errorf("default Parallel should be 0 (sequential), got %d", cfg.Parallel)
	}
}

func TestConfig_ResolveEvidraBinAndPromptFile_SuppressEnvFallbackForAuthoritativeEvidenceMode(t *testing.T) {
	t.Setenv("EVIDRA_BIN", "/env/evidra")
	t.Setenv("INFRA_BENCH_SYSTEM_PROMPT", "/env/system-prompt.md")

	for _, mode := range []string{"none", "smart"} {
		cfg := ApplyEvidenceMode(Default(), mode)
		if got := cfg.ResolveEvidraBin(); got != "" {
			t.Fatalf("mode %q ResolveEvidraBin = %q, want empty", mode, got)
		}
		if got := cfg.ResolveSystemPromptFile(); got != "" {
			t.Fatalf("mode %q ResolveSystemPromptFile = %q, want empty", mode, got)
		}
	}
}

func TestConfig_ResolveEnvFallback_EmptyEvidenceModePreservesLegacyBehavior(t *testing.T) {
	t.Setenv("EVIDRA_BIN", "/env/evidra")
	t.Setenv("INFRA_BENCH_SYSTEM_PROMPT", "/env/system-prompt.md")

	cfg := Default()
	if got := cfg.ResolveEvidraBin(); got != "/env/evidra" {
		t.Fatalf("ResolveEvidraBin = %q, want env fallback", got)
	}
	if got := cfg.ResolveSystemPromptFile(); got != "/env/system-prompt.md" {
		t.Fatalf("ResolveSystemPromptFile = %q, want env fallback", got)
	}
}
