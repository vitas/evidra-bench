package signalaudit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/signalaudit"
)

func TestLoadManifest_Valid(t *testing.T) {
	path := writeManifestFixture(t, `
broken-deployment:
  primary_signal: retry_loop
  expected_signals: [retry_loop]
  allowed_secondary_signals: []
  forbidden_signals: [blast_radius]
`)

	manifest, err := signalaudit.LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	entry, ok := manifest["broken-deployment"]
	if !ok {
		t.Fatalf("manifest missing broken-deployment entry")
	}
	if entry.PrimarySignal != "retry_loop" {
		t.Fatalf("primary_signal = %q, want retry_loop", entry.PrimarySignal)
	}
	if got := entry.ExpectedSignals; len(got) != 1 || got[0] != "retry_loop" {
		t.Fatalf("expected_signals = %v, want [retry_loop]", got)
	}
}

func TestLoadManifest_RejectsDuplicateScenarioIDs(t *testing.T) {
	path := writeManifestFixture(t, `
broken-deployment:
  primary_signal: retry_loop
broken-deployment:
  primary_signal: blast_radius
`)

	_, err := signalaudit.LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest succeeded, want duplicate scenario error")
	}
	if !strings.Contains(err.Error(), "duplicate scenario") {
		t.Fatalf("error = %q, want duplicate scenario", err)
	}
}

func TestLoadManifest_RejectsMissingPrimarySignal(t *testing.T) {
	path := writeManifestFixture(t, `
broken-deployment:
  expected_signals: [retry_loop]
`)

	_, err := signalaudit.LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest succeeded, want missing primary_signal error")
	}
	if !strings.Contains(err.Error(), "primary_signal") {
		t.Fatalf("error = %q, want primary_signal validation", err)
	}
}

func TestLoadManifest_RejectsPrimarySignalInForbiddenSignals(t *testing.T) {
	path := writeManifestFixture(t, `
broken-deployment:
  primary_signal: retry_loop
  expected_signals: [retry_loop]
  forbidden_signals: [retry_loop]
`)

	_, err := signalaudit.LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest succeeded, want forbidden primary signal error")
	}
	if !strings.Contains(err.Error(), "forbidden_signals") {
		t.Fatalf("error = %q, want forbidden_signals validation", err)
	}
}

func writeManifestFixture(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "signal-audit.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0644); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}
	return path
}
