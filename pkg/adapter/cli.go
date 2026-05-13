package adapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CLIAdapter launches an agent as an external process.
type CLIAdapter struct{}

// NewCLIAdapter returns a new CLIAdapter.
func NewCLIAdapter() *CLIAdapter {
	return &CLIAdapter{}
}

// Run executes the agent command with environment variables for kubeconfig,
// workspace, and prompt path, and captures stdout/stderr.
func (a *CLIAdapter) Run(ctx context.Context, input RunInput) (*RunResult, error) {
	if input.AgentCommand == "" {
		return nil, fmt.Errorf("adapter.CLIAdapter.Run: agent-command is required")
	}

	parts := strings.Fields(input.AgentCommand)
	name := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}
	args = append(args, input.AgentArgs...)

	ctx, cancel := context.WithTimeout(ctx, input.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	cmd.Env = append(os.Environ(),
		"KUBECONFIG="+input.KubeconfigPath,
		"INFRA_BENCH_WORKSPACE="+input.WorkspaceDir,
		"INFRA_BENCH_SCENARIO="+input.ScenarioID,
	)

	if input.PromptPath != "" {
		cmd.Env = append(cmd.Env, "INFRA_BENCH_PROMPT="+input.PromptPath)
	}
	if input.Model != "" {
		cmd.Env = append(cmd.Env, "INFRA_BENCH_MODEL="+input.Model)
	}

	for k, v := range input.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = input.WorkspaceDir

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("adapter.CLIAdapter.Run: %w", err)
		}
	}

	return &RunResult{
		ExitCode:   exitCode,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		Transcript: stdout.String(),
		Metadata: map[string]string{
			"adapter": "cli",
			"command": input.AgentCommand,
		},
	}, nil
}
