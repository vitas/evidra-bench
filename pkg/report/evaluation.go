package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/vitas/evidra-bench/pkg/evaluation"
)

// EvaluationReportOptions contains presentation-only metadata. It never
// changes evaluation verdicts or summaries.
type EvaluationReportOptions struct {
	Title       string
	Limitations []string
}

// RenderEvaluationJSON serializes the canonical result without recomputing it.
func RenderEvaluationJSON(w io.Writer, result evaluation.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("report: render evaluation JSON: %w", err)
	}
	return nil
}

// RenderEvaluationTerminal writes a concise local-run summary.
func RenderEvaluationTerminal(w io.Writer, result evaluation.Result) error {
	if _, err := fmt.Fprintln(w, "Evidra Infrastructure Agent Tests"); err != nil {
		return fmt.Errorf("report: render terminal heading: %w", err)
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return fmt.Errorf("report: render terminal heading spacing: %w", err)
	}
	for _, c := range result.Cases {
		line := fmt.Sprintf("%-32s %-18s %s", c.ScenarioID, string(c.Verdict), compactDuration(c.Duration))
		if c.Runtime.Unconfined {
			// Material gaps must not be silently PASS-shaped
			// (release review finding #2): the terminal says so inline.
			line += "  \u26a0 unconfined agent"
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("report: render terminal case: %w", err)
		}
	}
	if unconfined := countUnconfined(result.Cases); unconfined > 0 {
		if _, err := fmt.Fprintf(w, "\n\u26a0 %d/%d cases ran the agent outside the sandbox (gap agent_unconfined_execution). External --agent commands are sandboxed by default; --agent-unconfined opts out and profiled cases then grade INCOMPLETE.\n", unconfined, len(result.Cases)); err != nil {
			return fmt.Errorf("report: render terminal gap warning: %w", err)
		}
	}
	s := result.Summary
	if _, err := fmt.Fprintf(w, "\n%d passed · %d failed · %d unsafe · %d incomplete\n", s.Passed, s.Failed, s.Unsafe, s.Incomplete); err != nil {
		return fmt.Errorf("report: render terminal summary: %w", err)
	}
	if _, err := fmt.Fprintf(w, "Duration: %s\n", compactDuration(result.EndedAt.Sub(result.StartedAt))); err != nil {
		return fmt.Errorf("report: render terminal duration: %w", err)
	}
	knownPrompt, knownCompletion, unknown := aggregateUsage(result.Cases)
	if unknown > 0 {
		_, err := fmt.Fprintf(w, "Usage: unknown for %d/%d cases", unknown, len(result.Cases))
		if knownPrompt+knownCompletion > 0 {
			_, err = fmt.Fprintf(w, "; known tokens: %d", knownPrompt+knownCompletion)
		}
		if err == nil {
			_, err = fmt.Fprintln(w)
		}
		if err != nil {
			return fmt.Errorf("report: render terminal usage: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(w, "Usage: %d tokens\n", knownPrompt+knownCompletion); err != nil {
			return fmt.Errorf("report: render terminal usage: %w", err)
		}
	}
	return nil
}

type evaluationHTMLData struct {
	Title       string
	Result      evaluation.Result
	Duration    string
	Limitations []string
	Cases       []evaluationHTMLCase
}

type evaluationHTMLCase struct {
	VerdictLabel string
	Result       evaluation.CaseResult
	Duration     string
	Usage        string
}

// RenderEvaluationHTML writes one self-contained document with no remote
// scripts, fonts, stylesheets, or other network assets.
func RenderEvaluationHTML(w io.Writer, result evaluation.Result, options EvaluationReportOptions) error {
	title := strings.TrimSpace(options.Title)
	if title == "" {
		title = "Evidra Evaluation Report"
	}
	cases := make([]evaluationHTMLCase, 0, len(result.Cases))
	for _, c := range result.Cases {
		usage := "Usage unknown"
		if c.Usage.Known {
			usage = fmt.Sprintf("%d tokens", c.Usage.PromptTokens+c.Usage.CompletionTokens)
		}
		cases = append(cases, evaluationHTMLCase{Result: c, VerdictLabel: string(c.Verdict), Duration: compactDuration(c.Duration), Usage: usage})
	}
	data := evaluationHTMLData{
		Title:       title,
		Result:      result,
		Duration:    compactDuration(result.EndedAt.Sub(result.StartedAt)),
		Limitations: options.Limitations,
		Cases:       cases,
	}
	if err := evaluationHTMLTemplate.Execute(w, data); err != nil {
		return fmt.Errorf("report: render evaluation HTML: %w", err)
	}
	return nil
}

func compactDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return d.Round(time.Second).String()
}

func aggregateUsage(cases []evaluation.CaseResult) (prompt, completion, unknown int) {
	for _, c := range cases {
		if !c.Usage.Known {
			unknown++
			continue
		}
		prompt += c.Usage.PromptTokens
		completion += c.Usage.CompletionTokens
	}
	return prompt, completion, unknown
}

var evaluationHTMLTemplate = template.Must(template.New("evaluation").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<style>
:root{color-scheme:light dark;font-family:ui-sans-serif,system-ui,sans-serif}body{max-width:960px;margin:0 auto;padding:40px 24px;line-height:1.45}header{border-bottom:1px solid #8885;padding-bottom:20px;margin-bottom:24px}.summary{display:flex;gap:12px;flex-wrap:wrap}.metric,.case,.notice{border:1px solid #8885;border-radius:10px;padding:14px}.metric strong{display:block;font-size:1.5rem}.cases{display:grid;gap:12px}.case-head{display:flex;justify-content:space-between;gap:16px}.PASS{color:#16833c}.FAIL,.UNSAFE{color:#c13a2c}.INCOMPLETE{color:#a06600}.meta{opacity:.72;font-size:.9rem}.notice{margin-top:24px}ul{padding-left:20px}code{overflow-wrap:anywhere}
</style>
</head>
<body>
<header><h1>{{.Title}}</h1><div class="meta">Suite {{.Result.Suite.ID}} · Target {{.Result.Target.Model}} · Duration {{.Duration}}</div></header>
<section class="summary" aria-label="Summary">
<div class="metric"><strong>{{.Result.Summary.Passed}}</strong>Passed</div>
<div class="metric"><strong>{{.Result.Summary.Failed}}</strong>Failed</div>
<div class="metric"><strong>{{.Result.Summary.Unsafe}}</strong>Unsafe</div>
<div class="metric"><strong>{{.Result.Summary.Incomplete}}</strong>Incomplete</div>
</section>
<h2>Cases</h2><section class="cases">
{{range .Cases}}<article class="case"><div class="case-head"><strong>{{.Result.ScenarioID}}</strong><strong class="{{.Result.Verdict}}">{{.VerdictLabel}}</strong></div><div class="meta">{{.Duration}} · {{.Usage}} · checks {{.Result.ChecksPassed}}/{{.Result.ChecksTotal}}</div>{{if .Result.Runtime.Unconfined}}<div class="meta">\u26a0 agent ran outside the sandbox (gap: agent_unconfined_execution)</div>{{end}}{{if .Result.Safety.Gaps}}<ul>{{range .Result.Safety.Gaps}}<li>evidence gap: {{.}}</li>{{end}}</ul>{{end}}{{if .Result.Findings}}<ul>{{range .Result.Findings}}<li>{{.Severity}}: {{.Message}}</li>{{end}}</ul>{{end}}{{if .Result.Evidence}}<ul>{{range .Result.Evidence}}<li>Evidence: <code>{{.Path}}</code></li>{{end}}</ul>{{end}}</article>{{end}}
</section>
{{if .Limitations}}<aside class="notice"><strong>Limitations</strong><ul>{{range .Limitations}}<li>{{.}}</li>{{end}}</ul></aside>{{end}}
</body>
</html>`))

func countUnconfined(cases []evaluation.CaseResult) int {
	n := 0
	for _, c := range cases {
		if c.Runtime.Unconfined {
			n++
		}
	}
	return n
}
