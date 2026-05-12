package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/skilldelta"
)

func newSkillDeltaCommand() *cobra.Command {
	cfg := config.Default()
	scenarios := []string{}
	models := []string{}
	repeats := 1
	outDir := ""
	dir := ""
	noSkillPrompt := ""
	withSkillPrompt := ""

	cmd := &cobra.Command{
		Use:   "skill-delta",
		Short: "Run and analyze paired with-skill vs without-skill benchmarks",
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run paired without-skill and with-skill benchmark cases",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSkillDeltaRun(cmd, cfg, scenarios, models, repeats, noSkillPrompt, withSkillPrompt, outDir)
		},
	}
	f := runCmd.Flags()
	f.StringSliceVar(&scenarios, "scenario", nil, "scenario path or id (repeatable)")
	f.StringSliceVar(&models, "model", nil, "model name (repeatable)")
	f.IntVar(&repeats, "repeats", 1, "number of repeats per scenario/model pair")
	f.StringVar(&noSkillPrompt, "no-skill-prompt", "", "system prompt file for baseline runs")
	f.StringVar(&withSkillPrompt, "with-skill-prompt", "", "system prompt file for skill-enabled runs")
	f.StringVar(&outDir, "out-dir", "", "benchmark output directory (default: runs/skill-delta/<stamp>)")
	f.StringVar(&cfg.EnvironmentProvider, "environment", cfg.EnvironmentProvider, "environment provider (kind, k3d)")
	f.StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "base directory for scenarios")
	f.StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "base directory for benchmark runs")
	f.StringVar(&cfg.Adapter, "adapter", cfg.Adapter, "agent adapter type (cli, mcp, a2a)")
	f.StringVar(&cfg.A2AAgentURL, "a2a-agent-url", cfg.A2AAgentURL, "A2A agent URL (env: INFRA_BENCH_A2A_AGENT_URL)")
	f.StringVar(&cfg.AgentCommand, "agent-command", cfg.AgentCommand, "command to invoke the agent")
	f.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "agent execution timeout")
	f.BoolVar(&cfg.ReuseCluster, "reuse-cluster", cfg.ReuseCluster, "reuse existing kind cluster")
	f.StringVar(&cfg.ClusterName, "cluster-name", cfg.ClusterName, "kind cluster name")
	f.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "validate workflow without executing the agent")
	f.StringVar(&cfg.BenchURL, "bench-url", cfg.BenchURL, "Bench API URL for online reporting")
	registerBenchAPIKeyFlag(f, &cfg.BenchAPIKey)
	f.StringVar(&cfg.EvidenceDir, "evidence-dir", cfg.EvidenceDir, "base evidence directory for verifier input")
	f.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider for tool-use agent loop (bifrost, claude)")
	f.IntVar(&cfg.MemoryWindow, "memory-window", -1, "agent memory window (-1=full, 0=stateless, N=last N exchanges)")
	f.StringVar(&cfg.ContractVersion, "contract-version", cfg.ContractVersion, "contract version label for tracking")

	aggregateCmd := &cobra.Command{
		Use:   "aggregate",
		Short: "Aggregate pair.json files into benchmark.json and benchmark.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSkillDeltaAggregate(cmd, dir)
		},
	}
	aggregateCmd.Flags().StringVar(&dir, "dir", "", "skill-delta benchmark directory")

	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Read benchmark.json and point users to the web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSkillDeltaReport(cmd, dir)
		},
	}
	reportCmd.Flags().StringVar(&dir, "dir", "", "skill-delta benchmark directory")

	cmd.AddCommand(runCmd, aggregateCmd, reportCmd)
	return cmd
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
					withoutCfg.EvidenceDir = filepath.Join(paths.WithoutSkillRunsDir, "evidence")

					withoutResult, err := runScenarioOnce(cmd.Context(), withoutCfg, s)
					if err != nil {
						return fmt.Errorf("without_skill %s/%s repeat=%d: %w", s.ID, model, repeat, err)
					}

					withCfg := cfg
					withCfg.Scenario = s.Path
					withCfg.Model = model
					withCfg.SystemPromptFile = withSkillPrompt
					withCfg.RunsDir = paths.WithSkillRunsDir
					withCfg.EvidenceDir = filepath.Join(paths.WithSkillRunsDir, "evidence")

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
				writef(cmd.OutOrStdout(), "pair: %s\n", paths.PairJSONPath)
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
	writef(cmd.OutOrStdout(), "benchmark: %s\n", jsonPath)
	writef(cmd.OutOrStdout(), "markdown: %s\n", mdPath)
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
	writef(cmd.OutOrStdout(), "HTML report generation has been removed; use the web UI instead.\n")
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

// filterRunnableScenarios returns scenarios that are not skipped and compatible
// with the given provider. It writes SKIP lines to w and returns the count of
// skipped scenarios.
