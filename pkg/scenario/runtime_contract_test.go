package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImplementedScenarios_RuntimeContracts(t *testing.T) {
	t.Parallel()

	root := runtimeProjectRoot()
	scenariosDir := filepath.Join(root, "scenarios")
	scenarios, err := LoadAll(scenariosDir)
	if err != nil {
		t.Fatalf("load scenarios: %v", err)
	}

	for _, s := range scenarios {
		s := s
		t.Run(s.Path, func(t *testing.T) {
			t.Parallel()
			if err := validateRuntimeContract(s); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateRuntimeContract_ChaosKubectlApply(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "fixtures"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix it."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixtures", "baseline.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: bench
spec:
  template:
    spec:
      containers:
        - name: web
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: bench
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixtures", "broken.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: bench
spec:
  template:
    spec:
      containers:
        - name: web
          image: broken:v1
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixtures", "chaos.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: bench
spec:
  template:
    spec:
      containers:
        - name: web
          image: noisy:v2
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(`id: chaos-runtime
title: Chaos runtime contract
category: kubernetes
prompt: prompts/task.md
bootstrap:
  - type: kubectl-apply
    path: fixtures/baseline.yaml
break:
  type: apply
  path: fixtures/broken.yaml
chaos:
  steps:
    - at: 20s
      name: mutate-web
      type: kubectl-apply
      path: fixtures/chaos.yaml
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	if got := s.Chaos.Steps[0].Path; !filepath.IsAbs(got) {
		t.Fatalf("chaos path not resolved: %s", got)
	}
	if err := validateRuntimeContract(s); err != nil {
		t.Fatalf("validate runtime contract: %v", err)
	}
}

func TestValidateRuntimeContract_ChaosUnsupportedType(t *testing.T) {
	t.Parallel()

	s := &Scenario{
		ID:       "chaos-unsupported",
		Title:    "Chaos unsupported",
		Category: "kubernetes",
		Prompt:   "ignored",
		Bootstrap: []BootstrapStep{
			{Name: "baseline", Type: "kubectl-apply", Path: filepath.Join(runtimeProjectRoot(), "manifests", "baseline", "deployment.yaml")},
		},
		Break: Break{
			Type: "apply",
			Path: filepath.Join(runtimeProjectRoot(), "scenarios", "kubernetes", "broken-deployment", "fixtures", "broken.yaml"),
		},
		Chaos: ChaosConfig{
			Steps: []ChaosStep{
				{
					Name: "unsupported",
					Type: "explode",
					At:   Duration{Duration: time.Second, Set: true},
				},
			},
		},
		Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
	}

	err := validateRuntimeContract(s)
	if err == nil {
		t.Fatal("expected unsupported chaos step type to fail")
	}
	if !strings.Contains(err.Error(), `unsupported step type "explode"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
