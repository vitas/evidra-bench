package main

import (
	"bytes"
	"strings"
	"testing"
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

func TestRunCommand_DryRun(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"run", "--scenario", "kubernetes/broken-deployment", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(buf.String(), "scenario=kubernetes/broken-deployment") {
		t.Fatalf("unexpected output: %s", buf.String())
	}
}
