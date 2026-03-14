// Package config defines run configuration for infra-bench.
package config

import (
	"fmt"
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
	return nil
}
