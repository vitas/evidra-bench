package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/report"
)

type testRequest struct {
	Model       string
	Endpoint    string
	Agent       string
	Suite       string
	Environment string
	OutputDir   string
	ProjectRoot string
	Timeout     time.Duration
	CI          bool
}

type testRunner func(context.Context, testRequest) (evaluation.Result, error)

type cliExitError struct {
	Code int
	Err  error
}

func (e *cliExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("evaluation exited with code %d", e.Code)
}

func (e *cliExitError) Unwrap() error { return e.Err }

func exitCodeForError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *cliExitError
	if errors.As(err, &exitErr) && exitErr.Code > 0 {
		return exitErr.Code
	}
	return 2
}

func newTestCommand(run testRunner) *cobra.Command {
	req := testRequest{
		Suite:       "kubernetes-demo@1",
		Environment: "kind",
		OutputDir:   "./evidra-results",
		ProjectRoot: ".",
		Timeout:     5 * time.Minute,
	}
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test an infrastructure model or agent on live Kubernetes failures",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(req.Model) == "" && strings.TrimSpace(req.Agent) == "" {
				return fmt.Errorf("test: provide --model or --agent")
			}
			if strings.TrimSpace(req.Model) != "" && strings.TrimSpace(req.Agent) != "" {
				return fmt.Errorf("test: --model and --agent are mutually exclusive")
			}
			if run == nil {
				return fmt.Errorf("test: evaluation runner is unavailable")
			}

			result, runErr := run(cmd.Context(), req)
			if runErr != nil && result.Version == "" {
				return &cliExitError{Code: 2, Err: runErr}
			}
			if err := writeEvaluationOutputs(req.OutputDir, result, req.Suite); err != nil {
				return &cliExitError{Code: 2, Err: err}
			}
			if err := report.RenderEvaluationTerminal(cmd.OutOrStdout(), result); err != nil {
				return &cliExitError{Code: 2, Err: err}
			}
			reportPath, _ := filepath.Abs(filepath.Join(req.OutputDir, "report.html"))
			writef(cmd.OutOrStdout(), "\nReport: %s\n", reportPath)

			code := evaluation.ExitCode(result)
			if runErr != nil {
				code = 2
			}
			if code != 0 {
				return &cliExitError{Code: code, Err: runErr}
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&req.Model, "model", "", "model to test (for example openai/gpt-5)")
	flags.StringVar(&req.Endpoint, "endpoint", "", "OpenAI-compatible API base URL")
	flags.StringVar(&req.Agent, "agent", "", "external agent command")
	flags.StringVar(&req.Suite, "suite", req.Suite, "versioned test suite")
	flags.StringVar(&req.Environment, "environment", req.Environment, "local Kubernetes environment (kind or k3d)")
	flags.StringVar(&req.OutputDir, "output", req.OutputDir, "directory for local reports and evidence")
	flags.DurationVar(&req.Timeout, "timeout", req.Timeout, "timeout for each test case")
	flags.BoolVar(&req.CI, "ci", false, "disable interactive behavior and use stable exit codes")
	return cmd
}

func writeEvaluationOutputs(outputDir string, result evaluation.Result, suiteID string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("test: create output directory: %w", err)
	}
	jsonFile, err := os.Create(filepath.Join(outputDir, "result.json"))
	if err != nil {
		return fmt.Errorf("test: create JSON result: %w", err)
	}
	jsonErr := report.RenderEvaluationJSON(jsonFile, result)
	closeErr := jsonFile.Close()
	if jsonErr != nil {
		return jsonErr
	}
	if closeErr != nil {
		return fmt.Errorf("test: close JSON result: %w", closeErr)
	}

	htmlFile, err := os.Create(filepath.Join(outputDir, "report.html"))
	if err != nil {
		return fmt.Errorf("test: create HTML report: %w", err)
	}
	limitations := []string(nil)
	if suiteID == "kubernetes-demo@1" {
		limitations = []string{"This starter suite demonstrates core behavior; it does not certify production readiness."}
	}
	htmlErr := report.RenderEvaluationHTML(htmlFile, result, report.EvaluationReportOptions{
		Title:       "Evidra Infrastructure Agent Tests",
		Limitations: limitations,
	})
	closeErr = htmlFile.Close()
	if htmlErr != nil {
		return htmlErr
	}
	if closeErr != nil {
		return fmt.Errorf("test: close HTML report: %w", closeErr)
	}
	return nil
}
