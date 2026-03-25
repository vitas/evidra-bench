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
		// Replace namespace references with word-boundary awareness.
		// Each pattern includes a trailing boundary (newline, space, quote, EOF)
		// to avoid matching "benchmark" when oldNS is "bench".
		replacements := []struct{ old, new string }{
			{"namespace: " + oldNS + "\n", "namespace: " + newNS + "\n"},
			{"namespace: " + oldNS + "\r", "namespace: " + newNS + "\r"},
			{"-n " + oldNS + " ", "-n " + newNS + " "},
			{"-n " + oldNS + "\n", "-n " + newNS + "\n"},
			{"-n " + oldNS + "\r", "-n " + newNS + "\r"},
			{"-n=" + oldNS + " ", "-n=" + newNS + " "},
			{"-n=" + oldNS + "\n", "-n=" + newNS + "\n"},
			{"namespace=" + oldNS + " ", "namespace=" + newNS + " "},
			{"namespace=" + oldNS + "\n", "namespace=" + newNS + "\n"},
			{"`" + oldNS + "`", "`" + newNS + "`"},
		}
		// Also handle end-of-file (no trailing newline).
		eofReplacements := []struct{ old, new string }{
			{"namespace: " + oldNS, "namespace: " + newNS},
			{"-n " + oldNS, "-n " + newNS},
			{"-n=" + oldNS, "-n=" + newNS},
			{"namespace=" + oldNS, "namespace=" + newNS},
		}
		modified := data
		for _, r := range replacements {
			modified = bytes.ReplaceAll(modified, []byte(r.old), []byte(r.new))
		}
		// Apply EOF replacements only if the file ends with the pattern.
		for _, r := range eofReplacements {
			if bytes.HasSuffix(modified, []byte(r.old)) {
				modified = append(modified[:len(modified)-len(r.old)], []byte(r.new)...)
			}
		}
		if !bytes.Equal(data, modified) {
			return os.WriteFile(path, modified, 0644)
		}
		return nil
	})
}
