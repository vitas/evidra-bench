package signalaudit

import (
	"sort"
	"strings"
)

// Analyze compares observed run signals against the audit manifest.
func Analyze(manifest Manifest, runs []Run) Result {
	result := Result{}
	scenarioCounts := map[string]*ScenarioFinding{}
	auditedScenarios := map[string]struct{}{}
	instabilityGroups := map[string][]Run{}

	for _, run := range runs {
		expectation, ok := manifest[run.ScenarioID]
		if !ok {
			continue
		}

		result.RunCount++
		auditedScenarios[run.ScenarioID] = struct{}{}
		instabilityGroups[groupKey(run)] = append(instabilityGroups[groupKey(run)], withNormalizedSignals(run))

		missingExpected, forbiddenSignals, unexpectedExtras := classifyRun(expectation, run)
		if len(missingExpected) == 0 && len(forbiddenSignals) == 0 && len(unexpectedExtras) == 0 {
			ensureScenarioFinding(scenarioCounts, run.ScenarioID, expectation.PrimarySignal).RunCount++
			continue
		}

		result.RunFindings = append(result.RunFindings, RunFinding{
			RunDir:           run.RunDir,
			ScenarioID:       run.ScenarioID,
			Model:            run.Model,
			Provider:         run.Provider,
			SignalSource:     run.SignalSource,
			ObservedSignals:  observedSignals(run),
			MissingExpected:  missingExpected,
			ForbiddenSignals: forbiddenSignals,
			UnexpectedExtras: unexpectedExtras,
		})

		result.FindingTotals.MissingExpected += len(missingExpected)
		result.FindingTotals.ForbiddenSignals += len(forbiddenSignals)
		result.FindingTotals.UnexpectedExtras += len(unexpectedExtras)

		scenarioFinding := ensureScenarioFinding(scenarioCounts, run.ScenarioID, expectation.PrimarySignal)
		scenarioFinding.RunCount++
		scenarioFinding.MissingExpectedCount += len(missingExpected)
		scenarioFinding.ForbiddenSignalCount += len(forbiddenSignals)
		scenarioFinding.UnexpectedExtraCount += len(unexpectedExtras)
	}

	for _, groupedRuns := range instabilityGroups {
		if len(groupedRuns) < 2 {
			continue
		}

		signalSets := map[string]struct{}{}
		runDirs := make([]string, 0, len(groupedRuns))
		for _, run := range groupedRuns {
			signalSets[canonicalSignalSet(observedSignals(run))] = struct{}{}
			runDirs = append(runDirs, run.RunDir)
		}
		if len(signalSets) < 2 {
			continue
		}

		sort.Strings(runDirs)
		observedSets := sortedKeys(signalSets)
		first := groupedRuns[0]
		result.InstabilityFindings = append(result.InstabilityFindings, InstabilityFinding{
			ScenarioID:         first.ScenarioID,
			Model:              first.Model,
			Provider:           first.Provider,
			RunDirs:            runDirs,
			ObservedSignalSets: observedSets,
		})
		result.FindingTotals.UnstableGroups++
		ensureScenarioFinding(scenarioCounts, first.ScenarioID, manifest[first.ScenarioID].PrimarySignal).UnstableGroupCount++
	}

	result.AuditedScenarioCount = len(auditedScenarios)
	result.ScenarioFindings = flattenScenarioFindings(scenarioCounts)
	sortRunFindings(result.RunFindings)
	sortInstabilityFindings(result.InstabilityFindings)
	return result
}

func classifyRun(expectation Expectation, run Run) (missingExpected, forbiddenSignals, unexpectedExtras []string) {
	counts := run.SignalCounts
	allowed := make(map[string]struct{}, len(expectation.ExpectedSignals)+len(expectation.AllowedSecondarySignals))
	for _, signal := range expectation.ExpectedSignals {
		allowed[signal] = struct{}{}
		if counts[signal] <= 0 {
			missingExpected = append(missingExpected, signal)
		}
	}
	for _, signal := range expectation.AllowedSecondarySignals {
		allowed[signal] = struct{}{}
	}

	forbidden := make(map[string]struct{}, len(expectation.ForbiddenSignals))
	for _, signal := range expectation.ForbiddenSignals {
		forbidden[signal] = struct{}{}
		if counts[signal] > 0 {
			forbiddenSignals = append(forbiddenSignals, signal)
		}
	}

	for _, signal := range observedSignals(run) {
		if _, isForbidden := forbidden[signal]; isForbidden {
			continue
		}
		if _, isAllowed := allowed[signal]; isAllowed {
			continue
		}
		unexpectedExtras = append(unexpectedExtras, signal)
	}

	return missingExpected, forbiddenSignals, unexpectedExtras
}

func ensureScenarioFinding(scenarios map[string]*ScenarioFinding, scenarioID, primarySignal string) *ScenarioFinding {
	if finding, ok := scenarios[scenarioID]; ok {
		return finding
	}

	finding := &ScenarioFinding{
		ScenarioID:    scenarioID,
		PrimarySignal: primarySignal,
	}
	scenarios[scenarioID] = finding
	return finding
}

func observedSignals(run Run) []string {
	if len(run.Signals) > 0 {
		return append([]string(nil), run.Signals...)
	}
	return signalNames(run.SignalCounts)
}

func withNormalizedSignals(run Run) Run {
	run.Signals = observedSignals(run)
	return run
}

func flattenScenarioFindings(m map[string]*ScenarioFinding) []ScenarioFinding {
	if len(m) == 0 {
		return nil
	}

	out := make([]ScenarioFinding, 0, len(m))
	for _, finding := range m {
		out = append(out, *finding)
	}
	sort.Slice(out, func(i, j int) bool {
		left := totalScenarioFindings(out[i])
		right := totalScenarioFindings(out[j])
		if left != right {
			return left > right
		}
		return out[i].ScenarioID < out[j].ScenarioID
	})
	return out
}

func totalScenarioFindings(f ScenarioFinding) int {
	return f.MissingExpectedCount + f.ForbiddenSignalCount + f.UnexpectedExtraCount + f.UnstableGroupCount
}

func sortRunFindings(findings []RunFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].ScenarioID != findings[j].ScenarioID {
			return findings[i].ScenarioID < findings[j].ScenarioID
		}
		if findings[i].Model != findings[j].Model {
			return findings[i].Model < findings[j].Model
		}
		if findings[i].Provider != findings[j].Provider {
			return findings[i].Provider < findings[j].Provider
		}
		return findings[i].RunDir < findings[j].RunDir
	})
}

func sortInstabilityFindings(findings []InstabilityFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].ScenarioID != findings[j].ScenarioID {
			return findings[i].ScenarioID < findings[j].ScenarioID
		}
		if findings[i].Model != findings[j].Model {
			return findings[i].Model < findings[j].Model
		}
		return findings[i].Provider < findings[j].Provider
	})
}

func groupKey(run Run) string {
	return strings.Join([]string{run.ScenarioID, run.Model, run.Provider}, "\x00")
}

func canonicalSignalSet(signals []string) string {
	if len(signals) == 0 {
		return "(none)"
	}

	clone := append([]string(nil), signals...)
	sort.Strings(clone)
	return strings.Join(clone, ",")
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
