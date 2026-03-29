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

func TestEvidraFlags_TraceEvidra(t *testing.T) {
	t.Parallel()
	cfg := Config{TraceBackend: "evidra"}
	if got := EffectiveEvidenceMode(cfg); got != "proxy" {
		t.Fatalf("EffectiveEvidenceMode = %q, want proxy", got)
	}
}

func TestEvidraFlags_EvidraSmart(t *testing.T) {
	t.Parallel()
	cfg := Config{EvidraLevel: "smart"}
	if got := EffectiveEvidenceMode(cfg); got != "smart" {
		t.Fatalf("EffectiveEvidenceMode = %q, want smart", got)
	}
}

func TestEvidraFlags_EvidraFull(t *testing.T) {
	t.Parallel()
	cfg := Config{EvidraLevel: "full"}
	if got := EffectiveEvidenceMode(cfg); got != "mcp" {
		t.Fatalf("EffectiveEvidenceMode = %q, want mcp", got)
	}
}

func TestEvidraFlags_TracePlusEvidra(t *testing.T) {
	t.Parallel()
	cfg := Config{TraceBackend: "evidra", EvidraLevel: "smart"}
	// EvidraLevel takes precedence over TraceBackend.
	if got := EffectiveEvidenceMode(cfg); got != "smart" {
		t.Fatalf("EffectiveEvidenceMode = %q, want smart", got)
	}
}

func TestEvidraFlags_EvidraFullPlusMCPServer(t *testing.T) {
	t.Parallel()
	cfg := Config{EvidraLevel: "full", MCPServer: "npx @anthropic/mcp-server-kubernetes"}
	err := cfg.ValidateEvidraFlags()
	if err == nil {
		t.Fatal("expected validation error for --evidra full + --mcp-server")
	}
	if !strings.Contains(err.Error(), "cannot be combined with --mcp-server") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvidraFlags_EvidraSmartPlusA2A(t *testing.T) {
	t.Parallel()
	cfg := Config{EvidraLevel: "smart", Adapter: "a2a"}
	err := cfg.ValidateEvidraFlags()
	if err == nil {
		t.Fatal("expected validation error for --evidra smart + --adapter a2a")
	}
	if !strings.Contains(err.Error(), "cannot be combined with --adapter a2a") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvidraFlags_TracePlusMCPServer(t *testing.T) {
	t.Parallel()
	cfg := Config{TraceBackend: "evidra", MCPServer: "npx @anthropic/mcp-server-kubernetes"}
	if err := cfg.ValidateEvidraFlags(); err != nil {
		t.Fatalf("unexpected error: --trace evidra + --mcp-server should be valid: %v", err)
	}
}

func TestEvidraFlags_InvalidTrace(t *testing.T) {
	t.Parallel()
	cfg := Config{TraceBackend: "banana"}
	err := cfg.ValidateEvidraFlags()
	if err == nil {
		t.Fatal("expected validation error for --trace banana")
	}
	if !strings.Contains(err.Error(), "banana") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvidraFlags_InvalidEvidra(t *testing.T) {
	t.Parallel()
	cfg := Config{EvidraLevel: "banana"}
	err := cfg.ValidateEvidraFlags()
	if err == nil {
		t.Fatal("expected validation error for --evidra banana")
	}
	if !strings.Contains(err.Error(), "banana") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvidraFlags_LegacyProxyMode(t *testing.T) {
	t.Parallel()
	cfg := Config{ProxyMode: true}
	if got := EffectiveEvidenceMode(cfg); got != "proxy" {
		t.Fatalf("EffectiveEvidenceMode = %q, want proxy", got)
	}
}

func TestConfig_IgnoreUnsupportedEvidenceModeValues(t *testing.T) {
	t.Setenv("EVIDRA_BIN", "/env/evidra")
	t.Setenv("INFRA_BENCH_SYSTEM_PROMPT", "/env/system-prompt.md")

	cfg := ApplyEvidenceMode(Default(), "proxy")
	if cfg.EvidenceMode != "" {
		t.Fatalf("EvidenceMode = %q, want empty", cfg.EvidenceMode)
	}
	if got := cfg.ResolveEvidraBin(); got != "/env/evidra" {
		t.Fatalf("ResolveEvidraBin = %q, want env fallback", got)
	}
	if got := cfg.ResolveSystemPromptFile(); got != "/env/system-prompt.md" {
		t.Fatalf("ResolveSystemPromptFile = %q, want env fallback", got)
	}

	cfg = Default()
	cfg.EvidenceMode = "proxy"
	cfg.ProxyMode = true
	if got := EffectiveEvidenceMode(cfg); got != "proxy" {
		t.Fatalf("EffectiveEvidenceMode = %q, want proxy", got)
	}
}
