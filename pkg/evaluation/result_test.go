package evaluation

import "testing"

func TestSummarizeAndExitCodeUseCanonicalCaseVerdicts(t *testing.T) {
	result := Result{Cases: []CaseResult{
		{ScenarioID: "repair", Verdict: VerdictPass},
		{ScenarioID: "restraint", Verdict: VerdictUnsafe},
		{ScenarioID: "scope", Verdict: VerdictIncomplete},
	}}

	summary := Summarize(result)
	if summary.Total != 3 || summary.Passed != 1 || summary.Unsafe != 1 || summary.Incomplete != 1 || summary.Failed != 0 {
		t.Fatalf("Summarize() = %+v", summary)
	}
	result.Summary = summary
	result.Termination = Termination{Kind: TerminationIncomplete, Reason: "case_incomplete"}
	if got := ExitCode(result); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2 for incomplete evaluation", got)
	}
}

func TestExitCodeReturnsBehavioralFailureWithoutIncompleteCases(t *testing.T) {
	result := Result{Cases: []CaseResult{
		{ScenarioID: "repair", Verdict: VerdictPass},
		{ScenarioID: "scope", Verdict: VerdictFail},
	}}
	result.Summary = Summarize(result)
	result.Termination = Termination{Kind: TerminationComplete}
	result.Cleanup = CleanupResult{Attempted: true, Succeeded: true}

	if got := ExitCode(result); got != 1 {
		t.Fatalf("ExitCode() = %d, want 1 for behavioral failure", got)
	}
}

func TestExitCodeTreatsPreflightAndCleanupFailuresAsIncomplete(t *testing.T) {
	preflight := Result{Termination: Termination{Kind: TerminationIncomplete, Phase: "preflight"}}
	if got := ExitCode(preflight); got != 2 {
		t.Fatalf("preflight ExitCode() = %d, want 2", got)
	}

	cleanup := Result{
		Termination: Termination{Kind: TerminationComplete},
		Cleanup:     CleanupResult{Attempted: true, Succeeded: false, Error: "release failed"},
	}
	if got := ExitCode(cleanup); got != 2 {
		t.Fatalf("cleanup ExitCode() = %d, want 2", got)
	}
}
