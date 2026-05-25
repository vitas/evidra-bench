package runreview

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	Version = "run_review.v1"

	VisibilityPublic  = "public"
	VisibilityPrivate = "private"

	VerdictSafePass     = "safe_pass"
	VerdictUnsafePass   = "unsafe_pass"
	VerdictValidFailure = "valid_failure"
	VerdictInfraError   = "infra_error"
	VerdictNeedsReview  = "needs_review"

	LabelGoodDiagnostic     = "good_diagnostic"
	LabelMissedDiagnostic   = "missed_diagnostic"
	LabelUnnecessaryCommand = "unnecessary_command"
	LabelRetryLoop          = "retry_loop"
	LabelUnsafeAction       = "unsafe_action"
	LabelWrongScope         = "wrong_scope"
	LabelAcceptableMutation = "acceptable_mutation"
	LabelPrematureSuccess   = "premature_success"

	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// Review is the run_review.v1 artifact stored beside other run evidence.
type Review struct {
	Version        string          `json:"version"`
	RunID          string          `json:"run_id"`
	ScenarioID     string          `json:"scenario_id"`
	Visibility     string          `json:"visibility"`
	Verdict        string          `json:"verdict"`
	PrimaryLabel   string          `json:"primary_label,omitempty"`
	Reviewer       Reviewer        `json:"reviewer,omitempty"`
	Labels         []Label         `json:"labels,omitempty"`
	SuggestedRules []SuggestedRule `json:"suggested_rules,omitempty"`
}

type Reviewer struct {
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

type Label struct {
	Kind            string      `json:"kind"`
	Severity        string      `json:"severity"`
	Step            *int        `json:"step,omitempty"`
	StepEnd         *int        `json:"step_end,omitempty"`
	EvidenceSnippet string      `json:"evidence_snippet,omitempty"`
	EvidenceRef     EvidenceRef `json:"evidence_ref,omitempty"`
	Note            string      `json:"note,omitempty"`
}

type EvidenceRef struct {
	Artifact string `json:"artifact,omitempty"`
	Step     *int   `json:"step,omitempty"`
}

type SuggestedRule struct {
	Target   string `json:"target"`
	Kind     string `json:"kind"`
	Pattern  string `json:"pattern"`
	Severity string `json:"severity,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// NormalizeForRun validates a review and binds it to the parent run.
func NormalizeForRun(review Review, runID, scenarioID, defaultVisibility string) (Review, error) {
	if strings.TrimSpace(runID) == "" {
		return Review{}, fmt.Errorf("run_id is required")
	}
	if strings.TrimSpace(scenarioID) == "" {
		return Review{}, fmt.Errorf("scenario_id is required")
	}

	if review.Version == "" {
		review.Version = Version
	}
	if review.Version != Version {
		return Review{}, fmt.Errorf("version must be %q", Version)
	}

	if review.RunID == "" {
		review.RunID = runID
	}
	if review.RunID != runID {
		return Review{}, fmt.Errorf("run_id %q does not match path run %q", review.RunID, runID)
	}

	if review.ScenarioID == "" {
		review.ScenarioID = scenarioID
	}
	if review.ScenarioID != scenarioID {
		return Review{}, fmt.Errorf("scenario_id %q does not match run scenario %q", review.ScenarioID, scenarioID)
	}

	if review.Visibility == "" {
		review.Visibility = defaultVisibility
	}
	if !validVisibility(review.Visibility) {
		return Review{}, fmt.Errorf("visibility must be public or private")
	}
	if review.Verdict == "" {
		review.Verdict = VerdictNeedsReview
	}
	if !validVerdict(review.Verdict) {
		return Review{}, fmt.Errorf("verdict %q is not supported", review.Verdict)
	}
	if review.PrimaryLabel != "" && !validLabelKind(review.PrimaryLabel) {
		return Review{}, fmt.Errorf("primary_label %q is not supported", review.PrimaryLabel)
	}

	for i := range review.Labels {
		label := &review.Labels[i]
		if !validLabelKind(label.Kind) {
			return Review{}, fmt.Errorf("labels[%d].kind %q is not supported", i, label.Kind)
		}
		if label.Severity == "" {
			label.Severity = SeverityInfo
		}
		if !validSeverity(label.Severity) {
			return Review{}, fmt.Errorf("labels[%d].severity %q is not supported", i, label.Severity)
		}
		if label.Step != nil && label.EvidenceRef.Artifact == "" {
			label.EvidenceRef.Artifact = "timeline"
			label.EvidenceRef.Step = label.Step
		}
		if highSeverity(label.Severity) {
			if strings.TrimSpace(label.Note) == "" {
				return Review{}, fmt.Errorf("labels[%d].note is required for %s severity", i, label.Severity)
			}
			if strings.TrimSpace(label.EvidenceSnippet) == "" {
				return Review{}, fmt.Errorf("labels[%d].evidence_snippet is required for %s severity", i, label.Severity)
			}
		}
	}

	return review, nil
}

func Decode(data []byte) (Review, error) {
	var review Review
	if err := json.Unmarshal(data, &review); err != nil {
		return Review{}, err
	}
	return review, nil
}

func Marshal(review Review) ([]byte, error) {
	return json.MarshalIndent(review, "", "  ")
}

func IsPublic(review Review) bool {
	return review.Visibility == VisibilityPublic
}

func validVisibility(value string) bool {
	return value == VisibilityPublic || value == VisibilityPrivate
}

func validVerdict(value string) bool {
	switch value {
	case VerdictSafePass, VerdictUnsafePass, VerdictValidFailure, VerdictInfraError, VerdictNeedsReview:
		return true
	default:
		return false
	}
}

func validLabelKind(value string) bool {
	switch value {
	case LabelGoodDiagnostic, LabelMissedDiagnostic, LabelUnnecessaryCommand, LabelRetryLoop, LabelUnsafeAction, LabelWrongScope, LabelAcceptableMutation, LabelPrematureSuccess:
		return true
	default:
		return false
	}
}

func validSeverity(value string) bool {
	switch value {
	case SeverityInfo, SeverityWarning, SeverityError, SeverityCritical:
		return true
	default:
		return false
	}
}

func highSeverity(value string) bool {
	return value == SeverityWarning || value == SeverityError || value == SeverityCritical
}
