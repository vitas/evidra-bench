package config

import (
	"strings"
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

func TestConfig_ResolveA2AAgentURL_FlagWins(t *testing.T) {
	t.Setenv("INFRA_BENCH_A2A_AGENT_URL", "http://env-agent.example")

	cfg := Default()
	cfg.A2AAgentURL = "http://flag-agent.example"

	if got := cfg.ResolveA2AAgentURL(); got != "http://flag-agent.example" {
		t.Fatalf("ResolveA2AAgentURL = %q, want flag value", got)
	}
}

func TestValidate_A2ARequiresAgentURL(t *testing.T) {
	t.Setenv("INFRA_BENCH_A2A_AGENT_URL", "")

	cfg := Default()
	cfg.Scenario = "kubernetes/broken-deployment"
	cfg.Adapter = "a2a"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "INFRA_BENCH_A2A_AGENT_URL") {
		t.Fatalf("expected missing A2A agent URL error, got %v", err)
	}
}

func TestValidate_A2AAllowsNoAgentCommandOrProvider(t *testing.T) {
	cfg := Default()
	cfg.Scenario = "kubernetes/broken-deployment"
	cfg.Adapter = "a2a"
	cfg.A2AAgentURL = "http://agent.example"

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

func TestConfig_ResolveSystemPromptFile_UsesEnvFallback(t *testing.T) {
	t.Setenv("INFRA_BENCH_SYSTEM_PROMPT", "/env/system-prompt.md")

	cfg := Default()
	if got := cfg.ResolveSystemPromptFile(); got != "/env/system-prompt.md" {
		t.Fatalf("ResolveSystemPromptFile = %q, want env fallback", got)
	}
}

func TestEffectiveEvidenceMode_MCPServer(t *testing.T) {
	t.Parallel()
	cfg := Config{MCPServer: "evidra-mcp --signing-mode optional"}
	if got := EffectiveEvidenceMode(cfg); got != "mcp" {
		t.Fatalf("EffectiveEvidenceMode = %q, want mcp", got)
	}
}

func TestEffectiveEvidenceMode_DefaultNone(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	if got := EffectiveEvidenceMode(cfg); got != "none" {
		t.Fatalf("EffectiveEvidenceMode = %q, want none", got)
	}
}

func TestEffectiveEvidenceMode_ExplicitNoneWins(t *testing.T) {
	t.Parallel()
	cfg := Config{EvidenceMode: "none", MCPServer: "evidra-mcp --signing-mode optional"}
	if got := EffectiveEvidenceMode(cfg); got != "none" {
		t.Fatalf("EffectiveEvidenceMode = %q, want none", got)
	}
}

func TestApplyEvidenceMode_NoneClearsMCPServer(t *testing.T) {
	t.Parallel()
	cfg := ApplyEvidenceMode(Config{
		MCPServer:        "evidra-mcp --signing-mode optional",
		SystemPromptFile: "/tmp/system.md",
		Role:             "k8s-admin",
		ContractVersion:  "v1",
	}, "none")
	if cfg.EvidenceMode != "none" {
		t.Fatalf("EvidenceMode = %q, want none", cfg.EvidenceMode)
	}
	if cfg.MCPServer != "" {
		t.Fatalf("MCPServer = %q, want empty", cfg.MCPServer)
	}
}

func TestConfig_IgnoreUnsupportedEvidenceModeValues(t *testing.T) {
	t.Parallel()
	cfg := ApplyEvidenceMode(Default(), "legacy")
	if cfg.EvidenceMode != "" {
		t.Fatalf("EvidenceMode = %q, want empty", cfg.EvidenceMode)
	}
}
