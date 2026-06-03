package main

import (
	"github.com/spf13/pflag"

	"github.com/vitas/evidra-bench/pkg/config"
)

type executionFlagOptions struct {
	IncludeScenario bool
	ScenarioUsage   string
	DryRunUsage     string
}

type agentFlagOptions struct {
	IncludeModel bool
}

type resultMetadataFlagOptions struct {
	IncludeToolServer bool
	IncludeReportID   bool
}

type parallelFlagOptions struct {
	IncludeDatabaseURL bool
}

func registerExecutionFlags(f *pflag.FlagSet, cfg *config.Config, opt executionFlagOptions) {
	f.StringVar(&cfg.EnvironmentProvider, "environment", cfg.EnvironmentProvider, "environment provider (kind, k3d)")
	if opt.IncludeScenario {
		usage := opt.ScenarioUsage
		if usage == "" {
			usage = "scenario path relative to scenarios dir"
		}
		f.StringVar(&cfg.Scenario, "scenario", cfg.Scenario, usage)
	}
	f.StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "scenarios directory")
	f.StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "runs directory")
	f.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "per-scenario timeout")
	f.BoolVar(&cfg.ReuseCluster, "reuse-cluster", cfg.ReuseCluster, "reuse kind cluster")
	f.StringVar(&cfg.ClusterName, "cluster-name", cfg.ClusterName, "kind cluster name")
	dryRunUsage := opt.DryRunUsage
	if dryRunUsage == "" {
		dryRunUsage = "dry-run mode"
	}
	f.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, dryRunUsage)
}

func registerAgentFlags(f *pflag.FlagSet, cfg *config.Config, opt agentFlagOptions) {
	f.StringVar(&cfg.Adapter, "adapter", cfg.Adapter, "agent adapter type (cli, mcp, a2a)")
	f.StringVar(&cfg.A2AAgentURL, "a2a-agent-url", cfg.A2AAgentURL, "A2A agent URL (env: INFRA_BENCH_A2A_AGENT_URL)")
	f.StringVar(&cfg.AgentCommand, "agent-command", cfg.AgentCommand, "command to invoke the agent")
	f.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider")
	if opt.IncludeModel {
		f.StringVar(&cfg.Model, "model", cfg.Model, "model for agent")
	}
	f.IntVar(&cfg.MemoryWindow, "memory-window", -1, "agent memory window (-1=full, 0=stateless, N=last N exchanges)")
	f.StringVar(&cfg.SystemPromptFile, "system-prompt-file", cfg.SystemPromptFile, "system prompt file path (overrides default; env: INFRA_BENCH_SYSTEM_PROMPT)")
}

func registerResultMetadataFlags(f *pflag.FlagSet, cfg *config.Config, opt resultMetadataFlagOptions) {
	f.StringVar(&cfg.BenchURL, "bench-url", cfg.BenchURL, "Bench API URL for reporting results")
	registerBenchAPIKeyFlag(f, &cfg.BenchAPIKey)
	f.StringVar(&cfg.SkillFile, "skill-file", cfg.SkillFile, "local skill prompt file path (runner host path; env: BENCH_SKILL_FILE)")
	f.StringVar(&cfg.SkillID, "skill-id", cfg.SkillID, "stable skill identity for result comparison")
	f.StringVar(&cfg.SkillVersion, "skill-version", cfg.SkillVersion, "stable skill version for result comparison")
	f.StringVar(&cfg.SkillSource, "skill-source", cfg.SkillSource, "skill source label for result comparison")
	f.StringVar(&cfg.SkillSHA256, "skill-sha256", cfg.SkillSHA256, "expected sha256 for --skill-file")
	f.StringVar(&cfg.ContractVersion, "contract-version", cfg.ContractVersion, "contract version label for tracking")
	if opt.IncludeToolServer {
		f.StringVar(&cfg.MCPServer, "mcp-server", cfg.MCPServer, "MCP server command for tool execution")
		f.StringVar(&cfg.ToolServerID, "tool-server-id", cfg.ToolServerID, "stable MCP server identity for result comparison")
		f.StringVar(&cfg.ToolServerVersion, "tool-server-version", cfg.ToolServerVersion, "stable MCP server version for result comparison")
	}
	if opt.IncludeReportID {
		f.StringVar(&cfg.ReportID, "report-id", cfg.ReportID, "stable report campaign identifier for filtering")
	}
}

func registerParallelFlags(f *pflag.FlagSet, cfg *config.Config, opt parallelFlagOptions) {
	f.IntVar(&cfg.Parallel, "parallel", 1, "number of parallel workers (1 = sequential)")
	if opt.IncludeDatabaseURL {
		f.StringVar(&cfg.DatabaseURL, "database-url", "", "PostgreSQL URL for job queue (env: BENCH_DATABASE_URL)")
	}
}
