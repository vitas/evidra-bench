package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// RewriteNamespace replaces all occurrences of oldNS with newNS in YAML and
// shell files under dir. This enables parallel execution where each worker
// uses its own namespace.
func RewriteNamespace(dir string, oldNS, newNS string) error {
	extensions := map[string]bool{
		".yaml": true, ".yml": true, ".sh": true, ".md": true,
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !extensions[ext] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		replacements := []struct{ old, new string }{
			{"namespace: " + oldNS, "namespace: " + newNS},
			{"-n " + oldNS, "-n " + newNS},
			{"-n=" + oldNS, "-n=" + newNS},
			{"namespace=" + oldNS, "namespace=" + newNS},
			{"`" + oldNS + "`", "`" + newNS + "`"},
		}
		modified := data
		for _, r := range replacements {
			modified = bytes.ReplaceAll(modified, []byte(r.old), []byte(r.new))
		}
		if !bytes.Equal(data, modified) {
			return os.WriteFile(path, modified, 0644)
		}
		return nil
	})
}
