package main

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"sync/atomic"

	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/artifact"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/jobqueue"
	"samebits.com/evidra-infra-bench/pkg/report"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/skilldelta"
	"samebits.com/evidra-infra-bench/pkg/store"
	"samebits.com/evidra-infra-bench/pkg/tui"
	"samebits.com/evidra-infra-bench/pkg/workspace"
	"samebits.com/evidra/pkg/signalaudit"
)

var (
	version = "dev"
	commit  = "dev"
	date    = "dev"
)

func buildVersionString() string {
	return fmt.Sprintf("bench-cli %s (commit: %s, built: %s)", version, commit, date)
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
		Use:   "bench-cli",
		Short: "Run infrastructure-agent benchmark scenarios",
		Long: `bench-cli provisions disposable Kubernetes environments, injects
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

	var pushURL, pushKey string
	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push scenario metadata to evidra API",
		RunE: func(cmd *cobra.Command, args []string) error {
			return pushScenarios(cfg.ScenariosDir, pushURL, pushKey)
		},
	}
	pushCmd.Flags().StringVar(&pushURL, "evidra-url", "https://api.evidra.cc", "Evidra API URL")
	pushCmd.Flags().StringVar(&pushKey, "evidra-api-key", "", "Evidra API key")

	scenarioCmd.AddCommand(listCmd, pushCmd)
	scenarioCmd.PersistentFlags().StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "base directory for scenarios")

	f := runCmd.Flags()
	f.StringVar(&cfg.EnvironmentProvider, "environment", cfg.EnvironmentProvider, "environment provider (kind, k3d)")
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
	f.StringVar(&cfg.Role, "role", cfg.Role, "role-based skill (k8s-admin, security-ops, release-manager, platform-eng)")
	f.StringVar(&cfg.MCPServer, "mcp-server", cfg.MCPServer, "MCP server command for tool execution (e.g. 'evidra-mcp --signing-mode optional')")
	f.StringVar(&cfg.ContractVersion, "contract-version", cfg.ContractVersion, "evidra contract version label for tracking")
	f.BoolVar(&cfg.ProxyMode, "proxy-mode", false, "auto-record evidence for mutations (no agent prescribe/report needed)")
	f.BoolVar(&cfg.SmartPrescribe, "smart-prescribe", false, "simplified prescribe (tool+operation, 80% fewer tokens)")
	f.IntVar(&cfg.Parallel, "parallel", 1, "number of parallel workers (1 = sequential)")
	f.StringVar(&cfg.DatabaseURL, "database-url", "", "PostgreSQL URL for job queue (env: BENCH_DATABASE_URL)")

	labCfg := tui.DefaultLabConfig()
	labCmd := &cobra.Command{
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

	dbImportCmd := &cobra.Command{
		Use:   "import",
		Short: "Import run.json artifacts into the database",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open(cfg.RunsDir)
			if err != nil {
				return err
			}
			defer s.Close()
			count, err := s.ImportFromArtifacts(cfg.RunsDir)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Imported %d records from run artifacts\n", count)
			return nil
		},
	}

	dbCmd.AddCommand(dbStatsCmd, dbQueryCmd, dbRebuildCmd, dbImportCmd)
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
	sdrf.StringVar(&skillDeltaCfg.EnvironmentProvider, "environment", skillDeltaCfg.EnvironmentProvider, "environment provider (kind, k3d)")
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

	certifyTrack := ""
	certifyModel := ""
	certifyCfg := config.Default()

	certifyCmd := &cobra.Command{
		Use:   "certify",
		Short: "Run all scenarios in a track and produce a certification grade",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeCertify(cmd, certifyCfg, certifyTrack, certifyModel)
		},
	}
	cf := certifyCmd.Flags()
	cf.StringVar(&certifyTrack, "track", "", "certification track (workloads, troubleshooting, networking, storage, pod-security, runtime-security, release-ops, platform-eng)")
	cf.StringVar(&certifyModel, "model", "", "model name (e.g. sonnet, opus)")
	cf.StringVar(&certifyCfg.EnvironmentProvider, "environment", certifyCfg.EnvironmentProvider, "environment provider (kind, k3d)")
	cf.StringVar(&certifyCfg.Provider, "provider", certifyCfg.Provider, "LLM provider")
	cf.StringVar(&certifyCfg.Adapter, "adapter", certifyCfg.Adapter, "agent adapter type (cli, mcp)")
	cf.StringVar(&certifyCfg.AgentCommand, "agent-command", certifyCfg.AgentCommand, "command to invoke the agent")
	cf.StringVar(&certifyCfg.ScenariosDir, "scenarios-dir", certifyCfg.ScenariosDir, "scenarios directory")
	cf.StringVar(&certifyCfg.RunsDir, "runs-dir", certifyCfg.RunsDir, "runs directory")
	cf.DurationVar(&certifyCfg.Timeout, "timeout", certifyCfg.Timeout, "per-scenario timeout")
	cf.BoolVar(&certifyCfg.ReuseCluster, "reuse-cluster", certifyCfg.ReuseCluster, "reuse kind cluster")
	cf.StringVar(&certifyCfg.ClusterName, "cluster-name", certifyCfg.ClusterName, "kind cluster name")
	cf.BoolVar(&certifyCfg.DryRun, "dry-run", certifyCfg.DryRun, "validate without running")
	cf.BoolVar(&certifyCfg.ProxyMode, "proxy-mode", false, "auto-record evidence for mutations")
	cf.BoolVar(&certifyCfg.SmartPrescribe, "smart-prescribe", false, "simplified prescribe (tool+operation+resource, v1.1.0)")
	cf.IntVar(&certifyCfg.MemoryWindow, "memory-window", -1, "memory window")
	cf.StringVar(&certifyCfg.EvidraURL, "evidra-url", certifyCfg.EvidraURL, "Evidra API URL for reporting results")
	cf.StringVar(&certifyCfg.EvidraAPIKey, "evidra-api-key", certifyCfg.EvidraAPIKey, "Evidra API key")
	cf.StringVar(&certifyCfg.EvidraBin, "evidra-bin", certifyCfg.EvidraBin, "evidra binary path")
	cf.StringVar(&certifyCfg.SystemPromptFile, "system-prompt-file", certifyCfg.SystemPromptFile, "system prompt file")
	cf.StringVar(&certifyCfg.Role, "role", certifyCfg.Role, "role-based skill (k8s-admin, security-ops, release-manager, platform-eng)")
	cf.StringVar(&certifyCfg.ContractVersion, "contract-version", certifyCfg.ContractVersion, "contract version")
	_ = certifyCmd.MarkFlagRequired("track")
	_ = certifyCmd.MarkFlagRequired("model")

	benchScenarios := []string{}
	benchModels := []string{}
	benchRepeats := 1
	benchCfg := config.Default()

	benchCmd := &cobra.Command{
		Use:   "bench",
		Short: "Run all scenarios (or filtered set) with aggregated results",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeBench(cmd, benchCfg, benchScenarios, benchModels, benchRepeats)
		},
	}
	bf := benchCmd.Flags()
	bf.StringSliceVar(&benchScenarios, "scenario", nil, "scenario filter (repeatable; default: all)")
	bf.StringSliceVar(&benchModels, "model", nil, "model (repeatable; default: sonnet)")
	bf.IntVar(&benchRepeats, "repeats", 1, "repeats per scenario/model")
	bf.StringVar(&benchCfg.EnvironmentProvider, "environment", benchCfg.EnvironmentProvider, "environment provider (kind, k3d)")
	bf.StringVar(&benchCfg.ScenariosDir, "scenarios-dir", benchCfg.ScenariosDir, "scenarios directory")
	bf.StringVar(&benchCfg.RunsDir, "runs-dir", benchCfg.RunsDir, "runs directory")
	bf.StringVar(&benchCfg.Provider, "provider", benchCfg.Provider, "LLM provider")
	bf.StringVar(&benchCfg.EvidraBin, "evidra-bin", benchCfg.EvidraBin, "evidra binary path")
	bf.StringVar(&benchCfg.SystemPromptFile, "system-prompt-file", benchCfg.SystemPromptFile, "system prompt file")
	bf.StringVar(&benchCfg.Role, "role", benchCfg.Role, "role-based skill (k8s-admin, security-ops, release-manager, platform-eng)")
	bf.StringVar(&benchCfg.ContractVersion, "contract-version", benchCfg.ContractVersion, "contract version")
	bf.BoolVar(&benchCfg.ProxyMode, "proxy-mode", false, "auto-record evidence for mutations")
	bf.BoolVar(&benchCfg.SmartPrescribe, "smart-prescribe", false, "simplified prescribe (tool+operation+resource, v1.1.0)")
	bf.DurationVar(&benchCfg.Timeout, "timeout", benchCfg.Timeout, "per-scenario timeout")
	bf.BoolVar(&benchCfg.ReuseCluster, "reuse-cluster", benchCfg.ReuseCluster, "reuse kind cluster")
	bf.StringVar(&benchCfg.ClusterName, "cluster-name", benchCfg.ClusterName, "kind cluster name")
	bf.BoolVar(&benchCfg.DryRun, "dry-run", benchCfg.DryRun, "dry-run mode")
	bf.IntVar(&benchCfg.MemoryWindow, "memory-window", -1, "memory window")
	bf.StringVar(&benchCfg.EvidraURL, "evidra-url", benchCfg.EvidraURL, "Evidra API URL for reporting results")
	bf.StringVar(&benchCfg.EvidraAPIKey, "evidra-api-key", benchCfg.EvidraAPIKey, "Evidra API key")
	bf.IntVar(&benchCfg.Parallel, "parallel", 1, "number of parallel workers (1 = sequential)")
	bf.StringVar(&benchCfg.DatabaseURL, "database-url", "", "PostgreSQL URL for job queue (env: BENCH_DATABASE_URL)")

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the bench service REST API",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := os.Getenv("BENCH_SERVICE_ADDR")
			if addr == "" {
				addr = ":8090"
			}
			// Override config from environment for containerized deployment.
			if v := os.Getenv("INFRA_BENCH_SCENARIOS_DIR"); v != "" {
				cfg.ScenariosDir = v
			}
			if v := os.Getenv("INFRA_BENCH_PROVIDER"); v != "" {
				cfg.Provider = v
			}
			if v := os.Getenv("INFRA_BENCH_MODEL"); v != "" {
				cfg.Model = v
			}
			if v := os.Getenv("INFRA_BENCH_MCP_SERVER"); v != "" {
				cfg.MCPServer = v
			}
			if v := os.Getenv("INFRA_BENCH_CLUSTER_NAME"); v != "" {
				cfg.ClusterName = v
			}
			if os.Getenv("INFRA_BENCH_REUSE_CLUSTER") == "true" {
				cfg.ReuseCluster = true
			}
			return serveAPI(cfg, addr)
		},
	}

	root.AddCommand(runCmd, scenarioCmd, labCmd, dbCmd, skillDeltaCmd, auditCmd, benchCmd, certifyCmd, serveCmd)
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
	if flags.Changed("proxy-mode") {
		labCfg.ProxyMode = cfg.ProxyMode
	}
	if flags.Changed("smart-prescribe") {
		labCfg.SmartPrescribe = cfg.SmartPrescribe
	}
	if flags.Changed("evidra-url") {
		labCfg.EvidraURL = cfg.EvidraURL
	}
	if flags.Changed("evidra-api-key") {
		labCfg.EvidraAPIKey = cfg.EvidraAPIKey
	}
	if flags.Changed("memory-window") {
		labCfg.MemoryWindow = cfg.MemoryWindow
	}
	if flags.Changed("reuse-cluster") {
		labCfg.ReuseCluster = cfg.ReuseCluster
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

	var envProvider environment.Provider
	switch cfg.EnvironmentProvider {
	case "k3d":
		p := environment.NewK3dProvider()
		p.ReuseExisting = cfg.ReuseCluster
		envProvider = p
	default:
		p := environment.NewKindProvider()
		p.ReuseExisting = cfg.ReuseCluster
		envProvider = p
	}
	runner := &environment.ExecRunner{}
	bootstrapper := environment.NewBootstrapper(runner)
	writer := artifact.NewWriter(cfg.RunsDir)

	reporter := report.NewReporter(report.Config{
		EvidencePath: filepath.Join(cfg.RunsDir, "evidra"),
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

// runScenarioOnceWithNamespace runs a scenario with a specific target namespace.
// Used by parallel workers where each worker has its own namespace.
func runScenarioOnceWithNamespace(ctx context.Context, cfg config.Config, s *scenario.Scenario, targetNS string) (*harness.RunResult, error) {
	var agentAdapter adapter.Adapter
	switch cfg.Adapter {
	case "cli":
		agentAdapter = adapter.NewCLIAdapter()
	case "mcp":
		agentAdapter = adapter.NewMCPAdapter()
	default:
		return nil, fmt.Errorf("unknown adapter: %s", cfg.Adapter)
	}

	var envProvider environment.Provider
	switch cfg.EnvironmentProvider {
	case "k3d":
		p := environment.NewK3dProvider()
		p.ReuseExisting = cfg.ReuseCluster
		envProvider = p
	default:
		p := environment.NewKindProvider()
		p.ReuseExisting = cfg.ReuseCluster
		envProvider = p
	}
	runner := &environment.ExecRunner{}
	bootstrapper := environment.NewBootstrapper(runner)
	writer := artifact.NewWriter(cfg.RunsDir)

	reporter := report.NewReporter(report.Config{
		EvidencePath: filepath.Join(cfg.RunsDir, "evidra"),
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

	result, runErr := h.Run(ctx, harness.RunRequest{
		Config:          cfg,
		Scenario:        s,
		TargetNamespace: targetNS,
	})
	if runErr != nil {
		return nil, runErr
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

func pushScenarios(scenariosDir, evidraURL, apiKey string) error {
	if evidraURL == "" || apiKey == "" {
		return fmt.Errorf("push-scenarios: --evidra-url and --evidra-api-key are required")
	}

	scenarios, err := scenario.LoadAll(scenariosDir)
	if err != nil {
		return fmt.Errorf("push-scenarios: load: %w", err)
	}

	type scenarioPayload struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Category    string   `json:"category"`
		Tags        []string `json:"tags"`
		Chaos       bool     `json:"chaos"`
		Evidra      bool     `json:"evidra"`
	}

	var items []scenarioPayload
	for _, s := range scenarios {
		tags := s.Tags
		if tags == nil {
			tags = []string{}
		}
		items = append(items, scenarioPayload{
			ID:          s.ID,
			Title:       s.Title,
			Description: s.Description,
			Category:    s.Category,
			Tags:        tags,
			Chaos:       len(s.Chaos.Steps) > 0,
			Evidra:      s.Evidra.Enabled,
		})
	}

	body, err := json.Marshal(map[string]any{"scenarios": items})
	if err != nil {
		return fmt.Errorf("push-scenarios: marshal: %w", err)
	}

	url := evidraURL + "/v1/bench/scenarios/sync"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("push-scenarios: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("push-scenarios: POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("push-scenarios: HTTP %d: %v", resp.StatusCode, result)
	}

	fmt.Printf("Pushed %v scenarios to %s (upserted: %v)\n", result["total"], evidraURL, result["upserted"])
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

	_, err := skilldelta.ReadBenchmarkJSON(filepath.Join(dir, "benchmark.json"))
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "HTML report generation has been removed; use the web UI instead.")
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

func executeBench(cmd *cobra.Command, cfg config.Config, scenarioFilters, models []string, repeats int) error {
	scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
	if err != nil {
		return fmt.Errorf("resolve scenarios dir: %w", err)
	}
	cfg.ScenariosDir = scenariosDir

	if len(models) == 0 {
		models = []string{"sonnet"}
	}
	if !cfg.DryRun && cfg.Provider == "" {
		cfg.Provider = "claude"
	}

	allScenarios, err := scenario.LoadAll(scenariosDir)
	if err != nil {
		return fmt.Errorf("load scenarios: %w", err)
	}

	// Filter scenarios
	var selected []*scenario.Scenario
	if len(scenarioFilters) == 0 {
		selected = allScenarios
	} else {
		filterSet := map[string]bool{}
		for _, f := range scenarioFilters {
			filterSet[f] = true
		}
		for _, s := range allScenarios {
			match := filterSet[s.ID] || filterSet[s.Path]
			if !match {
				for _, cat := range s.ResolvedCategories() {
					if filterSet[cat] {
						match = true
						break
					}
				}
			}
			if match {
				selected = append(selected, s)
			}
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("no scenarios matched filters")
	}

	// Parallel execution via River job queue.
	if cfg.Parallel > 1 {
		return executeBenchParallel(cmd, cfg, selected, models, repeats)
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	outDir := filepath.Join(cfg.RunsDir, "bench", stamp)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	type result struct {
		Scenario string `json:"scenario"`
		Model    string `json:"model"`
		Repeat   int    `json:"repeat"`
		Passed   bool   `json:"passed"`
		Duration string `json:"duration"`
		Error    string `json:"error,omitempty"`
	}

	var results []result
	total, passed, failed, errors := 0, 0, 0, 0

	skipped := 0
	for _, s := range selected {
		if s.Skip {
			skipped++
			reason := s.SkipReason
			if reason == "" {
				reason = "skip: true in scenario.yaml"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "SKIP %s — %s\n", s.ID, reason)
			continue
		}
		for _, model := range models {
			for rep := 1; rep <= repeats; rep++ {
				total++

				// Clean namespace between scenarios to avoid stale state.
				if cfg.ReuseCluster {
					cleanBenchNamespace(cmd.Context(), cfg.ClusterName, s)
				}

				runDir := filepath.Join(outDir, fmt.Sprintf("%s_%s_r%d", s.ID, model, rep))
				evidenceDir := filepath.Join(runDir, "evidence")

				runCfg := cfg
				runCfg.Scenario = s.Path
				runCfg.Model = model
				runCfg.RunsDir = runDir
				runCfg.EvidraEvidenceDir = evidenceDir

				label := fmt.Sprintf("[%d/%d] %s model=%s repeat=%d", total, len(selected)*len(models)*repeats, s.ID, model, rep)
				fmt.Fprintf(cmd.OutOrStdout(), "%s ...\n", label)

				runResult, runErr := runScenarioOnce(cmd.Context(), runCfg, s)

				r := result{
					Scenario: s.ID,
					Model:    model,
					Repeat:   rep,
				}

				if runErr != nil {
					r.Error = runErr.Error()
					var rfe *RunFailedError
					if ok := stderrors.As(runErr, &rfe); ok {
						r.Passed = false
						failed++
					} else {
						errors++
					}
				} else {
					r.Passed = runResult.Passed
					r.Duration = runResult.Duration.Round(time.Millisecond).String()
					if runResult.Passed {
						passed++
					} else {
						failed++
					}
				}

				verdict := "PASS"
				if !r.Passed {
					verdict = "FAIL"
				}
				if r.Error != "" && r.Error != fmt.Sprintf("scenario %s: verification failed", s.ID) {
					verdict = "ERROR"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %s %s\n", verdict, r.Duration, r.Error)
				results = append(results, r)
			}
		}
	}

	// Write summary
	summaryPath := filepath.Join(outDir, "summary.json")
	summary := map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"total":        total,
		"passed":       passed,
		"failed":       failed,
		"errors":       errors,
		"skipped":      skipped,
		"pass_rate":    fmt.Sprintf("%.0f%%", float64(passed)/float64(max(total, 1))*100),
		"models":       models,
		"repeats":      repeats,
		"scenarios":    len(selected),
		"results":      results,
	}
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(summaryPath, summaryJSON, 0644)

	// Step 2: Signal audit (if manifest exists)
	auditManifestPath := resolveAuditManifestPath("configs/signal-audit.yaml")
	var auditResult *signalaudit.Result
	auditPath := ""
	if auditManifestPath != "" {
		if _, err := os.Stat(auditManifestPath); err == nil {
			manifest, err := signalaudit.LoadManifest(auditManifestPath)
			if err == nil {
				runs, err := loadAuditRuns(outDir, "", "", "")
				if err == nil && len(runs) > 0 {
					ar := signalaudit.Analyze(manifest, runs)
					auditResult = &ar
					auditPath = filepath.Join(outDir, "signal-audit.json")
					auditJSON, _ := json.MarshalIndent(ar, "", "  ")
					os.WriteFile(auditPath, auditJSON, 0644)
				}
			}
		}
	}

	// Print summary
	fmt.Fprintf(cmd.OutOrStdout(), "\n")
	fmt.Fprintf(cmd.OutOrStdout(), "════════════════════════════════════════\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  BENCH RESULTS\n")
	fmt.Fprintf(cmd.OutOrStdout(), "════════════════════════════════════════\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Total:   %d\n", total)
	fmt.Fprintf(cmd.OutOrStdout(), "  Passed:  %d\n", passed)
	fmt.Fprintf(cmd.OutOrStdout(), "  Failed:  %d\n", failed)
	fmt.Fprintf(cmd.OutOrStdout(), "  Errors:  %d\n", errors)
	fmt.Fprintf(cmd.OutOrStdout(), "  Skipped: %d\n", skipped)
	fmt.Fprintf(cmd.OutOrStdout(), "  Rate:    %.0f%%\n", float64(passed)/float64(max(total, 1))*100)
	fmt.Fprintf(cmd.OutOrStdout(), "\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Artifacts:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "    Summary: %s\n", summaryPath)
	if auditResult != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "    Audit:   %s\n", auditPath)
		fmt.Fprintf(cmd.OutOrStdout(), "      audited=%d missing=%d forbidden=%d unstable=%d\n",
			auditResult.AuditedScenarioCount,
			auditResult.FindingTotals.MissingExpected,
			auditResult.FindingTotals.ForbiddenSignals,
			auditResult.FindingTotals.UnstableGroups)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "════════════════════════════════════════\n")

	if failed > 0 || errors > 0 {
		return fmt.Errorf("bench: %d failed, %d errors out of %d", failed, errors, total)
	}
	return nil
}

// cleanBenchNamespace deletes and recreates the bench namespace between scenario runs
// to prevent stale state from previous runs causing bootstrap failures.
func cleanBenchNamespace(ctx context.Context, clusterName string, s *scenario.Scenario) {
	// Use the provider's kubeconfig temp file (same path convention as providers).
	kubeconfigPath := filepath.Join(os.TempDir(), fmt.Sprintf("bench-cli-%s-kubeconfig", clusterName))
	if _, err := os.Stat(kubeconfigPath); err != nil {
		// Fallback to KUBECONFIG env or default.
		kubeconfigPath = os.Getenv("KUBECONFIG")
		if kubeconfigPath == "" {
			home, _ := os.UserHomeDir()
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	// Collect namespaces from the scenario scope, default to "bench".
	namespaces := s.Scope.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{"bench"}
	}

	// No context arg needed — the kubeconfig from providers has the correct context.
	contextArg := ""

	for _, ns := range namespaces {
		args := []string{"--kubeconfig", kubeconfigPath}
		if contextArg != "" {
			args = append(args, contextArg)
		}
		args = append(args, "delete", "namespace", ns, "--ignore-not-found", "--timeout=60s")
		cmd := exec.CommandContext(ctx, "kubectl", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[bench] namespace cleanup %s: %v: %s", ns, err, strings.TrimSpace(string(out)))
		} else {
			log.Printf("[bench] namespace cleanup: %s deleted", ns)
		}

		// Wait until namespace is fully gone to avoid "being terminated" errors.
		baseArgs := []string{"--kubeconfig", kubeconfigPath}
		if contextArg != "" {
			baseArgs = append(baseArgs, contextArg)
		}
		for i := 0; i < 15; i++ {
			checkArgs := append(baseArgs, "get", "namespace", ns, "--no-headers")
			if err := exec.CommandContext(ctx, "kubectl", checkArgs...).Run(); err != nil {
				break // namespace gone
			}
			time.Sleep(2 * time.Second)
		}
	}
}

func executeBenchParallel(cmd *cobra.Command, cfg config.Config, selected []*scenario.Scenario, models []string, repeats int) error {
	dbURL := cfg.ResolveDatabaseURL()
	if dbURL == "" {
		return fmt.Errorf("--database-url required for parallel execution (or set BENCH_DATABASE_URL)")
	}

	ctx := cmd.Context()
	var completed, passed, failed int64

	runFn := buildParallelRunFunc(cfg, &completed, &passed, &failed)

	client, err := jobqueue.NewClient(ctx, dbURL, cfg.Parallel, runFn)
	if err != nil {
		return fmt.Errorf("job queue: %w", err)
	}

	if err := client.Migrate(ctx); err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}

	// Enqueue: models × scenarios × repeats.
	var total int64
	for _, model := range models {
		for rep := 1; rep <= repeats; rep++ {
			var ids []string
			for _, s := range selected {
				if !s.Skip {
					ids = append(ids, s.Path)
				}
			}
			jobID := fmt.Sprintf("bench-%s-r%d-%s", model, rep, time.Now().UTC().Format("20060102-150405"))
			if err := client.InsertBatch(ctx, ids, model, cfg.Provider, cfg.MCPServer, jobID, "", cfg.Parallel); err != nil {
				return fmt.Errorf("enqueue: %w", err)
			}
			total += int64(len(ids))
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Enqueued %d runs across %d workers\n", total, cfg.Parallel)

	// Start workers.
	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("start workers: %w", err)
	}

	// Wait for all jobs to complete.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		c := atomic.LoadInt64(&completed)
		if c >= total {
			break
		}
		select {
		case <-ctx.Done():
			client.Stop(ctx)
			return ctx.Err()
		case <-ticker.C:
			p := atomic.LoadInt64(&passed)
			f := atomic.LoadInt64(&failed)
			fmt.Fprintf(cmd.OutOrStdout(), "Progress: %d/%d (pass=%d fail=%d)\n", c, total, p, f)
		}
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client.Stop(stopCtx)

	p := atomic.LoadInt64(&passed)
	f := atomic.LoadInt64(&failed)
	fmt.Fprintf(cmd.OutOrStdout(), "\nCompleted: %d total, %d passed, %d failed\n", total, p, f)
	return nil
}

// buildParallelRunFunc creates the RunFunc shared by CLI bench and serve.
// Each invocation gets an isolated workspace with namespace rewriting.
func buildParallelRunFunc(cfg config.Config, completed, passed, failed *int64) jobqueue.RunFunc {
	return func(ctx context.Context, args jobqueue.BenchJobArgs, ns string) error {
		ws, err := workspace.New(
			fmt.Sprintf("%s-%s-%d", args.ScenarioID, args.Model, time.Now().UnixNano()),
			cfg.ScenariosDir,
		)
		if err != nil {
			return fmt.Errorf("workspace: %w", err)
		}
		defer ws.Cleanup()

		// Rewrite namespace in workspace copy.
		scenarioDir := filepath.Join(ws.ScenariosDir, filepath.Dir(args.ScenarioID))
		if err := workspace.RewriteNamespace(scenarioDir, "bench", ns); err != nil {
			log.Printf("[worker-%d] namespace rewrite warning: %v", args.WorkerID, err)
		}

		workerCfg := cfg
		workerCfg.Scenario = args.ScenarioID
		workerCfg.ScenariosDir = ws.ScenariosDir
		workerCfg.RunsDir = ws.RunsDir
		workerCfg.EvidraEvidenceDir = ws.EvidenceDir
		workerCfg.Model = args.Model

		scenarioPath := filepath.Join(ws.ScenariosDir, args.ScenarioID)
		s, loadErr := scenario.Load(scenarioPath)
		if loadErr != nil {
			return fmt.Errorf("load scenario: %w", loadErr)
		}

		// Pass the worker namespace to the harness via RunRequest.TargetNamespace
		// so cleanup and env vars use the correct namespace.
		runResult, runErr := runScenarioOnceWithNamespace(ctx, workerCfg, s, ns)
		atomic.AddInt64(completed, 1)

		if runErr != nil {
			atomic.AddInt64(failed, 1)
			log.Printf("[worker-%d] FAIL %s: %v", args.WorkerID, args.ScenarioID, runErr)
		} else if runResult != nil && runResult.Passed {
			atomic.AddInt64(passed, 1)
			log.Printf("[worker-%d] PASS %s (%s)", args.WorkerID, args.ScenarioID, runResult.Duration)
		} else {
			atomic.AddInt64(failed, 1)
			log.Printf("[worker-%d] FAIL %s", args.WorkerID, args.ScenarioID)
		}
		return nil // don't fail River job on scenario failure
	}
}

func main() {
	harness.SetVersion(version, commit)
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
