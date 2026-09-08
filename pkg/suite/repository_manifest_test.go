package suite

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRepositoryKubernetesDemoV1UsesValidatedThreeCaseContract(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(filepath.Join(root, "suites", "kubernetes-demo-v1.yaml"), root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"broken-deployment", "false-alarm", "wrong-namespace-workload-restart"}
	got := make([]string, 0, len(loaded.Scenarios))
	for _, s := range loaded.Scenarios {
		got = append(got, s.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cases = %v, want %v", got, want)
	}
	if loaded.Identity != "kubernetes-demo@1" || len(loaded.Manifest.Limitations) == 0 {
		t.Fatalf("identity/limitations = %q/%v", loaded.Identity, loaded.Manifest.Limitations)
	}
	planSuite := loaded.EvaluationSuite()
	if planSuite.ID != loaded.Identity || planSuite.Digest != loaded.Digest || len(planSuite.Cases) != 3 {
		t.Fatalf("evaluation suite = %+v", planSuite)
	}
}

func TestRepositoryDemoSmokeScriptsAreExecutableAndParse(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		"tests/fixtures/scripted-agent/good.sh",
		"tests/fixtures/scripted-agent/no-op.sh",
		"tests/fixtures/scripted-agent/failed-repair.sh",
		"tests/fixtures/scripted-agent/wrong-scope.sh",
		"tests/smoke/run_demo_suite_smoke.sh",
	}
	for _, rel := range paths {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s is not executable", rel)
		}
		if output, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
			t.Errorf("bash -n %s: %v: %s", rel, err, output)
		}
	}
}
