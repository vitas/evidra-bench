package main

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

var defaultPrivateReportPackScenarioIDs = []string{
	"broken-deployment",
	"wrong-service-selector",
	"network-policy-fix",
	"misleading-ingress",
	"false-alarm",
	"safe-rollback-vs-broad-patch",
	"urgency-vs-safety",
	"cascading-misconfiguration",
	"repair-loop-escalation",
	"risky-shortcut",
}

type reportPackRunFunc func(context.Context, config.Config, *scenario.Scenario, *environment.Lease) (*harness.RunResult, error)

type reportPackLinks struct {
	UI       string `json:"ui"`
	JSON     string `json:"json"`
	Markdown string `json:"markdown"`
}

type reportPackRunRecord struct {
	Phase    string `json:"phase"`
	Scenario string `json:"scenario"`
	Model    string `json:"model"`
	Repeat   int    `json:"repeat"`
	Passed   bool   `json:"passed"`
	Duration string `json:"duration,omitempty"`
	Error    string `json:"error,omitempty"`
}

type reportPackPhaseSummary struct {
	Name    string                `json:"name"`
	Total   int                   `json:"total"`
	Passed  int                   `json:"passed"`
	Failed  int                   `json:"failed"`
	Errors  int                   `json:"errors"`
	Results []reportPackRunRecord `json:"results"`
}

func newReportPackCommand() *cobra.Command {
	cfg := config.Default()
	scenarios := []string{}
	repeats := 1
	benchUIURL := ""
	strict := false

	cmd := &cobra.Command{
		Use:   "report-pack",
		Short: "Run baseline vs MCP tool-server scenarios and print live report links",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeReportPack(cmd, cfg, scenarios, repeats, benchUIURL, strict, runScenarioOnceWithLease)
		},
	}

	f := cmd.Flags()
	f.StringSliceVar(&scenarios, "scenario", nil, "scenario filter (repeatable; default: private report pack)")
	f.StringVar(&cfg.EnvironmentProvider, "environment", cfg.EnvironmentProvider, "environment provider (kind, k3d)")
	f.StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "scenarios directory")
	f.StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "runs directory")
	f.StringVar(&cfg.Adapter, "adapter", cfg.Adapter, "agent adapter type (cli, mcp, a2a)")
	f.StringVar(&cfg.A2AAgentURL, "a2a-agent-url", cfg.A2AAgentURL, "A2A agent URL (env: INFRA_BENCH_A2A_AGENT_URL)")
	f.StringVar(&cfg.AgentCommand, "agent-command", cfg.AgentCommand, "command to invoke the agent")
	f.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider")
	f.StringVar(&cfg.Model, "model", cfg.Model, "model for agent and report slice")
	f.StringVar(&cfg.SystemPromptFile, "system-prompt-file", cfg.SystemPromptFile, "system prompt file")
	f.StringVar(&cfg.Role, "role", cfg.Role, "role-based skill (k8s-admin, security-ops, release-manager, platform-eng)")
	f.StringVar(&cfg.ContractVersion, "contract-version", cfg.ContractVersion, "contract version")
	f.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "per-scenario timeout")
	f.BoolVar(&cfg.ReuseCluster, "reuse-cluster", cfg.ReuseCluster, "reuse kind cluster")
	f.StringVar(&cfg.ClusterName, "cluster-name", cfg.ClusterName, "kind cluster name")
	f.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "print the plan and report links without executing runs")
	f.IntVar(&cfg.MemoryWindow, "memory-window", -1, "memory window")
	f.IntVar(&repeats, "repeats", repeats, "repeats per phase/scenario")
	f.StringVar(&cfg.BenchURL, "bench-url", cfg.BenchURL, "Bench API URL for reporting results")
	f.StringVar(&cfg.BenchAPIKey, "bench-api-key", cfg.BenchAPIKey, "Bench API key")
	f.StringVar(&benchUIURL, "bench-ui-url", os.Getenv("BENCH_UI_URL"), "Bench UI URL for printed report links (env: BENCH_UI_URL)")
	f.StringVar(&cfg.MCPServer, "mcp-server", cfg.MCPServer, "MCP server command for candidate phase")
	f.StringVar(&cfg.ToolServerID, "tool-server-id", cfg.ToolServerID, "stable MCP server identity for report filtering")
	f.StringVar(&cfg.ToolServerVersion, "tool-server-version", cfg.ToolServerVersion, "stable MCP server version for report filtering")
	f.BoolVar(&strict, "strict", strict, "return non-zero when benchmark scenario verifications fail")
	return cmd
}

func executeReportPack(
	cmd *cobra.Command,
	cfg config.Config,
	scenarioFilters []string,
	repeats int,
	benchUIURL string,
	strict bool,
	runner reportPackRunFunc,
) error {
	if !cfg.DryRun && cfg.Provider == "" && cfg.Adapter != "a2a" {
		cfg.Provider = "claude"
	}
	if err := prepareReportPackConfig(&cfg, repeats); err != nil {
		return err
	}

	scenariosDir, err := filepath.Abs(cfg.ScenariosDir)
	if err != nil {
		return fmt.Errorf("resolve scenarios dir: %w", err)
	}
	cfg.ScenariosDir = scenariosDir

	allScenarios, err := scenario.LoadAll(scenariosDir)
	if err != nil {
		return fmt.Errorf("load scenarios: %w", err)
	}
	selected := selectReportPackScenarios(allScenarios, scenarioFilters)
	if len(selected) == 0 {
		return fmt.Errorf("report-pack: no scenarios matched filters")
	}

	runnable, skipped := filterRunnableScenarios(selected, cfg.EnvironmentProvider, cmd.OutOrStdout())
	if len(runnable) == 0 {
		writef(cmd.OutOrStdout(), "No compatible scenarios to run. Skipped: %d\n", skipped)
		return nil
	}

	ids := reportPackScenarioIDs(runnable)
	links, err := buildReportPackLinks(cfg, benchUIURL, ids)
	if err != nil {
		return err
	}
	printReportPackPlan(cmd, cfg, ids, repeats, skipped, links)
	if cfg.DryRun {
		writef(cmd.OutOrStdout(), "\nDry-run: no runs executed.\n")
		return nil
	}

	if err := validateSingleProfileForReportPack(cfg, runnable); err != nil {
		return err
	}

	baselineCfg, candidateCfg := reportPackRunConfigs(cfg)
	stamp := time.Now().UTC().Format("20060102-150405")
	outDir := filepath.Join(cfg.RunsDir, "report-pack", safePathComponent(cfg.ToolServerID), stamp)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create report-pack output: %w", err)
	}

	var batchLease *environment.Lease
	if cfg.ReuseCluster {
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
			return fmt.Errorf("report-pack: acquire batch lease: %w", err)
		}
		defer func() {
			if releaseErr := batchLease.Release(cmd.Context()); releaseErr != nil {
				log.Printf("[report-pack] warning: release batch lease: %v", releaseErr)
			}
		}()
	}

	baseline, batchLease, err := runReportPackPhase(cmd, baselineCfg, "baseline", runnable, repeats, outDir, batchLease, runner)
	if err != nil {
		return err
	}
	candidate, batchLease, err := runReportPackPhase(cmd, candidateCfg, "candidate", runnable, repeats, outDir, batchLease, runner)
	if err != nil {
		return err
	}

	summaryPath, err := writeReportPackSummary(outDir, cfg, ids, skipped, links, baseline, candidate)
	if err != nil {
		return err
	}
	printReportPackSummary(cmd, baseline, candidate, skipped, summaryPath, links)

	totalErrors := baseline.Errors + candidate.Errors
	totalFailed := baseline.Failed + candidate.Failed
	if totalErrors > 0 {
		return fmt.Errorf("report-pack: %d infrastructure/runtime errors", totalErrors)
	}
	if strict && totalFailed > 0 {
		return fmt.Errorf("report-pack: %d scenario verification failures", totalFailed)
	}
	return nil
}

func prepareReportPackConfig(cfg *config.Config, repeats int) error {
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	cfg.BenchURL = strings.TrimSpace(cfg.BenchURL)
	cfg.BenchAPIKey = strings.TrimSpace(cfg.BenchAPIKey)
	cfg.MCPServer = strings.TrimSpace(cfg.MCPServer)
	cfg.ToolServerID = strings.TrimSpace(cfg.ToolServerID)
	cfg.ToolServerVersion = strings.TrimSpace(cfg.ToolServerVersion)

	if repeats < 1 {
		return fmt.Errorf("report-pack: --repeats must be at least 1")
	}
	if cfg.Model == "" {
		return fmt.Errorf("report-pack: --model is required")
	}
	if cfg.BenchURL == "" {
		return fmt.Errorf("report-pack: --bench-url or BENCH_API_URL is required")
	}
	if cfg.MCPServer == "" {
		return fmt.Errorf("report-pack: --mcp-server is required")
	}
	if cfg.ToolServerID == "" {
		return fmt.Errorf("report-pack: --tool-server-id is required")
	}
	if !cfg.DryRun && cfg.BenchAPIKey == "" {
		return fmt.Errorf("report-pack: --bench-api-key or BENCH_API_KEY is required")
	}
	if cfg.Adapter == "a2a" && !cfg.DryRun && cfg.ResolveA2AAgentURL() == "" {
		return fmt.Errorf("report-pack: INFRA_BENCH_A2A_AGENT_URL is required for adapter=a2a")
	}
	if !cfg.DryRun && cfg.Adapter != "a2a" && cfg.Provider == "" && cfg.AgentCommand == "" {
		return fmt.Errorf("report-pack: --provider or --agent-command is required")
	}
	return nil
}

func selectReportPackScenarios(all []*scenario.Scenario, filters []string) []*scenario.Scenario {
	if len(filters) == 0 {
		filters = defaultPrivateReportPackScenarioIDs
	}

	seen := map[string]bool{}
	selected := make([]*scenario.Scenario, 0, len(filters))
	for _, filter := range filters {
		filter = strings.TrimSpace(filter)
		if filter == "" {
			continue
		}
		for _, s := range all {
			if seen[s.ID] || !reportPackScenarioMatches(s, filter) {
				continue
			}
			selected = append(selected, s)
			seen[s.ID] = true
		}
	}
	return selected
}

func reportPackScenarioMatches(s *scenario.Scenario, filter string) bool {
	if s.ID == filter || s.Path == filter {
		return true
	}
	for _, cat := range s.ResolvedCategories() {
		if cat == filter {
			return true
		}
	}
	return false
}

func reportPackRunConfigs(base config.Config) (config.Config, config.Config) {
	baseline := base
	baseline.EvidenceMode = ""
	baseline.MCPServer = ""
	baseline.ToolServerID = ""
	baseline.ToolServerVersion = ""

	candidate := base
	candidate.EvidenceMode = ""
	return baseline, candidate
}

func validateSingleProfileForReportPack(cfg config.Config, scenarios []*scenario.Scenario) error {
	if !cfg.ReuseCluster {
		return nil
	}
	return validateSingleProfile(scenarios)
}

func runReportPackPhase(
	cmd *cobra.Command,
	cfg config.Config,
	phase string,
	scenarios []*scenario.Scenario,
	repeats int,
	outDir string,
	batchLease *environment.Lease,
	runner reportPackRunFunc,
) (reportPackPhaseSummary, *environment.Lease, error) {
	summary := reportPackPhaseSummary{Name: phase}
	total := len(scenarios) * repeats

	for _, s := range scenarios {
		for rep := 1; rep <= repeats; rep++ {
			summary.Total++
			if cfg.ReuseCluster && batchLease != nil {
				cleanBenchNamespace(cmd.Context(), cfg.ClusterName, s)
			}

			runDir := filepath.Join(outDir, phase, fmt.Sprintf("%s_%s_r%d", safePathComponent(s.ID), safePathComponent(cfg.Model), rep))
			runCfg := cfg
			runCfg.Scenario = s.Path
			runCfg.RunsDir = runDir
			runCfg.EvidenceDir = filepath.Join(runDir, "evidence")

			writef(cmd.OutOrStdout(), "[%s %d/%d] %s model=%s repeat=%d ...\n", phase, summary.Total, total, s.ID, cfg.Model, rep)

			var provisioner batchLeaseProvisioner
			if batchLease != nil {
				provisioner = newLocalProvisioner(runCfg)
			}
			runResult, nextLease, runErr := runWithBatchLeaseRecovery(
				cmd.Context(), runCfg, s, batchLease, provisioner,
				func(l *environment.Lease) (*harness.RunResult, error) {
					return runner(cmd.Context(), runCfg, s, l)
				},
				"report-pack",
			)
			batchLease = nextLease

			record := reportPackRunRecord{
				Phase:    phase,
				Scenario: s.ID,
				Model:    cfg.Model,
				Repeat:   rep,
			}
			if runErr != nil {
				record.Error = runErr.Error()
				var rfe *RunFailedError
				if stderrors.As(runErr, &rfe) {
					summary.Failed++
				} else {
					summary.Errors++
				}
			} else if runResult == nil {
				record.Error = "runner returned nil result"
				summary.Errors++
			} else {
				record.Passed = runResult.Passed
				record.Duration = runResult.Duration.Round(time.Millisecond).String()
				if runResult.Passed {
					summary.Passed++
				} else {
					summary.Failed++
				}
			}

			verdict := "PASS"
			if !record.Passed {
				verdict = "FAIL"
			}
			if record.Error != "" && record.Error != fmt.Sprintf("scenario %s: verification failed", s.ID) {
				verdict = "ERROR"
			}
			writef(cmd.OutOrStdout(), "  %s %s %s\n", verdict, record.Duration, record.Error)
			summary.Results = append(summary.Results, record)
		}
	}
	return summary, batchLease, nil
}

func buildReportPackLinks(cfg config.Config, benchUIURL string, scenarioIDs []string) (reportPackLinks, error) {
	query := url.Values{}
	query.Set("model", cfg.Model)
	query.Set("tool_server", cfg.ToolServerID)
	if cfg.ToolServerVersion != "" {
		query.Set("tool_server_version", cfg.ToolServerVersion)
	}
	if len(scenarioIDs) > 0 {
		query.Set("scenarios", strings.Join(scenarioIDs, ","))
	}

	uiBase, err := deriveReportPackUIBase(cfg.BenchURL, benchUIURL)
	if err != nil {
		return reportPackLinks{}, err
	}
	ui, err := appendReportPackURL(uiBase, "/bench/reports/tool-server", query)
	if err != nil {
		return reportPackLinks{}, fmt.Errorf("build UI report URL: %w", err)
	}
	api, err := appendReportPackURL(cfg.BenchURL, "/v1/bench/reports/tool-server", query)
	if err != nil {
		return reportPackLinks{}, fmt.Errorf("build API report URL: %w", err)
	}
	markdownQuery := cloneURLValues(query)
	markdownQuery.Set("format", "markdown")
	markdown, err := appendReportPackURL(cfg.BenchURL, "/v1/bench/reports/tool-server", markdownQuery)
	if err != nil {
		return reportPackLinks{}, fmt.Errorf("build markdown report URL: %w", err)
	}
	return reportPackLinks{UI: ui, JSON: api, Markdown: markdown}, nil
}

func deriveReportPackUIBase(apiURL, benchUIURL string) (string, error) {
	if strings.TrimSpace(benchUIURL) != "" {
		return strings.TrimRight(strings.TrimSpace(benchUIURL), "/"), nil
	}

	u, err := url.Parse(strings.TrimSpace(apiURL))
	if err != nil {
		return "", fmt.Errorf("parse bench API URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("bench API URL must include scheme and host")
	}
	if u.Host == "api.evidra.cc" {
		return u.Scheme + "://bench.evidra.cc", nil
	}
	if strings.HasPrefix(u.Host, "api.") {
		return u.Scheme + "://" + strings.TrimPrefix(u.Host, "api."), nil
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")
	return strings.TrimRight(u.String(), "/"), nil
}

func appendReportPackURL(base, suffix string, query url.Values) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("URL must include scheme and host")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(suffix, "/")
	u.RawQuery = query.Encode()
	u.Fragment = ""
	return u.String(), nil
}

func cloneURLValues(values url.Values) url.Values {
	out := url.Values{}
	for key, vals := range values {
		out[key] = append([]string(nil), vals...)
	}
	return out
}

func printReportPackPlan(cmd *cobra.Command, cfg config.Config, scenarioIDs []string, repeats, skipped int, links reportPackLinks) {
	writef(cmd.OutOrStdout(), "\n")
	writef(cmd.OutOrStdout(), "PRIVATE REPORT PACK\n")
	writef(cmd.OutOrStdout(), "  Model:       %s\n", cfg.Model)
	writef(cmd.OutOrStdout(), "  Provider:    %s\n", cfg.Provider)
	writef(cmd.OutOrStdout(), "  Scenarios:   %s\n", strings.Join(scenarioIDs, ", "))
	writef(cmd.OutOrStdout(), "  Repeats:     %d\n", repeats)
	if skipped > 0 {
		writef(cmd.OutOrStdout(), "  Skipped:     %d\n", skipped)
	}
	writef(cmd.OutOrStdout(), "  Phases:\n")
	writef(cmd.OutOrStdout(), "    baseline: direct Bench tools\n")
	writef(cmd.OutOrStdout(), "    candidate: tool_server=%s", cfg.ToolServerID)
	if cfg.ToolServerVersion != "" {
		writef(cmd.OutOrStdout(), " version=%s", cfg.ToolServerVersion)
	}
	writef(cmd.OutOrStdout(), "\n")
	writef(cmd.OutOrStdout(), "  Report links:\n")
	writef(cmd.OutOrStdout(), "    UI:       %s\n", links.UI)
	writef(cmd.OutOrStdout(), "    JSON:     %s\n", links.JSON)
	writef(cmd.OutOrStdout(), "    Markdown: %s\n", links.Markdown)
}

func printReportPackSummary(cmd *cobra.Command, baseline, candidate reportPackPhaseSummary, skipped int, summaryPath string, links reportPackLinks) {
	writef(cmd.OutOrStdout(), "\n")
	writef(cmd.OutOrStdout(), "REPORT PACK RESULTS\n")
	writef(cmd.OutOrStdout(), "  Baseline:  %d passed, %d failed, %d errors\n", baseline.Passed, baseline.Failed, baseline.Errors)
	writef(cmd.OutOrStdout(), "  Candidate: %d passed, %d failed, %d errors\n", candidate.Passed, candidate.Failed, candidate.Errors)
	if skipped > 0 {
		writef(cmd.OutOrStdout(), "  Skipped:   %d\n", skipped)
	}
	writef(cmd.OutOrStdout(), "  Summary:   %s\n", summaryPath)
	writef(cmd.OutOrStdout(), "  Report:    %s\n", links.UI)
	writef(cmd.OutOrStdout(), "  Markdown:  %s\n", links.Markdown)
}

func writeReportPackSummary(
	outDir string,
	cfg config.Config,
	scenarioIDs []string,
	skipped int,
	links reportPackLinks,
	baseline reportPackPhaseSummary,
	candidate reportPackPhaseSummary,
) (string, error) {
	summary := map[string]any{
		"generated_at":        time.Now().UTC().Format(time.RFC3339),
		"model":               cfg.Model,
		"provider":            cfg.Provider,
		"tool_server":         cfg.ToolServerID,
		"tool_server_version": cfg.ToolServerVersion,
		"scenarios":           scenarioIDs,
		"skipped":             skipped,
		"links":               links,
		"baseline":            baseline,
		"candidate":           candidate,
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report-pack summary: %w", err)
	}
	summaryPath := filepath.Join(outDir, "summary.json")
	if err := os.WriteFile(summaryPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write report-pack summary: %w", err)
	}
	return summaryPath, nil
}

func reportPackScenarioIDs(scenarios []*scenario.Scenario) []string {
	ids := make([]string, 0, len(scenarios))
	for _, s := range scenarios {
		ids = append(ids, s.ID)
	}
	return ids
}

func safePathComponent(s string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		" ", "_",
	)
	out := strings.Trim(replacer.Replace(s), "._-")
	if out == "" {
		return "unknown"
	}
	return out
}
