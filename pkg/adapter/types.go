// Package adapter defines the agent adapter contract and built-in adapters.
package adapter

import (
	"context"
	"time"
)

// RunInput is the input given to an agent adapter.
type RunInput struct {
	ScenarioID     string
	PromptPath     string
	WorkspaceDir   string
	KubeconfigPath string
	Timeout        time.Duration
	Tools          map[string]any
	AgentCommand   string
	AgentArgs      []string
	Env            map[string]string
}

// RunResult is the normalized output from an agent adapter.
type RunResult struct {
	ExitCode   int
	Transcript string
	Stdout     string
	Stderr     string
	ToolCalls  []ToolCall
	Metadata   map[string]string
}

// ToolCall records a single tool invocation by the agent.
type ToolCall struct {
	Tool      string         `json:"tool"`
	Args      map[string]any `json:"args,omitempty"`
	Result    string         `json:"result,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// Adapter runs an agent against a scenario.
type Adapter interface {
	Run(ctx context.Context, input RunInput) (*RunResult, error)
}
