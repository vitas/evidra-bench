package adapter

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCLIAdapter_ImplementsAdapter(t *testing.T) {
	t.Parallel()
	var _ Adapter = (*CLIAdapter)(nil)
}

func TestCLIAdapter_MissingCommand(t *testing.T) {
	t.Parallel()
	a := NewCLIAdapter()
	_, err := a.Run(context.Background(), RunInput{
		Timeout: 5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestCLIAdapter_CapturesOutput(t *testing.T) {
	t.Parallel()
	a := NewCLIAdapter()

	cmd := "echo"
	args := []string{"hello from agent"}
	if runtime.GOOS == "windows" {
		t.Skip("echo test not portable on Windows")
	}

	result, err := a.Run(context.Background(), RunInput{
		AgentCommand:   cmd,
		AgentArgs:      args,
		WorkspaceDir:   t.TempDir(),
		KubeconfigPath: "/tmp/fake-kubeconfig",
		ScenarioID:     "test-scenario",
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello from agent") {
		t.Fatalf("unexpected stdout: %s", result.Stdout)
	}
}

func TestCLIAdapter_NonZeroExit(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("bash not available on Windows")
	}
	a := NewCLIAdapter()
	result, err := a.Run(context.Background(), RunInput{
		AgentCommand:   "bash",
		AgentArgs:      []string{"-c", "exit 42"},
		WorkspaceDir:   t.TempDir(),
		KubeconfigPath: "/tmp/fake-kubeconfig",
		ScenarioID:     "test-scenario",
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result.ExitCode != 42 {
		t.Fatalf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestCLIAdapter_SetsEnvironment(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("env test not portable on Windows")
	}
	a := NewCLIAdapter()
	result, err := a.Run(context.Background(), RunInput{
		AgentCommand:   "env",
		WorkspaceDir:   t.TempDir(),
		KubeconfigPath: "/tmp/test-kube",
		ScenarioID:     "my-scenario",
		Timeout:        5 * time.Second,
		Env:            map[string]string{"CUSTOM_VAR": "custom_value"},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(result.Stdout, "KUBECONFIG=/tmp/test-kube") {
		t.Fatalf("KUBECONFIG not set: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "INFRA_BENCH_SCENARIO=my-scenario") {
		t.Fatalf("scenario not set: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "CUSTOM_VAR=custom_value") {
		t.Fatalf("custom var not set: %s", result.Stdout)
	}
}
