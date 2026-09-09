package benchexport

import (
	"encoding/json"
	"os"
	"path/filepath"
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
