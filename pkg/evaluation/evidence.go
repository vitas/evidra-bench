package evaluation

// Evidence and Safety describe the authoritative-evidence provenance of a
// case result per docs/adr/0001-process-safety-matching.md. Phase 2 only
// establishes the schema and honest initial population: every
// case reports qualified=false with an explicit basis and gap list, so no
// downstream consumer can mistake a preview verdict for certified safety.
// Authoritative population lands with the collectors/verdict engine
// (Phases 5-8); the first qualified=true is only produced in Phase 10.

// SourceName identifies one of the three authoritative evidence layers.
type SourceName string

const (
	// SourceAPIAudit is the Kubernetes API audit stream: authoritative for
	// ACTIONS.
	SourceAPIAudit SourceName = "api_audit"
	// SourceStateSnapshot is the before/after state diff: authoritative for
	// EFFECTS.
	SourceStateSnapshot SourceName = "state_snapshot"
	// SourceToolTelemetry is agent tool-call telemetry: explanatory only,
	// never sufficient.
	SourceToolTelemetry SourceName = "tool_telemetry"
)

// SourceCoverage is the completeness state of one evidence source for a
// case.
type SourceCoverage string

const (
	// CoverageComplete: the source was captured for the whole run window
	// with no known gaps.
	CoverageComplete SourceCoverage = "complete"
	// CoverageIncomplete: captured but with a known loss (rotation, drain
	// gap, partial window). Never sufficient to qualify.
	CoverageIncomplete SourceCoverage = "incomplete"
	// CoverageAbsent: the source was not captured at all.
	CoverageAbsent SourceCoverage = "absent"
)

// SourceStatus records the observed coverage of one evidence source, plus
// where the raw material lives.
type SourceStatus struct {
	Name     SourceName     `json:"name"`
	Coverage SourceCoverage `json:"coverage"`
	// Reason explains non-complete coverage (e.g. "collector_not_implemented
	// _yet", "log_rotated"). Empty when complete.
	Reason string `json:"reason,omitempty"`
	// Digest is the sha256 of the collected, redacted source for this case
	// (empty until a collector lands).
	Digest string `json:"digest,omitempty"`
	// Path locates the on-disk artifact for this source (best-effort).
	Path string `json:"path,omitempty"`
}

// Evidence is the per-case evidence manifest. SemanticsVersion pins how the
// verdict was produced so cohorts with different semantics are never
// compared together (design §7).
type Evidence struct {
	SemanticsVersion string         `json:"semantics_version"`
	Sources          []SourceStatus `json:"sources"`
}

// Safety is the authoritative-safety evidence block: what was captured,
// what was missing, what the engine concluded. Since the certification
// layer was dropped (2026-09-11) it carries no qualification verdict —
// Gaps stay as honest diagnostics ("this run never saw the audit layer").
type Safety struct {
	// Gaps are stable identifiers for missing or degraded evidence.
	Gaps []string `json:"gaps,omitempty"`
	// Violations are safety findings sourced from authoritative evidence
	// (audit/snapshot). Preview telemetry findings remain in
	// CaseResult.Findings; this list is populated by Phase 8.
	Violations []SafetyFinding `json:"violations,omitempty"`
	// Engine is the authoritative verdict engine's assessment for this
	// case (ADR 0001 Phase 8). For scenarios with an authority profile the
	// engine verdict IS the case verdict (UNSAFE/INCOMPLETE dominance);
	// classifyVerdict remains the path for profile-less scenarios. Nil when
	// no authority profile existed.
	Engine *EngineVerdict `json:"engine,omitempty"`
}

// Gaps for the unqualified Phase 2 baseline. They document WHY nothing is
// yet authoritative; they are stable identifiers consumed by the ledger.
const (
	GapAuditNotCaptured       = "api_audit_not_captured"
	GapSnapshotNotCaptured    = "state_snapshot_not_captured"
	GapTelemetryNotSufficient = "tool_telemetry_not_sufficient"
	// GapAgentUnconfined marks runs whose agent executed outside the
	// hardened sandbox (ADR 0001 Phase 6): permanent until re-run confined.
	GapAgentUnconfined = "agent_unconfined_execution"
)

// SafetyEvidenceSemanticsVersion is stamped on every run produced by the
// ADR 0001 harness: the authoritative-safety evidence shape (engine,
// coverage). The tag is provenance bookkeeping kept after the
// certification-era cohort gate was dropped — readers may still filter
// on it; nothing refuses to join cohorts anymore.
const SafetyEvidenceSemanticsVersion = "safety-evidence.v2"

// PreviewSemanticsVersion is the semantics tag for Phase 2: verdicts are
// produced from process/telemetry signals only and can never qualify.
const PreviewSemanticsVersion = "preview-v1"

// InitialSafety returns the honest starting Safety block: every authoritative
// source is treated as not yet captured; the harness closes gaps as audit
// and snapshot evidence actually lands for the case.
func InitialSafety() Safety {
	return Safety{
		Gaps: []string{
			GapAuditNotCaptured,
			GapSnapshotNotCaptured,
			GapTelemetryNotSufficient,
		},
	}
}

// previewEvidence returns the static Evidence manifest. api_audit and
// state_snapshot are absent (collectors land later); tool_telemetry
// coverage is supplied by the caller based on the adapter path, since the
// harness is the only place that knows whether telemetry was recorded.
// EvidenceForRun stamps the evidence manifest every run this binary produces.
func EvidenceForRun(telemetry SourceStatus) Evidence {
	return Evidence{
		SemanticsVersion: SafetyEvidenceSemanticsVersion,
		Sources: []SourceStatus{
			{Name: SourceAPIAudit, Coverage: CoverageAbsent, Reason: GapAuditNotCaptured},
			{Name: SourceStateSnapshot, Coverage: CoverageAbsent, Reason: GapSnapshotNotCaptured},
			telemetry,
		},
	}
}

// telemetrySourceFor decides tool_telemetry coverage from whether the harness
// recorded tool calls for the case. It is explanatory evidence regardless.
func TelemetrySourceFor(recorded bool) SourceStatus {
	if recorded {
		return SourceStatus{
			Name:     SourceToolTelemetry,
			Coverage: CoverageComplete,
			Reason:   "explanatory_only",
		}
	}
	return SourceStatus{
		Name:     SourceToolTelemetry,
		Coverage: CoverageAbsent,
		Reason:   "no_tool_telemetry_recorded",
	}
}

// AuditSummary carries the per-run API-audit collection outcome from the
// harness into the qualification manifest.
type AuditSummary struct {
	Observed   bool // markers sealed a window (else coverage absent)
	Coverage   SourceCoverage
	Reason     string // coverage degradation explanation
	Path       string // evidence file within the run artifact dir
	Digest     string // hex sha256 of the persisted (redacted) file
	EventCount int
}

// ApplyAudit replaces the api_audit source entry with collection results.
func (e *Evidence) ApplyAudit(a AuditSummary) {
	e.setSource(SourceAPIAudit, SourceStatus{
		Name:     SourceAPIAudit,
		Coverage: a.Coverage,
		Reason:   a.Reason,
		Digest:   a.Digest,
		Path:     a.Path,
	})
}

// setSource replaces or appends a source entry by name.
func (e *Evidence) setSource(name SourceName, st SourceStatus) {
	for i := range e.Sources {
		if e.Sources[i].Name == name {
			e.Sources[i] = st
			return
		}
	}
	e.Sources = append(e.Sources, st)
}

// DropGap removes a gap id once the corresponding source really became complete.
func (s *Safety) DropGap(gap string) {
	out := s.Gaps[:0]
	for _, g := range s.Gaps {
		if g != gap {
			out = append(out, g)
		}
	}
	s.Gaps = out
}

// SnapshotSummary carries the per-run state-snapshot outcome into the
// qualification manifest (ADR 0001 Phase 7).
type SnapshotSummary struct {
	Coverage        SourceCoverage
	Reason          string
	BaselineDigest  string
	PostAgentDigest string
	StabilityDigest string
	Violations      int
}

// ApplySnapshot replaces the state_snapshot source entry.
func (e *Evidence) ApplySnapshot(a SnapshotSummary) {
	e.setSource(SourceStateSnapshot, SourceStatus{
		Name:     SourceStateSnapshot,
		Coverage: a.Coverage,
		Reason:   a.Reason,
		Digest:   a.PostAgentDigest,
		Path:     "snapshot-post-agent.json",
	})
}
