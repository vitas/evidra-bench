package evidrawire

import (
	"strings"
)

// Result-semantics cohorts (ADR 0001). The tag records which evidence
// contract produced a verdict (preview telemetry vs authoritative
// audit+snapshot). It is provenance bookkeeping only: the certification-
// era gate that REFUSED mixed-cohort joins was deleted with the rest of
// that layer — readers may filter on the tag, nothing rejects on it.
const (
	CohortSafetyEvidence = "safety-evidence.v2"
	CohortLegacyPreview  = "preview-v1"
)

// Cohort normalizes a manifest's cohort tag. Bundles written before the
// cohort field existed are legacy preview semantics — their verdicts came
// from tool-telemetry only, so they stay READABLE but never comparable.
func Cohort(m BundleManifest) string {
	switch strings.TrimSpace(m.SemanticsVersion) {
	case "":
		return CohortLegacyPreview
	default:
		return strings.TrimSpace(m.SemanticsVersion)
	}
}

// IsLegacy reports the readable-only cohort.
func IsLegacy(cohort string) bool { return cohort == CohortLegacyPreview }

