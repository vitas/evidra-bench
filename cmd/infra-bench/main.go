package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/artifact"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/report"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/tui"
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
			return executeRun(cmd, cfg)
		},
	}

	scenarioCmd := &cobra.Command{
		Use:   "scenario",
		Short: "Manage benchmark scenarios",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listScenarios(cmd, cfg)
		},
	}

	scenarioCmd.AddCommand(listCmd)
	scenarioCmd.PersistentFlags().StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "base directory for scenarios")

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
	f.StringVar(&cfg.EvidraEvidenceDir, "evidra-evidence-dir", cfg.EvidraEvidenceDir, "evidence directory for protocol verification")
	f.StringVar(&cfg.Model, "model", cfg.Model, "model for agent (e.g. sonnet, opus, haiku)")
	f.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider for tool-use agent loop (bifrost, claude)")
	f.StringVar(&cfg.EvidraBin, "evidra-bin", cfg.EvidraBin, "path to evidra binary for protocol tools")
	f.IntVar(&cfg.MemoryWindow, "memory-window", -1, "agent memory window (-1=full, 0=stateless, N=last N exchanges)")

	labCfg := tui.DefaultLabConfig()
	labCmd := &cobra.Command{
		Use:   "lab",
		Short: "Interactive TUI for browsing and running scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
			if err != nil {
				return fmt.Errorf("resolve scenarios dir: %w", err)
			}
			cfgPath := filepath.Join(cfg.RunsDir, ".lab-config.yaml")
			labCfg = tui.LoadLabConfig(cfgPath)
			if cmd.Flags().Changed("adapter") {
				labCfg.Adapter = cfg.Adapter
			}
			if cmd.Flags().Changed("agent-command") {
				labCfg.AgentCommand = cfg.AgentCommand
			}
			if cmd.Flags().Changed("dry-run") {
				labCfg.DryRun = cfg.DryRun
			}
			if cmd.Flags().Changed("model") {
				labCfg.Model = cfg.Model
			}
			if cmd.Flags().Changed("provider") {
				labCfg.Provider = cfg.Provider
			}
			deps := harness.Deps{}
			return tui.Run(scenariosDir, cfgPath, labCfg, deps)
		},
	}
	lf := labCmd.Flags()
	lf.StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "base directory for scenarios")
	lf.StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "output directory for run artifacts")
	lf.StringVar(&cfg.Adapter, "adapter", cfg.Adapter, "agent adapter type (cli, mcp)")
	lf.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider for tool-use agent loop (bifrost, claude)")
	lf.StringVar(&cfg.AgentCommand, "agent-command", cfg.AgentCommand, "command to invoke the agent")
	lf.StringVar(&cfg.Model, "model", cfg.Model, "model for agent (e.g. sonnet, opus, haiku)")
	lf.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "start in dry-run mode")

	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate HTML report from run artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
			if err != nil {
				return fmt.Errorf("resolve scenarios dir: %w", err)
			}
			outputPath := filepath.Join(cfg.RunsDir, "report.html")
			if len(args) > 0 {
				outputPath = args[0]
			}
			if err := report.GenerateHTML(scenariosDir, cfg.RunsDir, outputPath); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Report written to %s\n", outputPath)
			return nil
		},
	}
	reportCmd.Flags().StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "base directory for scenarios")
	reportCmd.Flags().StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "runs directory to scan")

	compareCmd := &cobra.Command{
		Use:   "compare <run-dir-A> <run-dir-B>",
		Short: "Compare two run artifacts side by side",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := report.CompareRuns(args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), report.FormatComparison(result))
			return nil
		},
	}

	root.AddCommand(runCmd, scenarioCmd, labCmd, reportCmd, compareCmd)
	return root
}

func executeRun(cmd *cobra.Command, cfg config.Config) error {
	scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
	if err != nil {
		return fmt.Errorf("resolve scenarios dir: %w", err)
	}
	cfg.ScenariosDir = scenariosDir

	s, err := scenario.Resolve(cfg.ScenariosDir, cfg.Scenario)
	if err != nil {
		return fmt.Errorf("load scenario: %w", err)
	}

	var agentAdapter adapter.Adapter
	switch cfg.Adapter {
	case "cli":
		agentAdapter = adapter.NewCLIAdapter()
	case "mcp":
		agentAdapter = adapter.NewMCPAdapter()
	default:
		return fmt.Errorf("unknown adapter: %s", cfg.Adapter)
	}

	envProvider := environment.NewKindProvider()
	envProvider.ReuseExisting = cfg.ReuseCluster
	runner := &environment.ExecRunner{}
	bootstrapper := environment.NewBootstrapper(runner)
	writer := artifact.NewWriter(cfg.RunsDir)

	reporter := report.NewReporter(report.Config{
		EvidencePath: filepath.Join(cfg.RunsDir, "evidra"),
		EvidraURL:    cfg.EvidraURL,
		EvidraAPIKey: cfg.EvidraAPIKey,
	})

	h := harness.New(harness.Deps{
		EnvProvider:  envProvider,
		Bootstrapper: bootstrapper,
		Adapter:      agentAdapter,
		Writer:       writer,
		Reporter:     reporter,
	})

	result, err := h.Run(cmd.Context(), harness.RunRequest{
		Config:   cfg,
		Scenario: s,
	})
	if err != nil {
		return err
	}

	verdict := "PASS"
	if !result.Passed {
		verdict = "FAIL"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "[%s] scenario=%s duration=%s exit_code=%d\n",
		verdict, result.ScenarioID, result.Duration.Round(time.Millisecond), result.ExitCode)
	if result.ArtifactDir != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "artifacts: %s\n", result.ArtifactDir)
	}

	if !result.Passed {
		return &RunFailedError{ScenarioID: result.ScenarioID}
	}
	return nil
}

// RunFailedError indicates a scenario run completed but verification failed.
type RunFailedError struct {
	ScenarioID string
}

func (e *RunFailedError) Error() string {
	return fmt.Sprintf("scenario %s: verification failed", e.ScenarioID)
}

func listScenarios(cmd *cobra.Command, cfg config.Config) error {
	scenarios, err := scenario.LoadAll(cfg.ScenariosDir)
	if err != nil {
		return fmt.Errorf("list scenarios: %w", err)
	}
	if len(scenarios) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no scenarios found")
		return nil
	}
	for _, s := range scenarios {
		fmt.Fprintf(cmd.OutOrStdout(), "%-30s %s (%s)\n", s.Path, s.Title, s.ID)
	}
	return nil
}

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
