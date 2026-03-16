package report

import (
	"encoding/json"
	"fmt"
	"html/template"
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

// GenerateCompareHTML writes a side-by-side HTML comparison report.
func GenerateCompareHTML(cr *CompareResult, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("compare html: mkdir: %w", err)
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("compare html: create: %w", err)
	}
	defer f.Close()

	tmpl, err := template.New("compare").Parse(compareHTMLTemplate)
	if err != nil {
		return fmt.Errorf("compare html: parse: %w", err)
	}
	return tmpl.Execute(f, cr)
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

const compareHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>infra-bench Compare</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, monospace; background: #0d1117; color: #c9d1d9; padding: 2rem; }
  h1 { color: #58a6ff; margin-bottom: 0.5rem; }
  h2 { color: #8b949e; margin: 2rem 0 1rem; border-bottom: 1px solid #21262d; padding-bottom: 0.5rem; }
  .meta { color: #8b949e; font-size: 0.9rem; margin-bottom: 2rem; }
  .pass { color: #3fb950; }
  .fail { color: #f85149; }
  .improved { color: #3fb950; }
  .regressed { color: #f85149; }
  .same { color: #8b949e; }
  .badge { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 3px; font-size: 0.8rem; font-weight: 600; }
  .badge-pass { background: #238636; color: #fff; }
  .badge-fail { background: #da3633; color: #fff; }
  .badge-improved { background: #238636; color: #fff; }
  .badge-regressed { background: #da3633; color: #fff; }
  .badge-same { background: #21262d; color: #8b949e; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; margin: 1.5rem 0; }
  .card { background: #161b22; border: 1px solid #21262d; border-radius: 8px; padding: 1.25rem; }
  .card h3 { color: #58a6ff; margin-bottom: 1rem; font-size: 0.95rem; }
  .row { display: flex; justify-content: space-between; padding: 0.4rem 0; border-bottom: 1px solid #21262d; font-size: 0.9rem; }
  .row:last-child { border-bottom: none; }
  .row .label { color: #8b949e; }
  .row .value { font-weight: 600; }
  .delta-bar { text-align: center; margin: 1.5rem 0; }
  .delta-bar .arrow { font-size: 2rem; }
  table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
  th, td { text-align: left; padding: 0.5rem 0.75rem; border-bottom: 1px solid #21262d; font-size: 0.9rem; }
  th { color: #8b949e; font-weight: 600; font-size: 0.8rem; text-transform: uppercase; }
  .check-pass { color: #3fb950; }
  .check-fail { color: #f85149; }
  .check-icon { cursor: help; padding: 0 2px; }
</style>
</head>
<body>
<h1>infra-bench Compare</h1>
<div class="meta">{{.RunA.ScenarioID}} &mdash; <span class="badge badge-{{.VerdictDelta}}">{{.VerdictDelta}}</span></div>

<div class="grid">
  <div class="card">
    <h3>Run A &mdash; {{if .RunA.Passed}}<span class="pass">PASS</span>{{else}}<span class="fail">FAIL</span>{{end}}</h3>
    <div class="row"><span class="label">Dir</span><span class="value">{{.RunA.Dir}}</span></div>
    <div class="row"><span class="label">Model</span><span class="value">{{.RunA.Model}}</span></div>
    <div class="row"><span class="label">Provider</span><span class="value">{{.RunA.Provider}}</span></div>
    <div class="row"><span class="label">Duration</span><span class="value">{{.RunA.Duration}}</span></div>
    <div class="row"><span class="label">Turns</span><span class="value">{{.RunA.Turns}}</span></div>
    <div class="row"><span class="label">Memory</span><span class="value">{{.RunA.Memory}}</span></div>
    <div class="row"><span class="label">Tokens</span><span class="value">{{.RunA.Tokens}}</span></div>
    <div class="row"><span class="label">Cost</span><span class="value">{{.RunA.Cost}}</span></div>
  </div>
  <div class="card">
    <h3>Run B &mdash; {{if .RunB.Passed}}<span class="pass">PASS</span>{{else}}<span class="fail">FAIL</span>{{end}}</h3>
    <div class="row"><span class="label">Dir</span><span class="value">{{.RunB.Dir}}</span></div>
    <div class="row"><span class="label">Model</span><span class="value">{{.RunB.Model}}</span></div>
    <div class="row"><span class="label">Provider</span><span class="value">{{.RunB.Provider}}</span></div>
    <div class="row"><span class="label">Duration</span><span class="value">{{.RunB.Duration}}</span></div>
    <div class="row"><span class="label">Turns</span><span class="value">{{.RunB.Turns}}</span></div>
    <div class="row"><span class="label">Memory</span><span class="value">{{.RunB.Memory}}</span></div>
    <div class="row"><span class="label">Tokens</span><span class="value">{{.RunB.Tokens}}</span></div>
    <div class="row"><span class="label">Cost</span><span class="value">{{.RunB.Cost}}</span></div>
  </div>
</div>

<h2>Check Comparison</h2>
<table>
<tr>
  <th>Check</th>
  <th>Run A</th>
  <th>Run B</th>
  <th>Delta</th>
</tr>
{{range .CheckDiffs}}
<tr>
  <td>{{.Name}}</td>
  <td>{{if .VerdictA}}<span class="check-icon {{if eq .VerdictA "pass"}}check-pass{{else}}check-fail{{end}}">{{if eq .VerdictA "pass"}}&#10003;{{else}}&#10007;{{end}} {{.VerdictA}}</span>{{else}}&mdash;{{end}}</td>
  <td>{{if .VerdictB}}<span class="check-icon {{if eq .VerdictB "pass"}}check-pass{{else}}check-fail{{end}}">{{if eq .VerdictB "pass"}}&#10003;{{else}}&#10007;{{end}} {{.VerdictB}}</span>{{else}}&mdash;{{end}}</td>
  <td><span class="badge badge-{{.Change}}">{{.Change}}</span></td>
</tr>
{{end}}
</table>

</body>
</html>`
