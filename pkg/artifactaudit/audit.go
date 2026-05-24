// Package artifactaudit measures whether stored runs have enough local
// artifacts for deterministic failure analysis.
package artifactaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	bench "github.com/vitas/evidra-bench/pkg/bench"
)

const (
	ArtifactDir        = "artifact_dir"
	RunJSON            = "run.json"
	ToolCallsJSON      = "tool-calls.json"
	TimelineJSON       = "timeline.json"
	RunEventsJSON      = "run-events.json"
	FailureAutopsyJSON = "failure-autopsy.json"
	RunErrorJSON       = "run-error.json"
)

// Result is the machine-readable artifact coverage audit report.
type Result struct {
	TotalRuns          int            `json:"total_runs"`
	CompleteRuns       int            `json:"complete_runs"`
	IncompleteRuns     int            `json:"incomplete_runs"`
	CoveragePercent    float64        `json:"coverage_percent"`
	MissingByArtifact  map[string]int `json:"missing_by_artifact,omitempty"`
	InvalidByArtifact  map[string]int `json:"invalid_by_artifact,omitempty"`
	MismatchByArtifact map[string]int `json:"mismatch_by_artifact,omitempty"`
	MissingByAdapter   map[string]int `json:"missing_by_adapter,omitempty"`
	Findings           []Finding      `json:"findings,omitempty"`
}

// Finding describes one artifact coverage gap on one run.
type Finding struct {
	RunID       string `json:"run_id"`
	ScenarioID  string `json:"scenario_id"`
	Model       string `json:"model,omitempty"`
	Adapter     string `json:"adapter,omitempty"`
	Passed      bool   `json:"passed"`
	ExitCode    int    `json:"exit_code"`
	ArtifactDir string `json:"artifact_dir,omitempty"`
	Artifact    string `json:"artifact"`
	Kind        string `json:"kind"`
	Message     string `json:"message"`
}

// Analyze inspects local artifact directories referenced by stored runs.
func Analyze(runs []bench.RunRecord) Result {
	result := Result{
		TotalRuns:          len(runs),
		MissingByArtifact:  map[string]int{},
		InvalidByArtifact:  map[string]int{},
		MismatchByArtifact: map[string]int{},
		MissingByAdapter:   map[string]int{},
	}

	for _, run := range runs {
		a := runAnalyzer{run: run, result: &result}
		a.analyze()
	}

	result.IncompleteRuns = len(incompleteRunIDs(result.Findings))
	result.CompleteRuns = result.TotalRuns - result.IncompleteRuns
	if result.TotalRuns > 0 {
		result.CoveragePercent = float64(result.CompleteRuns) / float64(result.TotalRuns) * 100
	}
	return result
}

type runAnalyzer struct {
	run    bench.RunRecord
	result *Result
}

func (a runAnalyzer) analyze() {
	if a.run.ArtifactDir == "" {
		a.addMissing(ArtifactDir, "run has no artifact_dir")
		return
	}
	info, err := os.Stat(a.run.ArtifactDir)
	if err != nil || !info.IsDir() {
		a.addMissing(ArtifactDir, fmt.Sprintf("artifact_dir is not readable: %v", err))
		return
	}

	required := []string{RunJSON, ToolCallsJSON, TimelineJSON, RunEventsJSON}
	if !a.run.Passed {
		required = append(required, FailureAutopsyJSON)
	}
	if a.run.ExitCode < 0 {
		required = append(required, RunErrorJSON)
	}

	payloads := map[string][]byte{}
	for _, artifact := range required {
		data, ok := a.readRequiredJSON(artifact)
		if ok {
			payloads[artifact] = data
		}
	}
	a.checkTimelineMatchesToolCalls(payloads[ToolCallsJSON], payloads[TimelineJSON])
}

func (a runAnalyzer) readRequiredJSON(artifact string) ([]byte, bool) {
	path := filepath.Join(a.run.ArtifactDir, artifact)
	data, err := os.ReadFile(path)
	if err != nil {
		a.addMissing(artifact, "artifact file is missing")
		return nil, false
	}
	if !json.Valid(data) {
		a.addInvalid(artifact, "artifact is not valid JSON")
		return nil, false
	}
	return data, true
}

func (a runAnalyzer) checkTimelineMatchesToolCalls(toolCallsData, timelineData []byte) {
	if len(toolCallsData) == 0 || len(timelineData) == 0 {
		return
	}
	var calls []json.RawMessage
	if err := json.Unmarshal(toolCallsData, &calls); err != nil {
		a.addInvalid(ToolCallsJSON, "tool calls artifact is not a JSON array")
		return
	}
	var timeline struct {
		TotalSteps int `json:"total_steps"`
	}
	if err := json.Unmarshal(timelineData, &timeline); err != nil {
		a.addInvalid(TimelineJSON, "timeline artifact is not a JSON object")
		return
	}
	if timeline.TotalSteps != len(calls) {
		a.addMismatch(TimelineJSON, fmt.Sprintf("timeline total_steps=%d but tool_calls=%d", timeline.TotalSteps, len(calls)))
	}
}

func (a runAnalyzer) addMissing(artifact, message string) {
	a.addFinding("missing", artifact, message)
	a.result.MissingByArtifact[artifact]++
	a.result.MissingByAdapter[a.adapter()]++
}

func (a runAnalyzer) addInvalid(artifact, message string) {
	a.addFinding("invalid_json", artifact, message)
	a.result.InvalidByArtifact[artifact]++
}

func (a runAnalyzer) addMismatch(artifact, message string) {
	a.addFinding("mismatch", artifact, message)
	a.result.MismatchByArtifact[artifact]++
}

func (a runAnalyzer) addFinding(kind, artifact, message string) {
	a.result.Findings = append(a.result.Findings, Finding{
		RunID:       a.run.ID,
		ScenarioID:  a.run.ScenarioID,
		Model:       a.run.Model,
		Adapter:     a.run.Adapter,
		Passed:      a.run.Passed,
		ExitCode:    a.run.ExitCode,
		ArtifactDir: a.run.ArtifactDir,
		Artifact:    artifact,
		Kind:        kind,
		Message:     message,
	})
}

func (a runAnalyzer) adapter() string {
	if a.run.Adapter == "" {
		return "(unknown)"
	}
	return a.run.Adapter
}

func incompleteRunIDs(findings []Finding) map[string]struct{} {
	ids := map[string]struct{}{}
	for _, finding := range findings {
		ids[finding.RunID] = struct{}{}
	}
	return ids
}

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
