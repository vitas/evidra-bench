package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/benchexport"
)

func newExportBundleCommand(defaultRunsDir, producerVersion string) *cobra.Command {
	runsDir := defaultRunsDir
	runID := ""
	runDir := ""
	outDir := ""

	cmd := &cobra.Command{
		Use:   "export-bundle",
		Short: "Export a benchmark run as an Evidra external evidence bundle",
		Long: `Convert one benchmark run's artifact directory into an
evidra-external-bundle/v1 evidence store (append-only signed chain).

The bundle opens with the Evidra flight recorder CLI:

  evidra validate  --evidence-dir <out>
  evidra scorecard --evidence-dir <out>

Select the run either directly with --run-dir or by --run <run-id>
(resolved against --runs-dir).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := runDir
			if dir == "" && runID != "" {
				found, err := findRunDirByRunID(runsDir, runID)
				if err != nil {
					return err
				}
				dir = found
			}
			if dir == "" {
				return fmt.Errorf("export-bundle: --run-dir or --run is required")
			}
			if outDir == "" {
				return fmt.Errorf("export-bundle: --out is required")
			}
			res, err := benchexport.Export(benchexport.Request{
				RunDir:          dir,
				OutDir:          outDir,
				ProducerVersion: producerVersion,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"bundle written to %s (%d entries, %d tool calls; verify with: evidra validate --evidence-dir %s)\n",
				res.BundlePath, res.Entries, res.ToolCalls, res.BundlePath)
			return nil
		},
	}

	cmd.Flags().StringVar(&runsDir, "runs-dir", runsDir, "local runs directory (searched by --run)")
	cmd.Flags().StringVar(&runID, "run", "", "run id to export (resolved under --runs-dir)")
	cmd.Flags().StringVar(&runDir, "run-dir", "", "run artifact directory containing run.json")
	cmd.Flags().StringVar(&outDir, "out", "", "destination bundle directory (must not exist)")
	return cmd
}

// findRunDirByRunID walks the runs directory for a run.json with a matching
// run_id, following the same discovery pattern used by the audit commands.
func findRunDirByRunID(runsDir, runID string) (string, error) {
	var found string
	err := filepath.WalkDir(runsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best effort over partially broken archives
		}
		if found != "" || d.IsDir() || d.Name() != artifact.RunJSON {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var probe struct {
			RunID string `json:"run_id"`
		}
		if json.Unmarshal(raw, &probe) == nil && probe.RunID == runID {
			found = filepath.Dir(path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("export-bundle: scan %s: %w", runsDir, err)
	}
	if found == "" {
		return "", fmt.Errorf("export-bundle: no run.json with run_id %q under %s", runID, runsDir)
	}
	return found, nil
}
