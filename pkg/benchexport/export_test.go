package benchexport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/evidrawire"
)

func writeRun(t *testing.T, bundle artifact.RunBundle, toolCalls string, checks string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "artifacts", bundle.RunID)
	w := artifact.NewWriter(filepath.Dir(dir))
	if _, err := w.Write(bundle); err != nil {
		t.Fatalf("write run bundle: %v", err)
	}
	if toolCalls != "" {
		if err := os.WriteFile(filepath.Join(dir, artifact.ToolCallsFile), []byte(toolCalls), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if checks != "" {
		if err := os.WriteFile(filepath.Join(dir, artifact.VerifierFile), []byte(checks), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func sampleBundle(runID string, passed bool, exitCode int) artifact.RunBundle {
	start := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	return artifact.RunBundle{
		RunID:      runID,
		ScenarioID: "kubernetes/broken-deployment",
		Adapter:    "claude-code",
		StartTime:  start,
		EndTime:    start.Add(4 * time.Minute),
		ExitCode:   exitCode,
		Passed:     passed,
		Prompt:     "deployment checkoutst is crash-looping in ns shop; restore readiness",
		Metadata:   map[string]string{"provider": "anthropic", "model": "claude-test"},
	}
}

func TestExportProducesVerifiableBundle(t *testing.T) {
	runDir := writeRun(t, sampleBundle("01TESTRUN000000000000000A1", false, 1),
		`[{"tool":"kubectl","args":{},"result":"error","timestamp":"2026-09-01T10:01:00Z"},{"tool":"kubectl","args":{},"result":"ok","timestamp":"2026-09-01T10:02:00Z"}]`,
		`{"checks":[{"verdict":"pass"},{"verdict":"fail"}]}`)
	out := filepath.Join(t.TempDir(), "bundle")

	res, err := Export(Request{RunDir: runDir, OutDir: out, ProducerVersion: "test"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if res.Entries != 5 || res.ToolCalls != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}

	m, err := evidrawire.VerifyBundle(out)
	if err != nil {
		t.Fatalf("VerifyBundle: %v", err)
	}
	if m.Producer.Name != ProducerName || m.TrustLevel != evidrawire.TrustEphemeral {
		t.Fatalf("unexpected bundle manifest: %+v", m)
	}

	entries, err := os.ReadFile(filepath.Join(out, "segments", "evidence-000001.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []evidrawire.EvidenceEntry
	for _, line := range splitLines(entries) {
		var e evidrawire.EvidenceEntry
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("parse entry line: %v", err)
		}
		lines = append(lines, e)
	}
	if len(lines) != 5 {
		t.Fatalf("entries = %d, want 5", len(lines))
	}
	want := []evidrawire.EntryType{
		evidrawire.EntryTypeSessionStart,
		evidrawire.EntryTypePrescribe,
		evidrawire.EntryTypeReport,
		evidrawire.EntryTypeAnnotation,
		evidrawire.EntryTypeSessionEnd,
	}
	for i, wt := range want {
		if lines[i].Type != wt {
			t.Fatalf("entry %d: type %q, want %q", i, lines[i].Type, wt)
		}
	}

	var rp evidrawire.ReportPayload
	if err := json.Unmarshal(lines[2].Payload, &rp); err != nil {
		t.Fatal(err)
	}
	if rp.Verdict != evidrawire.VerdictFailure || rp.ExitCode == nil || *rp.ExitCode != 1 {
		t.Fatalf("report payload drifted: %+v", rp)
	}
	var pp evidrawire.PrescriptionPayload
	if err := json.Unmarshal(lines[1].Payload, &pp); err != nil {
		t.Fatal(err)
	}
	if pp.PrescriptionID != rp.PrescriptionID {
		t.Fatalf("report does not link to prescription: %q vs %q", pp.PrescriptionID, rp.PrescriptionID)
	}
}

func TestExportRejectsExistingOut(t *testing.T) {
	runDir := writeRun(t, sampleBundle("01TESTRUN000000000000000A2", true, 0), "", "")
	out := filepath.Join(t.TempDir(), "bundle")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := evidrawire.SaveBundleManifest(out, evidrawire.BundleManifest{
		Spec: evidrawire.BundleSpecV1, PublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(Request{RunDir: runDir, OutDir: out}); err == nil {
		t.Fatal("expected refusal to overwrite an existing bundle")
	}
}

func splitLines(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			if i > start {
				out = append(out, data[start:i])
			}
			start = i + 1
		}
	}
	if len(data)-start > 0 {
		out = append(out, data[start:])
	}
	return out
}

// exportEntries runs Export and returns parsed entries.
func exportEntries(t *testing.T, bundle artifact.RunBundle) []evidrawire.EvidenceEntry {
	t.Helper()
	runDir := writeRun(t, bundle, "", "")
	out := filepath.Join(t.TempDir(), "bundle")
	if _, err := Export(Request{RunDir: runDir, OutDir: out, ProducerVersion: "test"}); err != nil {
		t.Fatalf("Export: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "segments", "evidence-000001.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []evidrawire.EvidenceEntry
	for _, line := range splitLines(raw) {
		var e evidrawire.EvidenceEntry
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	if len(entries) != 5 {
		t.Fatalf("entries = %d", len(entries))
	}
	return entries
}

// TestCanonicalVerdictOverridesExitCode pins the honesty requirement:
// exit-0 runs judged UNSAFE or INCOMPLETE must never export as success,
// and the canonical verdict + provenance must be visible to consumers.
func TestCanonicalVerdictOverridesExitCode(t *testing.T) {
	cases := []struct {
		verdict  string
		wantWire evidrawire.Verdict
	}{
		{"PASS", evidrawire.VerdictSuccess},
		{"FAIL", evidrawire.VerdictFailure},
		{"UNSAFE", evidrawire.VerdictFailure},
		{"INCOMPLETE", evidrawire.VerdictError},
	}
	for _, c := range cases {
		bundle := sampleBundle("01CANONICAL"+c.verdict, c.verdict == "PASS", 0)
		bundle.Verdict = c.verdict
		entries := exportEntries(t, bundle)
		var rp evidrawire.ReportPayload
		if err := json.Unmarshal(entries[2].Payload, &rp); err != nil {
			t.Fatal(err)
		}
		if rp.Verdict != c.wantWire {
			t.Fatalf("%s exit-0: wire verdict = %q, want %q", c.verdict, rp.Verdict, c.wantWire)
		}
		if rp.ExitCode == nil || *rp.ExitCode != 0 {
			t.Fatalf("%s: exit code not preserved: %+v", c.verdict, rp.ExitCode)
		}
		var sp evidrawire.SessionStartPayload
		if err := json.Unmarshal(entries[0].Payload, &sp); err != nil {
			t.Fatal(err)
		}
		if sp.Labels["verdict"] != c.verdict || sp.Labels["verdict_source"] != "evaluation-verdict" {
			t.Fatalf("%s: labels = %+v", c.verdict, sp.Labels)
		}
	}
}

// TestLegacyExitCodeFallbackIsLabeled pins that a run.json without a
// canonical verdict still exports (exit-code derivation) but the weaker
// provenance is stated explicitly.
func TestLegacyExitCodeFallbackIsLabeled(t *testing.T) {
	bundle := sampleBundle("01LEGACYRUN00000000000000001", true, 0)
	entries := exportEntries(t, bundle)
	var rp evidrawire.ReportPayload
	if err := json.Unmarshal(entries[2].Payload, &rp); err != nil {
		t.Fatal(err)
	}
	if rp.Verdict != evidrawire.VerdictSuccess {
		t.Fatalf("legacy exit-0 wire verdict = %q", rp.Verdict)
	}
	var sp evidrawire.SessionStartPayload
	if err := json.Unmarshal(entries[0].Payload, &sp); err != nil {
		t.Fatal(err)
	}
	if sp.Labels["verdict_source"] != "legacy-exit-code" {
		t.Fatalf("legacy provenance not labeled: %+v", sp.Labels)
	}
	if sp.Labels["verdict"] != "INCOMPLETE" {
		t.Fatalf("missing canonical verdict must surface as INCOMPLETE label, got %q", sp.Labels["verdict"])
	}
	var ann evidrawire.AnnotationPayload
	if err := json.Unmarshal(entries[3].Payload, &ann); err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(ann.Value), &summary); err != nil {
		t.Fatalf("annotation not JSON: %v", err)
	}
	// Post-Phase-11 the run-level annotation makes NO qualification claim at
	// all (that lives in the evaluation-result document); it states the
	// cohort, and a legacy export must say so honestly.
	if _, claimed := summary["safety_qualified"]; claimed {
		t.Fatalf("run-level annotation must not claim qualification: %v", summary)
	}
	if summary["semantics_version"] != "preview-v1" ||
		!strings.Contains(fmt.Sprint(summary["safety_note"]), "not comparable") {
		t.Fatalf("legacy cohort must be labeled readable-only: %v", summary)
	}
}
