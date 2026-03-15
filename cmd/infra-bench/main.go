package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/artifact"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/report"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/signalaudit"
	"samebits.com/evidra-infra-bench/pkg/skilldelta"
	"samebits.com/evidra-infra-bench/pkg/store"
	"samebits.com/evidra-infra-bench/pkg/tui"
)

var (
	version = "dev"
	commit  = "dev"
	date    = "dev"
)

func buildVersionString() string {
	return fmt.Sprintf("infra-bench %s (commit: %s, built: %s)", version, commit, date)
}

func newRootCommand() *cobra.Command {
	cfg := config.Default()
	skillDeltaCfg := config.Default()
	skillDeltaScenarios := []string{}
	skillDeltaModels := []string{}
	skillDeltaRepeats := 1
	skillDeltaOutDir := ""
	skillDeltaDir := ""
	skillDeltaNoSkillPrompt := ""
	skillDeltaWithSkillPrompt := ""
	auditRunsDir := cfg.RunsDir
	auditManifestPath := filepath.Join("configs", "signal-audit.yaml")
	auditScenarioFilter := ""
	auditModelFilter := ""
	auditProviderFilter := ""
	auditOutputPath := ""

	root := &cobra.Command{
		Use:   "infra-bench",
		Short: "Run infrastructure-agent benchmark scenarios",
		Long: `infra-bench provisions disposable Kubernetes environments, injects
failures, executes real agents, and verifies outcomes.

It produces local artifact bundles for debugging and dataset generation,
with optional Evidra reporting for behavioral analysis.`,
		Version: buildVersionString(),
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
	f.StringVar(&cfg.SystemPromptFile, "system-prompt-file", cfg.SystemPromptFile, "system prompt file path (overrides default; env: INFRA_BENCH_SYSTEM_PROMPT)")
	f.StringVar(&cfg.ContractVersion, "contract-version", cfg.ContractVersion, "evidra contract version label for tracking")

	labCfg := tui.DefaultLabConfig()
	labCmd := &cobra.Command{
		Use:   "lab",
		Short: "Interactive TUI for browsing and running scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
			if err != nil {
				return fmt.Errorf("resolve scenarios dir: %w", err)
			}
			cfgPath := ".infra-bench-lab.yaml"
			labCfg = tui.LoadLabConfig(cfgPath)
			// Inherit runs-dir from saved config if not overridden by flag
			if !cmd.Flags().Changed("runs-dir") && labCfg.RunsDir != "" {
				cfg.RunsDir = labCfg.RunsDir
			}
			applyLabFlagOverrides(&labCfg, cfg, cmd.Flags())
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

	dbCmd := &cobra.Command{
		Use:   "db",
		Short: "Query and manage the results database",
	}
	dbStatsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show aggregate run statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open(cfg.RunsDir)
			if err != nil {
				return err
			}
			defer s.Close()
			st, err := s.Stats()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Total: %d  Pass: %d  Fail: %d\n\n", st.TotalRuns, st.PassCount, st.FailCount)
			for _, ss := range st.ByScenario {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-35s %d/%d\n", ss.ScenarioID, ss.Passed, ss.Runs)
			}
			return nil
		},
	}
	dbQueryCmd := &cobra.Command{
		Use:   "query",
		Short: "Query runs with filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open(cfg.RunsDir)
			if err != nil {
				return err
			}
			defer s.Close()
			scenarioFilter, _ := cmd.Flags().GetString("scenario")
			modelFilter, _ := cmd.Flags().GetString("model")
			providerFilter, _ := cmd.Flags().GetString("provider")
			limit, _ := cmd.Flags().GetInt("limit")
			passedOnly, _ := cmd.Flags().GetBool("passed")
			failedOnly, _ := cmd.Flags().GetBool("failed")
			runs, err := s.Query(store.QueryFilters{
				ScenarioID: scenarioFilter,
				Model:      modelFilter,
				Provider:   providerFilter,
				PassedOnly: passedOnly,
				FailedOnly: failedOnly,
				Limit:      limit,
			})
			if err != nil {
				return err
			}
			for _, r := range runs {
				verdict := "PASS"
				if !r.Passed {
					verdict = "FAIL"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %-30s model=%-10s provider=%-8s dur=%.1fs checks=%d/%d cost=$%.4f %s\n",
					verdict, r.ScenarioID, r.Model, r.Provider, r.Duration,
					r.ChecksPassed, r.ChecksTotal, r.EstimatedCost,
					r.CreatedAt.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
	dbQueryCmd.Flags().String("scenario", "", "filter by scenario ID")
	dbQueryCmd.Flags().String("model", "", "filter by model")
	dbQueryCmd.Flags().String("provider", "", "filter by provider")
	dbQueryCmd.Flags().Int("limit", 20, "max results")
	dbQueryCmd.Flags().Bool("passed", false, "show only passed runs")
	dbQueryCmd.Flags().Bool("failed", false, "show only failed runs")

	dbRebuildCmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild database from JSONL backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open(cfg.RunsDir)
			if err != nil {
				return err
			}
			defer s.Close()
			count, err := s.Rebuild()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rebuilt %d records from results.jsonl\n", count)
			return nil
		},
	}

	dbCmd.AddCommand(dbStatsCmd, dbQueryCmd, dbRebuildCmd)
	dbCmd.PersistentFlags().StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "runs directory")

	skillDeltaCmd := &cobra.Command{
		Use:   "skill-delta",
		Short: "Run and analyze paired with-skill vs without-skill benchmarks",
	}

	auditCmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit existing benchmark artifacts",
	}
	auditSignalsCmd := &cobra.Command{
		Use:   "signals",
		Short: "Audit existing run artifacts for signal drift and false positives",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSignalAudit(cmd, auditRunsDir, auditManifestPath, auditScenarioFilter, auditModelFilter, auditProviderFilter, auditOutputPath)
		},
	}
	asf := auditSignalsCmd.Flags()
	asf.StringVar(&auditRunsDir, "runs-dir", auditRunsDir, "runs directory to scan")
	asf.StringVar(&auditManifestPath, "manifest", auditManifestPath, "signal audit manifest path")
	asf.StringVar(&auditScenarioFilter, "scenario", "", "filter by scenario ID")
	asf.StringVar(&auditModelFilter, "model", "", "filter by model")
	asf.StringVar(&auditProviderFilter, "provider", "", "filter by provider")
	asf.StringVar(&auditOutputPath, "output", "", "output path (default: <runs-dir>/signal-audit.json)")
	auditCmd.AddCommand(auditSignalsCmd)

	skillDeltaRunCmd := &cobra.Command{
		Use:   "run",
		Short: "Run paired without-skill and with-skill benchmark cases",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSkillDeltaRun(cmd, skillDeltaCfg, skillDeltaScenarios, skillDeltaModels, skillDeltaRepeats, skillDeltaNoSkillPrompt, skillDeltaWithSkillPrompt, skillDeltaOutDir)
		},
	}
	sdrf := skillDeltaRunCmd.Flags()
	sdrf.StringSliceVar(&skillDeltaScenarios, "scenario", nil, "scenario path or id (repeatable)")
	sdrf.StringSliceVar(&skillDeltaModels, "model", nil, "model name (repeatable)")
	sdrf.IntVar(&skillDeltaRepeats, "repeats", 1, "number of repeats per scenario/model pair")
	sdrf.StringVar(&skillDeltaNoSkillPrompt, "no-skill-prompt", "", "system prompt file for baseline runs")
	sdrf.StringVar(&skillDeltaWithSkillPrompt, "with-skill-prompt", "", "system prompt file for skill-enabled runs")
	sdrf.StringVar(&skillDeltaOutDir, "out-dir", "", "benchmark output directory (default: runs/skill-delta/<stamp>)")
	sdrf.StringVar(&skillDeltaCfg.ScenariosDir, "scenarios-dir", skillDeltaCfg.ScenariosDir, "base directory for scenarios")
	sdrf.StringVar(&skillDeltaCfg.RunsDir, "runs-dir", skillDeltaCfg.RunsDir, "base directory for benchmark runs")
	sdrf.StringVar(&skillDeltaCfg.Adapter, "adapter", skillDeltaCfg.Adapter, "agent adapter type (cli, mcp)")
	sdrf.StringVar(&skillDeltaCfg.AgentCommand, "agent-command", skillDeltaCfg.AgentCommand, "command to invoke the agent")
	sdrf.DurationVar(&skillDeltaCfg.Timeout, "timeout", skillDeltaCfg.Timeout, "agent execution timeout")
	sdrf.BoolVar(&skillDeltaCfg.ReuseCluster, "reuse-cluster", skillDeltaCfg.ReuseCluster, "reuse existing kind cluster")
	sdrf.StringVar(&skillDeltaCfg.ClusterName, "cluster-name", skillDeltaCfg.ClusterName, "kind cluster name")
	sdrf.BoolVar(&skillDeltaCfg.DryRun, "dry-run", skillDeltaCfg.DryRun, "validate workflow without executing the agent")
	sdrf.StringVar(&skillDeltaCfg.EvidraURL, "evidra-url", skillDeltaCfg.EvidraURL, "Evidra API URL for online reporting")
	sdrf.StringVar(&skillDeltaCfg.EvidraAPIKey, "evidra-api-key", skillDeltaCfg.EvidraAPIKey, "Evidra API key")
	sdrf.StringVar(&skillDeltaCfg.EvidraEvidenceDir, "evidra-evidence-dir", skillDeltaCfg.EvidraEvidenceDir, "base evidence directory for protocol verification")
	sdrf.StringVar(&skillDeltaCfg.Provider, "provider", skillDeltaCfg.Provider, "LLM provider for tool-use agent loop (bifrost, claude)")
	sdrf.StringVar(&skillDeltaCfg.EvidraBin, "evidra-bin", skillDeltaCfg.EvidraBin, "path to evidra binary for protocol tools")
	sdrf.IntVar(&skillDeltaCfg.MemoryWindow, "memory-window", -1, "agent memory window (-1=full, 0=stateless, N=last N exchanges)")
	sdrf.StringVar(&skillDeltaCfg.ContractVersion, "contract-version", skillDeltaCfg.ContractVersion, "evidra contract version label for tracking")

	skillDeltaAggregateCmd := &cobra.Command{
		Use:   "aggregate",
		Short: "Aggregate pair.json files into benchmark.json and benchmark.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSkillDeltaAggregate(cmd, skillDeltaDir)
		},
	}
	skillDeltaAggregateCmd.Flags().StringVar(&skillDeltaDir, "dir", "", "skill-delta benchmark directory")

	skillDeltaReportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate benchmark.html from benchmark.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSkillDeltaReport(cmd, skillDeltaDir)
		},
	}
	skillDeltaReportCmd.Flags().StringVar(&skillDeltaDir, "dir", "", "skill-delta benchmark directory")

	skillDeltaCmd.AddCommand(skillDeltaRunCmd, skillDeltaAggregateCmd, skillDeltaReportCmd)

	root.AddCommand(runCmd, scenarioCmd, labCmd, reportCmd, compareCmd, dbCmd, skillDeltaCmd, auditCmd)
	return root
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
}

func executeRun(cmd *cobra.Command, cfg config.Config) error {
	cfg, s, err := resolveScenarioConfig(cfg)
	if err != nil {
		return err
	}

	result, err := runScenarioOnce(cmd.Context(), cfg, s)
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

func resolveScenarioConfig(cfg config.Config) (config.Config, *scenario.Scenario, error) {
	scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
	if err != nil {
		return cfg, nil, fmt.Errorf("resolve scenarios dir: %w", err)
	}
	cfg.ScenariosDir = scenariosDir

	s, err := scenario.Resolve(cfg.ScenariosDir, cfg.Scenario)
	if err != nil {
		return cfg, nil, fmt.Errorf("load scenario: %w", err)
	}
	return cfg, s, nil
}

func runScenarioOnce(ctx context.Context, cfg config.Config, s *scenario.Scenario) (*harness.RunResult, error) {
	var agentAdapter adapter.Adapter
	switch cfg.Adapter {
	case "cli":
		agentAdapter = adapter.NewCLIAdapter()
	case "mcp":
		agentAdapter = adapter.NewMCPAdapter()
	default:
		return nil, fmt.Errorf("unknown adapter: %s", cfg.Adapter)
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

	resultsStore, err := store.Open(cfg.RunsDir)
	if err != nil {
		log.Printf("[harness] warning: could not open results store: %v", err)
	}
	if resultsStore != nil {
		defer resultsStore.Close()
	}

	h := harness.New(harness.Deps{
		EnvProvider:  envProvider,
		Bootstrapper: bootstrapper,
		Adapter:      agentAdapter,
		Writer:       writer,
		Reporter:     reporter,
		Store:        resultsStore,
	})

	result, err := h.Run(ctx, harness.RunRequest{
		Config:   cfg,
		Scenario: s,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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

func executeSkillDeltaRun(cmd *cobra.Command, cfg config.Config, scenarios []string, models []string, repeats int, noSkillPrompt, withSkillPrompt, outDir string) error {
	if len(scenarios) == 0 {
		return fmt.Errorf("skill-delta run: at least one --scenario is required")
	}
	if len(models) == 0 {
		return fmt.Errorf("skill-delta run: at least one --model is required")
	}
	if repeats < 1 {
		return fmt.Errorf("skill-delta run: --repeats must be >= 1")
	}
	if strings.TrimSpace(noSkillPrompt) == "" {
		return fmt.Errorf("skill-delta run: --no-skill-prompt is required")
	}
	if strings.TrimSpace(withSkillPrompt) == "" {
		return fmt.Errorf("skill-delta run: --with-skill-prompt is required")
	}
	if cfg.ReuseCluster {
		return fmt.Errorf("skill-delta run: --reuse-cluster is not supported because paired runs require clean state")
	}
	if !cfg.DryRun && cfg.AgentCommand == "" && cfg.Provider == "" {
		return fmt.Errorf("skill-delta run: --agent-command or --provider is required unless --dry-run is set")
	}

	scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
	if err != nil {
		return fmt.Errorf("resolve scenarios dir: %w", err)
	}
	cfg.ScenariosDir = scenariosDir

	if outDir == "" {
		outDir = filepath.Join(cfg.RunsDir, "skill-delta", time.Now().UTC().Format("20060102-150405"))
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	for _, scenarioRef := range scenarios {
		s, err := scenario.Resolve(cfg.ScenariosDir, scenarioRef)
		if err != nil {
			return fmt.Errorf("resolve scenario %q: %w", scenarioRef, err)
		}
		for _, model := range models {
			for repeat := 1; repeat <= repeats; repeat++ {
				paths := skilldelta.PairArtifactPaths(outDir, s.ID, model, repeat)
				if err := os.MkdirAll(paths.WithoutSkillRunsDir, 0o755); err != nil {
					return err
				}
				if err := os.MkdirAll(paths.WithSkillRunsDir, 0o755); err != nil {
					return err
				}

				spec := skilldelta.PairSpec{
					ScenarioID: s.ID,
					Model:      model,
					Provider:   cfg.Provider,
					Repeat:     repeat,
					Paths:      paths,
				}

				var pair skilldelta.PairResult
				if cfg.DryRun {
					pair = skilldelta.BuildDryRunPair(spec)
				} else {
					withoutCfg := cfg
					withoutCfg.Scenario = s.Path
					withoutCfg.Model = model
					withoutCfg.SystemPromptFile = noSkillPrompt
					withoutCfg.RunsDir = paths.WithoutSkillRunsDir
					withoutCfg.EvidraEvidenceDir = filepath.Join(paths.WithoutSkillRunsDir, "evidence")

					withoutResult, err := runScenarioOnce(cmd.Context(), withoutCfg, s)
					if err != nil {
						return fmt.Errorf("without_skill %s/%s repeat=%d: %w", s.ID, model, repeat, err)
					}

					withCfg := cfg
					withCfg.Scenario = s.Path
					withCfg.Model = model
					withCfg.SystemPromptFile = withSkillPrompt
					withCfg.RunsDir = paths.WithSkillRunsDir
					withCfg.EvidraEvidenceDir = filepath.Join(paths.WithSkillRunsDir, "evidence")

					withResult, err := runScenarioOnce(cmd.Context(), withCfg, s)
					if err != nil {
						return fmt.Errorf("with_skill %s/%s repeat=%d: %w", s.ID, model, repeat, err)
					}

					pair, err = skilldelta.BuildPairResult(withoutResult.ArtifactDir, withResult.ArtifactDir)
					if err != nil {
						return fmt.Errorf("build pair %s/%s repeat=%d: %w", s.ID, model, repeat, err)
					}
				}

				if err := skilldelta.WritePairJSON(paths.PairJSONPath, pair); err != nil {
					return fmt.Errorf("write pair.json: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "pair: %s\n", paths.PairJSONPath)
			}
		}
	}

	return nil
}

func executeSkillDeltaAggregate(cmd *cobra.Command, dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("skill-delta aggregate: --dir is required")
	}

	pairs, err := skilldelta.LoadPairs(dir)
	if err != nil {
		return err
	}
	benchmark := skilldelta.BuildBenchmark(buildSkillDeltaMetadata(dir, pairs), pairs)
	jsonPath := filepath.Join(dir, "benchmark.json")
	mdPath := filepath.Join(dir, "benchmark.md")
	if err := skilldelta.WriteBenchmarkJSON(jsonPath, benchmark); err != nil {
		return err
	}
	if err := skilldelta.WriteMarkdown(mdPath, benchmark); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "benchmark: %s\n", jsonPath)
	fmt.Fprintf(cmd.OutOrStdout(), "markdown: %s\n", mdPath)
	return nil
}

func executeSkillDeltaReport(cmd *cobra.Command, dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("skill-delta report: --dir is required")
	}

	benchmark, err := skilldelta.ReadBenchmarkJSON(filepath.Join(dir, "benchmark.json"))
	if err != nil {
		return err
	}
	outputPath := filepath.Join(dir, "benchmark.html")
	if err := report.WriteSkillDeltaHTML(outputPath, benchmark); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "html: %s\n", outputPath)
	return nil
}

func executeSignalAudit(cmd *cobra.Command, runsDir, manifestPath, scenarioFilter, modelFilter, providerFilter, outputPath string) error {
	if strings.TrimSpace(runsDir) == "" {
		return fmt.Errorf("audit signals: --runs-dir is required")
	}
	if strings.TrimSpace(manifestPath) == "" {
		return fmt.Errorf("audit signals: --manifest is required")
	}
	manifestPath = resolveAuditManifestPath(manifestPath)
	if outputPath == "" {
		outputPath = filepath.Join(runsDir, "signal-audit.json")
	}

	manifest, err := signalaudit.LoadManifest(manifestPath)
	if err != nil {
		return err
	}
	runs, err := loadAuditRuns(runsDir, scenarioFilter, modelFilter, providerFilter)
	if err != nil {
		return err
	}
	result := signalaudit.Analyze(manifest, runs)
	if err := signalaudit.WriteJSON(outputPath, result); err != nil {
		return err
	}

	fmt.Fprint(cmd.OutOrStdout(), signalaudit.FormatSummary(result))
	fmt.Fprintf(cmd.OutOrStdout(), "json: %s\n", outputPath)
	return nil
}

func buildSkillDeltaMetadata(dir string, pairs []skilldelta.PairResult) skilldelta.BenchmarkMetadata {
	scenarios := map[string]struct{}{}
	models := map[string]struct{}{}
	providers := map[string]struct{}{}
	repeats := 0
	meta := skilldelta.BenchmarkMetadata{
		Suite:       "skill-delta",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		RunsDir:     dir,
	}

	for _, pair := range pairs {
		scenarios[pair.ScenarioID] = struct{}{}
		models[pair.Model] = struct{}{}
		if pair.Provider != "" {
			providers[pair.Provider] = struct{}{}
		}
		if pair.Repeat > repeats {
			repeats = pair.Repeat
		}
		if meta.ContractVersion == "" {
			meta.ContractVersion = firstMetadata(pair, "contract_version")
		}
		if meta.PromptVersion == "" {
			meta.PromptVersion = firstMetadata(pair, "prompt_version")
		}
		if meta.SkillVersion == "" {
			meta.SkillVersion = firstMetadata(pair, "skill_version")
		}
		if meta.EvidraVersion == "" {
			meta.EvidraVersion = firstMetadata(pair, "evidra_version")
		}
		if meta.InfraBenchVersion == "" {
			meta.InfraBenchVersion = firstMetadata(pair, "infra_bench_version")
		}
		if meta.InfraBenchCommit == "" {
			meta.InfraBenchCommit = firstMetadata(pair, "infra_bench_commit")
		}
	}

	meta.Repeats = repeats
	meta.Scenarios = sortedKeys(scenarios)
	meta.Models = sortedKeys(models)
	providerList := sortedKeys(providers)
	if len(providerList) == 1 {
		meta.Provider = providerList[0]
	}

	return meta
}

func firstMetadata(pair skilldelta.PairResult, key string) string {
	if value := pair.WithSkill.Metadata[key]; strings.TrimSpace(value) != "" {
		return value
	}
	return pair.WithoutSkill.Metadata[key]
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func main() {
	harness.SetVersion(version, commit)
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
