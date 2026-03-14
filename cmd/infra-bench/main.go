package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "infra-bench",
		Short: "Run infrastructure-agent benchmark scenarios",
		Long: `infra-bench provisions disposable Kubernetes environments, injects
failures, executes real agents, and verifies outcomes.

It produces local artifact bundles for debugging and dataset generation,
with optional Evidra reporting for behavioral analysis.`,
		Version: version,
	}
	return root
}

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
