package harness

import (
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

func TestBuildSystemPromptDirectModeMentionsRunCommand(t *testing.T) {
	prompt, err := buildSystemPrompt(config.Config{}, &scenario.Scenario{
		Scope: scenario.Scope{Namespaces: []string{"bench"}},
	})
	if err != nil {
		t.Fatalf("buildSystemPrompt returned error: %v", err)
	}
	if !strings.Contains(prompt, "run_command") {
		t.Fatalf("direct prompt should mention run_command, got:\n%s", prompt)
	}
}

func TestBuildSystemPromptMCPModeDoesNotMentionRunCommand(t *testing.T) {
	prompt, err := buildSystemPrompt(config.Config{MCPServer: "npx -y kubernetes-mcp-server@0.0.62"}, &scenario.Scenario{
		Scope: scenario.Scope{Namespaces: []string{"bench"}},
	})
	if err != nil {
		t.Fatalf("buildSystemPrompt returned error: %v", err)
	}
	if strings.Contains(prompt, "run_command") {
		t.Fatalf("MCP prompt should not mention unavailable run_command tool, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "MCP") {
		t.Fatalf("MCP prompt should orient the model to MCP tools, got:\n%s", prompt)
	}
}
