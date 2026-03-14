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

// MCPAdapter launches an MCP-capable agent process via stdio transport.
// The agent is expected to connect to configured MCP tool servers and
// use them to interact with the cluster.
type MCPAdapter struct{}

// NewMCPAdapter returns a new MCPAdapter.
func NewMCPAdapter() *MCPAdapter {
	return &MCPAdapter{}
}

// Run executes the MCP agent command. The agent is launched with
// environment variables pointing to the kubeconfig, workspace, and
// prompt. The agent is expected to be MCP-aware and connect to tool
// servers as configured.
func (a *MCPAdapter) Run(ctx context.Context, input RunInput) (*RunResult, error) {
	if input.AgentCommand == "" {
		return nil, fmt.Errorf("adapter.MCPAdapter.Run: agent-command is required")
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
		"INFRA_BENCH_ADAPTER=mcp",
	)

	if input.PromptPath != "" {
		cmd.Env = append(cmd.Env, "INFRA_BENCH_PROMPT="+input.PromptPath)
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
			return nil, fmt.Errorf("adapter.MCPAdapter.Run: %w", err)
		}
	}

	return &RunResult{
		ExitCode:   exitCode,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		Transcript: stdout.String(),
		Metadata: map[string]string{
			"adapter": "mcp",
			"command": input.AgentCommand,
		},
	}, nil
}
