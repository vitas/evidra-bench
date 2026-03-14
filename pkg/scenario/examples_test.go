package scenario

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func TestStarterScenarios_Load(t *testing.T) {
	t.Parallel()
	root := projectRoot()
	dirs := []string{
		// Phase 1
		"scenarios/kubernetes/broken-deployment",
		"scenarios/helm/failed-upgrade",
		"scenarios/argocd/out-of-sync",
		// Phase 2
		"scenarios/kubernetes/crashloop-backoff",
		"scenarios/kubernetes/wrong-service-selector",
		"scenarios/kubernetes/missing-configmap",
		"scenarios/kubernetes/missing-secret",
		// Phase 3
		"scenarios/kubernetes/resource-quota-exceeded",
		"scenarios/kubernetes/wrong-probes",
		"scenarios/helm/pending-release",
		"scenarios/argocd/sync-failure",
		// Phase 4
		"scenarios/kubernetes/networkpolicy-blocking",
		"scenarios/kubernetes/wrong-pvc",
		"scenarios/helm/version-rollback",
		"scenarios/helm/dependency-conflict",
		"scenarios/argocd/degraded-after-sync",
		"scenarios/argocd/sync-wave-ordering",
	}
	for _, dir := range dirs {
		dir := dir
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			fullPath := filepath.Join(root, dir)
			if _, err := os.Stat(filepath.Join(fullPath, "scenario.yaml")); err != nil {
				t.Skipf("scenario not found: %s", fullPath)
			}
			s, err := Load(fullPath)
			if err != nil {
				t.Fatalf("load %s: %v", dir, err)
			}
			if s.ID == "" {
				t.Fatalf("missing id in %s", dir)
			}
			if s.Title == "" {
				t.Fatalf("missing title in %s", dir)
			}
			if len(s.Checks) == 0 {
				t.Fatalf("no checks in %s", dir)
			}
		})
	}
}

func TestStarterScenarios_LoadAll(t *testing.T) {
	t.Parallel()
	root := projectRoot()
	scenariosDir := filepath.Join(root, "scenarios")
	if _, err := os.Stat(scenariosDir); err != nil {
		t.Skip("scenarios dir not found")
	}
	scenarios, err := LoadAll(scenariosDir)
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(scenarios) < 17 {
		t.Fatalf("expected at least 17 scenarios, got %d", len(scenarios))
	}
}
