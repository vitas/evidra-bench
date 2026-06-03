package main

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/vitas/evidra-bench/pkg/config"
)

func TestRegisterExecutionFlags(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cmd := &cobra.Command{Use: "test"}
	registerExecutionFlags(cmd.Flags(), &cfg, executionFlagOptions{
		IncludeScenario: true,
		ScenarioUsage:   "scenario path relative to scenarios dir",
		DryRunUsage:     "validate scenario without executing",
	})

	for _, name := range []string{
		"environment", "scenario", "scenarios-dir", "runs-dir",
		"timeout", "reuse-cluster", "cluster-name", "dry-run",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q", name)
		}
	}
}

func TestRegisterAgentFlags(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cmd := &cobra.Command{Use: "test"}
	registerAgentFlags(cmd.Flags(), &cfg, agentFlagOptions{IncludeModel: true})

	for _, name := range []string{
		"adapter", "a2a-agent-url", "agent-command", "provider", "model",
		"memory-window", "system-prompt-file",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q", name)
		}
	}
}

func TestRegisterResultMetadataFlags(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cmd := &cobra.Command{Use: "test"}
	registerResultMetadataFlags(cmd.Flags(), &cfg, resultMetadataFlagOptions{
		IncludeToolServer: true,
		IncludeReportID:   true,
	})

	for _, name := range []string{
		"bench-url", "bench-api-key", "skill-file", "skill-id",
		"skill-version", "skill-source", "skill-sha256",
		"contract-version", "mcp-server", "tool-server-id",
		"tool-server-version", "report-id",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected flag %q", name)
		}
	}
}
