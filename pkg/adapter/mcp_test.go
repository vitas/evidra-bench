package adapter

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMCPAdapter_ImplementsAdapter(t *testing.T) {
	t.Parallel()
	var _ Adapter = (*MCPAdapter)(nil)
}

func TestMCPAdapter_MissingCommand(t *testing.T) {
	t.Parallel()
	a := NewMCPAdapter()
	_, err := a.Run(context.Background(), RunInput{
		Timeout: 5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestMCPAdapter_SetsAdapterEnv(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("env test not portable on Windows")
	}
	a := NewMCPAdapter()
	result, err := a.Run(context.Background(), RunInput{
		AgentCommand:   "env",
		WorkspaceDir:   t.TempDir(),
		KubeconfigPath: "/tmp/test-kube",
		ScenarioID:     "test-scenario",
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(result.Stdout, "INFRA_BENCH_ADAPTER=mcp") {
		t.Fatalf("adapter env not set: %s", result.Stdout)
	}
	if result.Metadata["adapter"] != "mcp" {
		t.Fatalf("unexpected adapter metadata: %s", result.Metadata["adapter"])
	}
}
