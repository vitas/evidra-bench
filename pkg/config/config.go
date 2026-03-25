// Package config defines run configuration for bench-cli.
package config

import (
	"fmt"
	"os"
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
	Parallel            int    // number of parallel workers (0 or 1 = sequential, >1 requires --database-url)
	DatabaseURL         string // PostgreSQL connection string for River job queue (env: BENCH_DATABASE_URL)
}

// ResolveSystemPromptFile returns the system prompt file path from flag, env, or empty.
// Priority: flag > INFRA_BENCH_SYSTEM_PROMPT > empty (use default).
func (c *Config) ResolveSystemPromptFile() string {
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

// ResolveEvidraBin returns the evidra binary path from flag, env, or empty.
// Priority: flag > EVIDRA_BIN > empty.
func (c *Config) ResolveEvidraBin() string {
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
	if !c.DryRun && c.AgentCommand == "" && c.Provider == "" {
		return fmt.Errorf("config: agent-command or provider is required (use --dry-run to skip)")
	}
	return nil
}
