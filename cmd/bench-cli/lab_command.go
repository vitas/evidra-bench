package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/tui"
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
	f.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider for tool-use agent loop (bifrost, claude)")
	f.StringVar(&cfg.AgentCommand, "agent-command", cfg.AgentCommand, "command to invoke the agent")
	f.StringVar(&cfg.Model, "model", cfg.Model, "model for agent (e.g. sonnet, opus, haiku)")
	f.StringVar(&cfg.EnvironmentProvider, "environment", cfg.EnvironmentProvider, "environment provider (kind, k3d)")
	f.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "start in dry-run mode")
	return cmd
}

func applyLabFlagOverrides(labCfg *tui.LabConfig, cfg config.Config, flags *pflag.FlagSet) {
	if flags.Changed("adapter") {
		labCfg.Adapter = cfg.Adapter
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
	if flags.Changed("bench-url") {
		labCfg.BenchURL = cfg.BenchURL
	}
	if flags.Changed("bench-api-key") {
		labCfg.BenchAPIKey = cfg.BenchAPIKey
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
}
