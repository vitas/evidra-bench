package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/evaluation"
)

func TestOneCommandClusterNameFitsK3dLimit(t *testing.T) {
	name := oneCommandClusterName(2_147_483_647, time.Unix(0, 1_788_950_959_197_737_251))
	if len(name) > 32 {
		t.Fatalf("cluster name %q has %d characters, want at most 32", name, len(name))
	}
	if !strings.HasPrefix(name, "evidra-") {
		t.Fatalf("cluster name = %q, want evidra- prefix", name)
	}
}

func writeSuiteSentinel(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "suites", "kubernetes-demo-v1.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("schema: evidra-suite/v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveTestAssetsRootUsesExplicitThenEnvThenImageThenCWD(t *testing.T) {
	explicit := t.TempDir()
	envRoot := t.TempDir()
	imageRoot := t.TempDir()
	cwd := t.TempDir()
	for _, root := range []string{explicit, envRoot, imageRoot, cwd} {
		writeSuiteSentinel(t, root)
	}

	tests := []struct {
		name     string
		explicit string
		env      string
		image    string
		want     string
	}{
		{name: "explicit", explicit: explicit, env: envRoot, image: imageRoot, want: explicit},
		{name: "environment", env: envRoot, image: imageRoot, want: envRoot},
		{name: "image", image: imageRoot, want: imageRoot},
		{name: "working directory", image: filepath.Join(t.TempDir(), "missing"), want: cwd},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTestAssetsRoot(tc.explicit, func(name string) string {
				if name == "EVIDRA_ASSETS_DIR" {
					return tc.env
				}
				return ""
			}, cwd, tc.image)
			if err != nil {
				t.Fatalf("resolveTestAssetsRoot() error = %v", err)
			}
			want, _ := filepath.Abs(tc.want)
			if got != want {
				t.Fatalf("root = %q, want %q", got, want)
			}
		})
	}
}

func TestResolveTestAssetsRootRejectsInvalidExplicitRoot(t *testing.T) {
	_, err := resolveTestAssetsRoot(filepath.Join(t.TempDir(), "missing"), func(string) string { return "" }, t.TempDir(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "suite assets") {
		t.Fatalf("error = %v, want missing suite assets", err)
	}
}

func TestPrepareTestEvaluationBuildsCanonicalCredentialFreePlan(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareTestEvaluation(context.Background(), testRequest{
		Model:       "openai/gpt-test",
		Suite:       "kubernetes-demo@1",
		Environment: "kind",
		OutputDir:   t.TempDir(),
		ProjectRoot: root,
		Timeout:     defaultTestCaseTimeout,
	}, func(name string) string {
		if name == "OPENAI_API_KEY" {
			return "super-secret"
		}
		return ""
	}, testRuntimeDeps{})
	if err != nil {
		t.Fatalf("prepareTestEvaluation() error = %v", err)
	}
	if prepared.Plan.Target.Kind != evaluation.TargetModel || prepared.Plan.Target.Provider != "openai" {
		t.Fatalf("target = %#v", prepared.Plan.Target)
	}
	if len(prepared.Plan.Suite.Cases) != 3 || prepared.Plan.Environment.Provider != "kind" {
		t.Fatalf("plan = %#v", prepared.Plan)
	}
	encoded, err := json.Marshal(prepared.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-secret") || strings.Contains(string(encoded), "api.openai.com") {
		t.Fatalf("plan leaked runtime provider configuration: %s", encoded)
	}
}

func TestPrepareTestEvaluationRejectsUnsupportedEnvironmentBeforeProvisioning(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	_, err := prepareTestEvaluation(context.Background(), testRequest{
		Agent:       "./agent",
		Suite:       "kubernetes-demo@1",
		Environment: "minikube",
		OutputDir:   t.TempDir(),
		ProjectRoot: root,
		Timeout:     defaultTestCaseTimeout,
	}, func(string) string { return "" }, testRuntimeDeps{})
	if err == nil || !strings.Contains(err.Error(), "kind or k3d") {
		t.Fatalf("error = %v", err)
	}
}
