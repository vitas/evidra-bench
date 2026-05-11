package main

import (
	"context"
	"fmt"
	"log"
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
	"samebits.com/evidra-infra-bench/pkg/store"
)

func newRunCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a benchmark scenario against an agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Validate(); err != nil {
				return err
			}
			return executeRun(cmd, *cfg)
		},
	}

	f := cmd.Flags()
	f.StringVar(&cfg.EnvironmentProvider, "environment", cfg.EnvironmentProvider, "environment provider (kind, k3d)")
	f.StringVar(&cfg.Scenario, "scenario", cfg.Scenario, "scenario path relative to scenarios dir")
	f.StringVar(&cfg.Adapter, "adapter", cfg.Adapter, "agent adapter type (cli, mcp, a2a)")
	f.StringVar(&cfg.A2AAgentURL, "a2a-agent-url", cfg.A2AAgentURL, "A2A agent URL (env: INFRA_BENCH_A2A_AGENT_URL)")
	f.StringVar(&cfg.AgentCommand, "agent-command", cfg.AgentCommand, "command to invoke the agent")
	f.StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "base directory for scenarios")
	f.StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "output directory for run artifacts")
	f.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "agent execution timeout")
	f.BoolVar(&cfg.ReuseCluster, "reuse-cluster", cfg.ReuseCluster, "reuse existing kind cluster")
	f.StringVar(&cfg.ClusterName, "cluster-name", cfg.ClusterName, "kind cluster name")
	f.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "validate scenario without executing")
	f.StringVar(&cfg.BenchURL, "bench-url", cfg.BenchURL, "Bench API URL for online reporting")
	f.StringVar(&cfg.BenchAPIKey, "bench-api-key", cfg.BenchAPIKey, "Bench API key")
	f.StringVar(&cfg.EvidenceDir, "evidence-dir", cfg.EvidenceDir, "evidence directory for verifier input")
	f.StringVar(&cfg.Model, "model", cfg.Model, "model for agent (e.g. sonnet, opus, haiku)")
	f.StringVar(&cfg.Provider, "provider", cfg.Provider, "LLM provider for tool-use agent loop (bifrost, claude)")
	f.IntVar(&cfg.MemoryWindow, "memory-window", -1, "agent memory window (-1=full, 0=stateless, N=last N exchanges)")
	f.StringVar(&cfg.SystemPromptFile, "system-prompt-file", cfg.SystemPromptFile, "system prompt file path (overrides default; env: INFRA_BENCH_SYSTEM_PROMPT)")
	f.StringVar(&cfg.Role, "role", cfg.Role, "role-based skill (k8s-admin, security-ops, release-manager, platform-eng)")
	f.StringVar(&cfg.MCPServer, "mcp-server", cfg.MCPServer, "MCP server command for tool execution")
	f.StringVar(&cfg.ToolServerID, "tool-server-id", cfg.ToolServerID, "stable MCP server identity for result comparison")
	f.StringVar(&cfg.ToolServerVersion, "tool-server-version", cfg.ToolServerVersion, "stable MCP server version for result comparison")
	f.StringVar(&cfg.ReportID, "report-id", cfg.ReportID, "stable report campaign identifier for filtering")
	f.StringVar(&cfg.ContractVersion, "contract-version", cfg.ContractVersion, "contract version label for tracking")
	f.IntVar(&cfg.Parallel, "parallel", 1, "number of parallel workers (1 = sequential)")
	f.StringVar(&cfg.DatabaseURL, "database-url", "", "PostgreSQL URL for job queue (env: BENCH_DATABASE_URL)")
	return cmd
}

func executeRun(cmd *cobra.Command, cfg config.Config) error {
	cfg, s, err := resolveScenarioConfig(cfg)
	if err != nil {
		return err
	}

	if s.Skip {
		reason := s.SkipReason
		if reason == "" {
			reason = "skip: true in scenario.yaml"
		}
		return fmt.Errorf("scenario %s is skipped: %s", s.ID, reason)
	}
	if !s.IsProviderCompatible(cfg.EnvironmentProvider) {
		return fmt.Errorf("scenario %s requires %v provider, running on %s",
			s.ID, s.Environment.Providers, cfg.EnvironmentProvider)
	}

	result, err := runScenarioOnce(cmd.Context(), cfg, s)
	if err != nil {
		return err
	}

	verdict := "PASS"
	if !result.Passed {
		verdict = "FAIL"
	}
	writef(cmd.OutOrStdout(), "[%s] scenario=%s duration=%s exit_code=%d\n",
		verdict, result.ScenarioID, result.Duration.Round(time.Millisecond), result.ExitCode)
	if result.ArtifactDir != "" {
		writef(cmd.OutOrStdout(), "artifacts: %s\n", result.ArtifactDir)
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

func resolveLocalAdapter(name string) (adapter.Adapter, error) {
	switch name {
	case "cli":
		return adapter.NewCLIAdapter(), nil
	case "mcp":
		return adapter.NewMCPAdapter(), nil
	default:
		return nil, fmt.Errorf("unknown adapter: %s", name)
	}
}

func runScenarioOnce(ctx context.Context, cfg config.Config, s *scenario.Scenario) (*harness.RunResult, error) {
	return runScenarioOnceWithLease(ctx, cfg, s, nil)
}

// runScenarioOnceWithLease runs a single scenario. When lease is non-nil the
// caller owns the lease lifetime; when nil a dedicated lease is acquired and
// released within this call.
func runScenarioOnceWithLease(ctx context.Context, cfg config.Config, s *scenario.Scenario, lease *environment.Lease) (*harness.RunResult, error) {
	if err := s.ProviderCompatibilityError(cfg.EnvironmentProvider); err != nil {
		return nil, err
	}

	var agentAdapter adapter.Adapter
	var err error
	if cfg.Adapter != "a2a" {
		agentAdapter, err = resolveLocalAdapter(cfg.Adapter)
		if err != nil {
			return nil, err
		}
	}

	// Acquire a dedicated lease when the caller did not provide one.
	if lease == nil && !cfg.DryRun {
		provisioner := newLocalProvisioner(cfg)
		acquired, acquireErr := provisioner.Acquire(ctx, environment.ProvisionRequest{
			Scenario:           s,
			Profile:            s.ResolvedProfile(),
			ProviderName:       cfg.EnvironmentProvider,
			ClusterName:        cfg.ClusterName,
			ReuseCluster:       cfg.ReuseCluster,
			ExistingKubeconfig: cfg.KubeconfigPath,
		})
		if acquireErr != nil {
			return nil, fmt.Errorf("runScenarioOnce: acquire lease: %w", acquireErr)
		}
		defer func() {
			if releaseErr := acquired.Release(ctx); releaseErr != nil {
				log.Printf("[run] warning: release lease: %v", releaseErr)
			}
		}()
		lease = acquired
	}

	var envProvider environment.ClusterLifecycle
	var kubeconfigPath string
	var extraEnv []string
	if lease != nil {
		envProvider = lease.Provider
		kubeconfigPath = lease.KubeconfigPath
		extraEnv = lease.ExtraEnv
	}

	runner := &environment.ExecRunner{}
	bootstrapper := environment.NewBootstrapper(runner)
	writer := artifact.NewWriter(cfg.RunsDir)

	reporter := report.NewReporter(report.Config{
		EvidencePath: filepath.Join(cfg.RunsDir, "evidence"),
	})

	resultsStore, err := store.Open(cfg.RunsDir)
	if err != nil {
		log.Printf("[harness] warning: could not open results store: %v", err)
	}
	if resultsStore != nil {
		defer func() {
			if closeErr := resultsStore.Close(); closeErr != nil {
				log.Printf("[harness] warning: could not close results store: %v", closeErr)
			}
		}()
	}

	deps := harness.Deps{
		EnvProvider:  envProvider,
		Bootstrapper: bootstrapper,
		Adapter:      agentAdapter,
		Writer:       writer,
		Reporter:     reporter,
		Store:        resultsStore,
	}
	h := harness.New(deps)

	result, err := h.Run(ctx, harness.RunRequest{
		Config:         cfg,
		Scenario:       s,
		KubeconfigPath: kubeconfigPath,
		ExtraEnv:       extraEnv,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// newLocalProvisioner builds a LocalProvisioner from the current config.
// The asset root is derived from the absolute ScenariosDir parent (repo root).
func newLocalProvisioner(cfg config.Config) *environment.LocalProvisioner {
	providers := map[string]environment.ClusterLifecycle{
		"kind": newKindProvider(cfg),
		"k3d":  newK3dProvider(cfg),
	}
	assetsRoot := filepath.Dir(cfg.ScenariosDir)
	return environment.NewLocalProvisioner(providers, &environment.ExecRunner{}, assetsRoot)
}

func newKindProvider(cfg config.Config) *environment.KindProvider {
	p := environment.NewKindProvider()
	p.ReuseExisting = cfg.ReuseCluster
	return p
}

func newK3dProvider(cfg config.Config) *environment.K3dProvider {
	p := environment.NewK3dProvider()
	p.ReuseExisting = cfg.ReuseCluster
	return p
}

// ParallelRunOpts configures a parallel worker run.
type ParallelRunOpts struct {
	TargetNamespace string       // Worker namespace (e.g. "bench-w0")
	KubeconfigPath  string       // Pre-provisioned cluster kubeconfig
	SharedStore     *store.Store // Shared results store (survives workspace cleanup)
}

// runScenarioOnceWithNamespace runs a scenario with a specific target namespace.
// Used by parallel workers where each worker has its own namespace and pre-provisioned cluster.
func runScenarioOnceWithNamespace(ctx context.Context, cfg config.Config, s *scenario.Scenario,
	targetNS, kubeconfigPath string, sharedStore *store.Store,
	provider environment.ClusterLifecycle) (*harness.RunResult, error) {
	if err := s.ProviderCompatibilityError(cfg.EnvironmentProvider); err != nil {
		return nil, err
	}

	var agentAdapter adapter.Adapter
	var err error
	if cfg.Adapter != "a2a" {
		agentAdapter, err = resolveLocalAdapter(cfg.Adapter)
		if err != nil {
			return nil, err
		}
	}

	// No environment provider needed — cluster is pre-provisioned.
	// The harness will skip create/destroy when KubeconfigPath is set.
	runner := &environment.ExecRunner{}
	bootstrapper := environment.NewBootstrapper(runner)
	writer := artifact.NewWriter(cfg.RunsDir)

	reporter := report.NewReporter(report.Config{
		EvidencePath: filepath.Join(cfg.RunsDir, "evidence"),
	})

	// Use shared store if provided (parallel mode), otherwise open workspace-local.
	var resultsStore *store.Store
	if sharedStore != nil {
		resultsStore = sharedStore
	} else {
		var storeErr error
		resultsStore, storeErr = store.Open(cfg.RunsDir)
		if storeErr != nil {
			log.Printf("[harness] warning: could not open results store: %v", storeErr)
		}
		if resultsStore != nil {
			defer func() {
				if closeErr := resultsStore.Close(); closeErr != nil {
					log.Printf("[harness] warning: could not close workspace-local results store: %v", closeErr)
				}
			}()
		}
	}

	h := harness.New(harness.Deps{
		EnvProvider:  provider,
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
		KubeconfigPath:  kubeconfigPath,
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
