package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultLabConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultLabConfig()
	if cfg.Adapter != "cli" {
		t.Fatalf("expected adapter=cli, got %s", cfg.Adapter)
	}
	if cfg.Timeout != "5m" {
		t.Fatalf("expected timeout=5m, got %s", cfg.Timeout)
	}
	if !cfg.DryRun {
		t.Fatal("expected dry_run=true by default")
	}
}

func TestLabConfig_TimeoutDuration(t *testing.T) {
	t.Parallel()
	cfg := LabConfig{Timeout: "3m"}
	if cfg.TimeoutDuration().Minutes() != 3 {
		t.Fatalf("expected 3m, got %v", cfg.TimeoutDuration())
	}
}

func TestLabConfig_TimeoutDuration_Invalid(t *testing.T) {
	t.Parallel()
	cfg := LabConfig{Timeout: "bogus"}
	if cfg.TimeoutDuration().Minutes() != 5 {
		t.Fatalf("expected 5m fallback, got %v", cfg.TimeoutDuration())
	}
}

func TestSaveAndLoadLabConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := LabConfig{
		Adapter:      "mcp",
		AgentCommand: "/usr/bin/agent",
		Timeout:      "10m",
		DryRun:       false,
	}
	if err := SaveLabConfig(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded := LoadLabConfig(path)
	if loaded.Adapter != "mcp" {
		t.Fatalf("expected adapter=mcp, got %s", loaded.Adapter)
	}
	if loaded.AgentCommand != "/usr/bin/agent" {
		t.Fatalf("expected agent_command, got %s", loaded.AgentCommand)
	}
	if loaded.DryRun {
		t.Fatal("expected dry_run=false")
	}
}

func TestLoadLabConfig_Missing(t *testing.T) {
	t.Parallel()
	cfg := LoadLabConfig("/nonexistent/path")
	if cfg.Adapter != "cli" {
		t.Fatalf("expected defaults, got adapter=%s", cfg.Adapter)
	}
}

func TestSaveLabConfig_CreatesDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")
	if err := SaveLabConfig(path, DefaultLabConfig()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}
