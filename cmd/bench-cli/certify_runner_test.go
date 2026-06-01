package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vitas/evidra-bench/pkg/config"
)

func TestRunCertifyTrackDryRunWritesCertification(t *testing.T) {
	t.Parallel()

	scenariosDir := t.TempDir()
	writeCertifyScenario(t, scenariosDir, "workloads", "L1", "one")

	cfg := config.Default()
	cfg.ScenariosDir = scenariosDir
	cfg.RunsDir = t.TempDir()
	cfg.DryRun = true
	cfg.Provider = "bifrost"
	cfg.EnvironmentProvider = "kind"

	cert, err := runCertifyTrack(t.Context(), cfg, "workloads", "sonnet")
	if err != nil {
		t.Fatalf("runCertifyTrack: %v", err)
	}
	if cert.Track != "workloads" || cert.Model != "sonnet" {
		t.Fatalf("unexpected cert identity: %#v", cert)
	}
	if cert.Total != 1 {
		t.Fatalf("expected one selected scenario, got %d", cert.Total)
	}

	matches, err := filepath.Glob(filepath.Join(cfg.RunsDir, "certify", "workloads_sonnet_*", "certification.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one certification artifact, got %d: %v", len(matches), matches)
	}
	if _, err := os.Stat(matches[0]); err != nil {
		t.Fatalf("stat certification artifact: %v", err)
	}
}

func writeCertifyScenario(t *testing.T, root, track, level, id string) {
	t.Helper()

	dir := filepath.Join(root, "kubernetes", id)
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `id: ` + id + `
title: Test Scenario
category: kubernetes
track: ` + track + `
level: ` + level + `
prompt: prompts/task.md
environment:
  providers: [kind]
break:
  type: command
  command: "echo break"
checks:
  - type: command
    name: noop
`
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix it."), 0o644); err != nil {
		t.Fatal(err)
	}
}
