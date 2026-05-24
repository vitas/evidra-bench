package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/localstore"
	"github.com/vitas/evidra-bench/pkg/report"
)

func (a *App) runScenario() tea.Cmd {
	s := a.filtered[a.cursor].Scenario
	a.view = viewRunning
	a.runOutput = fmt.Sprintf("Running scenario: %s ...\n", s.ID)
	a.runResult = nil
	a.runErr = nil

	return func() tea.Msg {
		cfg := buildRunConfig(s, a.scenariosDir, a.cfg)

		if a.cfg.DryRun {
			log.SetOutput(os.Stderr)
			h := harness.New(a.harnessDeps)
			result, err := h.Run(context.Background(), harness.RunRequest{
				Config:   cfg,
				Scenario: s,
			})
			return RunFinishedMsg{Result: result, Err: err}
		}

		if agentCommandRequired(a.cfg) {
			return RunFinishedMsg{Err: fmt.Errorf("agent command not set — press 'e' to configure")}
		}
		if err := cfg.Validate(); err != nil {
			return RunFinishedMsg{Err: err}
		}

		ctx := context.Background()

		// Acquire a lease from the provisioner.
		providerName := cfg.EnvironmentProvider
		providers := map[string]environment.ClusterLifecycle{
			"kind": environment.NewKindProvider(),
			"k3d":  environment.NewK3dProvider(),
		}
		assetsRoot := filepath.Dir(a.scenariosDir)
		provisioner := environment.NewLocalProvisioner(providers, &environment.ExecRunner{}, assetsRoot)
		lease, err := provisioner.Acquire(ctx, environment.ProvisionRequest{
			Scenario:     s,
			Profile:      s.ResolvedProfile(),
			ProviderName: providerName,
			ClusterName:  cfg.ClusterName,
			ReuseCluster: cfg.ReuseCluster,
		})
		if err != nil {
			return RunFinishedMsg{Err: fmt.Errorf("acquire lease: %w", err)}
		}
		defer func() { _ = lease.Release(ctx) }()

		deps, depErr := buildRunDeps(a.harnessDeps, cfg.Adapter, lease.Provider)
		if depErr != nil {
			return RunFinishedMsg{Err: depErr}
		}
		deps.Writer = artifact.NewWriter(cfg.RunsDir)
		deps.Reporter = report.NewReporter(report.Config{
			EvidencePath: filepath.Join(cfg.RunsDir, "evidence"),
		})
		if deps.Store == nil {
			resultsStore, storeErr := localstore.Open(cfg.RunsDir)
			if storeErr != nil {
				log.Printf("[tui] warning: could not open results store: %v", storeErr)
			} else {
				deps.Store = resultsStore
				defer func() {
					if closeErr := resultsStore.Close(); closeErr != nil {
						log.Printf("[tui] warning: could not close results store: %v", closeErr)
					}
				}()
			}
		}
		h := harness.New(deps)
		result, err := h.Run(ctx, harness.RunRequest{
			Config:         cfg,
			Scenario:       s,
			KubeconfigPath: lease.KubeconfigPath,
			ExtraEnv:       lease.ExtraEnv,
		})
		return RunFinishedMsg{Result: result, Err: err}
	}
}

func agentCommandRequired(cfg LabConfig) bool {
	return cfg.Adapter != "a2a" && cfg.Provider == "" && cfg.AgentCommand == ""
}
