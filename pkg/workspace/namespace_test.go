package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteNamespace_YAMLFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := filepath.Join(dir, "deployment.yaml")
	os.WriteFile(fixture, []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: bench\n---\napiVersion: apps/v1\nmetadata:\n  namespace: bench\n"), 0644)

	if err := RewriteNamespace(dir, "bench", "bench-w3"); err != nil {
		t.Fatalf("RewriteNamespace: %v", err)
	}

	data, _ := os.ReadFile(fixture)
	content := string(data)
	if !strings.Contains(content, "namespace: bench-w3") {
		t.Errorf("expected 'namespace: bench-w3' in output, got:\n%s", content)
	}
	if strings.Contains(content, "namespace: bench\n") {
		t.Errorf("old namespace still present")
	}
}

func TestRewriteNamespace_ShellFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	script := filepath.Join(dir, "verify.sh")
	os.WriteFile(script, []byte("kubectl get pods -n bench\nkubectl -n=bench describe pod"), 0644)

	if err := RewriteNamespace(dir, "bench", "bench-w1"); err != nil {
		t.Fatalf("RewriteNamespace: %v", err)
	}

	data, _ := os.ReadFile(script)
	content := string(data)
	if !strings.Contains(content, "-n bench-w1") {
		t.Errorf("expected '-n bench-w1', got:\n%s", content)
	}
	if !strings.Contains(content, "-n=bench-w1") {
		t.Errorf("expected '-n=bench-w1', got:\n%s", content)
	}
}

func TestRewriteNamespace_SkipsNonTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	txt := filepath.Join(dir, "data.json")
	os.WriteFile(txt, []byte(`{"namespace": "bench"}`), 0644)

	if err := RewriteNamespace(dir, "bench", "bench-w0"); err != nil {
		t.Fatalf("RewriteNamespace: %v", err)
	}

	data, _ := os.ReadFile(txt)
	if string(data) != `{"namespace": "bench"}` {
		t.Errorf("non-target file was modified: %s", data)
	}
}

func TestRewriteNamespace_TaskPrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "prompts"), 0755)
	prompt := filepath.Join(dir, "prompts", "task.md")
	os.WriteFile(prompt, []byte("Fix the app in the `bench` namespace."), 0644)

	if err := RewriteNamespace(dir, "bench", "bench-w1"); err != nil {
		t.Fatalf("RewriteNamespace: %v", err)
	}

	data, _ := os.ReadFile(prompt)
	if !strings.Contains(string(data), "`bench-w1`") {
		t.Errorf("expected backtick-quoted namespace rewrite, got:\n%s", string(data))
	}
}
