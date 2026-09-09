package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/benchexport"
	"github.com/vitas/evidra-bench/pkg/evidrawire"
)

// exportEvaluationBundles converts every run artifact written under
// <outputDir>/runs into an Evidra external evidence bundle under
// <outputDir>/bundles/<run-id>. Existing bundles are never overwritten, so
// reruns stay idempotent.
//
// It is best-effort by contract: per-run export failures are joined into the
// returned error while successful bundles keep their exported state; the
// caller warns the user without changing the evaluation exit code, because
// result.json/report.html are already the authoritative outcome.
func exportEvaluationBundles(outputDir, producerVersion string) (int, error) {
	runsRoot := filepath.Join(outputDir, "runs")
	if info, err := os.Stat(runsRoot); err != nil || !info.IsDir() {
		return 0, nil
	}
	var exported int
	var failures []string
	err := filepath.WalkDir(runsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // tolerate partially broken archives, same as audit commands
		}
		if d.IsDir() || d.Name() != artifact.RunJSON {
			return nil
		}
		runDir := filepath.Dir(path)
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var probe struct {
			RunID string `json:"run_id"`
		}
		if json.Unmarshal(raw, &probe) != nil || strings.TrimSpace(probe.RunID) == "" {
			return nil
		}
		dest := filepath.Join(outputDir, "bundles", safeBundleDirName(probe.RunID))
		if _, statErr := os.Stat(filepath.Join(dest, evidrawire.BundleFileName)); statErr == nil {
			exported++ // already exported; counts toward the summary line
			return filepath.SkipDir
		}
		if _, expErr := benchexport.Export(benchexport.Request{
			RunDir:          runDir,
			OutDir:          dest,
			ProducerVersion: producerVersion,
		}); expErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", probe.RunID, expErr))
		} else {
			exported++
		}
		return filepath.SkipDir
	})
	if err != nil {
		return exported, fmt.Errorf("scan %s: %w", runsRoot, err)
	}
	if len(failures) > 0 {
		return exported, fmt.Errorf("%d run(s) failed bundle export: %s", len(failures), strings.Join(failures, "; "))
	}
	return exported, nil
}

// safeBundleDirName maps a run id to a single safe path element.
func safeBundleDirName(runID string) string {
	var b strings.Builder
	for _, r := range runID {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	trimmed := strings.Trim(out, ".") // guard against "." and ".."
	if trimmed == "" {
		return "run"
	}
	return trimmed
}
