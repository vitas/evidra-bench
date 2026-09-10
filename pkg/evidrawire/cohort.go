package evidrawire

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Result-semantics cohorts (ADR 0001 Phase 11). Bundles are only ever
// compared, diffed or aggregated WITHIN one cohort; a mixed-cohort join
// would silently compare verdicts produced under different evidence
// contracts (preview telemetry vs authoritative audit+snapshot+ledger).
const (
	CohortSafetyEvidence = "safety-evidence.v1"
	CohortLegacyPreview  = "preview-v1"
)

// ErrMixedCohorts is returned when bundles from different result-semantics
// cohorts are asked to compare.
var ErrMixedCohorts = errors.New("mixed semantics cohorts")

// ErrNotComparable is returned when a bundle from a readable-only cohort
// (legacy preview-v1) participates in a comparison at all.
var ErrNotComparable = errors.New("cohort is readable, not comparable")

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

// CohortNotice is the human line tools print when a legacy bundle takes
// part in an operation.
func CohortNotice(cohort string) string {
	if IsLegacy(cohort) {
		return "notice: " + cohort + " bundle is readable but NOT comparable: its verdicts predate authoritative evidence capture (preview telemetry)."
	}
	return ""
}

// SameCohort checks a set of manifests for joinability. It returns the
// cohort when uniform, a notice when the uniform cohort is legacy, and an
// ErrMixedCohorts-tagged error when versions differ.
func SameCohort(manifests ...BundleManifest) (cohort string, notice string, err error) {
	seen := map[string][]string{}
	for _, m := range manifests {
		c := Cohort(m)
		seen[c] = append(seen[c], m.Spec)
	}
	if len(seen) == 0 {
		return "", "", errors.New("no bundles to compare")
	}
	if len(seen) > 1 {
		var parts []string
		for c := range seen {
			parts = append(parts, c)
		}
		return "", "", fmt.Errorf("%w: %s — refuse to join verdicts across evidence contracts", ErrMixedCohorts, strings.Join(sortedUniq(parts), " + "))
	}
	for c := range seen {
		if n := CohortNotice(c); n != "" {
			return c, n, nil
		}
		return c, "", nil
	}
	return "", "", nil
}

func sortedUniq(v []string) []string {
	out := append([]string(nil), v...)
	sort.Strings(out)
	return out
}
