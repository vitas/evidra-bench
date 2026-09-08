package suite

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadResolvesCasesInManifestOrder(t *testing.T) {
	root := newTestProject(t)
	writeTestScenario(t, root, "kubernetes/repair", "repair", "repair prompt")
	writeTestScenario(t, root, "kubernetes/scope", "scope", "scope prompt")
	manifestPath := writeTestManifest(t, root, `schema: evidra-suite/v1
id: kubernetes-demo
revision: 1
environment:
  profile: default
  providers: [kind, k3d]
cases:
  - kubernetes/scope
  - kubernetes/repair
includes:
  - manifests/core
limitations:
  - Not a production readiness certification.
`)

	loaded, err := Load(manifestPath, root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Identity != "kubernetes-demo@1" {
		t.Fatalf("Identity = %q", loaded.Identity)
	}
	gotIDs := []string{loaded.Scenarios[0].ID, loaded.Scenarios[1].ID}
	if !reflect.DeepEqual(gotIDs, []string{"scope", "repair"}) {
		t.Fatalf("scenario order = %v", gotIDs)
	}
	if !strings.HasPrefix(loaded.Digest, "sha256:") {
		t.Fatalf("Digest = %q", loaded.Digest)
	}
}

func TestLoadDigestChangesWhenExecutableInputChanges(t *testing.T) {
	root := newTestProject(t)
	writeTestScenario(t, root, "kubernetes/repair", "repair", "first prompt")
	manifestPath := writeTestManifest(t, root, validTestManifest("kubernetes/repair"))

	first, err := Load(manifestPath, root)
	if err != nil {
		t.Fatal(err)
	}
	promptPath := filepath.Join(root, "scenarios", "kubernetes", "repair", "prompts", "task.md")
	if err := os.WriteFile(promptPath, []byte("changed prompt"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Load(manifestPath, root)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest == second.Digest {
		t.Fatalf("digest did not change after prompt mutation: %s", first.Digest)
	}

	sharedPath := filepath.Join(root, "manifests", "core", "namespace.yaml")
	if err := os.WriteFile(sharedPath, []byte("changed shared manifest"), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := Load(manifestPath, root)
	if err != nil {
		t.Fatal(err)
	}
	if second.Digest == third.Digest {
		t.Fatalf("digest did not change after shared asset mutation: %s", second.Digest)
	}
}

func TestLoadRejectsDuplicateAndMissingCases(t *testing.T) {
	root := newTestProject(t)
	writeTestScenario(t, root, "kubernetes/repair", "repair", "repair prompt")

	duplicate := writeTestManifest(t, root, validTestManifest("kubernetes/repair", "kubernetes/repair"))
	if _, err := Load(duplicate, root); err == nil || !strings.Contains(err.Error(), "duplicate case") {
		t.Fatalf("duplicate Load() error = %v", err)
	}

	duplicateAlias := writeTestManifest(t, root, validTestManifest("kubernetes/repair", "repair"))
	if _, err := Load(duplicateAlias, root); err == nil || !strings.Contains(err.Error(), "duplicate scenario id") {
		t.Fatalf("duplicate alias Load() error = %v", err)
	}

	missing := writeTestManifest(t, root, validTestManifest("kubernetes/missing"))
	if _, err := Load(missing, root); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing Load() error = %v", err)
	}
}

func TestLoadRejectsTraversalAndOutOfTreeSymlinks(t *testing.T) {
	root := newTestProject(t)
	writeTestScenario(t, root, "kubernetes/repair", "repair", "repair prompt")

	traversal := writeTestManifest(t, root, strings.Replace(validTestManifest("kubernetes/repair"), "manifests/core", "../outside", 1))
	if _, err := Load(traversal, root); err == nil || !strings.Contains(err.Error(), "outside project root") {
		t.Fatalf("traversal Load() error = %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.yaml")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "manifests", "core", "linked.yaml")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	symlinkManifest := writeTestManifest(t, root, validTestManifest("kubernetes/repair"))
	if _, err := Load(symlinkManifest, root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink Load() error = %v", err)
	}
}

func TestLoadRequiresCompatibleProfileAndProviders(t *testing.T) {
	root := newTestProject(t)
	writeTestScenario(t, root, "kubernetes/repair", "repair", "repair prompt")

	badProfile := writeTestManifest(t, root, strings.Replace(validTestManifest("kubernetes/repair"), "profile: default", "profile: unknown", 1))
	if _, err := Load(badProfile, root); err == nil || !strings.Contains(err.Error(), "profile") {
		t.Fatalf("profile Load() error = %v", err)
	}

	badProvider := writeTestManifest(t, root, strings.Replace(validTestManifest("kubernetes/repair"), "providers: [kind, k3d]", "providers: [kind, minikube]", 1))
	if _, err := Load(badProvider, root); err == nil || !strings.Contains(err.Error(), "provider") {
		t.Fatalf("provider Load() error = %v", err)
	}
}

func newTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "manifests", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifests", "core", "namespace.yaml"), []byte("kind: Namespace\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeTestScenario(t *testing.T, root, ref, id, prompt string) {
	t.Helper()
	dir := filepath.Join(root, "scenarios", filepath.FromSlash(ref))
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: " + id + `
title: Test case
category: kubernetes
prompt: prompts/task.md
break:
  type: kubectl
  command: get pods
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestManifest(t *testing.T, root, contents string) string {
	t.Helper()
	dir := filepath.Join(root, "suites")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func validTestManifest(cases ...string) string {
	return `schema: evidra-suite/v1
id: test-suite
revision: 1
environment:
  profile: default
  providers: [kind, k3d]
cases:
  - ` + strings.Join(cases, "\n  - ") + `
includes:
  - manifests/core
`
}
