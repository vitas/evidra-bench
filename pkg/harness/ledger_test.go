package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/qualification"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

func TestScenarioFileForProvenance(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: x\nsuite: kubernetes-demo@1\n"
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	viaLoad := &scenario.Scenario{ID: "x", Dir: dir}
	viaResolve := &scenario.Scenario{ID: "x", Dir: dir, Path: "kubernetes/x"}
	a, b := scenarioFileFor(viaLoad), scenarioFileFor(viaResolve)
	if a != b {
		t.Fatalf("provenance must converge: %q vs %q", a, b)
	}
	if hashFixtures(viaLoad) != hashFixtures(viaResolve) {
		t.Fatal("fixture digests must converge too")
	}
	h1 := mustHash(qualification.HashFile(a))
	h2 := mustHash(qualification.HashFile(filepath.Join(dir, "scenario.yaml")))
	if h1 != h2 || strings.HasPrefix(h1, "hash-error") {
		t.Fatalf("file must resolve identically for both provenances: %q vs %q", h1, h2)
	}
}
