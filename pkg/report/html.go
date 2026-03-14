package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"

	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/verifier"
)

// RunSummary is a single run's data for the report.
type RunSummary struct {
	RunID      string
	ScenarioID string
	Adapter    string
	Model      string
	Provider   string
	Passed     bool
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	ExitCode   int
	Checks     []CheckSummary
	Turns      string
	Memory     string
	Tokens     string
}

// CheckSummary is a single check result.
type CheckSummary struct {
	Name    string
	Type    string
	Verdict string
	Message string
}

// ScenarioInfo holds scenario metadata for the report.
type ScenarioInfo struct {
	ID       string
	Title    string
	Category string
	Tags     []string
	Evidra   bool
	Runs     []RunSummary
	PassRate string
}

// ReportData is the full data passed to the HTML template.
type ReportData struct {
	GeneratedAt string
	TotalRuns   int
	TotalPass   int
	TotalFail   int
	PassRate    string
	Scenarios   []ScenarioInfo
	Models      []string
	Providers   []string
}

// GenerateHTML creates an HTML report from run artifacts and scenario catalog.
func GenerateHTML(scenariosDir, runsDir, outputPath string) error {
	scenarios, err := scenario.LoadAll(scenariosDir)
	if err != nil {
		return fmt.Errorf("report: load scenarios: %w", err)
	}

	runs := loadRuns(runsDir)

	scenarioMap := map[string]*ScenarioInfo{}
	for _, s := range scenarios {
		scenarioMap[s.ID] = &ScenarioInfo{
			ID:       s.ID,
			Title:    s.Title,
			Category: s.Category,
			Tags:     s.Tags,
			Evidra:   s.Evidra.Enabled,
		}
	}

	models := map[string]bool{}
	providers := map[string]bool{}
	totalPass := 0

	for _, r := range runs {
		si, ok := scenarioMap[r.ScenarioID]
		if !ok {
			continue
		}
		si.Runs = append(si.Runs, r)
		if r.Passed {
			totalPass++
		}
		if r.Model != "" {
			models[r.Model] = true
		}
		if r.Provider != "" {
			providers[r.Provider] = true
		}
	}

	var scenarioList []ScenarioInfo
	for _, s := range scenarios {
		si := scenarioMap[s.ID]
		if len(si.Runs) > 0 {
			passCount := 0
			for _, r := range si.Runs {
				if r.Passed {
					passCount++
				}
			}
			si.PassRate = fmt.Sprintf("%d/%d", passCount, len(si.Runs))
		}
		scenarioList = append(scenarioList, *si)
	}

	modelList := sortedKeys(models)
	providerList := sortedKeys(providers)

	passRate := "0%"
	if len(runs) > 0 {
		passRate = fmt.Sprintf("%.0f%%", float64(totalPass)/float64(len(runs))*100)
	}

	data := ReportData{
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		TotalRuns:   len(runs),
		TotalPass:   totalPass,
		TotalFail:   len(runs) - totalPass,
		PassRate:    passRate,
		Scenarios:   scenarioList,
		Models:      modelList,
		Providers:   providerList,
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("report: mkdir: %w", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("report: create file: %w", err)
	}
	defer f.Close()

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("report: parse template: %w", err)
	}
	return tmpl.Execute(f, data)
}

func loadRuns(runsDir string) []RunSummary {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}
	var runs []RunSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runJSON := filepath.Join(runsDir, entry.Name(), "run.json")
		data, err := os.ReadFile(runJSON)
		if err != nil {
			continue
		}
		var raw struct {
			ScenarioID string            `json:"scenario_id"`
			Adapter    string            `json:"adapter"`
			Passed     bool              `json:"passed"`
			StartTime  time.Time         `json:"start_time"`
			EndTime    time.Time         `json:"end_time"`
			ExitCode   int               `json:"exit_code"`
			Checks     json.RawMessage   `json:"checks"`
			Metadata   map[string]string `json:"metadata"`
		}
		if json.Unmarshal(data, &raw) != nil {
			continue
		}

		var checks []CheckSummary
		if len(raw.Checks) > 0 {
			var vr verifier.VerifyResult
			if json.Unmarshal(raw.Checks, &vr) == nil {
				for _, c := range vr.Checks {
					checks = append(checks, CheckSummary{
						Name:    c.Name,
						Type:    c.Type,
						Verdict: string(c.Verdict),
						Message: c.Message,
					})
				}
			}
		}

		runs = append(runs, RunSummary{
			RunID:      entry.Name(),
			ScenarioID: raw.ScenarioID,
			Adapter:    raw.Adapter,
			Model:      raw.Metadata["model"],
			Provider:   raw.Metadata["provider"],
			Passed:     raw.Passed,
			StartTime:  raw.StartTime,
			EndTime:    raw.EndTime,
			Duration:   raw.EndTime.Sub(raw.StartTime),
			ExitCode:   raw.ExitCode,
			Checks:     checks,
			Turns:      raw.Metadata["turns"],
			Memory:     raw.Metadata["memory_window"],
			Tokens:     formatTokens(raw.Metadata),
		})
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartTime.After(runs[j].StartTime)
	})
	return runs
}

func formatTokens(meta map[string]string) string {
	pt := meta["prompt_tokens"]
	ct := meta["completion_tokens"]
	if pt == "" && ct == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", pt, ct)
}

func sortedKeys(m map[string]bool) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>infra-bench Report</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, monospace; background: #0d1117; color: #c9d1d9; padding: 2rem; }
  h1 { color: #58a6ff; margin-bottom: 0.5rem; }
  h2 { color: #8b949e; margin: 2rem 0 1rem; border-bottom: 1px solid #21262d; padding-bottom: 0.5rem; }
  h3 { color: #c9d1d9; margin: 1.5rem 0 0.5rem; }
  .meta { color: #8b949e; font-size: 0.9rem; margin-bottom: 2rem; }
  .summary { display: flex; gap: 2rem; margin-bottom: 2rem; }
  .stat { background: #161b22; border: 1px solid #21262d; border-radius: 6px; padding: 1rem 1.5rem; }
  .stat .value { font-size: 2rem; font-weight: bold; }
  .stat .label { color: #8b949e; font-size: 0.85rem; }
  .pass { color: #3fb950; }
  .fail { color: #f85149; }
  table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
  th, td { text-align: left; padding: 0.5rem 0.75rem; border-bottom: 1px solid #21262d; }
  th { color: #8b949e; font-weight: 600; font-size: 0.85rem; text-transform: uppercase; }
  tr:hover { background: #161b22; }
  .badge { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 3px; font-size: 0.8rem; font-weight: 600; }
  .badge-pass { background: #238636; color: #fff; }
  .badge-fail { background: #da3633; color: #fff; }
  .badge-evidra { background: #1f6feb; color: #fff; }
  .badge-category { background: #21262d; color: #8b949e; }
  .tags { color: #8b949e; font-size: 0.8rem; }
  .checks { font-size: 0.85rem; margin-top: 0.25rem; }
  .check-pass { color: #3fb950; }
  .check-fail { color: #f85149; }
  .run-details { margin-left: 1rem; }
  details { margin: 0.5rem 0; }
  summary { cursor: pointer; color: #58a6ff; }
  .section { margin-bottom: 3rem; }
</style>
</head>
<body>
<h1>infra-bench Report</h1>
<div class="meta">Generated: {{.GeneratedAt}}{{if .Models}} | Models: {{range $i, $m := .Models}}{{if $i}}, {{end}}{{$m}}{{end}}{{end}}{{if .Providers}} | Providers: {{range $i, $p := .Providers}}{{if $i}}, {{end}}{{$p}}{{end}}{{end}}</div>

<div class="summary">
  <div class="stat"><div class="value">{{.TotalRuns}}</div><div class="label">Total Runs</div></div>
  <div class="stat"><div class="value pass">{{.TotalPass}}</div><div class="label">Passed</div></div>
  <div class="stat"><div class="value fail">{{.TotalFail}}</div><div class="label">Failed</div></div>
  <div class="stat"><div class="value">{{.PassRate}}</div><div class="label">Pass Rate</div></div>
</div>

<div class="section">
<h2>Scenario Matrix</h2>
<table>
<tr>
  <th>ID</th>
  <th>Title</th>
  <th>Category</th>
  <th>Evidra</th>
  <th>Runs</th>
  <th>Pass Rate</th>
</tr>
{{range .Scenarios}}
<tr>
  <td><strong>{{.ID}}</strong></td>
  <td>{{.Title}}</td>
  <td><span class="badge badge-category">{{.Category}}</span></td>
  <td>{{if .Evidra}}<span class="badge badge-evidra">E</span>{{end}}</td>
  <td>{{len .Runs}}</td>
  <td>{{if .PassRate}}{{.PassRate}}{{else}}—{{end}}</td>
</tr>
{{end}}
</table>
</div>

{{range .Scenarios}}{{if .Runs}}
<div class="section">
<h3>{{.ID}} — {{.Title}}</h3>
<div class="tags">{{range .Tags}}{{.}} {{end}}</div>
<table>
<tr>
  <th>Time</th>
  <th>Result</th>
  <th>Model</th>
  <th>Provider</th>
  <th>Duration</th>
  <th>Turns</th>
  <th>Memory</th>
  <th>Tokens</th>
  <th>Checks</th>
</tr>
{{range .Runs}}
<tr>
  <td>{{.StartTime.Format "2006-01-02 15:04"}}</td>
  <td>{{if .Passed}}<span class="badge badge-pass">PASS</span>{{else}}<span class="badge badge-fail">FAIL</span>{{end}}</td>
  <td>{{.Model}}</td>
  <td>{{.Provider}}</td>
  <td>{{.Duration}}</td>
  <td>{{.Turns}}</td>
  <td>{{.Memory}}</td>
  <td>{{.Tokens}}</td>
  <td>
    {{range .Checks}}
      <span class="{{if eq .Verdict "pass"}}check-pass{{else}}check-fail{{end}}">{{if eq .Verdict "pass"}}✓{{else}}✗{{end}}</span>
    {{end}}
  </td>
</tr>
{{end}}
</table>
</div>
{{end}}{{end}}

</body>
</html>`
