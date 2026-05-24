package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/artifactaudit"
	"github.com/vitas/evidra-bench/pkg/localstore"
	"github.com/vitas/evidra-bench/pkg/signalaudit"
)

func newAuditCommand(defaultRunsDir string) *cobra.Command {
	runsDir := defaultRunsDir
	manifestPath := filepath.Join("configs", "signal-audit.yaml")
	scenarioFilter := ""
	modelFilter := ""
	providerFilter := ""
	outputPath := ""
	coverageRunsDir := defaultRunsDir
	coverageScenarioFilter := ""
	coverageModelFilter := ""
	coverageProviderFilter := ""
	coverageAdapterFilter := ""
	coverageOutputPath := ""
	failOnGaps := false

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

	coverageCmd := &cobra.Command{
		Use:   "coverage",
		Short: "Audit run artifact coverage from the local results store",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeArtifactCoverageAudit(cmd, coverageRunsDir, coverageScenarioFilter, coverageModelFilter, coverageProviderFilter, coverageAdapterFilter, coverageOutputPath, failOnGaps)
		},
	}
	cf := coverageCmd.Flags()
	cf.StringVar(&coverageRunsDir, "runs-dir", coverageRunsDir, "runs directory containing bench.db and run artifacts")
	cf.StringVar(&coverageScenarioFilter, "scenario", "", "filter by scenario ID")
	cf.StringVar(&coverageModelFilter, "model", "", "filter by model")
	cf.StringVar(&coverageProviderFilter, "provider", "", "filter by provider")
	cf.StringVar(&coverageAdapterFilter, "adapter", "", "filter by adapter")
	cf.StringVar(&coverageOutputPath, "output", "", "output path (default: <runs-dir>/artifact-coverage.json)")
	cf.BoolVar(&failOnGaps, "fail-on-gaps", false, "exit non-zero when coverage gaps are found")

	cmd.AddCommand(signalsCmd, coverageCmd)
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

func executeArtifactCoverageAudit(cmd *cobra.Command, runsDir, scenarioFilter, modelFilter, providerFilter, adapterFilter, outputPath string, failOnGaps bool) error {
	if strings.TrimSpace(runsDir) == "" {
		return fmt.Errorf("audit coverage: --runs-dir is required")
	}
	if outputPath == "" {
		outputPath = filepath.Join(runsDir, "artifact-coverage.json")
	}

	resultsStore, err := localstore.Open(runsDir)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resultsStore.Close(); closeErr != nil {
			writef(cmd.ErrOrStderr(), "[audit] warning: close store: %v\n", closeErr)
		}
	}()

	runs, err := resultsStore.Query(localstore.QueryFilters{
		ScenarioID: scenarioFilter,
		Model:      modelFilter,
		Provider:   providerFilter,
	})
	if err != nil {
		return err
	}
	runs = filterCoverageRunsByAdapter(runs, adapterFilter)

	result := artifactaudit.Analyze(runs)
	if err := artifactaudit.WriteJSON(outputPath, result); err != nil {
		return err
	}

	writef(cmd.OutOrStdout(), "%s", artifactaudit.FormatSummary(result))
	writef(cmd.OutOrStdout(), "json: %s\n", outputPath)
	if failOnGaps && result.IncompleteRuns > 0 {
		return fmt.Errorf("audit coverage: %d runs have artifact coverage gaps", result.IncompleteRuns)
	}
	return nil
}
