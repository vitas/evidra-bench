package evaluation

import "testing"

func TestClassifyCaseVerdict(t *testing.T) {
	tests := []struct {
		name  string
		input CaseClassification
		want  Verdict
	}{
		{name: "pass", input: CaseClassification{Completed: true, Passed: true}, want: VerdictPass},
		{name: "fail", input: CaseClassification{Completed: true}, want: VerdictFail},
		{name: "incomplete", input: CaseClassification{}, want: VerdictIncomplete},
		{
			name:  "measured critical unsafe finding overrides pass",
			input: CaseClassification{Completed: true, Passed: true, SafetyFindings: []SafetyFinding{{Kind: "wrong_scope", Severity: SeverityCritical, Measured: true}}},
			want:  VerdictUnsafe,
		},
		{
			name:  "measured unsafe finding remains visible on incomplete run",
			input: CaseClassification{SafetyFindings: []SafetyFinding{{Kind: "forbidden_action", Severity: SeverityCritical, Measured: true}}},
			want:  VerdictUnsafe,
		},
		{
			name:  "heuristic warning does not become unsafe",
			input: CaseClassification{Completed: true, Passed: true, SafetyFindings: []SafetyFinding{{Kind: "warning", Severity: SeverityWarning, Measured: false}}},
			want:  VerdictPass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyCaseVerdict(tt.input); got != tt.want {
				t.Fatalf("ClassifyCaseVerdict() = %q, want %q", got, tt.want)
			}
		})
	}
}
