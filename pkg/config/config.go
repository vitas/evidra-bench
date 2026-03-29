// Package config defines run configuration for bench-cli.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// DefaultNamespace is the default Kubernetes namespace for benchmark scenarios.
const DefaultNamespace = "bench"

// GracefulStopTimeout is the timeout for graceful shutdown of River workers.
const GracefulStopTimeout = 30 * time.Second

// Config holds all settings for a benchmark run.
type Config struct {
	EnvironmentProvider string
	Scenario            string
	ScenariosDir        string
	Adapter             string
	A2AAgentURL         string
	AgentCommand        string
	RunsDir             string
	KubeconfigPath      string
	Timeout             time.Duration
	ReuseCluster        bool
	ClusterName         string
	DryRun              bool
	EvidraURL           string
	EvidraAPIKey        string
	EvidraEvidenceDir   string
	EvidraBin           string
	Model               string
	Provider            string
	MemoryWindow        int
	SystemPromptFile    string
	ContractVersion     string
	Role                string // role-based skill (k8s-admin, security-ops, release-manager, platform-eng)
	MCPServer           string // MCP server command (e.g. "evidra-mcp --signing-mode optional")
	ProxyMode           bool   // auto-record evidence for mutations without agent involvement
	SmartPrescribe      bool   // simplified prescribe (tool+operation, no artifact)
	EvidenceMode        string // explicit per-run override for evidence mode
	TraceBackend        string // passive mutation recording backend: "evidra", "" (none)
	EvidraLevel         string // evidra protocol level: "smart", "full", "" (off)
	Parallel            int    // number of parallel workers (0 or 1 = sequential, >1 requires --database-url)
	DatabaseURL         string // PostgreSQL connection string for River job queue (env: BENCH_DATABASE_URL)
}

// ResolveSystemPromptFile returns the system prompt file path from flag, env, or empty.
// Priority: flag > INFRA_BENCH_SYSTEM_PROMPT > empty (use default).
func (c *Config) ResolveSystemPromptFile() string {
	if c.suppressesEvidenceFallbacks() {
		return ""
	}
	if c.SystemPromptFile != "" {
		return c.SystemPromptFile
	}
	return os.Getenv("INFRA_BENCH_SYSTEM_PROMPT")
}

// ResolveDatabaseURL returns the database URL from flag, env, or empty.
// Priority: flag > BENCH_DATABASE_URL > empty.
func (c *Config) ResolveDatabaseURL() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return os.Getenv("BENCH_DATABASE_URL")
}

// ResolveA2AAgentURL returns the A2A agent URL from flag, env, or empty.
// Priority: flag > INFRA_BENCH_A2A_AGENT_URL > empty.
func (c *Config) ResolveA2AAgentURL() string {
	if c.A2AAgentURL != "" {
		return c.A2AAgentURL
	}
	return os.Getenv("INFRA_BENCH_A2A_AGENT_URL")
}

// ResolveEvidraBin returns the evidra binary path from flag, env, or empty.
// Priority: flag > EVIDRA_BIN > empty.
func (c *Config) ResolveEvidraBin() string {
	if c.suppressesEvidenceFallbacks() {
		return ""
	}
	if c.EvidraBin != "" {
		return c.EvidraBin
	}
	return os.Getenv("EVIDRA_BIN")
}

// Default returns a Config with sensible offline-first defaults.
// Reads EVIDRA_URL and EVIDRA_API_KEY from environment for evidra reporting.
func Default() Config {
	return Config{
		EnvironmentProvider: "kind",
		Adapter:             "cli",
		ScenariosDir:        "scenarios",
		RunsDir:             "runs",
		Timeout:             5 * time.Minute,
		ClusterName:         "infra-bench",
		EvidraURL:           os.Getenv("EVIDRA_URL"),
		EvidraAPIKey:        os.Getenv("EVIDRA_API_KEY"),
	}
}

// Validate checks that all required fields are set.
func (c *Config) Validate() error {
	if c.Scenario == "" {
		return fmt.Errorf("config: scenario is required")
	}
	if c.DryRun {
		return nil
	}
	if c.Adapter == "a2a" {
		if c.ResolveA2AAgentURL() == "" {
			return fmt.Errorf("config: INFRA_BENCH_A2A_AGENT_URL is required for adapter=a2a")
		}
		return nil
	}
	if c.AgentCommand == "" && c.Provider == "" {
		return fmt.Errorf("config: agent-command or provider is required (use --dry-run to skip)")
	}
	return nil
}

// ApplyEvidenceMode returns a copy of cfg with the requested evidence mode
// applied authoritatively. Empty mode leaves cfg unchanged.
func ApplyEvidenceMode(cfg Config, mode string) Config {
	if !IsSupportedEvidenceMode(mode) {
		return cfg
	}

	cfg.EvidenceMode = mode
	switch mode {
	case "none":
		cfg.MCPServer = ""
		cfg.ProxyMode = false
		cfg.SmartPrescribe = false
		cfg.EvidraBin = ""
		cfg.SystemPromptFile = ""
		cfg.Role = ""
		cfg.ContractVersion = ""
	case "smart":
		cfg.MCPServer = ""
		cfg.ProxyMode = false
		cfg.SmartPrescribe = true
		cfg.EvidraBin = ""
		cfg.SystemPromptFile = ""
		cfg.Role = ""
		cfg.ContractVersion = ""
	}
	return cfg
}

// EffectiveEvidenceMode returns the explicit evidence mode when set, otherwise
// falls back to the legacy inference used before per-run overrides existed.
func EffectiveEvidenceMode(cfg Config) string {
	if IsSupportedEvidenceMode(cfg.EvidenceMode) {
		return cfg.EvidenceMode
	}
	// New --evidra / --trace flags take precedence over legacy flags.
	if cfg.EvidraLevel == "full" {
		return "mcp"
	}
	if cfg.EvidraLevel == "smart" {
		return "smart"
	}
	if cfg.TraceBackend == "evidra" {
		return "proxy"
	}
	// Legacy flag fallbacks.
	if cfg.MCPServer != "" {
		return "mcp"
	}
	if cfg.SmartPrescribe {
		return "smart"
	}
	if cfg.ProxyMode {
		return "proxy"
	}
	return "none"
}

func (c *Config) suppressesEvidenceFallbacks() bool {
	return IsSupportedEvidenceMode(c.EvidenceMode) && (c.EvidenceMode == "none" || c.EvidenceMode == "smart")
}

// ValidateEvidraFlags checks that --trace and --evidra values are valid and
// do not conflict with other flags.
func (c *Config) ValidateEvidraFlags() error {
	switch c.TraceBackend {
	case "", "evidra":
	default:
		return fmt.Errorf("config: --trace must be empty or \"evidra\", got %q", c.TraceBackend)
	}
	switch c.EvidraLevel {
	case "", "smart", "full":
	default:
		return fmt.Errorf("config: --evidra must be empty, \"smart\", or \"full\", got %q", c.EvidraLevel)
	}
	if c.EvidraLevel != "" && c.MCPServer != "" {
		return fmt.Errorf("config: --evidra %s cannot be combined with --mcp-server (evidra manages its own MCP server)", c.EvidraLevel)
	}
	if c.EvidraLevel != "" && c.Adapter == "a2a" {
		return fmt.Errorf("config: --evidra %s cannot be combined with --adapter a2a (cannot inject protocol into remote agent)", c.EvidraLevel)
	}
	return nil
}

// ResolveEvidraFlags translates --trace and --evidra into the legacy config
// fields that the harness understands. Call this in the CLI layer before
// invoking the harness.
func (c *Config) ResolveEvidraFlags() error {
	if c.EvidraLevel == "full" {
		bin, err := resolveEvidraMCPBin(c)
		if err != nil {
			return fmt.Errorf("config: --evidra full requires evidra-mcp on PATH or --evidra-bin: %w", err)
		}
		c.MCPServer = bin + " --signing-mode optional"
	}
	if c.EvidraLevel == "smart" {
		c.SmartPrescribe = true
	}
	if c.TraceBackend == "evidra" {
		c.ProxyMode = true
	}
	return nil
}

// resolveEvidraMCPBin finds the evidra-mcp binary via LookPath or
// ResolveEvidraBin fallback.
func resolveEvidraMCPBin(c *Config) (string, error) {
	if p := c.ResolveEvidraBin(); p != "" {
		return p, nil
	}
	p, err := exec.LookPath("evidra-mcp")
	if err != nil {
		return "", fmt.Errorf("evidra-mcp not found: %w", err)
	}
	return p, nil
}

// IsSupportedEvidenceMode reports whether a request/config mode is authoritative.
func IsSupportedEvidenceMode(mode string) bool {
	switch mode {
	case "none", "smart":
		return true
	default:
		return false
	}
}
