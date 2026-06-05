package main

import (
	"path/filepath"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

func prepareScenarioRunConfig(base config.Config, s *scenario.Scenario, model, runDir string) config.Config {
	runCfg := base
	runCfg.Scenario = s.Path
	runCfg.Model = model
	runCfg.RunsDir = runDir
	runCfg.EvidenceDir = filepath.Join(runDir, "evidence")
	return runCfg
}

func runDisplayVerdict(passed bool, errMessage, scenarioID string) string {
	if errMessage != "" && errMessage != (&RunFailedError{ScenarioID: scenarioID}).Error() {
		return "ERROR"
	}
	if !passed {
		return "FAIL"
	}
	return "PASS"
}
