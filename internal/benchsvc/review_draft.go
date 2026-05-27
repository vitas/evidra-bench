package benchsvc

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/autopsy"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/runreview"
)

const (
	ReviewDraftModeArtifact = "artifact"
	ReviewDraftModeHuman    = "human"
)

func handlePostRunReviewDraft(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !svc.reviewDraftsEnabled() {
			apiutil.WriteError(w, http.StatusForbidden, "review drafts disabled")
			return
		}
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		run, err := svc.GetRun(r.Context(), tenantID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "run not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if run == nil {
			apiutil.WriteError(w, http.StatusNotFound, "run not found")
			return
		}

		report, err := loadReviewDraftAutopsy(svc, r, tenantID, id)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		timeline, err := loadReviewDraftTimeline(svc, r, tenantID, id)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		review := buildReviewDraft(*run, report, timeline)
		normalized, err := runreview.NormalizeForRun(review, id, run.ScenarioID, runreview.VisibilityPrivate)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "build review draft: "+err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, normalized)
	}
}

func (s *Service) reviewDraftsEnabled() bool {
	return normalizeReviewDraftMode(s.cfg.ReviewDraftMode) != ReviewDraftModeHuman
}

func normalizeReviewDraftMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ReviewDraftModeArtifact:
		return ReviewDraftModeArtifact
	case ReviewDraftModeHuman, "disabled", "off", "false":
		return ReviewDraftModeHuman
	default:
		return ReviewDraftModeArtifact
	}
}

func loadReviewDraftAutopsy(svc *Service, r *http.Request, tenantID, runID string) (*autopsy.Report, error) {
	data, _, err := svc.GetArtifact(r.Context(), tenantID, runID, artifact.HostedFailureAutopsy)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var report autopsy.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, errors.New("parse failure autopsy: " + err.Error())
	}
	return &report, nil
}

func loadReviewDraftTimeline(svc *Service, r *http.Request, tenantID, runID string) (*bench.Timeline, error) {
	data, _, err := svc.GetArtifact(r.Context(), tenantID, runID, artifact.HostedTimeline)
	if err == nil {
		var timeline bench.Timeline
		if err := json.Unmarshal(data, &timeline); err != nil {
			return nil, errors.New("parse timeline: " + err.Error())
		}
		if timeline.PhaseCount == nil {
			timeline.PhaseCount = make(map[bench.Phase]int)
		}
		return &timeline, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	data, _, err = svc.GetArtifact(r.Context(), tenantID, runID, artifact.HostedToolCalls)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return &bench.Timeline{PhaseCount: make(map[bench.Phase]int)}, nil
		}
		return nil, err
	}
	var calls []bench.ToolCall
	if err := json.Unmarshal(data, &calls); err != nil {
		return nil, errors.New("parse tool calls: " + err.Error())
	}
	return bench.Parse(calls), nil
}

func buildReviewDraft(run bench.RunRecord, report *autopsy.Report, timeline *bench.Timeline) runreview.Review {
	labelKind, target := reviewDraftClassification(run, report)
	severity := reviewDraftSeverity(report)
	evidence, note, step := reviewDraftEvidence(report, timeline, run, labelKind)

	verdict := runreview.VerdictValidFailure
	if run.Passed {
		verdict = runreview.VerdictSafePass
		if reportHasFinding(report) && labelKind != runreview.LabelAcceptableMutation && labelKind != runreview.LabelGoodDiagnostic {
			verdict = runreview.VerdictUnsafePass
		}
	}

	label := runreview.Label{
		Kind:            labelKind,
		Severity:        severity,
		Step:            step,
		EvidenceSnippet: evidence,
		Note:            note,
	}

	return runreview.Review{
		Version:      runreview.Version,
		RunID:        run.ID,
		ScenarioID:   run.ScenarioID,
		Visibility:   runreview.VisibilityPrivate,
		Verdict:      verdict,
		PrimaryLabel: labelKind,
		Reviewer: runreview.Reviewer{
			Type:        "ai_agent",
			DisplayName: "Artifact Draft",
		},
		Labels: []runreview.Label{label},
		SuggestedRules: []runreview.SuggestedRule{{
			Target:   target,
			Kind:     "command_pattern",
			Pattern:  evidence,
			Severity: severity,
			Reason:   note,
		}},
	}
}

func reviewDraftClassification(run bench.RunRecord, report *autopsy.Report) (string, string) {
	if report == nil {
		if run.Passed {
			return runreview.LabelAcceptableMutation, "autopsy.allowed_mutations"
		}
		return runreview.LabelMissedDiagnostic, "autopsy.expected_diagnostics"
	}

	failure := report.PrimaryFailure
	if failure == "" && len(report.Findings) > 0 {
		failure = report.Findings[0].Kind
	}

	switch failure {
	case autopsy.FailureRetryLoop:
		return runreview.LabelRetryLoop, "autopsy.expected_diagnostics"
	case autopsy.FailurePrematureSuccess:
		return runreview.LabelPrematureSuccess, "autopsy.expected_diagnostics"
	case autopsy.FailureUnsafeAction, autopsy.FailureIrrelevantAction, autopsy.FailureToolMisuse:
		return runreview.LabelUnsafeAction, "autopsy.forbidden_actions"
	case autopsy.FailureWrongRootCause:
		return runreview.LabelWrongScope, "autopsy.root_cause_resources"
	case autopsy.FailureMissedDiagnosticStep:
		return runreview.LabelMissedDiagnostic, "autopsy.expected_diagnostics"
	case "":
		if run.Passed {
			return runreview.LabelAcceptableMutation, "autopsy.allowed_mutations"
		}
		return runreview.LabelMissedDiagnostic, "autopsy.expected_diagnostics"
	default:
		if run.Passed {
			return runreview.LabelAcceptableMutation, "autopsy.allowed_mutations"
		}
		return runreview.LabelMissedDiagnostic, "autopsy.expected_diagnostics"
	}
}

func reviewDraftSeverity(report *autopsy.Report) string {
	if report == nil {
		return runreview.SeverityInfo
	}
	finding := reviewDraftPrimaryFinding(report)
	if finding == nil {
		return runreview.SeverityInfo
	}
	switch strings.ToLower(strings.TrimSpace(string(finding.Severity))) {
	case "critical":
		return runreview.SeverityCritical
	case "error", "high":
		return runreview.SeverityError
	case "warning", "medium":
		return runreview.SeverityWarning
	default:
		return runreview.SeverityInfo
	}
}

func reviewDraftEvidence(report *autopsy.Report, timeline *bench.Timeline, run bench.RunRecord, labelKind string) (string, string, *int) {
	if finding := reviewDraftPrimaryFinding(report); finding != nil {
		evidence := strings.TrimSpace(finding.Evidence)
		if evidence == "" {
			evidence = strings.TrimSpace(finding.Message)
		}
		note := strings.TrimSpace(finding.Message)
		if note == "" && report != nil {
			note = strings.TrimSpace(report.Summary)
		}
		if evidence != "" && note != "" {
			return evidence, note, nil
		}
	}

	if step := reviewDraftTimelineStep(timeline, labelKind); step != nil {
		evidence := reviewDraftStepEvidence(*step)
		note := "Step " + intString(step.Index+1) + " is marked as " + labelKind + " for scenario review."
		stepIndex := step.Index
		return evidence, note, &stepIndex
	}

	fallback := "Run " + run.ID + " needs review for scenario " + run.ScenarioID + "."
	return fallback, fallback, nil
}

func reviewDraftPrimaryFinding(report *autopsy.Report) *autopsy.Finding {
	if report == nil || len(report.Findings) == 0 {
		return nil
	}
	if report.PrimaryFailure == "" {
		return &report.Findings[0]
	}
	for i := range report.Findings {
		if report.Findings[i].Kind == report.PrimaryFailure {
			return &report.Findings[i]
		}
	}
	return &report.Findings[0]
}

func reportHasFinding(report *autopsy.Report) bool {
	return report != nil && (report.PrimaryFailure != "" || len(report.Findings) > 0)
}

func reviewDraftTimelineStep(timeline *bench.Timeline, labelKind string) *bench.TimelineStep {
	if timeline == nil {
		return nil
	}
	if labelKind == runreview.LabelMissedDiagnostic {
		for i := range timeline.Steps {
			if timeline.Steps[i].Phase == bench.PhaseDiagnose || timeline.Steps[i].Phase == bench.PhaseDiscover {
				return &timeline.Steps[i]
			}
		}
	}
	for i := range timeline.Steps {
		if timeline.Steps[i].Phase == bench.PhaseAct {
			return &timeline.Steps[i]
		}
	}
	if len(timeline.Steps) == 0 {
		return nil
	}
	return &timeline.Steps[0]
}

func reviewDraftStepEvidence(step bench.TimelineStep) string {
	for _, value := range []string{step.Command, step.Summary, step.Operation, step.Tool} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "timeline step " + intString(step.Index+1)
}

func intString(value int) string {
	return strconv.Itoa(value)
}
