package evaluation

import "time"

const ResultVersion = "evaluation-result.v1"

type TerminationKind string

const (
	TerminationComplete   TerminationKind = "complete"
	TerminationIncomplete TerminationKind = "incomplete"
	TerminationCancelled  TerminationKind = "cancelled"
)

type Result struct {
	Version         string              `json:"version"`
	ID              string              `json:"id"`
	PlanFingerprint string              `json:"plan_fingerprint"`
	Suite           SuitePlan           `json:"suite"`
	Environment     EnvironmentPlan     `json:"environment"`
	Target          TargetPlan          `json:"target"`
	StartedAt       time.Time           `json:"started_at"`
	EndedAt         time.Time           `json:"ended_at"`
	Termination     Termination         `json:"termination"`
	Cleanup         CleanupResult       `json:"cleanup"`
	Preflight       []PreflightEvidence `json:"preflight,omitempty"`
	Cases           []CaseResult        `json:"cases"`
	Summary         Summary             `json:"summary"`
}

// PreflightEvidence records checks performed before infrastructure
// acquisition. It is run evidence and deliberately does not participate in
// the evaluation plan fingerprint.
type PreflightEvidence struct {
	Kind   string `json:"kind"`
	Method string `json:"method"`
	Usage  Usage  `json:"usage"`
}

type CaseResult struct {
	ScenarioID   string          `json:"scenario_id"`
	RunID        string          `json:"run_id,omitempty"`
	Verdict      Verdict         `json:"verdict"`
	Duration     time.Duration   `json:"duration_ns"`
	ExitCode     int             `json:"agent_exit_code,omitempty"`
	ChecksPassed int             `json:"checks_passed"`
	ChecksTotal  int             `json:"checks_total"`
	Usage        Usage           `json:"usage"`
	Findings     []SafetyFinding `json:"safety_findings,omitempty"`
	Evidence     []EvidenceRef   `json:"evidence,omitempty"`
	Termination  Termination     `json:"termination"`
}

type Usage struct {
	Known            bool `json:"known"`
	PromptTokens     int  `json:"prompt_tokens,omitempty"`
	CompletionTokens int  `json:"completion_tokens,omitempty"`
}

type EvidenceRef struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type Termination struct {
	Kind    TerminationKind `json:"kind"`
	Phase   string          `json:"phase,omitempty"`
	Reason  string          `json:"reason,omitempty"`
	Details string          `json:"details,omitempty"`
}

type CleanupResult struct {
	Attempted bool   `json:"attempted"`
	Succeeded bool   `json:"succeeded"`
	Error     string `json:"error,omitempty"`
}

type Summary struct {
	Total      int `json:"total"`
	Passed     int `json:"passed"`
	Failed     int `json:"failed"`
	Unsafe     int `json:"unsafe"`
	Incomplete int `json:"incomplete"`
}

func Summarize(result Result) Summary {
	summary := Summary{Total: len(result.Cases)}
	for _, c := range result.Cases {
		switch c.Verdict {
		case VerdictPass:
			summary.Passed++
		case VerdictFail:
			summary.Failed++
		case VerdictUnsafe:
			summary.Unsafe++
		case VerdictIncomplete:
			summary.Incomplete++
		}
	}
	return summary
}

func ExitCode(result Result) int {
	if result.Termination.Kind != TerminationComplete {
		return 2
	}
	if result.Cleanup.Attempted && !result.Cleanup.Succeeded {
		return 2
	}
	if result.Summary.Incomplete > 0 {
		return 2
	}
	if result.Summary.Failed > 0 || result.Summary.Unsafe > 0 {
		return 1
	}
	return 0
}
