package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
)

type artifactTab string

const (
	artifactTabSummary    artifactTab = "summary"
	artifactTabAutopsy    artifactTab = "autopsy"
	artifactTabTimeline   artifactTab = "timeline"
	artifactTabTranscript artifactTab = "transcript"
	artifactTabToolCalls  artifactTab = "tool-calls"
	artifactTabScorecard  artifactTab = "scorecard"
)

var artifactTabs = []artifactTab{
	artifactTabSummary,
	artifactTabAutopsy,
	artifactTabTimeline,
	artifactTabTranscript,
	artifactTabToolCalls,
	artifactTabScorecard,
}

// RunArtifacts is the local artifact bundle rendered by the TUI artifact view.
type RunArtifacts struct {
	Dir          string
	Transcript   string
	ToolCallsRaw string
	ToolCalls    []bench.ToolCall
	Timeline     *bench.Timeline
	AutopsyRaw   string
	ScorecardRaw string
}

func LoadRunArtifacts(dir string) RunArtifacts {
	artifacts := RunArtifacts{Dir: dir}
	artifacts.Transcript = readArtifactText(dir, artifact.TranscriptFile)
	artifacts.ToolCallsRaw = readArtifactText(dir, artifact.ToolCallsFile)
	if artifacts.ToolCallsRaw != "" {
		_ = json.Unmarshal([]byte(artifacts.ToolCallsRaw), &artifacts.ToolCalls)
		if len(artifacts.ToolCalls) > 0 {
			artifacts.Timeline = bench.Parse(artifacts.ToolCalls)
		}
	}
	artifacts.AutopsyRaw = readArtifactText(dir, artifact.FailureAutopsyFile)
	artifacts.ScorecardRaw = readArtifactText(dir, artifact.ScorecardFile)
	return artifacts
}

func (a *App) openLatestArtifactsForSelectedScenario() bool {
	if len(a.filtered) == 0 || a.cursor >= len(a.filtered) {
		return false
	}
	s := a.filtered[a.cursor].Scenario
	for _, run := range HistoryForScenario(a.history, s.ID) {
		if run.Dir != "" {
			return a.openArtifactsForDir(run.Dir)
		}
	}
	return false
}

func (a *App) openArtifactsForDir(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	artifacts := LoadRunArtifacts(dir)
	a.artifacts = &artifacts
	a.artifactTab = firstAvailableArtifactTab(artifacts)
	a.view = viewArtifact
	return true
}

func firstAvailableArtifactTab(artifacts RunArtifacts) int {
	for i, tab := range artifactTabs {
		if artifacts.Has(tab) {
			return i
		}
	}
	return 0
}

func (a RunArtifacts) Has(tab artifactTab) bool {
	switch tab {
	case artifactTabSummary:
		return a.Dir != ""
	case artifactTabAutopsy:
		return a.AutopsyRaw != ""
	case artifactTabTimeline:
		return a.Timeline != nil
	case artifactTabTranscript:
		return a.Transcript != ""
	case artifactTabToolCalls:
		return a.ToolCallsRaw != ""
	case artifactTabScorecard:
		return a.ScorecardRaw != ""
	default:
		return false
	}
}

func (a RunArtifacts) Render(tab artifactTab) string {
	switch tab {
	case artifactTabSummary:
		return a.renderSummary()
	case artifactTabAutopsy:
		return a.renderAutopsy()
	case artifactTabTimeline:
		return a.renderTimeline()
	case artifactTabTranscript:
		return renderTextArtifact("Transcript", a.Transcript)
	case artifactTabToolCalls:
		return renderTextArtifact("Tool Calls", prettyJSON(a.ToolCallsRaw))
	case artifactTabScorecard:
		return renderTextArtifact("Scorecard", prettyJSON(a.ScorecardRaw))
	default:
		return "Artifact tab unavailable"
	}
}

func (a RunArtifacts) renderSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Artifacts\n  dir: %s\n\n", a.Dir)
	for _, tab := range artifactTabs[1:] {
		state := "missing"
		if a.Has(tab) {
			state = "available"
		}
		fmt.Fprintf(&b, "  %-11s %s\n", tab, state)
	}
	return b.String()
}

func (a RunArtifacts) renderAutopsy() string {
	if a.AutopsyRaw == "" {
		return "Autopsy unavailable"
	}
	var report struct {
		Outcome        string `json:"outcome"`
		PrimaryFailure string `json:"primary_failure"`
		Summary        string `json:"summary"`
		Confidence     string `json:"confidence"`
		Findings       []struct {
			Kind     string          `json:"kind"`
			Severity string          `json:"severity"`
			Message  string          `json:"message"`
			Evidence json.RawMessage `json:"evidence"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(a.AutopsyRaw), &report); err != nil {
		return renderTextArtifact("Autopsy", prettyJSON(a.AutopsyRaw))
	}
	var b strings.Builder
	b.WriteString("Autopsy\n")
	fmt.Fprintf(&b, "  outcome: %s\n", valueOrDefault(report.Outcome, "unknown"))
	fmt.Fprintf(&b, "  primary: %s\n", valueOrDefault(report.PrimaryFailure, "none"))
	if report.Confidence != "" {
		fmt.Fprintf(&b, "  confidence: %s\n", report.Confidence)
	}
	if report.Summary != "" {
		fmt.Fprintf(&b, "\n%s\n", report.Summary)
	}
	if len(report.Findings) > 0 {
		b.WriteString("\nFindings\n")
		for _, finding := range report.Findings {
			fmt.Fprintf(&b, "  [%s] %s: %s\n", finding.Severity, finding.Kind, finding.Message)
			evidence := formatEvidenceRaw(finding.Evidence)
			if evidence != "" {
				fmt.Fprintf(&b, "    evidence: %s\n", evidence)
			}
		}
	}
	return b.String()
}

func (a RunArtifacts) renderTimeline() string {
	if a.Timeline == nil {
		return "Timeline unavailable"
	}
	var b strings.Builder
	b.WriteString("Timeline\n")
	fmt.Fprintf(&b, "  steps: %d  mutations: %d  diagnosis_depth: %d\n\n", a.Timeline.TotalSteps, a.Timeline.MutationCount, a.Timeline.DiagnosisDepth)
	for _, step := range a.Timeline.Steps {
		label := step.Summary
		if label == "" {
			label = step.Command
		}
		fmt.Fprintf(&b, "  %02d %-9s %s\n", step.Index+1, step.Phase, label)
		if step.Command != "" && step.Command != label {
			fmt.Fprintf(&b, "      %s\n", step.Command)
		}
	}
	return b.String()
}

func readArtifactText(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return string(data)
}

func renderTextArtifact(title, text string) string {
	if strings.TrimSpace(text) == "" {
		return title + " unavailable"
	}
	return title + "\n\n" + text
}

func prettyJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(raw), "", "  "); err != nil {
		return raw
	}
	return out.String()
}

func formatEvidenceRaw(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return string(raw)
}
