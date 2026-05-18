package tui

import (
	"fmt"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

func buildRunConfig(s *scenario.Scenario, scenariosDir string, labCfg LabConfig) config.Config {
	cfg := config.Default()
	cfg.Scenario = s.ID
	cfg.ScenariosDir = scenariosDir
	cfg.Adapter = valueOrDefault(labCfg.Adapter, cfg.Adapter)
	cfg.A2AAgentURL = labCfg.A2AAgentURL
	cfg.AgentCommand = labCfg.AgentCommand
	cfg.RunsDir = valueOrDefault(labCfg.RunsDir, cfg.RunsDir)
	cfg.Timeout = labCfg.TimeoutDuration()
	cfg.ReuseCluster = labCfg.ReuseCluster
	cfg.ClusterName = valueOrDefault(labCfg.ClusterName, "bench-cli")
	cfg.DryRun = labCfg.DryRun
	if labCfg.BenchURL != "" {
		cfg.BenchURL = labCfg.BenchURL
	}
	if labCfg.BenchAPIKey != "" {
		cfg.BenchAPIKey = labCfg.BenchAPIKey
	}
	cfg.EvidenceDir = labCfg.EvidenceDir
	cfg.Model = labCfg.Model
	cfg.Provider = labCfg.Provider
	cfg.MemoryWindow = labCfg.MemoryWindow
	cfg.SystemPromptFile = labCfg.SystemPromptFile
	cfg.ContractVersion = labCfg.ContractVersion
	cfg.SkillFile = labCfg.SkillFile
	cfg.SkillID = labCfg.SkillID
	cfg.SkillVersion = labCfg.SkillVersion
	cfg.SkillSource = labCfg.SkillSource
	cfg.SkillSHA256 = labCfg.SkillSHA256
	cfg.MCPServer = labCfg.MCPServer
	cfg.ToolServerID = labCfg.ToolServerID
	cfg.ToolServerVersion = labCfg.ToolServerVersion
	cfg.ReportID = labCfg.ReportID
	cfg.Parallel = labCfg.Parallel
	cfg.DatabaseURL = labCfg.DatabaseURL
	cfg.EnvironmentProvider = valueOrDefault(labCfg.EnvironmentProvider, cfg.EnvironmentProvider)
	return cfg
}

func buildRunDeps(base harness.Deps, adapterName string, envProvider environment.ClusterLifecycle) (harness.Deps, error) {
	deps := base
	if envProvider != nil {
		deps.EnvProvider = envProvider
	}
	if deps.Bootstrapper == nil {
		deps.Bootstrapper = environment.NewBootstrapper(&environment.ExecRunner{})
	}
	if adapterName == "a2a" || deps.Adapter != nil {
		return deps, nil
	}
	switch adapterName {
	case "cli", "":
		deps.Adapter = adapter.NewCLIAdapter()
	case "mcp":
		deps.Adapter = adapter.NewMCPAdapter()
	default:
		return deps, fmt.Errorf("unknown adapter: %s", adapterName)
	}
	return deps, nil
}

func valueOrDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
