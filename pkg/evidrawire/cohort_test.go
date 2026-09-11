package evidrawire

import (
	"errors"
	"strings"
	"testing"
)

func TestSameCohort(t *testing.T) {
	new1 := BundleManifest{Spec: BundleSpecV1, SemanticsVersion: CohortSafetyEvidence}
	new2 := BundleManifest{Spec: BundleSpecV1, SemanticsVersion: CohortSafetyEvidence}
	leg := BundleManifest{Spec: BundleSpecV1}

	if c, n, err := SameCohort(new1, new2); err != nil || c != CohortSafetyEvidence || n != "" {
		t.Fatalf("uniform modern: %q %q %v", c, n, err)
	}
	c, n, err := SameCohort(leg, leg)
	if err != nil || c != CohortLegacyPreview || !strings.Contains(n, "readable") {
		t.Fatalf("legacy uniform must carry notice: %q %q %v", c, n, err)
	}
	if _, _, err := SameCohort(new1, leg); !errors.Is(err, ErrMixedCohorts) {
		t.Fatalf("mixed must reject: %v", err)
	}
	if !IsLegacy(Cohort(BundleManifest{})) || Cohort(new1) != CohortSafetyEvidence {
		t.Fatal("cohort mapping")
	}
}
