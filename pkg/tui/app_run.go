package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/harness"
)

func (a *App) runScenario() tea.Cmd {
	s := a.filtered[a.cursor].Scenario
	a.view = viewRunning
	a.runOutput = fmt.Sprintf("Running scenario: %s ...\n", s.ID)
	a.runResult = nil
	a.runErr = nil

	return func() tea.Msg {
		cfg := config.Config{
			Scenario:     s.ID,
			ScenariosDir: a.scenariosDir,
			Adapter:      a.cfg.Adapter,
			Provider:     a.cfg.Provider,
			AgentCommand: a.cfg.AgentCommand,
			Model:        a.cfg.Model,
			Timeout:      a.cfg.TimeoutDuration(),
			DryRun:       a.cfg.DryRun,
			RunsDir:      a.runsDir,
			ClusterName:  "bench-cli",
			EvidenceDir:  a.cfg.EvidenceDir,
			BenchURL:     a.cfg.BenchURL,
			BenchAPIKey:  a.cfg.BenchAPIKey,
			MemoryWindow: a.cfg.MemoryWindow,
			ReuseCluster: a.cfg.ReuseCluster,
		}

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

		ctx := context.Background()

		// Acquire a lease from the provisioner.
		cfg.EnvironmentProvider = a.cfg.EnvironmentProvider
		if cfg.EnvironmentProvider == "" {
			cfg.EnvironmentProvider = "kind"
		}
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

		deps := a.harnessDeps
		deps.EnvProvider = lease.Provider
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
	return cfg.Provider == "" && cfg.AgentCommand == ""
}
