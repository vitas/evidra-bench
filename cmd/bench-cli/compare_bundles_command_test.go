package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/benchexport"
	"github.com/vitas/evidra-bench/pkg/evidrawire"
)

func exportFixtureBundle(t *testing.T, dir string, semantics string) string {
	t.Helper()
	meta := map[string]string{"model": "fixture"}
	if semantics != "" {
		meta["semantics_version"] = semantics
	}
	runDir := filepath.Join(t.TempDir(), "run-"+filepath.Base(dir))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := artifact.RunBundle{RunID: "r-" + filepath.Base(dir), ScenarioID: "s", Verdict: "pass",
		ExitCode: 0, Passed: true, Metadata: meta}
	data, _ := json.MarshalIndent(doc, "", "  ")
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := benchexport.Export(benchexport.Request{RunDir: runDir, OutDir: dir, ProducerVersion: "test"}); err != nil {
		t.Fatalf("export fixture: %v", err)
	}
	return dir
}

func runCompare(t *testing.T, dirs ...string) (string, error) {
	t.Helper()
	cmd := newCompareBundlesCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(dirs)
	err := cmd.Execute()
	return buf.String(), err
}

func TestCompareBundlesCohortGates(t *testing.T) {
	base := t.TempDir()
	a := exportFixtureBundle(t, filepath.Join(base, "a"), evidrawire.CohortSafetyEvidence)
	b := exportFixtureBundle(t, filepath.Join(base, "b"), evidrawire.CohortSafetyEvidence)
	legacy := exportFixtureBundle(t, filepath.Join(base, "c"), "") // unstamped = legacy

	out, err := runCompare(t, a, b)
	if err != nil {
		t.Fatalf("same modern cohort must compare: %v\n%s", err, out)
	}
	if !strings.Contains(out, "comparable") {
		t.Fatalf("want comparable line: %s", out)
	}
	out, err = runCompare(t, a, legacy)
	if !errors.Is(err, evidrawire.ErrMixedCohorts) {
		t.Fatalf("mixed must reject: %v\n%s", err, out)
	}
	out, err = runCompare(t, legacy, filepath.Join(base, "c"))
	_ = out
	if !errors.Is(err, evidrawire.ErrNotComparable) || !strings.Contains(err.Error(), "readable") {
		t.Fatalf("legacy-vs-legacy still not comparable: %v\n%s", err, out)
	}
	if !strings.Contains(out, "readable") && !strings.Contains(out, "notice") {
		t.Fatalf("legacy participation must print the notice: %s", out)
	}
}
