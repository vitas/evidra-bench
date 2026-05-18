package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// LabConfig holds persistent TUI settings.
type LabConfig struct {
	Adapter             string `yaml:"adapter"`
	A2AAgentURL         string `yaml:"a2a_agent_url,omitempty"`
	EnvironmentProvider string `yaml:"environment_provider,omitempty"`
	Provider            string `yaml:"provider,omitempty"`
	AgentCommand        string `yaml:"agent_command"`
	Model               string `yaml:"model,omitempty"`
	RunsDir             string `yaml:"runs_dir,omitempty"`
	ClusterName         string `yaml:"cluster_name,omitempty"`
	Timeout             string `yaml:"timeout"`
	DryRun              bool   `yaml:"dry_run"`
	EvidenceDir         string `yaml:"evidence_dir,omitempty"`
	BenchURL            string `yaml:"bench_url,omitempty"`
	BenchAPIKey         string `yaml:"bench_api_key,omitempty"`
	MemoryWindow        int    `yaml:"memory_window,omitempty"`
	ReuseCluster        bool   `yaml:"reuse_cluster,omitempty"`
	SystemPromptFile    string `yaml:"system_prompt_file,omitempty"`
	SkillFile           string `yaml:"skill_file,omitempty"`
	SkillID             string `yaml:"skill_id,omitempty"`
	SkillVersion        string `yaml:"skill_version,omitempty"`
	SkillSource         string `yaml:"skill_source,omitempty"`
	SkillSHA256         string `yaml:"skill_sha256,omitempty"`
	MCPServer           string `yaml:"mcp_server,omitempty"`
	ToolServerID        string `yaml:"tool_server_id,omitempty"`
	ToolServerVersion   string `yaml:"tool_server_version,omitempty"`
	ReportID            string `yaml:"report_id,omitempty"`
	ContractVersion     string `yaml:"contract_version,omitempty"`
	Parallel            int    `yaml:"parallel,omitempty"`
	DatabaseURL         string `yaml:"database_url,omitempty"`
}

// DefaultLabConfig returns sensible defaults.
func DefaultLabConfig() LabConfig {
	return LabConfig{
		Adapter:             "cli",
		EnvironmentProvider: "kind",
		RunsDir:             "runs",
		ClusterName:         "bench-cli",
		Timeout:             "5m",
		DryRun:              true,
		MemoryWindow:        -1,
		Parallel:            1,
	}
}

// TimeoutDuration parses the timeout string.
func (c LabConfig) TimeoutDuration() time.Duration {
	d, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}

// LoadLabConfig reads config from the given path, returning defaults if missing.
func LoadLabConfig(path string) LabConfig {
	cfg := DefaultLabConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}

// SaveLabConfig writes config to the given path.
func SaveLabConfig(path string, cfg LabConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("tui.SaveLabConfig: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("tui.SaveLabConfig: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
