// Package workspace provides isolated directories for parallel bench jobs.
package workspace

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// Workspace provides an isolated directory for a single bench job.
// Scenarios and supporting directories (manifests, charts) are copied
// so agent writes never touch the source repo.
type Workspace struct {
	Root         string
	ScenariosDir string
	RunsDir      string
	EvidenceDir  string
}

// New creates an isolated workspace for the given job ID.
// Copies srcScenariosDir and sibling directories (manifests/, charts/)
// into the workspace so relative paths (../../../manifests/) resolve correctly.
// The caller must call Cleanup when done to remove the temp directory.
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

	// Copy sibling directories that scenarios reference via ../../../ xxx paths.
	repoRoot := filepath.Dir(srcScenariosDir)
	for _, sibling := range []string{"manifests", "charts"} {
		srcPath := filepath.Join(repoRoot, sibling)
		if info, err := os.Stat(srcPath); err == nil && info.IsDir() {
			if err := copyDir(srcPath, filepath.Join(root, sibling)); err != nil {
				return nil, fmt.Errorf("workspace: copy %s: %w", sibling, err)
			}
		}
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
	if err := os.RemoveAll(ws.Root); err != nil {
		log.Printf("[workspace] cleanup warning: %v", err)
	}
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return fmt.Errorf("workspace: rel path: %w", relErr)
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		// Preserve file permissions (execute bit for shell scripts).
		info, infoErr := d.Info()
		if infoErr != nil {
			return fmt.Errorf("workspace: file info %s: %w", path, infoErr)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
