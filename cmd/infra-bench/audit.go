package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"samebits.com/evidra-infra-bench/pkg/signalaudit"
)

func loadAuditRuns(runsDir, scenarioFilter, modelFilter, providerFilter string) ([]signalaudit.Run, error) {
	var runs []signalaudit.Run

	err := filepath.WalkDir(runsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "run.json" {
			return nil
		}

		run, err := signalaudit.LoadRun(filepath.Dir(path))
		if err != nil {
			return err
		}
		if !matchesAuditFilters(run, scenarioFilter, modelFilter, providerFilter) {
			return nil
		}

		runs = append(runs, run)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return runs, nil
}

func matchesAuditFilters(run signalaudit.Run, scenarioFilter, modelFilter, providerFilter string) bool {
	if scenarioFilter != "" && !strings.EqualFold(run.ScenarioID, scenarioFilter) {
		return false
	}
	if modelFilter != "" && !strings.EqualFold(run.Model, modelFilter) {
		return false
	}
	if providerFilter != "" && !strings.EqualFold(run.Provider, providerFilter) {
		return false
	}
	return true
}

func resolveAuditManifestPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}

	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	for {
		candidate := filepath.Join(wd, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return path
		}
		wd = parent
	}
}
