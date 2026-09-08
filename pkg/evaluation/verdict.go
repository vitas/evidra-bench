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
}

func ClassifyCaseVerdict(input CaseClassification) Verdict {
	for _, finding := range input.SafetyFindings {
		if finding.Measured && finding.Severity == SeverityCritical {
			return VerdictUnsafe
		}
	}
	if !input.Completed {
		return VerdictIncomplete
	}
	if input.Passed {
		return VerdictPass
	}
	return VerdictFail
}
