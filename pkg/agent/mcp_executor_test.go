package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type blockingMCPSession struct{}

func (blockingMCPSession) ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{}, nil
}

func (blockingMCPSession) CallTool(ctx context.Context, _ *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingMCPSession) Close() error { return nil }

func TestMCPExecutorExecuteHasPerToolTimeout(t *testing.T) {
	originalTimeout := mcpToolCallTimeout
	mcpToolCallTimeout = 50 * time.Millisecond
	t.Cleanup(func() {
		mcpToolCallTimeout = originalTimeout
	})

	executor := &MCPExecutor{session: blockingMCPSession{}}
	started := time.Now()
	result := executor.Execute(context.Background(), ToolCall{
		Name:      "kubectl_rollout",
		Arguments: `{"namespace":"bench-staging","resourceType":"deployment","name":"api","subCommand":"status","watch":true}`,
	})

	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Execute took %s, want bounded per-tool timeout", elapsed)
	}
	if !strings.Contains(result, "MCP tool kubectl_rollout timed out after 50ms") {
		t.Fatalf("result = %q, want MCP per-tool timeout message", result)
	}
}

func TestNewMCPCommandUsesWorkspaceDir(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	cmd, err := newMCPCommand("echo hello", []string{"KUBECONFIG=/tmp/kubeconfig"}, workspace)
	if err != nil {
		t.Fatalf("newMCPCommand: %v", err)
	}
	if cmd.Dir != workspace {
		t.Fatalf("Dir = %q, want %q", cmd.Dir, workspace)
	}
	if got := envValue(cmd.Env, "INFRA_BENCH_WORKSPACE"); got != workspace {
		t.Fatalf("INFRA_BENCH_WORKSPACE = %q, want %q", got, workspace)
	}
	if got := envValue(cmd.Env, "KUBECONFIG"); got != "/tmp/kubeconfig" {
		t.Fatalf("KUBECONFIG = %q, want /tmp/kubeconfig", got)
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return strings.TrimPrefix(env[i], prefix)
		}
	}
	return ""
}
