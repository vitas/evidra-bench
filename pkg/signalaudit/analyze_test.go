package signalaudit_test

import (
	"testing"

	"github.com/vitas/evidra-bench/pkg/signalaudit"
)

func TestAnalyze_FlagsMissingExpectedSignal(t *testing.T) {
	manifest := signalaudit.Manifest{
		"broken-deployment": {
			PrimarySignal:   "retry_loop",
			ExpectedSignals: []string{"retry_loop"},
		},
	}
	runs := []signalaudit.Run{
		{
			RunDir:       "/runs/r1",
			ScenarioID:   "broken-deployment",
			Model:        "sonnet",
			Provider:     "claude",
			SignalCounts: map[string]int{},
		},
	}

	result := signalaudit.Analyze(manifest, runs)
	if result.FindingTotals.MissingExpected != 1 {
		t.Fatalf("missing_expected = %d, want 1", result.FindingTotals.MissingExpected)
	}
	if len(result.RunFindings) != 1 {
		t.Fatalf("run findings = %d, want 1", len(result.RunFindings))
	}
	if got := result.RunFindings[0].MissingExpected; len(got) != 1 || got[0] != "retry_loop" {
		t.Fatalf("missing expected = %v, want [retry_loop]", got)
	}
}

func TestAnalyze_FlagsForbiddenSignal(t *testing.T) {
	manifest := signalaudit.Manifest{
		"broken-deployment": {
			PrimarySignal:    "retry_loop",
			ExpectedSignals:  []string{"retry_loop"},
			ForbiddenSignals: []string{"blast_radius"},
		},
	}
	runs := []signalaudit.Run{
		{
			RunDir:       "/runs/r1",
			ScenarioID:   "broken-deployment",
			Model:        "sonnet",
			Provider:     "claude",
			Signals:      []string{"blast_radius", "retry_loop"},
			SignalCounts: map[string]int{"retry_loop": 1, "blast_radius": 1},
		},
	}

	result := signalaudit.Analyze(manifest, runs)
	if result.FindingTotals.ForbiddenSignals != 1 {
		t.Fatalf("forbidden_signals = %d, want 1", result.FindingTotals.ForbiddenSignals)
	}
	if got := result.RunFindings[0].ForbiddenSignals; len(got) != 1 || got[0] != "blast_radius" {
		t.Fatalf("forbidden signals = %v, want [blast_radius]", got)
	}
}

func TestAnalyze_FlagsUnexpectedExtraSignal(t *testing.T) {
	manifest := signalaudit.Manifest{
		"networkpolicy-blocking": {
			PrimarySignal:           "blast_radius",
			ExpectedSignals:         []string{"blast_radius"},
			AllowedSecondarySignals: []string{"new_scope"},
		},
	}
	runs := []signalaudit.Run{
		{
			RunDir:       "/runs/r1",
			ScenarioID:   "networkpolicy-blocking",
			Model:        "sonnet",
			Provider:     "claude",
			SignalCounts: map[string]int{"blast_radius": 1, "thrashing": 1, "new_scope": 1},
		},
	}

	result := signalaudit.Analyze(manifest, runs)
	if result.FindingTotals.UnexpectedExtras != 1 {
		t.Fatalf("unexpected_extras = %d, want 1", result.FindingTotals.UnexpectedExtras)
	}
	if got := result.RunFindings[0].UnexpectedExtras; len(got) != 1 || got[0] != "thrashing" {
		t.Fatalf("unexpected extras = %v, want [thrashing]", got)
	}
}

func TestAnalyze_FlagsInstabilityAcrossRepeats(t *testing.T) {
	manifest := signalaudit.Manifest{
		"broken-deployment": {
			PrimarySignal:   "retry_loop",
			ExpectedSignals: []string{"retry_loop"},
		},
	}
	runs := []signalaudit.Run{
		{
			RunDir:       "/runs/r1",
			ScenarioID:   "broken-deployment",
			Model:        "sonnet",
			Provider:     "claude",
			SignalCounts: map[string]int{"retry_loop": 1},
		},
		{
			RunDir:       "/runs/r2",
			ScenarioID:   "broken-deployment",
			Model:        "sonnet",
			Provider:     "claude",
			SignalCounts: map[string]int{"retry_loop": 1, "thrashing": 1},
		},
	}

	result := signalaudit.Analyze(manifest, runs)
	if len(result.InstabilityFindings) != 1 {
		t.Fatalf("instability findings = %d, want 1", len(result.InstabilityFindings))
	}
	if result.FindingTotals.UnstableGroups != 1 {
		t.Fatalf("unstable_groups = %d, want 1", result.FindingTotals.UnstableGroups)
	}
	got := result.InstabilityFindings[0]
	if got.ScenarioID != "broken-deployment" {
		t.Fatalf("scenario_id = %q, want broken-deployment", got.ScenarioID)
	}
	if len(got.ObservedSignalSets) != 2 {
		t.Fatalf("observed signal sets = %d, want 2", len(got.ObservedSignalSets))
	}
}
