package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/tui"
)

func TestMainHelp(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}
}

func TestRunCommand_MissingScenario(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"run"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing scenario")
	}
}

func TestRunHelpDoesNotExposeEvidraSpecialModes(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"run", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}

	help := buf.String()
	for _, flag := range []string{
		"--evidra ",
		"--evidra-url",
		"--evidra-api-key",
		"--trace ",
		"--proxy-mode",
		"--smart-prescribe",
		"--evidra-bin",
		"--evidra-evidence-dir",
	} {
		if strings.Contains(help, flag) {
			t.Fatalf("run help exposes removed special mode flag %q:\n%s", flag, help)
		}
	}
	if !strings.Contains(help, "--mcp-server") {
		t.Fatalf("run help must retain generic --mcp-server support:\n%s", help)
	}
}

func TestHelpDoesNotExposeLegacyRoleFlag(t *testing.T) {
	t.Parallel()

	commands := [][]string{
		{"run", "--help"},
		{"bench", "--help"},
		{"certify", "--help"},
		{"report-pack", "--help"},
	}
	for _, args := range commands {
		var buf bytes.Buffer
		cmd := newRootCommand()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v help failed: %v", args, err)
		}
		if strings.Contains(buf.String(), "--role") {
			t.Fatalf("%v help exposes legacy --role flag:\n%s", args, buf.String())
		}
	}
}

func TestBenchAPIKeyFlagHelpDoesNotExposeEnvSecret(t *testing.T) {
	t.Setenv("BENCH_API_KEY", "secret-value-must-not-appear")

	commands := [][]string{
		{"run", "--help"},
		{"bench", "--help"},
		{"report-pack", "--help"},
		{"certify", "--help"},
		{"skill-delta", "run", "--help"},
	}
	for _, args := range commands {
		var buf bytes.Buffer
		cmd := newRootCommand()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v help failed: %v", args, err)
		}
		if strings.Contains(buf.String(), "secret-value-must-not-appear") {
			t.Fatalf("%v help exposed BENCH_API_KEY:\n%s", args, buf.String())
		}
	}
}

func TestApplyServeEnvOptions_ControlPlaneOnlyUsesCanonicalEnv(t *testing.T) {
	t.Setenv("BENCH_CONTROL_PLANE_ONLY", "true")

	opts := applyServeEnvOptions(serveOptions{})

	if !opts.ControlPlaneOnly {
		t.Fatal("expected BENCH_CONTROL_PLANE_ONLY to enable control-plane-only mode")
	}
}

func TestApplyLabFlagOverrides_PropagatesRunsDir(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.RunsDir = "/tmp/custom-runs"
	cfg.Provider = "bifrost"

	labCfg := tui.DefaultLabConfig()
	flags := pflag.NewFlagSet("lab", pflag.ContinueOnError)
	flags.String("runs-dir", "", "")
	flags.String("provider", "", "")
	if err := flags.Set("runs-dir", cfg.RunsDir); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("provider", cfg.Provider); err != nil {
		t.Fatal(err)
	}

	applyLabFlagOverrides(&labCfg, cfg, flags)

	if labCfg.RunsDir != cfg.RunsDir {
		t.Fatalf("runs dir = %q, want %q", labCfg.RunsDir, cfg.RunsDir)
	}
	if labCfg.Provider != cfg.Provider {
		t.Fatalf("provider = %q, want %q", labCfg.Provider, cfg.Provider)
	}
}

func TestApplyLabFlagOverrides_PropagatesRunParityFlags(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.A2AAgentURL = "https://agent.example/rpc"
	cfg.ClusterName = "cluster-a"
	cfg.SystemPromptFile = "prompts/system.md"
	cfg.SkillFile = "skills/k8s.md"
	cfg.SkillID = "k8s-admin"
	cfg.SkillVersion = "2026.05"
	cfg.SkillSource = "local-file"
	cfg.SkillSHA256 = "abc123"
	cfg.MCPServer = "npx -y kubernetes-mcp-server"
	cfg.ToolServerID = "kubernetes-mcp"
	cfg.ToolServerVersion = "1.2.3"
	cfg.ReportID = "report-a"
	cfg.ContractVersion = "v1.2.0"
	cfg.Parallel = 2
	cfg.DatabaseURL = "postgres://bench"
	cfg.MemoryWindow = 4
	cfg.ReuseCluster = true
	cfg.EvidenceDir = "evidence-in"
	cfg.BenchURL = "https://bench.example"
	cfg.BenchAPIKey = "secret"

	labCfg := tui.DefaultLabConfig()
	flags := pflag.NewFlagSet("lab", pflag.ContinueOnError)
	flags.String("a2a-agent-url", "", "")
	flags.String("cluster-name", "", "")
	flags.String("system-prompt-file", "", "")
	flags.String("skill-file", "", "")
	flags.String("skill-id", "", "")
	flags.String("skill-version", "", "")
	flags.String("skill-source", "", "")
	flags.String("skill-sha256", "", "")
	flags.String("mcp-server", "", "")
	flags.String("tool-server-id", "", "")
	flags.String("tool-server-version", "", "")
	flags.String("report-id", "", "")
	flags.String("contract-version", "", "")
	flags.Int("parallel", 1, "")
	flags.String("database-url", "", "")
	flags.Int("memory-window", -1, "")
	flags.Bool("reuse-cluster", false, "")
	flags.String("evidence-dir", "", "")
	flags.String("bench-url", "", "")
	flags.String("bench-api-key", "", "")

	for name, value := range map[string]string{
		"a2a-agent-url":       cfg.A2AAgentURL,
		"cluster-name":        cfg.ClusterName,
		"system-prompt-file":  cfg.SystemPromptFile,
		"skill-file":          cfg.SkillFile,
		"skill-id":            cfg.SkillID,
		"skill-version":       cfg.SkillVersion,
		"skill-source":        cfg.SkillSource,
		"skill-sha256":        cfg.SkillSHA256,
		"mcp-server":          cfg.MCPServer,
		"tool-server-id":      cfg.ToolServerID,
		"tool-server-version": cfg.ToolServerVersion,
		"report-id":           cfg.ReportID,
		"contract-version":    cfg.ContractVersion,
		"database-url":        cfg.DatabaseURL,
		"evidence-dir":        cfg.EvidenceDir,
		"bench-url":           cfg.BenchURL,
		"bench-api-key":       cfg.BenchAPIKey,
	} {
		if err := flags.Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	if err := flags.Set("parallel", "2"); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("memory-window", "4"); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("reuse-cluster", "true"); err != nil {
		t.Fatal(err)
	}

	applyLabFlagOverrides(&labCfg, cfg, flags)

	if labCfg.A2AAgentURL != cfg.A2AAgentURL || labCfg.ClusterName != cfg.ClusterName {
		t.Fatalf("a2a/cluster = %q/%q", labCfg.A2AAgentURL, labCfg.ClusterName)
	}
	if labCfg.SystemPromptFile != cfg.SystemPromptFile || labCfg.SkillFile != cfg.SkillFile {
		t.Fatalf("prompt files = %q/%q", labCfg.SystemPromptFile, labCfg.SkillFile)
	}
	if labCfg.SkillID != cfg.SkillID || labCfg.SkillVersion != cfg.SkillVersion || labCfg.SkillSource != cfg.SkillSource || labCfg.SkillSHA256 != cfg.SkillSHA256 {
		t.Fatalf("skill identity = %q/%q/%q/%q", labCfg.SkillID, labCfg.SkillVersion, labCfg.SkillSource, labCfg.SkillSHA256)
	}
	if labCfg.MCPServer != cfg.MCPServer || labCfg.ToolServerID != cfg.ToolServerID || labCfg.ToolServerVersion != cfg.ToolServerVersion {
		t.Fatalf("tool server = %q/%q/%q", labCfg.MCPServer, labCfg.ToolServerID, labCfg.ToolServerVersion)
	}
	if labCfg.ReportID != cfg.ReportID || labCfg.ContractVersion != cfg.ContractVersion {
		t.Fatalf("report/contract = %q/%q", labCfg.ReportID, labCfg.ContractVersion)
	}
	if labCfg.Parallel != cfg.Parallel || labCfg.DatabaseURL != cfg.DatabaseURL {
		t.Fatalf("parallel/database = %d/%q", labCfg.Parallel, labCfg.DatabaseURL)
	}
	if labCfg.MemoryWindow != cfg.MemoryWindow || !labCfg.ReuseCluster {
		t.Fatalf("memory/reuse = %d/%v", labCfg.MemoryWindow, labCfg.ReuseCluster)
	}
	if labCfg.EvidenceDir != cfg.EvidenceDir || labCfg.BenchURL != cfg.BenchURL || labCfg.BenchAPIKey != cfg.BenchAPIKey {
		t.Fatalf("reporting paths = %q/%q/%q", labCfg.EvidenceDir, labCfg.BenchURL, labCfg.BenchAPIKey)
	}
}

func TestBuildVersionString_UsesBuildMetadata(t *testing.T) {
	// Not parallel: mutates package-level version/commit/date vars.
	originalVersion, originalCommit, originalDate := version, commit, date
	t.Cleanup(func() {
		version = originalVersion
		commit = originalCommit
		date = originalDate
	})

	version = "v0.1.0-3-gabcdef0"
	commit = "abcdef0"
	date = "2026-03-15T12:00:00Z"

	got := buildVersionString()
	want := "bench-cli v0.1.0-3-gabcdef0 (commit: abcdef0, built: 2026-03-15T12:00:00Z)"
	if got != want {
		t.Fatalf("buildVersionString() = %q, want %q", got, want)
	}
}

func TestResolveLocalAdapter_CLI(t *testing.T) {
	t.Parallel()

	got, err := resolveLocalAdapter("cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected CLI adapter")
	}
}

func TestResolveLocalAdapter_UnknownFails(t *testing.T) {
	t.Parallel()

	if _, err := resolveLocalAdapter("wat"); err == nil {
		t.Fatal("expected error for unknown adapter")
	}
}
