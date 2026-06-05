package main

import (
	"path/filepath"
	"testing"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

func TestPrepareScenarioRunConfigSetsRunPathsWithoutMutatingBase(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Scenario = "old-scenario"
	base.Model = "old-model"
	base.RunsDir = "/old/runs"
	base.EvidenceDir = "/old/evidence"
	s := &scenario.Scenario{
		ID:   "s1",
		Path: "kubernetes/s1",
	}
	runDir := filepath.Join(t.TempDir(), "run")

	got := prepareScenarioRunConfig(base, s, "sonnet", runDir)

	if got.Scenario != "kubernetes/s1" {
		t.Fatalf("Scenario = %q, want kubernetes/s1", got.Scenario)
	}
	if got.Model != "sonnet" {
		t.Fatalf("Model = %q, want sonnet", got.Model)
	}
	if got.RunsDir != runDir {
		t.Fatalf("RunsDir = %q, want %q", got.RunsDir, runDir)
	}
	if got.EvidenceDir != filepath.Join(runDir, "evidence") {
		t.Fatalf("EvidenceDir = %q, want run evidence dir", got.EvidenceDir)
	}
	if base.Scenario != "old-scenario" || base.Model != "old-model" || base.RunsDir != "/old/runs" || base.EvidenceDir != "/old/evidence" {
		t.Fatalf("base config was mutated: %#v", base)
	}
}

func TestRunDisplayVerdictClassifiesVerificationFailureAsFail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		passed     bool
		errMessage string
		want       string
	}{
		{name: "pass", passed: true, want: "PASS"},
		{name: "plain fail", passed: false, want: "FAIL"},
		{name: "verification failure", passed: false, errMessage: (&RunFailedError{ScenarioID: "s1"}).Error(), want: "FAIL"},
		{name: "unexpected error", passed: false, errMessage: "runner returned nil result", want: "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := runDisplayVerdict(tt.passed, tt.errMessage, "s1"); got != tt.want {
				t.Fatalf("runDisplayVerdict() = %q, want %q", got, tt.want)
			}
		})
	}
}
