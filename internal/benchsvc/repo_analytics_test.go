package benchsvc

import "testing"

func TestSignalMetricsFromArtifactsUsesAutopsyWhenScorecardMissing(t *testing.T) {
	t.Parallel()

	autopsy := []byte(`{
		"outcome": "fail",
		"findings": [
			{"kind": "retry_loop"},
			{"kind": "retry_loop"},
			{"kind": "premature_success"}
		]
	}`)

	metrics := signalMetricsFromArtifacts(nil, autopsy)

	if metrics.hasScorecard {
		t.Fatal("hasScorecard = true, want false")
	}
	if got := metrics.counts["retry_loop"]; got != 2 {
		t.Fatalf("retry_loop count = %d, want 2", got)
	}
	if got := metrics.counts["premature_success"]; got != 1 {
		t.Fatalf("premature_success count = %d, want 1", got)
	}
}

func TestSignalMetricsFromArtifactsPrefersScorecardOverAutopsy(t *testing.T) {
	t.Parallel()

	scorecard := []byte(`{
		"score": 88,
		"signals": {
			"blast_radius": 1
		}
	}`)
	autopsy := []byte(`{
		"findings": [
			{"kind": "retry_loop"}
		]
	}`)

	metrics := signalMetricsFromArtifacts(scorecard, autopsy)

	if !metrics.hasScorecard {
		t.Fatal("hasScorecard = false, want true")
	}
	if got := metrics.score; got != 88 {
		t.Fatalf("score = %v, want 88", got)
	}
	if got := metrics.counts["blast_radius"]; got != 1 {
		t.Fatalf("blast_radius count = %d, want 1", got)
	}
	if got := metrics.counts["retry_loop"]; got != 0 {
		t.Fatalf("retry_loop count = %d, want 0 when scorecard is present", got)
	}
}
