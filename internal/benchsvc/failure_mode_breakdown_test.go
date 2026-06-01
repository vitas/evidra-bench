package benchsvc

import (
	"reflect"
	"testing"
)

func TestFailureModeFromClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		classification string
		primaryFailure string
		findingKinds   []string
		want           string
	}{
		{"unsafe pass maps to safety", ToolServerReportUnsafePass, "", nil, failureModeSafety},
		{"missing evidence maps directly", ToolServerReportMissingEvidence, "", nil, failureModeMissingEvidence},
		{"primary failure maps root cause", ToolServerReportFail, "wrong_root_cause", nil, failureModeRootCause},
		{"primary failure maps missed diagnostics", ToolServerReportFail, "missed_diagnostic_step", nil, failureModeDiagnosis},
		{"primary failure maps premature success", ToolServerReportFail, "premature_success", nil, failureModeVerification},
		{"primary failure maps irrelevant action", ToolServerReportFail, "irrelevant_action", nil, failureModePatching},
		{"finding kind maps tool misuse", ToolServerReportFail, "", []string{"tool_misuse"}, failureModeToolMisuse},
		{"finding kind maps retry loop", ToolServerReportFail, "", []string{"retry_loop"}, failureModeDiagnosis},
		{"unmapped fail falls back to other", ToolServerReportFail, "", nil, failureModeOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			findings := make([]ToolServerReportFinding, 0, len(tt.findingKinds))
			for _, kind := range tt.findingKinds {
				findings = append(findings, ToolServerReportFinding{Kind: kind})
			}

			got := deriveFailureMode(tt.classification, tt.primaryFailure, findings)
			if got != tt.want {
				t.Fatalf("deriveFailureMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFailureModeBreakdownAccumulator(t *testing.T) {
	t.Parallel()

	acc := newFailureModeBreakdownAccumulator()
	acc.add("flux", "flux159-mcp-server-kubernetes", "s2", ToolServerReportUnsafePass, failureModeSafety)
	acc.add("flux", "flux159-mcp-server-kubernetes", "s2", ToolServerReportUnsafePass, failureModeSafety)
	acc.add("flux", "flux159-mcp-server-kubernetes", "s3", ToolServerReportFail, failureModeRootCause)
	acc.add("flux", "flux159-mcp-server-kubernetes", "s4", ToolServerReportMissingEvidence, failureModeMissingEvidence)
	acc.add("containers", "containers-kubernetes-mcp-server", "s2", ToolServerReportSafePass, failureModeSafety)

	rows := acc.rows()
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3 rows", rows)
	}

	if got := rows[0].FailureMode; got != failureModeSafety {
		t.Fatalf("first row mode = %q, want %q", got, failureModeSafety)
	}
	if rows[0].UnsafePass != 1 || rows[0].Fail != 0 || rows[0].MissingEvidence != 0 {
		t.Fatalf("safety counts = %+v, want unsafe=1 fail=0 missing=0", rows[0])
	}
	if !reflect.DeepEqual(rows[0].ScenarioIDs, []string{"s2"}) {
		t.Fatalf("safety scenarios = %+v, want [s2]", rows[0].ScenarioIDs)
	}

	if got := rows[1].FailureMode; got != failureModeRootCause {
		t.Fatalf("second row mode = %q, want %q", got, failureModeRootCause)
	}
	if got := rows[2].FailureMode; got != failureModeMissingEvidence {
		t.Fatalf("third row mode = %q, want %q", got, failureModeMissingEvidence)
	}
}
