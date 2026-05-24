package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/localstore"
)

func newDBCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Query and manage the results database",
	}
	dbStatsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show aggregate run statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := localstore.Open(cfg.RunsDir)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := s.Close(); closeErr != nil {
					log.Printf("[db] close stats store: %v", closeErr)
				}
			}()
			st, err := s.Stats()
			if err != nil {
				return err
			}
			writef(cmd.OutOrStdout(), "Total: %d  Pass: %d  Fail: %d\n\n", st.TotalRuns, st.PassCount, st.FailCount)
			for _, ss := range st.ByScenario {
				writef(cmd.OutOrStdout(), "  %-35s %d/%d\n", ss.ScenarioID, ss.Passed, ss.Runs)
			}
			return nil
		},
	}
	dbQueryCmd := &cobra.Command{
		Use:   "query",
		Short: "Query runs with filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := localstore.Open(cfg.RunsDir)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := s.Close(); closeErr != nil {
					log.Printf("[db] close query store: %v", closeErr)
				}
			}()
			scenarioFilter, _ := cmd.Flags().GetString("scenario")
			modelFilter, _ := cmd.Flags().GetString("model")
			providerFilter, _ := cmd.Flags().GetString("provider")
			limit, _ := cmd.Flags().GetInt("limit")
			passedOnly, _ := cmd.Flags().GetBool("passed")
			failedOnly, _ := cmd.Flags().GetBool("failed")
			runs, err := s.Query(localstore.QueryFilters{
				ScenarioID: scenarioFilter,
				Model:      modelFilter,
				Provider:   providerFilter,
				PassedOnly: passedOnly,
				FailedOnly: failedOnly,
				Limit:      limit,
			})
			if err != nil {
				return err
			}
			for _, r := range runs {
				verdict := "PASS"
				if !r.Passed {
					verdict = "FAIL"
				}
				writef(cmd.OutOrStdout(), "[%s] %-30s model=%-10s provider=%-8s dur=%.1fs checks=%d/%d cost=$%.4f %s\n",
					verdict, r.ScenarioID, r.Model, r.Provider, r.Duration,
					r.ChecksPassed, r.ChecksTotal, r.EstimatedCost,
					r.CreatedAt.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
	dbQueryCmd.Flags().String("scenario", "", "filter by scenario ID")
	dbQueryCmd.Flags().String("model", "", "filter by model")
	dbQueryCmd.Flags().String("provider", "", "filter by provider")
	dbQueryCmd.Flags().Int("limit", 20, "max results")
	dbQueryCmd.Flags().Bool("passed", false, "show only passed runs")
	dbQueryCmd.Flags().Bool("failed", false, "show only failed runs")

	dbRebuildCmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild database from JSONL backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := localstore.Open(cfg.RunsDir)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := s.Close(); closeErr != nil {
					log.Printf("[db] close rebuild store: %v", closeErr)
				}
			}()
			count, err := s.Rebuild()
			if err != nil {
				return err
			}
			writef(cmd.OutOrStdout(), "Rebuilt %d records from results.jsonl\n", count)
			return nil
		},
	}

	dbImportCmd := &cobra.Command{
		Use:   "import",
		Short: "Import run.json artifacts into the database",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := localstore.Open(cfg.RunsDir)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := s.Close(); closeErr != nil {
					log.Printf("[db] close import store: %v", closeErr)
				}
			}()
			count, err := s.ImportFromArtifacts(cfg.RunsDir)
			if err != nil {
				return err
			}
			writef(cmd.OutOrStdout(), "Imported %d records from run artifacts\n", count)
			return nil
		},
	}

	cmd.AddCommand(dbStatsCmd, dbQueryCmd, dbRebuildCmd, dbImportCmd)
	cmd.PersistentFlags().StringVar(&cfg.RunsDir, "runs-dir", cfg.RunsDir, "runs directory")
	return cmd
}
