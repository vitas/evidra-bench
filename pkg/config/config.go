// Package config defines run configuration for infra-bench.
package config

import (
	"fmt"
	"os"
	"time"
)

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
	ProxyMode           bool // auto-record evidence for mutations without agent involvement
	SmartPrescribe      bool // simplified prescribe (tool+operation, no artifact)
}

// ResolveSystemPromptFile returns the system prompt file path from flag, env, or empty.
// Priority: flag > INFRA_BENCH_SYSTEM_PROMPT > empty (use default).
func (c *Config) ResolveSystemPromptFile() string {
	if c.SystemPromptFile != "" {
		return c.SystemPromptFile
	}
	return os.Getenv("INFRA_BENCH_SYSTEM_PROMPT")
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
func Default() Config {
	return Config{
		EnvironmentProvider: "kind",
		Adapter:             "cli",
		ScenariosDir:        "scenarios",
		RunsDir:             "runs",
		Timeout:             5 * time.Minute,
		ClusterName:         "infra-bench",
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
