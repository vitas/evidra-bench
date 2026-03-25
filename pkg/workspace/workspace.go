// Package workspace provides isolated directories for parallel bench jobs.
package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Workspace provides an isolated directory for a single bench job.
// Scenarios are copied so agent writes never touch the source repo.
type Workspace struct {
	Root         string
	ScenariosDir string
	RunsDir      string
	EvidenceDir  string
}

// New creates an isolated workspace for the given job ID.
// Copies srcScenariosDir into the workspace so it's writable.
func New(jobID string, srcScenariosDir string) (*Workspace, error) {
	root := filepath.Join(os.TempDir(), "bench-jobs", jobID)
	ws := &Workspace{
		Root:         root,
		ScenariosDir: filepath.Join(root, "scenarios"),
		RunsDir:      filepath.Join(root, "runs"),
		EvidenceDir:  filepath.Join(root, "evidence"),
	}

	if err := copyDir(srcScenariosDir, ws.ScenariosDir); err != nil {
		return nil, fmt.Errorf("workspace: copy scenarios: %w", err)
	}
	if err := os.MkdirAll(ws.RunsDir, 0755); err != nil {
		return nil, fmt.Errorf("workspace: mkdir runs: %w", err)
	}
	if err := os.MkdirAll(ws.EvidenceDir, 0755); err != nil {
		return nil, fmt.Errorf("workspace: mkdir evidence: %w", err)
	}
	return ws, nil
}

// Cleanup removes the entire workspace directory.
func (ws *Workspace) Cleanup() {
	os.RemoveAll(ws.Root)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}
