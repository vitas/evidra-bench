package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/modelconfig"
)

// demoDiscover lists Ollama models compatible with the suite's declared
// capability requirements. Discovery uses the native Ollama API only; all
// inference runs later through the shared evaluation pipeline.
type demoDiscover func(ctx context.Context, required []string) ([]modelconfig.LocalModel, error)

type demoCapabilities func(context.Context, testRequest) ([]string, error)

var defaultDemoDiscover demoDiscover = func(ctx context.Context, required []string) ([]modelconfig.LocalModel, error) {
	client := modelconfig.OllamaClient{BaseURL: modelconfig.OllamaAPIEndpoint}
	return client.CompatibleModels(ctx, required)
}

var defaultDemoCapabilities demoCapabilities = func(_ context.Context, req testRequest) ([]string, error) {
	loaded, _, err := loadTestSuite(req, os.Getenv)
	if err != nil {
		return nil, err
	}
	return loaded.EvaluationSuite().RequiredModelCapabilities, nil
}

// newDemoCommand builds the thin local-model entry point. It resolves a
// model, then delegates to exactly the same testRunner and completion logic
// as `evidra test`; it contains no evaluation behavior of its own.
func newDemoCommand(run testRunner, discover demoDiscover, interactive func(io.Reader) bool, capabilities demoCapabilities) *cobra.Command {
	if interactive == nil {
		interactive = inputIsTerminal
	}
	if capabilities == nil {
		capabilities = defaultDemoCapabilities
	}
	req := testRequest{
		Suite:       "kubernetes-demo@1",
		Environment: "kind",
		OutputDir:   "./evidra-results",
		Timeout:     5 * time.Minute,
	}
	var model string
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Run the Kubernetes demo suite against a local Ollama model",
		Long: `demo is the quickest way to see Evidra Bench work on a local model.

It prefers a reachable Ollama runtime, selects the only compatible installed
model automatically, asks once when several are available, and requires an
explicit --model in --ci mode. Models are never downloaded automatically and
the paid cloud path is never chosen silently. The evaluation itself is the
same pipeline, report, and exit-code contract as "evidra test".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if run == nil {
				return fmt.Errorf("demo: evaluation runner is unavailable")
			}
			model = strings.TrimSpace(model)
			if model == "" {
				required, err := capabilities(cmd.Context(), req)
				if err != nil {
					return &cliExitError{Code: 2, Err: fmt.Errorf("demo: resolve suite capabilities: %w", err)}
				}
				resolved, err := chooseDemoModel(cmd, req.CI, interactive(cmd.InOrStdin()), required, discover)
				if err != nil {
					return &cliExitError{Code: 2, Err: err}
				}
				model = resolved
			}
			req.Model = normalizeDemoModel(model)
			return finishEvaluation(cmd, req, run)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&model, "model", "", "local model to demo; bare Ollama tags are accepted, cloud models require provider/model")
	flags.StringVar(&req.Environment, "environment", req.Environment, "local Kubernetes environment (kind or k3d)")
	flags.StringVar(&req.OutputDir, "output", req.OutputDir, "directory for local reports and evidence")
	flags.DurationVar(&req.Timeout, "timeout", req.Timeout, "timeout for each test case")
	flags.BoolVar(&req.CI, "ci", req.CI, "disable interactive selection and use stable exit codes")
	return cmd
}

// normalizeDemoModel maps bare Ollama tags (qwen3:8b) onto the ollama/
// provider prefix; explicitly namespaced models pass through untouched.
func normalizeDemoModel(model string) string {
	if strings.Contains(model, "/") {
		return model
	}
	return "ollama/" + model
}

func chooseDemoModel(cmd *cobra.Command, ci, interactive bool, required []string, discover demoDiscover) (string, error) {
	if ci {
		return "", fmt.Errorf("demo: --model is required in --ci mode; nothing is guessed or downloaded automatically")
	}
	if !interactive {
		return "", fmt.Errorf("demo: --model is required when input is non-interactive")
	}
	if discover == nil {
		discover = defaultDemoDiscover
	}
	models, err := discover(cmd.Context(), required)
	if err != nil {
		return "", fmt.Errorf("demo: %w; start Ollama with `ollama serve` or pass --model explicitly", err)
	}
	switch len(models) {
	case 0:
		return "", fmt.Errorf("demo: no installed Ollama model supports the demo suite; install one with `ollama pull qwen3:8b` (or similar tool-calling model) or pass --model explicitly")
	case 1:
		writef(cmd.OutOrStdout(), "Using the only compatible local model: %s\n", models[0].Name)
		return models[0].Name, nil
	default:
		writef(cmd.OutOrStdout(), "Compatible local models:\n")
		for i, m := range models {
			if detail := strings.TrimSpace(m.ParameterSize + " " + m.Quantization); detail != "" {
				writef(cmd.OutOrStdout(), "  %d) %s (%s)\n", i+1, m.Name, detail)
			} else {
				writef(cmd.OutOrStdout(), "  %d) %s\n", i+1, m.Name)
			}
		}
		writef(cmd.OutOrStdout(), "Select a local model [1]: ")
		line, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if readErr != nil && strings.TrimSpace(line) == "" {
			return "", fmt.Errorf("demo: no interactive input available; pass --model explicitly")
		}
		choice := strings.TrimSpace(line)
		if choice == "" {
			choice = "1"
		}
		index, convErr := strconv.Atoi(choice)
		if convErr != nil || index < 1 || index > len(models) {
			return "", fmt.Errorf("demo: invalid model selection %q; pass --model explicitly", choice)
		}
		return models[index-1].Name, nil
	}
}

func inputIsTerminal(input io.Reader) bool {
	file, ok := input.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
}
