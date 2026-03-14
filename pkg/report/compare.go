package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"samebits.com/evidra-infra-bench/pkg/verifier"
)

// CompareResult is the output of comparing two runs.
type CompareResult struct {
	RunA          RunInfo
	RunB          RunInfo
	VerdictDelta  string // "same", "improved", "regressed"
	DurationDelta time.Duration
	CheckDiffs    []CheckDiff
	TokenDelta    TokenDelta
	CostDelta     string
}

// RunInfo summarizes a single run for comparison.
type RunInfo struct {
	Dir        string
	ScenarioID string
	Model      string
	Provider   string
	Passed     bool
	Duration   time.Duration
	Turns      string
	Memory     string
	Tokens     string
	Cost       string
	Checks     []CheckSummary
}

// CheckDiff describes how a single check changed between runs.
type CheckDiff struct {
	Name     string
	VerdictA string
	VerdictB string
	Change   string // "same", "improved", "regressed", "new", "removed"
}

// TokenDelta shows token usage change.
type TokenDelta struct {
	InputA  int
	InputB  int
	OutputA int
	OutputB int
}

// CompareRuns loads two run directories and produces a comparison.
func CompareRuns(dirA, dirB string) (*CompareResult, error) {
	a, err := loadRunInfo(dirA)
	if err != nil {
		return nil, fmt.Errorf("compare: load run A: %w", err)
	}
	b, err := loadRunInfo(dirB)
	if err != nil {
		return nil, fmt.Errorf("compare: load run B: %w", err)
	}

	verdictDelta := "same"
	if !a.Passed && b.Passed {
		verdictDelta = "improved"
	} else if a.Passed && !b.Passed {
		verdictDelta = "regressed"
	}

	checkDiffs := diffChecks(a.Checks, b.Checks)

	return &CompareResult{
		RunA:          *a,
		RunB:          *b,
		VerdictDelta:  verdictDelta,
		DurationDelta: b.Duration - a.Duration,
		CheckDiffs:    checkDiffs,
		TokenDelta:    TokenDelta{InputA: parseTokens(a.Tokens, 0), InputB: parseTokens(b.Tokens, 0), OutputA: parseTokens(a.Tokens, 1), OutputB: parseTokens(b.Tokens, 1)},
		CostDelta:     fmt.Sprintf("%s → %s", a.Cost, b.Cost),
	}, nil
}

// FormatComparison returns a human-readable comparison string.
func FormatComparison(cr *CompareResult) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Compare: %s vs %s\n", filepath.Base(cr.RunA.Dir), filepath.Base(cr.RunB.Dir)))
	b.WriteString(fmt.Sprintf("Scenario: %s\n\n", cr.RunA.ScenarioID))

	b.WriteString(fmt.Sprintf("  %-20s  %-20s  %-20s\n", "", "Run A", "Run B"))
	b.WriteString(fmt.Sprintf("  %-20s  %-20s  %-20s\n", "Verdict", fmtVerdict(cr.RunA.Passed), fmtVerdict(cr.RunB.Passed)))
	b.WriteString(fmt.Sprintf("  %-20s  %-20s  %-20s\n", "Model", cr.RunA.Model, cr.RunB.Model))
	b.WriteString(fmt.Sprintf("  %-20s  %-20s  %-20s\n", "Provider", cr.RunA.Provider, cr.RunB.Provider))
	b.WriteString(fmt.Sprintf("  %-20s  %-20s  %-20s\n", "Duration", cr.RunA.Duration.Round(time.Millisecond).String(), cr.RunB.Duration.Round(time.Millisecond).String()))
	b.WriteString(fmt.Sprintf("  %-20s  %-20s  %-20s\n", "Turns", cr.RunA.Turns, cr.RunB.Turns))
	b.WriteString(fmt.Sprintf("  %-20s  %-20s  %-20s\n", "Memory", cr.RunA.Memory, cr.RunB.Memory))
	b.WriteString(fmt.Sprintf("  %-20s  %-20s  %-20s\n", "Tokens", cr.RunA.Tokens, cr.RunB.Tokens))
	b.WriteString(fmt.Sprintf("  %-20s  %-20s  %-20s\n", "Cost", cr.RunA.Cost, cr.RunB.Cost))

	b.WriteString(fmt.Sprintf("\n  Verdict: %s\n", cr.VerdictDelta))

	if len(cr.CheckDiffs) > 0 {
		b.WriteString("\n  Check Changes:\n")
		for _, d := range cr.CheckDiffs {
			switch d.Change {
			case "same":
				continue
			case "improved":
				b.WriteString(fmt.Sprintf("    + %s: %s → %s\n", d.Name, d.VerdictA, d.VerdictB))
			case "regressed":
				b.WriteString(fmt.Sprintf("    - %s: %s → %s\n", d.Name, d.VerdictA, d.VerdictB))
			case "new":
				b.WriteString(fmt.Sprintf("    + %s: (new) %s\n", d.Name, d.VerdictB))
			case "removed":
				b.WriteString(fmt.Sprintf("    - %s: %s (removed)\n", d.Name, d.VerdictA))
			}
		}
	}

	return b.String()
}

func loadRunInfo(dir string) (*RunInfo, error) {
	data, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		return nil, err
	}
	var raw struct {
		ScenarioID string            `json:"scenario_id"`
		Passed     bool              `json:"passed"`
		StartTime  time.Time         `json:"start_time"`
		EndTime    time.Time         `json:"end_time"`
		Checks     json.RawMessage   `json:"checks"`
		Metadata   map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var checks []CheckSummary
	if len(raw.Checks) > 0 {
		var vr verifier.VerifyResult
		if json.Unmarshal(raw.Checks, &vr) == nil {
			for _, c := range vr.Checks {
				checks = append(checks, CheckSummary{
					Name:    c.Name,
					Verdict: string(c.Verdict),
				})
			}
		}
	}

	tokens := ""
	pt := raw.Metadata["prompt_tokens"]
	ct := raw.Metadata["completion_tokens"]
	if pt != "" || ct != "" {
		tokens = pt + "/" + ct
	}

	return &RunInfo{
		Dir:        dir,
		ScenarioID: raw.ScenarioID,
		Model:      raw.Metadata["model"],
		Provider:   raw.Metadata["provider"],
		Passed:     raw.Passed,
		Duration:   raw.EndTime.Sub(raw.StartTime),
		Turns:      raw.Metadata["turns"],
		Memory:     raw.Metadata["memory_window"],
		Tokens:     tokens,
		Cost:       raw.Metadata["estimated_cost"],
		Checks:     checks,
	}, nil
}

func diffChecks(a, b []CheckSummary) []CheckDiff {
	aMap := map[string]string{}
	for _, c := range a {
		aMap[c.Name] = c.Verdict
	}
	bMap := map[string]string{}
	for _, c := range b {
		bMap[c.Name] = c.Verdict
	}

	var diffs []CheckDiff
	// Checks in both
	for _, c := range a {
		bVerdict, exists := bMap[c.Name]
		if !exists {
			diffs = append(diffs, CheckDiff{Name: c.Name, VerdictA: c.Verdict, Change: "removed"})
			continue
		}
		change := "same"
		if c.Verdict != bVerdict {
			if c.Verdict == "fail" && bVerdict == "pass" {
				change = "improved"
			} else {
				change = "regressed"
			}
		}
		diffs = append(diffs, CheckDiff{Name: c.Name, VerdictA: c.Verdict, VerdictB: bVerdict, Change: change})
	}
	// New checks in B
	for _, c := range b {
		if _, exists := aMap[c.Name]; !exists {
			diffs = append(diffs, CheckDiff{Name: c.Name, VerdictB: c.Verdict, Change: "new"})
		}
	}
	return diffs
}

func fmtVerdict(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

func parseTokens(tokens string, idx int) int {
	parts := strings.Split(tokens, "/")
	if idx >= len(parts) {
		return 0
	}
	var n int
	fmt.Sscanf(parts[idx], "%d", &n)
	return n
}
