package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/tui"
)

func newLabCommand(cfg *config.Config) *cobra.Command {
	labCfg := tui.DefaultLabConfig()
	cmd := &cobra.Command{
		Use:   "lab",
		Short: "Interactive TUI for browsing and running scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
			if err != nil {
				return fmt.Errorf("resolve scenarios dir: %w", err)
			}
			cfgPath := ".bench-cli-lab.yaml"
			labCfg = tui.LoadLabConfig(cfgPath)
			// Inherit runs-dir from saved config if not overridden by flag
			if !cmd.Flags().Changed("runs-dir") && labCfg.RunsDir != "" {
				cfg.RunsDir = labCfg.RunsDir
			}
			applyLabFlagOverrides(&labCfg, *cfg, cmd.Flags())
			deps := harness.Deps{}
			return tui.Run(scenariosDir, cfgPath, labCfg, deps)
		},
	}
	f := cmd.Flags()
	f.StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "base directory for scenarios")
	f.StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "output directory for run artifacts")
	f.StringVar(&cfg.Adapter, "adapter", cfg.Adapter, "agent adapter type (cli, mcp, a2a)")
	f.StringVar(&cfg.A2AAgentURL, "a2a-agent-url", cfg.A2AAgentURL, "A2A agent URL (env: INFRA_BENCH_A2A_AGENT_URL)")
	f.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider for tool-use agent loop (bifrost, claude)")
	f.StringVar(&cfg.AgentCommand, "agent-command", cfg.AgentCommand, "command to invoke the agent")
	f.StringVar(&cfg.Model, "model", cfg.Model, "model for agent (e.g. sonnet, opus, haiku)")
	f.StringVar(&cfg.EnvironmentProvider, "environment", cfg.EnvironmentProvider, "environment provider (kind, k3d)")
	f.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "agent execution timeout")
	f.BoolVar(&cfg.ReuseCluster, "reuse-cluster", cfg.ReuseCluster, "reuse existing kind cluster")
	f.StringVar(&cfg.ClusterName, "cluster-name", cfg.ClusterName, "kind cluster name")
	f.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "start in dry-run mode")
	f.StringVar(&cfg.BenchURL, "bench-url", cfg.BenchURL, "Bench API URL for online reporting")
	registerBenchAPIKeyFlag(f, &cfg.BenchAPIKey)
	f.StringVar(&cfg.EvidenceDir, "evidence-dir", cfg.EvidenceDir, "evidence directory for verifier input")
	f.IntVar(&cfg.MemoryWindow, "memory-window", -1, "agent memory window (-1=full, 0=stateless, N=last N exchanges)")
	f.StringVar(&cfg.SystemPromptFile, "system-prompt-file", cfg.SystemPromptFile, "system prompt file path (overrides default; env: INFRA_BENCH_SYSTEM_PROMPT)")
	f.StringVar(&cfg.SkillFile, "skill-file", cfg.SkillFile, "local skill prompt file path (runner host path; env: BENCH_SKILL_FILE)")
	f.StringVar(&cfg.SkillID, "skill-id", cfg.SkillID, "stable skill identity for result comparison")
	f.StringVar(&cfg.SkillVersion, "skill-version", cfg.SkillVersion, "stable skill version for result comparison")
	f.StringVar(&cfg.SkillSource, "skill-source", cfg.SkillSource, "skill source label for result comparison")
	f.StringVar(&cfg.SkillSHA256, "skill-sha256", cfg.SkillSHA256, "expected sha256 for --skill-file")
	f.StringVar(&cfg.MCPServer, "mcp-server", cfg.MCPServer, "MCP server command for tool execution")
	f.StringVar(&cfg.ToolServerID, "tool-server-id", cfg.ToolServerID, "stable MCP server identity for result comparison")
	f.StringVar(&cfg.ToolServerVersion, "tool-server-version", cfg.ToolServerVersion, "stable MCP server version for result comparison")
	f.StringVar(&cfg.ReportID, "report-id", cfg.ReportID, "stable report campaign identifier for filtering")
	f.StringVar(&cfg.ContractVersion, "contract-version", cfg.ContractVersion, "contract version label for tracking")
	f.IntVar(&cfg.Parallel, "parallel", 1, "number of parallel workers (1 = sequential)")
	f.StringVar(&cfg.DatabaseURL, "database-url", "", "PostgreSQL URL for job queue (env: BENCH_DATABASE_URL)")
	return cmd
}

func applyLabFlagOverrides(labCfg *tui.LabConfig, cfg config.Config, flags *pflag.FlagSet) {
	if flags.Changed("adapter") {
		labCfg.Adapter = cfg.Adapter
	}
	if flags.Changed("a2a-agent-url") {
		labCfg.A2AAgentURL = cfg.A2AAgentURL
	}
	if flags.Changed("agent-command") {
		labCfg.AgentCommand = cfg.AgentCommand
	}
	if flags.Changed("dry-run") {
		labCfg.DryRun = cfg.DryRun
	}
	if flags.Changed("model") {
		labCfg.Model = cfg.Model
	}
	if flags.Changed("provider") {
		labCfg.Provider = cfg.Provider
	}
	if flags.Changed("runs-dir") || labCfg.RunsDir == "" {
		labCfg.RunsDir = cfg.RunsDir
	}
	if flags.Changed("timeout") {
		labCfg.Timeout = cfg.Timeout.String()
	}
	if flags.Changed("cluster-name") {
		labCfg.ClusterName = cfg.ClusterName
	}
	if flags.Changed("bench-url") {
		labCfg.BenchURL = cfg.BenchURL
	}
	if flags.Changed("bench-api-key") {
		labCfg.BenchAPIKey = cfg.BenchAPIKey
	}
	if flags.Changed("evidence-dir") {
		labCfg.EvidenceDir = cfg.EvidenceDir
	}
	if flags.Changed("memory-window") {
		labCfg.MemoryWindow = cfg.MemoryWindow
	}
	if flags.Changed("reuse-cluster") {
		labCfg.ReuseCluster = cfg.ReuseCluster
	}
	if flags.Changed("environment") {
		labCfg.EnvironmentProvider = cfg.EnvironmentProvider
	}
	if flags.Changed("system-prompt-file") {
		labCfg.SystemPromptFile = cfg.SystemPromptFile
	}
	if flags.Changed("skill-file") {
		labCfg.SkillFile = cfg.SkillFile
	}
	if flags.Changed("skill-id") {
		labCfg.SkillID = cfg.SkillID
	}
	if flags.Changed("skill-version") {
		labCfg.SkillVersion = cfg.SkillVersion
	}
	if flags.Changed("skill-source") {
		labCfg.SkillSource = cfg.SkillSource
	}
	if flags.Changed("skill-sha256") {
		labCfg.SkillSHA256 = cfg.SkillSHA256
	}
	if flags.Changed("mcp-server") {
		labCfg.MCPServer = cfg.MCPServer
	}
	if flags.Changed("tool-server-id") {
		labCfg.ToolServerID = cfg.ToolServerID
	}
	if flags.Changed("tool-server-version") {
		labCfg.ToolServerVersion = cfg.ToolServerVersion
	}
	if flags.Changed("report-id") {
		labCfg.ReportID = cfg.ReportID
	}
	if flags.Changed("contract-version") {
		labCfg.ContractVersion = cfg.ContractVersion
	}
	if flags.Changed("parallel") {
		labCfg.Parallel = cfg.Parallel
	}
	if flags.Changed("database-url") {
		labCfg.DatabaseURL = cfg.DatabaseURL
	}
}
