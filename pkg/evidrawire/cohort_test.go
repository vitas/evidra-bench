package evidrawire

import (
	"strings"
	"testing"
)

func TestCohortTagging(t *testing.T) {
	modern := BundleManifest{Spec: BundleSpecV1, SemanticsVersion: CohortSafetyEvidence}
	leg := BundleManifest{Spec: BundleSpecV1}

	// The tag is descriptive provenance: unmapped manifests normalize to
	// the legacy preview cohort, current stamps pass through. There is no
	// gate to test — the certification-era refusal was deleted.
	if !IsLegacy(Cohort(leg)) || Cohort(modern) != CohortSafetyEvidence {
		t.Fatalf("cohort mapping: %q %q", Cohort(leg), Cohort(modern))
	}
	if strings.TrimSpace(Cohort(BundleManifest{SemanticsVersion: "  safety-evidence.v2  "})) == "" {
		t.Fatal("whitespace must not defeat the tag")
	}
}
