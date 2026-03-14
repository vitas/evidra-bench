package verifier

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeEvidenceDir creates a temp evidence directory with segments/evidence.jsonl.
func writeEvidenceDir(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	segDir := filepath.Join(dir, "segments")
	if err := os.MkdirAll(segDir, 0755); err != nil {
		t.Fatal(err)
	}
	var content string
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(filepath.Join(segDir, "evidence.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func prescribeEntry(id, prescriptionID, effectiveRisk string, riskTags []string, intentDigest string) string {
	riskInputsJSON := "[]"
	if effectiveRisk != "" {
		tagsJSON := "[]"
		if len(riskTags) > 0 {
			tagsJSON = `[`
			for i, t := range riskTags {
				if i > 0 {
					tagsJSON += ","
				}
				tagsJSON += fmt.Sprintf(`"%s"`, t)
			}
			tagsJSON += `]`
		}
		riskInputsJSON = fmt.Sprintf(`[{"source":"evidra/native","risk_level":"%s","risk_tags":%s}]`, effectiveRisk, tagsJSON)
	}
	canonicalJSON := `{}`
	if intentDigest != "" {
		canonicalJSON = fmt.Sprintf(`{"intent_digest":"%s"}`, intentDigest)
	}
	return fmt.Sprintf(`{"entry_id":"%s","type":"prescribe","actor":{"id":"agent"},"timestamp":"2026-01-01T00:00:00Z","payload":{"prescription_id":"%s","effective_risk":"%s","risk_inputs":%s,"canonical_action":%s}}`,
		id, prescriptionID, effectiveRisk, riskInputsJSON, canonicalJSON)
}

func reportEntry(id, prescriptionID, verdict string) string {
	return fmt.Sprintf(`{"entry_id":"%s","type":"report","actor":{"id":"agent"},"timestamp":"2026-01-01T00:00:01Z","payload":{"report_id":"%s","prescription_id":"%s","verdict":"%s","exit_code":0}}`,
		id, id, prescriptionID, verdict)
}

func signalEntry(id, signalName string) string {
	return fmt.Sprintf(`{"entry_id":"%s","type":"signal","actor":{"id":"evidra"},"timestamp":"2026-01-01T00:00:02Z","payload":{"signal_name":"%s"}}`,
		id, signalName)
}

func findCheck(results []CheckResult, name string) *CheckResult {
	for _, r := range results {
		if r.Name == name {
			return &r
		}
	}
	return nil
}

func TestEvidraProtocolCheck_CleanRun(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{
		prescribeEntry("e1", "rx-1", "low", nil, ""),
		reportEntry("e2", "rx-1", "success"),
		prescribeEntry("e3", "rx-2", "low", nil, ""),
		reportEntry("e4", "rx-2", "success"),
	})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		MinPrescriptions:      2,
		MinReports:            2,
		OrphanedPrescriptions: 0,
		ProtocolViolations:    0,
		AllReportsHaveVerdict: true,
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	if !result.Passed {
		for _, c := range result.Checks {
			if c.Verdict == VerdictFail {
				t.Errorf("unexpected failure: %s: %s", c.Name, c.Message)
			}
		}
	}
}

func TestEvidraProtocolCheck_OrphanedPrescription(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{
		prescribeEntry("e1", "rx-1", "low", nil, ""),
		prescribeEntry("e2", "rx-2", "low", nil, ""),
		reportEntry("e3", "rx-1", "success"),
		// rx-2 has no matching report
	})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		OrphanedPrescriptions: 0,
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	orphaned := findCheck(result.Checks, "evidra-protocol/orphaned-prescriptions")
	if orphaned == nil {
		t.Fatal("orphaned check not found")
	}
	if orphaned.Verdict != VerdictFail {
		t.Fatalf("expected orphaned check to fail, got %s", orphaned.Verdict)
	}
}

func TestEvidraProtocolCheck_MissingVerdict(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{
		prescribeEntry("e1", "rx-1", "low", nil, ""),
		// report with empty verdict
		`{"entry_id":"e2","type":"report","actor":{"id":"agent"},"timestamp":"2026-01-01T00:00:01Z","payload":{"report_id":"e2","prescription_id":"rx-1","verdict":"","exit_code":0}}`,
	})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		AllReportsHaveVerdict: true,
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	verdictCheck := findCheck(result.Checks, "evidra-protocol/all-reports-have-verdict")
	if verdictCheck == nil {
		t.Fatal("verdict check not found")
	}
	if verdictCheck.Verdict != VerdictFail {
		t.Fatalf("expected verdict check to fail, got %s", verdictCheck.Verdict)
	}
}

func TestEvidraProtocolCheck_RiskLevelMatch(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{
		prescribeEntry("e1", "rx-1", "critical", nil, ""),
		reportEntry("e2", "rx-1", "declined"),
	})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		ExpectedRiskLevel: "critical",
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	riskCheck := findCheck(result.Checks, "evidra-protocol/expected-risk-level")
	if riskCheck == nil {
		t.Fatal("risk level check not found")
	}
	if riskCheck.Verdict != VerdictPass {
		t.Fatalf("expected risk level check to pass, got %s: %s", riskCheck.Verdict, riskCheck.Message)
	}
}

func TestEvidraProtocolCheck_RiskLevelMismatch(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{
		prescribeEntry("e1", "rx-1", "low", nil, ""),
		reportEntry("e2", "rx-1", "success"),
	})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		ExpectedRiskLevel: "critical",
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	riskCheck := findCheck(result.Checks, "evidra-protocol/expected-risk-level")
	if riskCheck == nil {
		t.Fatal("risk level check not found")
	}
	if riskCheck.Verdict != VerdictFail {
		t.Fatalf("expected risk level check to fail, got %s", riskCheck.Verdict)
	}
}

func TestEvidraProtocolCheck_DeclinedVerdict(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{
		prescribeEntry("e1", "rx-1", "critical", []string{"k8s.privileged_container"}, ""),
		reportEntry("e2", "rx-1", "declined"),
	})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		DeclinedMin: 1,
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	declinedCheck := findCheck(result.Checks, "evidra-protocol/declined-verdicts-min")
	if declinedCheck == nil {
		t.Fatal("declined check not found")
	}
	if declinedCheck.Verdict != VerdictPass {
		t.Fatalf("expected declined check to pass, got %s: %s", declinedCheck.Verdict, declinedCheck.Message)
	}
}

func TestEvidraProtocolCheck_RetryLoop(t *testing.T) {
	t.Parallel()
	lines := []string{}
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("e%d", i*2+1)
		rxID := fmt.Sprintf("rx-%d", i+1)
		lines = append(lines, prescribeEntry(id, rxID, "low", nil, "same-digest"))
		lines = append(lines, reportEntry(fmt.Sprintf("e%d", i*2+2), rxID, "failure"))
	}
	dir := writeEvidenceDir(t, lines)
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		RetryLoopMax: 5,
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	retryCheck := findCheck(result.Checks, "evidra-protocol/retry-loop-max")
	if retryCheck == nil {
		t.Fatal("retry loop check not found")
	}
	if retryCheck.Verdict != VerdictFail {
		t.Fatalf("expected retry loop check to fail, got %s", retryCheck.Verdict)
	}
}

func TestEvidraProtocolCheck_RiskTags(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{
		prescribeEntry("e1", "rx-1", "critical", []string{"k8s.privileged_container", "k8s.host_path"}, ""),
		reportEntry("e2", "rx-1", "declined"),
	})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		ExpectedRiskTags: []string{"k8s.privileged_container"},
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	tagsCheck := findCheck(result.Checks, "evidra-protocol/expected-risk-tags")
	if tagsCheck == nil {
		t.Fatal("risk tags check not found")
	}
	if tagsCheck.Verdict != VerdictPass {
		t.Fatalf("expected risk tags check to pass, got %s: %s", tagsCheck.Verdict, tagsCheck.Message)
	}
}

func TestEvidraProtocolCheck_EmptyEvidenceDir(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		MinPrescriptions: 1,
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	if result.Passed {
		t.Fatal("expected failure for empty evidence")
	}
}

func TestEvidraProtocolCheck_SignalCount(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{
		prescribeEntry("e1", "rx-1", "low", nil, ""),
		reportEntry("e2", "rx-1", "success"),
		signalEntry("e3", "artifact_drift"),
		signalEntry("e4", "artifact_drift"),
		signalEntry("e5", "thrashing"),
	})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		ExpectedSignals: map[string]int{
			"artifact_drift": 2,
			"thrashing":      1,
		},
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	driftCheck := findCheck(result.Checks, "evidra-protocol/expected-signal/artifact_drift")
	if driftCheck == nil {
		t.Fatal("artifact_drift signal check not found")
	}
	if driftCheck.Verdict != VerdictPass {
		t.Fatalf("expected artifact_drift check to pass, got %s: %s", driftCheck.Verdict, driftCheck.Message)
	}
	thrashCheck := findCheck(result.Checks, "evidra-protocol/expected-signal/thrashing")
	if thrashCheck == nil {
		t.Fatal("thrashing signal check not found")
	}
	if thrashCheck.Verdict != VerdictPass {
		t.Fatalf("expected thrashing check to pass, got %s: %s", thrashCheck.Verdict, thrashCheck.Message)
	}
}

func TestEvidraProtocolCheck_SignalCount_Insufficient(t *testing.T) {
	t.Parallel()
	dir := writeEvidenceDir(t, []string{
		signalEntry("e1", "repair_loop"),
	})
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		ExpectedSignals: map[string]int{
			"repair_loop": 2,
		},
	}, dir)
	result := RunChecks(context.Background(), "", checkers)
	repairCheck := findCheck(result.Checks, "evidra-protocol/expected-signal/repair_loop")
	if repairCheck == nil {
		t.Fatal("repair_loop signal check not found")
	}
	if repairCheck.Verdict != VerdictFail {
		t.Fatalf("expected repair_loop check to fail, got %s", repairCheck.Verdict)
	}
}

func TestBuildEvidraCheckers_ImplementsChecker(t *testing.T) {
	t.Parallel()
	max := 3
	checkers := BuildEvidraCheckers(EvidraCheckConfig{
		MinPrescriptions:      1,
		MinReports:            1,
		AllReportsHaveVerdict: true,
		ExpectedRiskLevel:     "low",
		ExpectedRiskTags:      []string{"test"},
		DeclinedMin:           1,
		DeclinedMax:           &max,
		RetryLoopMax:          5,
	}, t.TempDir())
	// 2 always-on (orphaned, violations) + 8 conditional = 10
	if len(checkers) != 10 {
		t.Fatalf("expected 10 checkers, got %d", len(checkers))
	}
	for _, c := range checkers {
		var _ Checker = c
	}
}
