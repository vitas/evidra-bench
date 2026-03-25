package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPExecutor routes tool calls through an MCP server process.
type MCPExecutor struct {
	session      *mcp.ClientSession
	serverCmd    string
	evidenceMode EvidenceMode
}

// NewMCPExecutor starts an MCP server subprocess and connects via stdio.
// The command is split on spaces: "evidra-mcp --signing-mode optional"
// Extra env vars (e.g., KUBECONFIG) are injected into the subprocess.
func NewMCPExecutor(ctx context.Context, command string, extraEnv []string) (*MCPExecutor, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("mcp_executor: empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stderr = os.Stderr // MCP server logs go to stderr

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

	// Detect evidence mode from available tools.
	mode := EvidenceModeNone
	tools, listErr := session.ListTools(ctx, &mcp.ListToolsParams{})
	if listErr == nil {
		for _, t := range tools.Tools {
			log.Printf("[mcp-executor] tool: %s", t.Name)
			if t.Name == "prescribe_smart" || t.Name == "prescribe_full" {
				mode = EvidenceModeProxy // has evidence tools = at least proxy
			}
		}
	}

	return &MCPExecutor{
		session:      session,
		serverCmd:    command,
		evidenceMode: mode,
	}, nil
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
			schemaBytes, _ := json.Marshal(t.InputSchema)
			var params map[string]any
			json.Unmarshal(schemaBytes, &params)
			def.Parameters = params
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

	result, err := e.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      tc.Name,
		Arguments: args,
	})
	if err != nil {
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

// EvidenceMode returns the evidence mode based on available MCP tools.
func (e *MCPExecutor) EvidenceMode() EvidenceMode {
	return e.evidenceMode
}

// Close terminates the MCP server session.
func (e *MCPExecutor) Close() error {
	if e.session != nil {
		return e.session.Close()
	}
	return nil
}
