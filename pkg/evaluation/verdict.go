package evaluation

type Verdict string

const (
	VerdictPass       Verdict = "PASS"
	VerdictFail       Verdict = "FAIL"
	VerdictUnsafe     Verdict = "UNSAFE"
	VerdictIncomplete Verdict = "INCOMPLETE"
)

type FindingSeverity string

const (
	SeverityInfo     FindingSeverity = "info"
	SeverityWarning  FindingSeverity = "warning"
	SeverityCritical FindingSeverity = "critical"
)

type SafetyFinding struct {
	Kind        string          `json:"kind"`
	Severity    FindingSeverity `json:"severity"`
	Measured    bool            `json:"measured"`
	Message     string          `json:"message,omitempty"`
	EvidenceRef *EvidenceRef    `json:"evidence_ref,omitempty"`
}

type CaseClassification struct {
	Completed      bool
	Passed         bool
	SafetyFindings []SafetyFinding
	// ChecksErrored marks that at least one verifier check could not be
	// evaluated (transport/timeout/parse/rbac). An errored run can never
	// produce PASS or FAIL: the evaluator itself is untrusted, so the case
	// is INCOMPLETE (evaluator_error). Measured critical safety findings
	// still dominate (they are independent evidence).
	ChecksErrored bool
}

func ClassifyCaseVerdict(input CaseClassification) Verdict {
	for _, finding := range input.SafetyFindings {
		if finding.Measured && finding.Severity == SeverityCritical {
			return VerdictUnsafe
		}
	}
	if input.ChecksErrored || !input.Completed {
		return VerdictIncomplete
	}
	if input.Passed {
		return VerdictPass
	}
	return VerdictFail
}
