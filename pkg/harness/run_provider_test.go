package harness

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/agent"
	"github.com/vitas/evidra-bench/pkg/config"
)

func TestProviderEvidenceDir_IsIsolatedPerRun(t *testing.T) {
	t.Parallel()

	runsDir := t.TempDir()
	a := providerEvidenceDir("", runsDir, "broken-deployment", time.Unix(100, 0))
	b := providerEvidenceDir("", runsDir, "broken-deployment", time.Unix(101, 0))

	shared := filepath.Join(runsDir, "evidence")
	if a == shared {
		t.Fatalf("provider evidence dir = %q, want per-run path", a)
	}
	if a == b {
		t.Fatalf("provider evidence dirs should differ per run: %q", a)
	}
}

func TestNewProviderToolExecutorUsesRunsDirWorkspace(t *testing.T) {
	t.Parallel()

	runsDir := t.TempDir()
	executor := newProviderToolExecutor(RunRequest{
		Config: config.Config{RunsDir: runsDir},
		ExtraEnv: []string{
			"AWS_ENDPOINT_URL=http://localhost:4566",
		},
	}, "/tmp/kubeconfig")

	if executor.WorkspaceDir != runsDir {
		t.Fatalf("WorkspaceDir = %q, want %q", executor.WorkspaceDir, runsDir)
	}
	if executor.KubeconfigPath != "/tmp/kubeconfig" {
		t.Fatalf("KubeconfigPath = %q, want /tmp/kubeconfig", executor.KubeconfigPath)
	}
	if len(executor.ExtraEnv) != 1 || executor.ExtraEnv[0] != "AWS_ENDPOINT_URL=http://localhost:4566" {
		t.Fatalf("ExtraEnv = %#v, want lease env preserved", executor.ExtraEnv)
	}
}

func TestProviderToolCalls_ExportsStructuredResults(t *testing.T) {
	t.Parallel()

	messages := []agent.Message{
		{
			Role: "assistant",
			ToolCalls: []agent.ToolCall{
				{
					ID:        "call-1",
					Name:      "run_command",
					Arguments: `{"command":"kubectl get pods -n bench"}`,
				},
				{
					ID:        "call-2",
					Name:      "evidra_report",
					Arguments: `{"prescription_id":"p1","verdict":"success","exit_code":0}`,
				},
			},
		},
		{Role: "tool", ToolCallID: "call-1", Content: "pod/web Ready"},
		{Role: "tool", ToolCallID: "call-2", Content: "reported"},
	}

	got := providerToolCalls(messages)
	if len(got) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(got))
	}
	if got[0].Tool != "run_command" {
		t.Fatalf("first tool = %q, want run_command", got[0].Tool)
	}
	if got[0].Result != "pod/web Ready" {
		t.Fatalf("first result = %q, want tool output", got[0].Result)
	}
	if got[0].Args["command"] != "kubectl get pods -n bench" {
		t.Fatalf("unexpected args: %#v", got[0].Args)
	}
	if got[1].Tool != "evidra_report" {
		t.Fatalf("second tool = %q, want evidra_report", got[1].Tool)
	}
	if got[1].Args["prescription_id"] != "p1" {
		t.Fatalf("unexpected second args: %#v", got[1].Args)
	}
}
