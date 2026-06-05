package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var mcpToolCallTimeout = 60 * time.Second

type mcpSession interface {
	ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error)
	CallTool(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error)
	Close() error
}

// MCPExecutor routes tool calls through an MCP server process.
type MCPExecutor struct {
	session   mcpSession
	serverCmd string
}

// NewMCPExecutor starts an MCP server subprocess and connects via stdio.
// The command is split on spaces: "MCP server --signing-mode optional"
// Extra env vars (e.g., KUBECONFIG) are injected into the subprocess.
func NewMCPExecutor(ctx context.Context, command string, extraEnv []string, workspaceDirOpt ...string) (*MCPExecutor, error) {
	workspaceDir := ""
	if len(workspaceDirOpt) > 0 {
		workspaceDir = workspaceDirOpt[0]
	}
	cmd, err := newMCPCommand(command, extraEnv, workspaceDir)
	if err != nil {
		return nil, err
	}

	transport := &mcp.CommandTransport{Command: cmd}

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "bench-cli",
			Version: "v1.0.0",
		},
		nil,
	)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp_executor: connect to %q: %w", command, err)
	}

	log.Printf("[mcp-executor] connected to %s", command)

	tools, listErr := session.ListTools(ctx, &mcp.ListToolsParams{})
	if listErr == nil {
		for _, t := range tools.Tools {
			log.Printf("[mcp-executor] tool: %s", t.Name)
		}
	}

	return &MCPExecutor{
		session:   session,
		serverCmd: command,
	}, nil
}

func newMCPCommand(command string, extraEnv []string, workspaceDir string) (*exec.Cmd, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("mcp_executor: empty command")
	}
	normalizedWorkspace, err := normalizeWorkspaceDir(workspaceDir)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Env = toolCommandEnv("", extraEnv, normalizedWorkspace)
	if normalizedWorkspace != "" {
		cmd.Dir = normalizedWorkspace
	}
	cmd.Stderr = os.Stderr // MCP server logs go to stderr
	return cmd, nil
}

// Tools returns the tool definitions from the MCP server,
// converted to the harness ToolDef format for the LLM.
func (e *MCPExecutor) Tools(ctx context.Context) ([]ToolDef, error) {
	result, err := e.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("mcp_executor: list tools: %w", err)
	}

	var defs []ToolDef
	for _, t := range result.Tools {
		def := ToolDef{
			Name:        t.Name,
			Description: t.Description,
		}
		if t.InputSchema != nil {
			// Convert JSON schema to map for OpenAI format
			if schemaBytes, err := json.Marshal(t.InputSchema); err == nil {
				var params map[string]any
				if err := json.Unmarshal(schemaBytes, &params); err == nil {
					def.Parameters = params
				}
			}
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// Execute calls a tool on the MCP server and returns the result text.
func (e *MCPExecutor) Execute(ctx context.Context, tc ToolCall) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
		// Try as raw string
		args = map[string]any{"command": tc.Arguments}
	}

	callCtx := ctx
	cancel := func() {}
	if mcpToolCallTimeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, mcpToolCallTimeout)
	}
	defer cancel()

	result, err := e.session.CallTool(callCtx, &mcp.CallToolParams{
		Name:      tc.Name,
		Arguments: args,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil && mcpToolCallTimeout > 0 {
			return fmt.Sprintf("error: MCP tool %s timed out after %s", tc.Name, mcpToolCallTimeout)
		}
		return fmt.Sprintf("error: MCP tool %s: %v", tc.Name, err)
	}

	if result.IsError {
		// Extract error text from content
		var texts []string
		for _, c := range result.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				texts = append(texts, tc.Text)
			}
		}
		if len(texts) > 0 {
			return "error: " + strings.Join(texts, "\n")
		}
		return "error: tool returned error"
	}

	// Extract text content
	var texts []string
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			texts = append(texts, tc.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// Close terminates the MCP server session.
func (e *MCPExecutor) Close() error {
	if e.session != nil {
		return e.session.Close()
	}
	return nil
}
