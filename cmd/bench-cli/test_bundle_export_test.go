package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/evidrawire"
)

func writeTestRunArtifact(t *testing.T, baseDir, runID string) {
	t.Helper()
	w := artifact.NewWriter(baseDir)
	start := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	if _, err := w.Write(artifact.RunBundle{
		RunID:      runID,
		ScenarioID: "kubernetes/broken-deployment",
		Adapter:    "cli",
		StartTime:  start,
		EndTime:    start.Add(2 * time.Minute),
		ExitCode:   0,
		Passed:     true,
		Prompt:     "restore readiness",
	}); err != nil {
		t.Fatalf("write artifact %s: %v", runID, err)
	}
}

func TestExportEvaluationBundlesCreatesAndSkips(t *testing.T) {
	out := t.TempDir()
	runsDir := filepath.Join(out, "runs")
	writeTestRunArtifact(t, runsDir, "01BUNDLETEST00000000000A1")
	writeTestRunArtifact(t, runsDir, "01BUNDLETEST00000000000A2")

	exported, err := exportEvaluationBundles(out, "test")
	if err != nil {
		t.Fatalf("first export: %v", err)
	}
	if exported != 2 {
		t.Fatalf("exported = %d, want 2", exported)
	}
	for _, runID := range []string{"01BUNDLETEST00000000000A1", "01BUNDLETEST00000000000A2"} {
		if _, err := evidrawire.VerifyBundle(filepath.Join(out, "bundles", runID)); err != nil {
			t.Fatalf("bundle %s does not verify: %v", runID, err)
		}
	}

	// Idempotent rerun: nothing rewritten, count stable.
	exported, err = exportEvaluationBundles(out, "test")
	if err != nil {
		t.Fatalf("rerun export: %v", err)
	}
	if exported != 2 {
		t.Fatalf("rerun exported = %d, want 2", exported)
	}
}

func TestExportEvaluationBundlesNoRunsDir(t *testing.T) {
	exported, err := exportEvaluationBundles(t.TempDir(), "test")
	if exported != 0 || err != nil {
		t.Fatalf("missing runs dir must be a no-op, got (%d, %v)", exported, err)
	}
}

func TestExportEvaluationBundlesReportsPartialFailures(t *testing.T) {
	out := t.TempDir()
	runsDir := filepath.Join(out, "runs")
	writeTestRunArtifact(t, runsDir, "01BUNDLETEST00000000000B1")

	// The destination for run "broken" is blocked by a plain file, so bundle
	// creation must surface as a joined failure while the healthy run still
	// exports.
	if err := os.MkdirAll(filepath.Join(out, "bundles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "bundles", "broken"), []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	badDir := filepath.Join(runsDir, "broken")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, artifact.RunJSON), []byte(`{"run_id":"broken"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	exported, err := exportEvaluationBundles(out, "test")
	if err == nil {
		t.Fatal("expected joined failure for the blocked run")
	}
	if exported != 1 {
		t.Fatalf("exported = %d, want 1 successful bundle", exported)
	}
	if _, err := evidrawire.VerifyBundle(filepath.Join(out, "bundles", "01BUNDLETEST00000000000B1")); err != nil {
		t.Fatalf("healthy bundle missing or invalid: %v", err)
	}
}

func TestSafeBundleDirName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"01ABC_def-1.2", "01ABC_def-1.2"},
		{"a/b\\c", "a-b-c"},
		{"..", "run"},
		{"", "run"},
		{"  ", "--"},
	} {
		if got := safeBundleDirName(tc.in); got != tc.want {
			t.Errorf("safeBundleDirName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
