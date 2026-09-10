package main

import (
	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/config"
)

func newRootCommand() *cobra.Command {
	cfg := config.Default()

	root := &cobra.Command{
		Use:   "evidra",
		Short: "Run infrastructure-agent benchmark scenarios",
		Long: `bench-cli provisions disposable Kubernetes environments, injects
failures, executes real agents, and verifies outcomes.

It produces local artifact bundles for debugging and dataset generation,
with optional Bench reporting for behavioral analysis.`,
		Version: buildVersionString(),
	}

	root.AddCommand(
		newTestCommand(runOneCommandEvaluation),
		newDemoCommand(runOneCommandEvaluation, defaultDemoDiscover, inputIsTerminal, defaultDemoCapabilities),
		newRunCommand(&cfg),
		newScenarioCommand(&cfg),
		newLabCommand(&cfg),
		newDBCommand(&cfg),
		newSkillDeltaCommand(),
		newAuditCommand(cfg.RunsDir),
		newExportBundleCommand(cfg.RunsDir, version),
		newBenchCommand(),
		newReportPackCommand(),
		newCertifyCommand(),
		newQualifyCommand(),
		newServeCommand(&cfg),
	)
	return root
}
