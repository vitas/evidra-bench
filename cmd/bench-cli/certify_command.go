package main

import (
	"github.com/spf13/cobra"

	"samebits.com/evidra-infra-bench/pkg/config"
)

func newCertifyCommand() *cobra.Command {
	track := ""
	model := ""
	cfg := config.Default()

	cmd := &cobra.Command{
		Use:   "certify",
		Short: "Run all scenarios in a track and produce a certification grade",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeCertify(cmd, cfg, track, model)
		},
	}
	f := cmd.Flags()
	f.StringVar(&track, "track", "", "certification track (workloads, troubleshooting, networking, storage, pod-security, runtime-security, release-ops, platform-eng)")
	f.StringVar(&model, "model", "", "model name (e.g. sonnet, opus)")
	f.StringVar(&cfg.EnvironmentProvider, "environment", cfg.EnvironmentProvider, "environment provider (kind, k3d)")
	f.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider")
	f.StringVar(&cfg.Adapter, "adapter", cfg.Adapter, "agent adapter type (cli, mcp, a2a)")
	f.StringVar(&cfg.A2AAgentURL, "a2a-agent-url", cfg.A2AAgentURL, "A2A agent URL (env: INFRA_BENCH_A2A_AGENT_URL)")
	f.StringVar(&cfg.AgentCommand, "agent-command", cfg.AgentCommand, "command to invoke the agent")
	f.StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "scenarios directory")
	f.StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "runs directory")
	f.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "per-scenario timeout")
	f.BoolVar(&cfg.ReuseCluster, "reuse-cluster", cfg.ReuseCluster, "reuse kind cluster")
	f.StringVar(&cfg.ClusterName, "cluster-name", cfg.ClusterName, "kind cluster name")
	f.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "validate without running")
	f.IntVar(&cfg.MemoryWindow, "memory-window", -1, "memory window")
	f.StringVar(&cfg.BenchURL, "bench-url", cfg.BenchURL, "Bench API URL for reporting results")
	registerBenchAPIKeyFlag(f, &cfg.BenchAPIKey)
	f.StringVar(&cfg.SystemPromptFile, "system-prompt-file", cfg.SystemPromptFile, "system prompt file")
	f.StringVar(&cfg.Role, "role", cfg.Role, "role-based skill (k8s-admin, security-ops, release-manager, platform-eng)")
	f.StringVar(&cfg.ContractVersion, "contract-version", cfg.ContractVersion, "contract version")
	_ = cmd.MarkFlagRequired("track")
	_ = cmd.MarkFlagRequired("model")
	return cmd
}
