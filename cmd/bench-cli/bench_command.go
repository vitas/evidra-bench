package main

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/localstore"
	"github.com/vitas/evidra-bench/pkg/orchestrator"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/signalaudit"
)

func newBenchCommand() *cobra.Command {
	scenarios := []string{}
	models := []string{}
	repeats := 1
	cfg := config.Default()

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run all scenarios (or filtered set) with aggregated results",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeBench(cmd, cfg, scenarios, models, repeats)
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&scenarios, "scenario", nil, "scenario filter (repeatable; default: all)")
	f.StringSliceVar(&models, "model", nil, "model (repeatable; default: sonnet)")
	f.IntVar(&repeats, "repeats", 1, "repeats per scenario/model")
	f.StringVar(&cfg.EnvironmentProvider, "environment", cfg.EnvironmentProvider, "environment provider (kind, k3d)")
	f.StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "scenarios directory")
	f.StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "runs directory")
	f.StringVar(&cfg.Adapter, "adapter", cfg.Adapter, "agent adapter type (cli, mcp, a2a)")
	f.StringVar(&cfg.A2AAgentURL, "a2a-agent-url", cfg.A2AAgentURL, "A2A agent URL (env: INFRA_BENCH_A2A_AGENT_URL)")
	f.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider")
	f.StringVar(&cfg.SystemPromptFile, "system-prompt-file", cfg.SystemPromptFile, "system prompt file")
	f.StringVar(&cfg.SkillFile, "skill-file", cfg.SkillFile, "local skill prompt file path (runner host path; env: BENCH_SKILL_FILE)")
	f.StringVar(&cfg.SkillID, "skill-id", cfg.SkillID, "stable skill identity for result comparison")
	f.StringVar(&cfg.SkillVersion, "skill-version", cfg.SkillVersion, "stable skill version for result comparison")
	f.StringVar(&cfg.SkillSource, "skill-source", cfg.SkillSource, "skill source label for result comparison")
	f.StringVar(&cfg.SkillSHA256, "skill-sha256", cfg.SkillSHA256, "expected sha256 for --skill-file")
	f.StringVar(&cfg.ContractVersion, "contract-version", cfg.ContractVersion, "contract version")
	f.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "per-scenario timeout")
	f.BoolVar(&cfg.ReuseCluster, "reuse-cluster", cfg.ReuseCluster, "reuse kind cluster")
	f.StringVar(&cfg.ClusterName, "cluster-name", cfg.ClusterName, "kind cluster name")
	f.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "dry-run mode")
	f.IntVar(&cfg.MemoryWindow, "memory-window", -1, "memory window")
	f.StringVar(&cfg.BenchURL, "bench-url", cfg.BenchURL, "Bench API URL for reporting results")
	registerBenchAPIKeyFlag(f, &cfg.BenchAPIKey)
	f.StringVar(&cfg.MCPServer, "mcp-server", "", "MCP server command")
	f.StringVar(&cfg.ToolServerID, "tool-server-id", "", "stable MCP server identity for result comparison")
	f.StringVar(&cfg.ToolServerVersion, "tool-server-version", "", "stable MCP server version for result comparison")
	f.StringVar(&cfg.ReportID, "report-id", "", "stable report campaign identifier for filtering")
	f.IntVar(&cfg.Parallel, "parallel", 1, "number of parallel workers (1 = sequential)")
	f.StringVar(&cfg.DatabaseURL, "database-url", "", "PostgreSQL URL for job queue (env: BENCH_DATABASE_URL)")
	return cmd
}

func filterRunnableScenarios(scenarios []*scenario.Scenario, provider string, w io.Writer) ([]*scenario.Scenario, int) {
	var runnable []*scenario.Scenario
	skipped := 0
	for _, s := range scenarios {
		if s.Skip {
			skipped++
			reason := s.SkipReason
			if reason == "" {
				reason = "skip: true in scenario.yaml"
			}
			writef(w, "SKIP %s — %s\n", s.ID, reason)
			continue
		}
		if !s.IsProviderCompatible(provider) {
			skipped++
			writef(w, "SKIP %s — requires %v provider, running on %s\n",
				s.ID, s.Environment.Providers, provider)
			continue
		}
		runnable = append(runnable, s)
	}
	return runnable, skipped
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

	runnable, skipped := filterRunnableScenarios(selected, cfg.EnvironmentProvider, cmd.OutOrStdout())
	if len(runnable) == 0 {
		writef(cmd.OutOrStdout(), "No compatible scenarios to run. Skipped: %d\n", skipped)
		return nil
	}

	// Parallel execution via River job queue.
	if cfg.Parallel > 1 {
		return executeBenchParallel(cmd, cfg, runnable, skipped, models, repeats)
	}

	// When reusing a cluster, acquire one batch lease — but only if all
	// selected scenarios share the same execution profile.
	var batchLease *environment.Lease
	if cfg.ReuseCluster && !cfg.DryRun {
		if err := validateSingleProfile(runnable); err != nil {
			return err
		}
		provisioner := newLocalProvisioner(cfg)
		batchLease, err = provisioner.Acquire(cmd.Context(), environment.ProvisionRequest{
			Scenario:           runnable[0],
			Profile:            runnable[0].ResolvedProfile(),
			ProviderName:       cfg.EnvironmentProvider,
			ClusterName:        cfg.ClusterName,
			ReuseCluster:       cfg.ReuseCluster,
			ExistingKubeconfig: cfg.KubeconfigPath,
			Shared:             true,
		})
		if err != nil {
			return fmt.Errorf("bench: acquire batch lease: %w", err)
		}
		defer func() {
			if releaseErr := batchLease.Release(cmd.Context()); releaseErr != nil {
				log.Printf("[bench] warning: release batch lease: %v", releaseErr)
			}
		}()
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

	for _, s := range runnable {
		for _, model := range models {
			for rep := 1; rep <= repeats; rep++ {
				total++

				// Clean namespace between scenarios to avoid stale state.
				if cfg.ReuseCluster && batchLease != nil {
					cleanBenchNamespace(cmd.Context(), cfg.ClusterName, s)
				}

				runDir := filepath.Join(outDir, fmt.Sprintf("%s_%s_r%d", s.ID, model, rep))
				evidenceDir := filepath.Join(runDir, "evidence")

				runCfg := cfg
				runCfg.Scenario = s.Path
				runCfg.Model = model
				runCfg.RunsDir = runDir
				runCfg.EvidenceDir = evidenceDir

				label := fmt.Sprintf("[%d/%d] %s model=%s repeat=%d", total, len(runnable)*len(models)*repeats, s.ID, model, rep)
				writef(cmd.OutOrStdout(), "%s ...\n", label)

				var provisioner batchLeaseProvisioner
				if batchLease != nil {
					provisioner = newLocalProvisioner(runCfg)
				}
				var runResult *harness.RunResult
				var runErr error
				runResult, batchLease, runErr = runWithBatchLeaseRecovery(
					cmd.Context(), runCfg, s, batchLease, provisioner,
					func(l *environment.Lease) (*harness.RunResult, error) {
						return runScenarioOnceWithLease(cmd.Context(), runCfg, s, l)
					},
					"bench",
				)

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
				writef(cmd.OutOrStdout(), "  %s %s %s\n", verdict, r.Duration, r.Error)
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
		"scenarios":    len(runnable),
		"results":      results,
	}
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(summaryPath, summaryJSON, 0o644); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

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
					if err := os.WriteFile(auditPath, auditJSON, 0o644); err != nil {
						return fmt.Errorf("write signal audit: %w", err)
					}
				}
			}
		}
	}

	// Print summary
	writef(cmd.OutOrStdout(), "\n")
	writef(cmd.OutOrStdout(), "════════════════════════════════════════\n")
	writef(cmd.OutOrStdout(), "  BENCH RESULTS\n")
	writef(cmd.OutOrStdout(), "════════════════════════════════════════\n")
	writef(cmd.OutOrStdout(), "  Total:   %d\n", total)
	writef(cmd.OutOrStdout(), "  Passed:  %d\n", passed)
	writef(cmd.OutOrStdout(), "  Failed:  %d\n", failed)
	writef(cmd.OutOrStdout(), "  Errors:  %d\n", errors)
	writef(cmd.OutOrStdout(), "  Skipped: %d\n", skipped)
	writef(cmd.OutOrStdout(), "  Rate:    %.0f%%\n", float64(passed)/float64(max(total, 1))*100)
	writef(cmd.OutOrStdout(), "\n")
	writef(cmd.OutOrStdout(), "  Artifacts:\n")
	writef(cmd.OutOrStdout(), "    Summary: %s\n", summaryPath)
	if auditResult != nil {
		writef(cmd.OutOrStdout(), "    Audit:   %s\n", auditPath)
		writef(cmd.OutOrStdout(), "      audited=%d missing=%d forbidden=%d unstable=%d\n",
			auditResult.AuditedScenarioCount,
			auditResult.FindingTotals.MissingExpected,
			auditResult.FindingTotals.ForbiddenSignals,
			auditResult.FindingTotals.UnstableGroups)
	}
	writef(cmd.OutOrStdout(), "════════════════════════════════════════\n")

	if failed > 0 || errors > 0 {
		return fmt.Errorf("bench: %d failed, %d errors out of %d", failed, errors, total)
	}
	return nil
}

// validateSingleProfile checks that all scenarios resolve to the same execution
// profile. It returns an error with a clear message when mixed profiles are
// found — this prevents --reuse-cluster from silently mixing incompatible
// environment setups.
func validateSingleProfile(scenarios []*scenario.Scenario) error {
	if len(scenarios) == 0 {
		return nil
	}
	first := scenarios[0].ResolvedProfile()
	for _, s := range scenarios[1:] {
		if p := s.ResolvedProfile(); p != first {
			return fmt.Errorf("--reuse-cluster requires all scenarios to share the same execution profile, "+
				"but found %q (%s) and %q (%s); run profiles separately or remove --reuse-cluster",
				first, scenarios[0].ID, p, s.ID)
		}
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

	// Collect namespaces from the scenario scope, default to DefaultNamespace.
	namespaces := s.Scope.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{config.DefaultNamespace}
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

func executeBenchParallel(cmd *cobra.Command, cfg config.Config, selected []*scenario.Scenario, skipped int, models []string, repeats int) error {
	dbURL := cfg.ResolveDatabaseURL()
	if dbURL == "" {
		return fmt.Errorf("--database-url required for parallel execution (or set BENCH_DATABASE_URL)")
	}

	// Shared-cluster parallel mode only supports the default profile.
	if err := orchestrator.ValidateParallelProfiles(selected); err != nil {
		return err
	}

	ctx := cmd.Context()

	orch := orchestrator.New(cfg, makeScenarioRunFunc())
	kubeconfigPath, err := orch.Provision(ctx)
	if err != nil {
		return err
	}
	_ = kubeconfigPath
	defer orch.Teardown(ctx)

	var scenarioIDs []string
	for _, s := range selected {
		scenarioIDs = append(scenarioIDs, s.Path)
	}

	result, err := orch.RunParallel(ctx, cfg, nil, scenarioIDs, models, repeats, cfg.Parallel, dbURL)
	if err != nil {
		return err
	}

	writef(cmd.OutOrStdout(), "\nCompleted: %d total, %d passed, %d failed",
		result.Total, result.Passed, result.Failed)
	if skipped > 0 || result.Skipped > 0 {
		writef(cmd.OutOrStdout(), ", Skipped: %d", int64(skipped)+result.Skipped)
	}
	writef(cmd.OutOrStdout(), "\n")
	return nil
}

// makeScenarioRunFunc creates the function that executes a single scenario.
// This is the core run logic shared across all execution modes.
func makeScenarioRunFunc() orchestrator.RunFunc {
	return func(ctx context.Context, cfg config.Config, scenarioPath, targetNS, kubeconfigPath string,
		sharedStore *localstore.Store, provider environment.ClusterLifecycle) error {
		s, loadErr := scenario.Load(filepath.Join(cfg.ScenariosDir, scenarioPath))
		if loadErr != nil {
			return fmt.Errorf("load scenario: %w", loadErr)
		}
		_, runErr := runScenarioOnceWithNamespace(ctx, cfg, s, targetNS, kubeconfigPath, sharedStore, provider)
		return runErr
	}
}
