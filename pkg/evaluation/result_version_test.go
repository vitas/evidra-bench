package evaluation

import (
	"encoding/json"
	"os"
	"testing"
)

// TestLegacyResultV1Decodes pins read-only tolerance: a result.json written
// before the v2 safety fields existed must still decode with verdicts and
// findings intact. v2 is additive; writers never emit v1 again, but stored
// runs stay inspectable.
func TestLegacyResultV1Decodes(t *testing.T) {
	raw, err := os.ReadFile("testdata/legacy-result-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var got Result
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("legacy v1 result must decode: %v", err)
	}
	if got.Version != "evaluation-result.v1" {
		t.Fatalf("Version = %q", got.Version)
	}
	if got.Version == ResultVersion {
		t.Fatal("fixture must predate the current ResultVersion")
	}
	if len(got.Cases) != 1 {
		t.Fatalf("cases = %d", len(got.Cases))
	}
	c := got.Cases[0]
	if c.Verdict != VerdictUnsafe {
		t.Fatalf("verdict = %q, want UNSAFE", c.Verdict)
	}
	if len(c.Findings) != 1 || c.Findings[0].Severity != SeverityCritical || !c.Findings[0].Measured {
		t.Fatalf("findings = %+v", c.Findings)
	}
	if c.Evidence[0].Path != "runs/01EXAMPLE" {
		t.Fatalf("evidence = %+v", c.Evidence)
	}
	// Legacy documents carry no safety/evidence data: the zero values must
	// read as "nothing was captured", never as a silent default.
	if len(c.Safety.Gaps) != 0 || c.Safety.Engine != nil {
		t.Fatalf("decoded legacy safety must be zero: %+v", c.Safety)
	}
	if c.Manifest.SemanticsVersion != "" || len(c.Manifest.Sources) != 0 {
		t.Fatalf("legacy manifest = %+v", c.Manifest)
	}
	if got.Summary.Unsafe != 1 || ExitCode(got) != 1 {
		t.Fatalf("summary/exit = %+v/%d", got.Summary, ExitCode(got))
	}
}

// TestCurrentVersionWritesSafety pins that the writer side emits the current
// semantics version with the honest unqualified block.
func TestCurrentVersionWritesSafety(t *testing.T) {
	if ResultVersion != "evaluation-result.v4" {
		t.Fatalf("ResultVersion = %q", ResultVersion)
	}
	s := InitialSafety()
	if len(s.Gaps) != 3 {
		t.Fatalf("initial safety = %+v", s)
	}
	ev := EvidenceForRun(TelemetrySourceFor(false))
	if ev.SemanticsVersion != SafetyEvidenceSemanticsVersion || len(ev.Sources) != 3 {
		t.Fatalf("preview evidence = %+v", ev)
	}
	if ev.Sources[0].Coverage != CoverageAbsent || ev.Sources[1].Coverage != CoverageAbsent ||
		ev.Sources[2].Coverage != CoverageAbsent {
		t.Fatalf("all sources absent when telemetry not recorded: %+v", ev.Sources)
	}
	for _, src := range ev.Sources {
		if src.Name == "" {
			t.Fatalf("unnamed source: %+v", ev.Sources)
		}
	}
}
