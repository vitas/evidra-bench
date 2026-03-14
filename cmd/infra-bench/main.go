package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"samebits.com/evidra-infra-bench/pkg/config"
)

var version = "dev"

func newRootCommand() *cobra.Command {
	cfg := config.Default()

	root := &cobra.Command{
		Use:   "infra-bench",
		Short: "Run infrastructure-agent benchmark scenarios",
		Long: `infra-bench provisions disposable Kubernetes environments, injects
failures, executes real agents, and verifies outcomes.

It produces local artifact bundles for debugging and dataset generation,
with optional Evidra reporting for behavioral analysis.`,
		Version: version,
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run a benchmark scenario against an agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Validate(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "scenario=%s adapter=%s\n", cfg.Scenario, cfg.Adapter)
			return nil
		},
	}

	f := runCmd.Flags()
	f.StringVar(&cfg.Scenario, "scenario", cfg.Scenario, "scenario path relative to scenarios dir")
	f.StringVar(&cfg.Adapter, "adapter", cfg.Adapter, "agent adapter type (cli, mcp)")
	f.StringVar(&cfg.AgentCommand, "agent-command", cfg.AgentCommand, "command to invoke the agent")
	f.StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "base directory for scenarios")
	f.StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "output directory for run artifacts")
	f.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "agent execution timeout")
	f.BoolVar(&cfg.ReuseCluster, "reuse-cluster", cfg.ReuseCluster, "reuse existing kind cluster")
	f.StringVar(&cfg.ClusterName, "cluster-name", cfg.ClusterName, "kind cluster name")
	f.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "validate scenario without executing")
	f.StringVar(&cfg.EvidraURL, "evidra-url", cfg.EvidraURL, "Evidra API URL for online reporting")
	f.StringVar(&cfg.EvidraAPIKey, "evidra-api-key", cfg.EvidraAPIKey, "Evidra API key")

	root.AddCommand(runCmd)
	return root
}

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
