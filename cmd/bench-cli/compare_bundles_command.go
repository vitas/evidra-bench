package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/evidrawire"
)

// newCompareBundlesCommand gates joins across exported evidence bundles by
// result-semantics cohort (ADR 0001 Phase 11). Verdicts minted under the
// preview telemetry contract must never be diffed or aggregated with
// authoritative-evidence verdicts; the bundles remain individually
// verifiable and readable, exactly as ADR 0001 promises for v1.
func newCompareBundlesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "compare-bundles <dir> <dir>...",
		Short: "Verify exported evidence bundles and gate cohort comparability",
		Long: `Run full chain verification on each bundle, then decide whether the
set may be compared at all:

  same cohort, authoritative evidence  -> comparable
  same cohort, legacy preview          -> readable, NOT comparable (notice)
  mixed cohorts                        -> rejected

Exit code is nonzero for a rejected mix or a broken chain.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var manifests []evidrawire.BundleManifest
			for _, dir := range args {
				m, err := evidrawire.VerifyBundle(dir)
				if err != nil {
					return fmt.Errorf("bundle %s: verify failed: %w", dir, err)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: chain verified (cohort %s)\n", dir, evidrawire.Cohort(m))
				manifests = append(manifests, m)
			}
			cohort, notice, err := evidrawire.SameCohort(manifests...)
			if err != nil {
				if errors.Is(err, evidrawire.ErrMixedCohorts) {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "refused:", err)
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "bundles stay individually readable; compare within one cohort only (docs/adr/0001-process-safety-matching.md)")
					cmd.SilenceErrors = true
					return err
				}
				return err
			}
			if notice != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), notice)
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return fmt.Errorf("%w (%s)", evidrawire.ErrNotComparable, cohort)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "comparable: all bundles share cohort %s\n", cohort)
			return nil
		},
	}
}
