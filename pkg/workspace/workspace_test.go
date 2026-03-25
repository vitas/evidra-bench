package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew_CreatesDirectories(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "kubernetes", "broken-deployment", "fixtures"), 0755); err != nil {
		t.Fatalf("mkdir fixtures: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "kubernetes", "broken-deployment", "scenario.yaml"), []byte("id: broken-deployment"), 0644); err != nil {
		t.Fatalf("write scenario: %v", err)
	}

	ws, err := New("test-job-1", srcDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer ws.Cleanup()

	if _, err := os.Stat(ws.ScenariosDir); err != nil {
		t.Errorf("scenarios dir not created: %v", err)
	}
	if _, err := os.Stat(ws.RunsDir); err != nil {
		t.Errorf("runs dir not created: %v", err)
	}
	if _, err := os.Stat(ws.EvidenceDir); err != nil {
		t.Errorf("evidence dir not created: %v", err)
	}
	copied := filepath.Join(ws.ScenariosDir, "kubernetes", "broken-deployment", "scenario.yaml")
	if _, err := os.Stat(copied); err != nil {
		t.Errorf("scenario file not copied: %v", err)
	}
}

func TestNew_IsolatesWrites(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "test.yaml"), []byte("original"), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	ws, err := New("test-job-2", srcDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer ws.Cleanup()

	if err := os.WriteFile(filepath.Join(ws.ScenariosDir, "test.yaml"), []byte("modified"), 0644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(srcDir, "test.yaml"))
	if string(data) != "original" {
		t.Errorf("source was modified: %s", data)
	}
}

func TestCleanup_RemovesWorkspace(t *testing.T) {
	t.Parallel()
	srcDir := t.TempDir()
	ws, err := New("test-job-3", srcDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	root := ws.Root
	ws.Cleanup()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("workspace not cleaned up: %v", err)
	}
}
