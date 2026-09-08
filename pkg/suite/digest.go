package suite

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func digestInputs(projectRoot, manifestPath string, loaded *Loaded) (string, error) {
	h := sha256.New()
	if err := hashPath(h, projectRoot, manifestPath, "manifest"); err != nil {
		return "", err
	}
	for i, s := range loaded.Scenarios {
		label := fmt.Sprintf("case:%06d:%s", i, loaded.Manifest.Cases[i])
		if err := hashPath(h, projectRoot, s.Dir, label); err != nil {
			return "", err
		}
	}
	for i, include := range loaded.Manifest.Includes {
		path, err := secureProjectPath(projectRoot, include)
		if err != nil {
			return "", err
		}
		if err := hashPath(h, projectRoot, path, fmt.Sprintf("include:%06d:%s", i, include)); err != nil {
			return "", err
		}
	}
	digest := h.Sum(nil)
	return "sha256:" + hex.EncodeToString(digest), nil
}

func hashPath(h hash.Hash, projectRoot, path, label string) error {
	writeHashField(h, label)
	return filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink input is not allowed: %s", current)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular suite input is not allowed: %s", current)
		}
		rel, err := filepath.Rel(projectRoot, current)
		if err != nil || !pathWithinRoot(projectRoot, current) {
			return fmt.Errorf("suite input %q is outside project root", current)
		}
		writeHashField(h, filepath.ToSlash(rel))
		writeHashField(h, info.Mode().Perm().String())
		f, err := os.Open(current)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func writeHashField(h hash.Hash, value string) {
	_, _ = fmt.Fprintf(h, "%d:%s\x00", len(value), value)
}
