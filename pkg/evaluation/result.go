package evaluation

import "time"

// ResultVersion is the canonical result schema. v4 replaces the boolean
// RuntimeInfo.Unconfined with the explicit Mode (mediated / sandboxed /
// external_unconfined / remote_unattributed) — a bool wrongly labeled
// mediated --model runs as unconfined. v3's field simply reads as absent
// on decode (Mode ""), older documents stay readable (see
// TestLegacyResultV1Decodes). v3 itself dropped the certification-era
// safety block.
const ResultVersion = "evaluation-result.v4"

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
	// Runtime records how the agent actually executed (ADR 0001 Phase 6).
	// Unconfined runs (bare --agent, provider adapters, MCP/A2A) carry a
	// permanent evidence gap.
	Runtime RuntimeInfo `json:"runtime"`

	// Safety is the authoritative-safety block (v2). Phase 2 writes it
	// statically: Qualified=false, Basis=none, explicit gaps.
	Safety Safety `json:"safety"`
	// Manifest is the evidence manifest backing Safety: per-source
	// coverage plus the semantics version of the verdict engine that
	// produced this result (v2).
	Manifest Evidence `json:"manifest"`
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

// Agent execution modes (round-4 review: a boolean "unconfined" wrongly
// labeled mediated model runs). Every case states exactly one.
const (
	// ModeMediated: in-process provider/MCP adapter. Actions travel
	// through the harness-controlled tool executor under the harness
	// identity — no unattended external process exists at all.
	ModeMediated = "mediated"
	// ModeSandboxed: external agent executed in the hardened sibling
	// container; SandboxImage carries the provenance.
	ModeSandboxed = "sandboxed"
	// ModeExternalUnconfined: explicit --agent-unconfined opt-out — an
	// external process ran with runner privileges. Profiled cases grade
	// INCOMPLETE because evidence tampering cannot be excluded.
	ModeExternalUnconfined = "external_unconfined"
	// ModeRemoteUnattributed: remote A2A agent — its execution sits
	// outside the runner's process boundary and is attributed only to
	// the remote endpoint's identity.
	ModeRemoteUnattributed = "remote_unattributed"
)

// RuntimeInfo describes the agent execution boundary for one case.
type RuntimeInfo struct {
	// Mode is one of the Mode* constants.
	Mode string `json:"mode"`
	// SandboxImage is the hardened-execution image when sandboxed.
	SandboxImage string `json:"sandbox_image,omitempty"`
}
