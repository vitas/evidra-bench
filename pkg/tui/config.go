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
	Adapter           string `yaml:"adapter"`
	Provider          string `yaml:"provider,omitempty"`
	AgentCommand      string `yaml:"agent_command"`
	Model             string `yaml:"model,omitempty"`
	EvidraBin         string `yaml:"evidra_bin,omitempty"`
	RunsDir           string `yaml:"runs_dir,omitempty"`
	Timeout           string `yaml:"timeout"`
	DryRun            bool   `yaml:"dry_run"`
	EvidraEvidenceDir string `yaml:"evidra_evidence_dir,omitempty"`
}

// DefaultLabConfig returns sensible defaults.
func DefaultLabConfig() LabConfig {
	return LabConfig{
		Adapter: "cli",
		Timeout: "5m",
		DryRun:  true,
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
