package main

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/runreview"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/scenariopatch"
)

func newScenarioCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scenario",
		Short: "Manage benchmark scenarios",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listScenarios(cmd, *cfg)
		},
	}

	var pushURL, pushKey string
	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push scenario metadata to bench API",
		RunE: func(cmd *cobra.Command, args []string) error {
			return pushScenarios(cfg.ScenariosDir, pushURL, pushKey)
		},
	}
	pushCmd.Flags().StringVar(&pushURL, "bench-url", "", "Bench API URL")
	pushCmd.Flags().StringVar(&pushKey, "bench-api-key", "", "Bench API key")

	var patchScenario, patchReviewFile string
	patchCmd := &cobra.Command{
		Use:   "patch-preview",
		Short: "Preview scenario YAML changes from a run review",
		RunE: func(cmd *cobra.Command, args []string) error {
			return previewScenarioPatch(cmd, *cfg, patchScenario, patchReviewFile)
		},
	}
	patchCmd.Flags().StringVar(&patchScenario, "scenario", "", "scenario path or id")
	patchCmd.Flags().StringVar(&patchReviewFile, "review-file", "", "path to run_review.json")

	cmd.AddCommand(listCmd, pushCmd, patchCmd)
	cmd.PersistentFlags().StringVar(&cfg.ScenariosDir, "scenarios-dir", cfg.ScenariosDir, "base directory for scenarios")
	return cmd
}

func listScenarios(cmd *cobra.Command, cfg config.Config) error {
	scenarios, err := scenario.LoadAll(cfg.ScenariosDir)
	if err != nil {
		return fmt.Errorf("list scenarios: %w", err)
	}
	if len(scenarios) == 0 {
		writef(cmd.OutOrStdout(), "no scenarios found\n")
		return nil
	}
	for _, s := range scenarios {
		writef(cmd.OutOrStdout(), "%-30s %s (%s)\n", s.Path, s.Title, s.ID)
	}
	return nil
}

func previewScenarioPatch(cmd *cobra.Command, cfg config.Config, scenarioRef, reviewFile string) error {
	if scenarioRef == "" {
		return fmt.Errorf("patch-preview: --scenario is required")
	}
	if reviewFile == "" {
		return fmt.Errorf("patch-preview: --review-file is required")
	}

	s, err := scenario.Resolve(cfg.ScenariosDir, scenarioRef)
	if err != nil {
		return fmt.Errorf("patch-preview: resolve scenario: %w", err)
	}
	scenarioPath := filepath.Join(s.Dir, "scenario.yaml")
	scenarioYAML, err := os.ReadFile(scenarioPath)
	if err != nil {
		return fmt.Errorf("patch-preview: read scenario YAML: %w", err)
	}

	reviewData, err := os.ReadFile(reviewFile)
	if err != nil {
		return fmt.Errorf("patch-preview: read review: %w", err)
	}
	review, err := runreview.Decode(reviewData)
	if err != nil {
		return fmt.Errorf("patch-preview: parse review: %w", err)
	}
	if review.ScenarioID != "" && review.ScenarioID != s.ID {
		return fmt.Errorf("patch-preview: review scenario_id %q does not match scenario %q", review.ScenarioID, s.ID)
	}

	displayPath := filepath.ToSlash(filepath.Join(s.Path, "scenario.yaml"))
	result, err := scenariopatch.Preview(scenarioYAML, review, displayPath)
	if err != nil {
		return fmt.Errorf("patch-preview: %w", err)
	}
	if !result.Changed {
		writef(cmd.OutOrStdout(), "No scenario patch suggestions to apply.\n")
		return nil
	}
	writef(cmd.OutOrStdout(), "%s", result.Diff)
	return nil
}

func pushScenarios(scenariosDir, benchURL, apiKey string) error {
	if benchURL == "" || apiKey == "" {
		return fmt.Errorf("push-scenarios: --bench-url and --bench-api-key are required")
	}

	scenarios, err := scenario.LoadAll(scenariosDir)
	if err != nil {
		return fmt.Errorf("push-scenarios: load: %w", err)
	}

	type scenarioPayload struct {
		ID                 string   `json:"id"`
		Title              string   `json:"title"`
		Description        string   `json:"description"`
		AutopsyDescription string   `json:"autopsy_description,omitempty"`
		Category           string   `json:"category"`
		Track              string   `json:"track,omitempty"`
		Level              string   `json:"level,omitempty"`
		Timeout            string   `json:"timeout,omitempty"`
		Tags               []string `json:"tags"`
		Chaos              bool     `json:"chaos"`
		Skip               bool     `json:"skip,omitempty"`
	}

	var items []scenarioPayload
	for _, s := range scenarios {
		tags := s.Tags
		if tags == nil {
			tags = []string{}
		}
		items = append(items, scenarioPayload{
			ID:                 s.ID,
			Title:              s.Title,
			Description:        s.Description,
			AutopsyDescription: s.Autopsy.Description,
			Category:           s.Category,
			Track:              s.Track,
			Level:              s.Level,
			Timeout:            s.Timeout.String(),
			Tags:               tags,
			Chaos:              len(s.Chaos.Steps) > 0,
			Skip:               s.Skip,
		})
	}

	body, err := json.Marshal(map[string]any{"scenarios": items})
	if err != nil {
		return fmt.Errorf("push-scenarios: marshal: %w", err)
	}

	url := benchURL + "/v1/bench/scenarios/sync"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("push-scenarios: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("push-scenarios: POST %s: %w", url, err)
	}

	var result map[string]any
	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	closeErr := resp.Body.Close()
	if decodeErr != nil && !stderrors.Is(decodeErr, io.EOF) {
		return fmt.Errorf("push-scenarios: decode response: %w", decodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("push-scenarios: close response body: %w", closeErr)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("push-scenarios: HTTP %d: %v", resp.StatusCode, result)
	}

	fmt.Printf("Pushed %v scenarios to %s (upserted: %v)\n", result["total"], benchURL, result["upserted"])
	return nil
}
