package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/localstore"
	"github.com/vitas/evidra-bench/pkg/scenario"
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
	registerExecutionFlags(f, cfg, executionFlagOptions{
		IncludeScenario: true,
		ScenarioUsage:   "scenario path relative to scenarios dir",
		DryRunUsage:     "validate scenario without executing",
	})
	registerAgentFlags(f, cfg, agentFlagOptions{IncludeModel: true})
	f.StringVar(&cfg.EvidenceDir, "evidence-dir", cfg.EvidenceDir, "evidence directory for verifier input")
	registerResultMetadataFlags(f, cfg, resultMetadataFlagOptions{
		IncludeToolServer: true,
		IncludeReportID:   true,
	})
	registerParallelFlags(f, cfg, parallelFlagOptions{IncludeDatabaseURL: true})
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

	rt, err := buildLocalHarnessRuntime(cfg, envProvider, nil)
	if err != nil {
		return nil, err
	}
	defer rt.Close()
	h := harness.New(rt.Deps)

	result, err := h.Run(ctx, harness.RunRequest{
		Config:         cfg,
		Scenario:       s,
		KubeconfigPath: kubeconfigPath,
		ExtraEnv:       extraEnv,
	})
	if err != nil {
		return result, err
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
	TargetNamespace string            // Worker namespace (e.g. "bench-w0")
	KubeconfigPath  string            // Pre-provisioned cluster kubeconfig
	SharedStore     *localstore.Store // Shared results store (survives workspace cleanup)
}

// runScenarioOnceWithNamespace runs a scenario with a specific target namespace.
// Used by parallel workers where each worker has its own namespace and pre-provisioned cluster.
func runScenarioOnceWithNamespace(ctx context.Context, cfg config.Config, s *scenario.Scenario,
	targetNS, kubeconfigPath string, sharedStore *localstore.Store,
	provider environment.ClusterLifecycle) (*harness.RunResult, error) {
	if err := s.ProviderCompatibilityError(cfg.EnvironmentProvider); err != nil {
		return nil, err
	}

	// No environment provider needed — cluster is pre-provisioned.
	// The harness will skip create/destroy when KubeconfigPath is set.
	rt, err := buildLocalHarnessRuntime(cfg, provider, sharedStore)
	if err != nil {
		return nil, err
	}
	defer rt.Close()
	h := harness.New(rt.Deps)

	result, runErr := h.Run(ctx, harness.RunRequest{
		Config:          cfg,
		Scenario:        s,
		TargetNamespace: targetNS,
		KubeconfigPath:  kubeconfigPath,
	})
	if runErr != nil {
		return result, runErr
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
