package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/signalaudit"
)

func newAuditCommand(defaultRunsDir string) *cobra.Command {
	runsDir := defaultRunsDir
	manifestPath := filepath.Join("configs", "signal-audit.yaml")
	scenarioFilter := ""
	modelFilter := ""
	providerFilter := ""
	outputPath := ""

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit existing benchmark artifacts",
	}
	signalsCmd := &cobra.Command{
		Use:   "signals",
		Short: "Audit existing run artifacts for signal drift and false positives",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSignalAudit(cmd, runsDir, manifestPath, scenarioFilter, modelFilter, providerFilter, outputPath)
		},
	}
	f := signalsCmd.Flags()
	f.StringVar(&runsDir, "runs-dir", runsDir, "runs directory to scan")
	f.StringVar(&manifestPath, "manifest", manifestPath, "signal audit manifest path")
	f.StringVar(&scenarioFilter, "scenario", "", "filter by scenario ID")
	f.StringVar(&modelFilter, "model", "", "filter by model")
	f.StringVar(&providerFilter, "provider", "", "filter by provider")
	f.StringVar(&outputPath, "output", "", "output path (default: <runs-dir>/signal-audit.json)")
	cmd.AddCommand(signalsCmd)
	return cmd
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

	writef(cmd.OutOrStdout(), "%s", signalaudit.FormatSummary(result))
	writef(cmd.OutOrStdout(), "json: %s\n", outputPath)
	return nil
}
