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
	A2AAgentURL         string
	AgentCommand        string
	RunsDir             string
	KubeconfigPath      string
	Timeout             time.Duration
	ReuseCluster        bool
	ClusterName         string
	DryRun              bool
	BenchURL            string
	BenchAPIKey         string
	EvidenceDir         string
	Model               string
	Provider            string
	MemoryWindow        int
	SystemPromptFile    string
	ContractVersion     string
	SkillFile           string // local skill prompt file; runner host must already have this path
	SkillID             string // stable skill identity for comparison/filtering
	SkillVersion        string // stable skill version for comparison/filtering
	SkillSource         string // source label such as local-file, local-temp, or registry name
	SkillSHA256         string // optional expected sha256 for the skill file
	MCPServer           string // MCP server command
	ToolServerID        string // stable MCP server identity for comparison/filtering
	ToolServerVersion   string // stable MCP server version for comparison/filtering
	ReportID            string // stable public/private report campaign identifier
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

// ResolveSkillFile returns the local skill prompt path from flag, env, or empty.
// Priority: flag > BENCH_SKILL_FILE > empty.
func (c *Config) ResolveSkillFile() string {
	if c.SkillFile != "" {
		return c.SkillFile
	}
	return os.Getenv("BENCH_SKILL_FILE")
}

// ResolvePromptFile returns the prompt file used for agent execution.
// Skill files are more specific than the legacy system-prompt file.
func (c *Config) ResolvePromptFile() string {
	if skillFile := c.ResolveSkillFile(); skillFile != "" {
		return skillFile
	}
	return c.ResolveSystemPromptFile()
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

// Default returns a Config with sensible offline-first defaults.
// Reads BENCH_API_URL and BENCH_API_KEY from environment for bench reporting.
func Default() Config {
	return Config{
		EnvironmentProvider: "kind",
		Adapter:             "cli",
		ScenariosDir:        "scenarios",
		RunsDir:             "runs",
		Timeout:             5 * time.Minute,
		ClusterName:         "infra-bench",
		BenchURL:            os.Getenv("BENCH_API_URL"),
		BenchAPIKey:         os.Getenv("BENCH_API_KEY"),
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
