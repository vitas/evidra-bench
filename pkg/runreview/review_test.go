package runreview

import (
	"strings"
	"testing"
)

func TestNormalizeForRunDefaultsReviewIdentity(t *testing.T) {
	t.Parallel()

	review, err := NormalizeForRun(Review{
		Verdict:      VerdictUnsafePass,
		PrimaryLabel: LabelUnsafeAction,
		Labels: []Label{{
			Kind:            LabelUnsafeAction,
			Severity:        SeverityWarning,
			Step:            intPtr(17),
			EvidenceSnippet: "pods_delete Pod/web-77b5997d98-bvghz in bench",
			Note:            "Direct Pod deletion is a risky restart shortcut.",
		}},
	}, "run-1", "shared-configmap-trap", VisibilityPublic)
	if err != nil {
		t.Fatalf("NormalizeForRun returned error: %v", err)
	}

	if review.Version != Version {
		t.Fatalf("Version = %q, want %q", review.Version, Version)
	}
	if review.RunID != "run-1" {
		t.Fatalf("RunID = %q, want run-1", review.RunID)
	}
	if review.ScenarioID != "shared-configmap-trap" {
		t.Fatalf("ScenarioID = %q, want shared-configmap-trap", review.ScenarioID)
	}
	if review.Visibility != VisibilityPublic {
		t.Fatalf("Visibility = %q, want %q", review.Visibility, VisibilityPublic)
	}
	if review.Labels[0].EvidenceRef.Artifact != "timeline" || review.Labels[0].EvidenceRef.Step == nil || *review.Labels[0].EvidenceRef.Step != 17 {
		t.Fatalf("EvidenceRef = %#v, want timeline step 17", review.Labels[0].EvidenceRef)
	}
}

func TestNormalizeForRunRejectsPrivateVisibilityTypo(t *testing.T) {
	t.Parallel()

	_, err := NormalizeForRun(Review{
		Visibility:   "published",
		Verdict:      VerdictNeedsReview,
		PrimaryLabel: LabelMissedDiagnostic,
		Labels: []Label{{
			Kind:            LabelMissedDiagnostic,
			Severity:        SeverityInfo,
			EvidenceSnippet: "kubectl get pods -n bench",
			Note:            "Needs another reviewer.",
		}},
	}, "run-1", "scenario-1", VisibilityPrivate)
	if err == nil || !strings.Contains(err.Error(), "visibility") {
		t.Fatalf("NormalizeForRun error = %v, want visibility error", err)
	}
}

func TestNormalizeForRunRejectsMismatchedParent(t *testing.T) {
	t.Parallel()

	_, err := NormalizeForRun(Review{
		RunID:        "other-run",
		ScenarioID:   "scenario-1",
		Verdict:      VerdictNeedsReview,
		PrimaryLabel: LabelMissedDiagnostic,
		Labels: []Label{{
			Kind:            LabelMissedDiagnostic,
			Severity:        SeverityInfo,
			EvidenceSnippet: "kubectl get pods -n bench",
			Note:            "Needs another reviewer.",
		}},
	}, "run-1", "scenario-1", VisibilityPrivate)
	if err == nil || !strings.Contains(err.Error(), "run_id") {
		t.Fatalf("NormalizeForRun error = %v, want run_id error", err)
	}
}

func TestNormalizeForRunRequiresEvidenceForHighSeverityLabels(t *testing.T) {
	t.Parallel()

	_, err := NormalizeForRun(Review{
		Verdict:      VerdictUnsafePass,
		PrimaryLabel: LabelUnsafeAction,
		Labels: []Label{{
			Kind:     LabelUnsafeAction,
			Severity: SeverityError,
			Step:     intPtr(3),
			Note:     "This is unsafe.",
		}},
	}, "run-1", "scenario-1", VisibilityPublic)
	if err == nil || !strings.Contains(err.Error(), "evidence_snippet") {
		t.Fatalf("NormalizeForRun error = %v, want evidence_snippet error", err)
	}
}

func intPtr(v int) *int {
	return &v
}
