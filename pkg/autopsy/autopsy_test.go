package autopsy

import (
	"encoding/json"
	"testing"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

func toolCall(t *testing.T, command, result string) bench.ToolCall {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		t.Fatal(err)
	}
	return bench.ToolCall{
		Tool:   "run_command",
		Args:   raw,
		Result: result,
	}
}

func hasFinding(report Report, kind FailureKind) bool {
	for _, finding := range report.Findings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}

func TestAnalyze_EmitsVersionedReport(t *testing.T) {
	t.Parallel()

	report := Analyze(Input{Run: bench.RunRecord{Passed: false}})

	if report.Version != ReportVersion {
		t.Fatalf("version = %q, want %q", report.Version, ReportVersion)
	}
	if report.Confidence != ConfidenceLow {
		t.Fatalf("confidence = %q, want %q", report.Confidence, ConfidenceLow)
	}
}

func TestAnalyze_DetectsRetryLoop(t *testing.T) {
	t.Parallel()

	report := Analyze(Input{
		Run: bench.RunRecord{
			Passed: false,
			Turns:  6,
		},
		ToolCalls: []bench.ToolCall{
			toolCall(t, "kubectl get pods -n bench", "web 0/1 ErrImagePull"),
			toolCall(t, "kubectl get pods -n bench", "web 0/1 ErrImagePull"),
			toolCall(t, "kubectl get pods -n bench", "web 0/1 ErrImagePull"),
		},
	})

	if !hasFinding(report, FailureRetryLoop) {
		t.Fatalf("expected retry_loop finding, got %#v", report.Findings)
	}
	if report.PrimaryFailure != FailureRetryLoop {
		t.Fatalf("primary_failure = %q, want %q", report.PrimaryFailure, FailureRetryLoop)
	}
	if report.Confidence != ConfidenceMedium {
		t.Fatalf("confidence = %q, want %q", report.Confidence, ConfidenceMedium)
	}
	if report.WastedTurns != 2 {
		t.Fatalf("wasted_turns = %d, want 2", report.WastedTurns)
	}
}

func TestAnalyze_DoesNotFlagProgressingRepeatedCommandAsRetryLoop(t *testing.T) {
	t.Parallel()

	report := Analyze(Input{
		Run: bench.RunRecord{
			Passed: false,
			Turns:  6,
		},
		ToolCalls: []bench.ToolCall{
			toolCall(t, "kubectl rollout status deployment/web -n bench", "Waiting for deployment spec update to be observed..."),
			toolCall(t, "kubectl rollout status deployment/web -n bench", "Waiting for deployment web rollout to finish: 1 old replicas are pending termination..."),
			toolCall(t, "kubectl rollout status deployment/web -n bench", "Waiting for deployment web rollout to finish: 1 of 2 updated replicas are available..."),
		},
	})

	if hasFinding(report, FailureRetryLoop) {
		t.Fatalf("did not expect retry_loop finding for progressing repeated command, got %#v", report.Findings)
	}
}

func TestAnalyze_DetectsPrematureSuccess(t *testing.T) {
	t.Parallel()

	report := Analyze(Input{
		Run: bench.RunRecord{Passed: false, ChecksPassed: 1, ChecksTotal: 2},
		Transcript: `[assistant] The deployment is fixed and everything is working.
[tool] verifier failed: deployment/web not ready`,
	})

	if !hasFinding(report, FailurePrematureSuccess) {
		t.Fatalf("expected premature_success finding, got %#v", report.Findings)
	}
}

func TestAnalyze_DetectsMissedDiagnosticStep(t *testing.T) {
	t.Parallel()

	report := Analyze(Input{
		Run: bench.RunRecord{Passed: false},
		ToolCalls: []bench.ToolCall{
			toolCall(t, "kubectl patch deployment/web -n bench --type=json -p=[]", "deployment.apps/web patched"),
			toolCall(t, "kubectl rollout status deployment/web -n bench", "timed out"),
		},
	})

	if !hasFinding(report, FailureMissedDiagnosticStep) {
		t.Fatalf("expected missed_diagnostic_step finding, got %#v", report.Findings)
	}
}

func TestAnalyze_DetectsGaveUp(t *testing.T) {
	t.Parallel()

	report := Analyze(Input{
		Run:        bench.RunRecord{Passed: false},
		Transcript: "[assistant] I cannot continue because the cluster is unavailable.",
	})

	if !hasFinding(report, FailureGaveUp) {
		t.Fatalf("expected gave_up finding, got %#v", report.Findings)
	}
}

func TestAnalyze_DetectsExcessiveTokenBurn(t *testing.T) {
	t.Parallel()

	report := Analyze(Input{
		Run: bench.RunRecord{
			Passed:           false,
			PromptTokens:     7000,
			CompletionTokens: 2500,
			Turns:            12,
		},
	})

	if !hasFinding(report, FailureExcessiveTokenBurn) {
		t.Fatalf("expected excessive_token_burn finding, got %#v", report.Findings)
	}
	if report.WastedTokens != 9500 {
		t.Fatalf("wasted_tokens = %d, want 9500", report.WastedTokens)
	}
}

func TestAnalyze_PassingRunHasNoPrimaryFailure(t *testing.T) {
	t.Parallel()

	report := Analyze(Input{
		Run: bench.RunRecord{
			Passed:           true,
			PromptTokens:     9000,
			CompletionTokens: 2000,
			Turns:            10,
			ChecksPassed:     3,
			ChecksTotal:      3,
		},
	})

	if report.PrimaryFailure != "" {
		t.Fatalf("primary_failure = %q, want empty for passing run", report.PrimaryFailure)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("findings = %#v, want none for passing run", report.Findings)
	}
	if report.Confidence != ConfidenceHigh {
		t.Fatalf("confidence = %q, want %q", report.Confidence, ConfidenceHigh)
	}
}
