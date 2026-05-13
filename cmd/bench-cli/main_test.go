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
