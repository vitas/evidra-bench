//go:build ignore

// generate-core-report converts a kubernetes-core pack run directory into the
// typed TypeScript module the public report page renders.
//
// Usage:
//
//	go run scripts/generate-core-report.go \
//	  -dir runs/public-2026-09-13-core-gemini-flash \
//	  -out ui/src/data/coreReport.ts \
//	  -report-id kubernetes-core-v1-2026-09 \
//	  -label "Gemini 2.5 Flash (free tier)" \
//	  -model gemini-2.5-flash -provider openai-compatible
//
// The generator reads the per-case run.json artifacts the harness writes as
// each case completes. It never invents a verdict: every field it emits comes
// from the run's own evidence, and a run directory without per-case artifacts
// fails loudly instead of producing an empty report.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type auditInfo struct {
	Coverage   string `json:"coverage"`
	Events     int    `json:"events"`
	DigestHash string `json:"digest_sha256"`
}

type snapshotInfo struct {
	Coverage        string   `json:"coverage"`
	BaselineDigest  string   `json:"baseline_digest"`
	PostAgentDigest string   `json:"post_agent_digest"`
	StabilityDigest string   `json:"stability_digest"`
	AllowedChanges  int      `json:"allowed_changes"`
	Files           []string `json:"files"`
}

type autopsyMetrics struct {
	Turns            int     `json:"turns"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	ChecksPassed     int     `json:"checks_passed"`
	ChecksTotal      int     `json:"checks_total"`
	MutationCount    int     `json:"mutation_count"`
	DiagnosisDepth   int     `json:"diagnosis_depth"`
	TotalSteps       int     `json:"total_steps"`
}

type autopsy struct {
	Version    string          `json:"version"`
	Outcome    string          `json:"outcome"`
	Summary    string          `json:"summary"`
	Confidence string          `json:"confidence"`
	Metrics    autopsyMetrics  `json:"metrics"`
	Findings   json.RawMessage `json:"findings"`
}

type checkItem struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Verdict string `json:"verdict"`
	Message string `json:"message"`
}

type checks struct {
	Passed bool        `json:"passed"`
	Items  []checkItem `json:"checks"`
}

type runFile struct {
	RunID      string       `json:"run_id"`
	ScenarioID string       `json:"scenario_id"`
	Verdict    string       `json:"verdict"`
	Passed     bool         `json:"passed"`
	StartTime  string       `json:"start_time"`
	EndTime    string       `json:"end_time"`
	ExitCode   int          `json:"exit_code"`
	Audit      auditInfo    `json:"audit"`
	Snapshots  snapshotInfo `json:"snapshots"`
	Autopsy    autopsy      `json:"autopsy"`
	Checks     checks       `json:"checks"`
}

type reportCase struct {
	ID               string
	RunID            string
	Verdict          string
	Passed           bool
	Start            string
	End              string
	DurationSeconds  float64
	AuditCoverage    string
	AuditEvents      int
	AuditDigest      string
	SnapshotCoverage string
	BaselineDigest   string
	PostAgentDigest  string
	StabilityDigest  string
	AllowedChanges   int
	Turns            int
	TotalSteps       int
	DiagnosisDepth   int
	MutationCount    int
	PromptTokens     int
	CompletionTokens int
	EstimatedCost    float64
	ChecksPassed     int
	ChecksTotal      int
	Checks           []checkItem
	AutopsyOutcome   string
	AutopsySummary   string
	AutopsyConf      string
	ArtifactDir      string
}

func main() {
	dir := flag.String("dir", "", "run output directory (required)")
	out := flag.String("out", "ui/src/data/coreReport.ts", "generated TypeScript module")
	reportID := flag.String("report-id", "kubernetes-core-v1", "public report id")
	label := flag.String("label", "", "human label for the evaluated target (required)")
	model := flag.String("model", "", "model id (required)")
	provider := flag.String("provider", "", "provider id")
	suite := flag.String("suite", "kubernetes-core@1", "suite the run executed")
	environment := flag.String("environment", "kind", "cluster provider the run used")
	endpoint := flag.String("endpoint", "", "OpenAI-compatible endpoint used, when not an official provider")
	notes := flag.String("notes", "", "operator note rendered with the report")
	flag.Parse()

	if *dir == "" || *label == "" || *model == "" {
		fmt.Fprintln(os.Stderr, "generate-core-report: -dir, -label and -model are required")
		os.Exit(2)
	}

	pattern := filepath.Join(*dir, "runs", "*", "run.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate-core-report: glob %s: %v\n", pattern, err)
		os.Exit(1)
	}
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "generate-core-report: no per-case run.json under %s\n", pattern)
		os.Exit(1)
	}

	cases := make([]reportCase, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate-core-report: read %s: %v\n", path, err)
			os.Exit(1)
		}
		var run runFile
		if err := json.Unmarshal(raw, &run); err != nil {
			fmt.Fprintf(os.Stderr, "generate-core-report: parse %s: %v\n", path, err)
			os.Exit(1)
		}
		if run.ScenarioID == "" {
			fmt.Fprintf(os.Stderr, "generate-core-report: %s has no scenario_id\n", path)
			os.Exit(1)
		}

		item := reportCase{
			ID:               run.ScenarioID,
			RunID:            run.RunID,
			Verdict:          strings.ToUpper(strings.TrimSpace(run.Verdict)),
			Passed:           run.Passed,
			Start:            run.StartTime,
			End:              run.EndTime,
			AuditCoverage:    run.Audit.Coverage,
			AuditEvents:      run.Audit.Events,
			AuditDigest:      run.Audit.DigestHash,
			SnapshotCoverage: run.Snapshots.Coverage,
			BaselineDigest:   run.Snapshots.BaselineDigest,
			PostAgentDigest:  run.Snapshots.PostAgentDigest,
			StabilityDigest:  run.Snapshots.StabilityDigest,
			AllowedChanges:   run.Snapshots.AllowedChanges,
			Turns:            run.Autopsy.Metrics.Turns,
			TotalSteps:       run.Autopsy.Metrics.TotalSteps,
			DiagnosisDepth:   run.Autopsy.Metrics.DiagnosisDepth,
			MutationCount:    run.Autopsy.Metrics.MutationCount,
			PromptTokens:     run.Autopsy.Metrics.PromptTokens,
			CompletionTokens: run.Autopsy.Metrics.CompletionTokens,
			EstimatedCost:    run.Autopsy.Metrics.EstimatedCostUSD,
			ChecksPassed:     run.Autopsy.Metrics.ChecksPassed,
			ChecksTotal:      run.Autopsy.Metrics.ChecksTotal,
			Checks:           run.Checks.Items,
			AutopsyOutcome:   run.Autopsy.Outcome,
			AutopsySummary:   run.Autopsy.Summary,
			AutopsyConf:      run.Autopsy.Confidence,
			ArtifactDir:      filepath.ToSlash(path[:len(path)-len("/run.json")]),
		}
		if item.ChecksTotal == 0 {
			item.ChecksTotal = len(run.Checks.Items)
		}
		if item.ChecksPassed == 0 {
			for _, c := range run.Checks.Items {
				if strings.EqualFold(c.Verdict, "pass") {
					item.ChecksPassed++
				}
			}
		}
		if item.Verdict == "" {
			fmt.Fprintf(os.Stderr, "generate-core-report: %s has no verdict\n", path)
			os.Exit(1)
		}
		item.DurationSeconds = durationSeconds(run.StartTime, run.EndTime)
		cases = append(cases, item)
	}

	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })

	var builder strings.Builder
	builder.WriteString("// Auto-generated by: go run scripts/generate-core-report.go\n")
	builder.WriteString("// Do not edit manually — rerun the generator against the run directory.\n\n")
	builder.WriteString("export interface CoreReportCheck {\n  name: string;\n  type: string;\n  verdict: string;\n  message?: string;\n}\n\n")
	builder.WriteString("export interface CoreReportCase {\n")
	for _, line := range []string{
		"  id: string;",
		"  runId: string;",
		"  verdict: \"PASS\" | \"FAIL\" | \"UNSAFE\" | \"INCOMPLETE\" | string;",
		"  passed: boolean;",
		"  start: string;",
		"  end: string;",
		"  durationSeconds: number;",
		"  evidence: {",
		"    auditCoverage: string;",
		"    auditEvents: number;",
		"    auditDigest: string;",
		"    snapshotCoverage: string;",
		"    baselineDigest: string;",
		"    postAgentDigest: string;",
		"    stabilityDigest: string;",
		"    allowedChanges: number;",
		"  };",
		"  behavior: {",
		"    turns: number;",
		"    totalSteps: number;",
		"    diagnosisDepth: number;",
		"    mutationCount: number;",
		"    promptTokens: number;",
		"    completionTokens: number;",
		"    estimatedCostUsd: number;",
		"  };",
		"  checks: { passed: number; total: number; items: CoreReportCheck[] };",
		"  autopsy: { outcome: string; confidence: string; summary: string };",
		"  artifactDir: string;",
	} {
		builder.WriteString(line + "\n")
	}
	builder.WriteString("}\n\n")
	builder.WriteString("export interface CoreReport {\n  id: string;\n  label: string;\n  suite: string;\n  environment: string;\n  model: string;\n  provider: string;\n  endpoint: string;\n  notes: string;\n  generatedAt: string;\n  cases: CoreReportCase[];\n}\n\n")

	fmt.Fprintf(&builder, "export const CORE_REPORT: CoreReport = {\n")
	fmt.Fprintf(&builder, "  id: %s,\n", tsString(*reportID))
	fmt.Fprintf(&builder, "  label: %s,\n", tsString(*label))
	fmt.Fprintf(&builder, "  suite: %s,\n", tsString(*suite))
	fmt.Fprintf(&builder, "  environment: %s,\n", tsString(*environment))
	fmt.Fprintf(&builder, "  model: %s,\n", tsString(*model))
	fmt.Fprintf(&builder, "  provider: %s,\n", tsString(*provider))
	fmt.Fprintf(&builder, "  endpoint: %s,\n", tsString(*endpoint))
	fmt.Fprintf(&builder, "  notes: %s,\n", tsString(*notes))
	fmt.Fprintf(&builder, "  generatedAt: %s,\n", tsString(time.Now().UTC().Format(time.RFC3339)))
	builder.WriteString("  cases: [\n")
	for _, c := range cases {
		builder.WriteString("    {\n")
		fmt.Fprintf(&builder, "      id: %s,\n", tsString(c.ID))
		fmt.Fprintf(&builder, "      runId: %s,\n", tsString(c.RunID))
		fmt.Fprintf(&builder, "      verdict: %s,\n", tsString(c.Verdict))
		fmt.Fprintf(&builder, "      passed: %t,\n", c.Passed)
		fmt.Fprintf(&builder, "      start: %s,\n", tsString(c.Start))
		fmt.Fprintf(&builder, "      end: %s,\n", tsString(c.End))
		fmt.Fprintf(&builder, "      durationSeconds: %s,\n", tsNumber(c.DurationSeconds))
		builder.WriteString("      evidence: {\n")
		fmt.Fprintf(&builder, "        auditCoverage: %s,\n", tsString(c.AuditCoverage))
		fmt.Fprintf(&builder, "        auditEvents: %d,\n", c.AuditEvents)
		fmt.Fprintf(&builder, "        auditDigest: %s,\n", tsString(c.AuditDigest))
		fmt.Fprintf(&builder, "        snapshotCoverage: %s,\n", tsString(c.SnapshotCoverage))
		fmt.Fprintf(&builder, "        baselineDigest: %s,\n", tsString(c.BaselineDigest))
		fmt.Fprintf(&builder, "        postAgentDigest: %s,\n", tsString(c.PostAgentDigest))
		fmt.Fprintf(&builder, "        stabilityDigest: %s,\n", tsString(c.StabilityDigest))
		fmt.Fprintf(&builder, "        allowedChanges: %d,\n", c.AllowedChanges)
		builder.WriteString("      },\n")
		builder.WriteString("      behavior: {\n")
		fmt.Fprintf(&builder, "        turns: %d,\n", c.Turns)
		fmt.Fprintf(&builder, "        totalSteps: %d,\n", c.TotalSteps)
		fmt.Fprintf(&builder, "        diagnosisDepth: %d,\n", c.DiagnosisDepth)
		fmt.Fprintf(&builder, "        mutationCount: %d,\n", c.MutationCount)
		fmt.Fprintf(&builder, "        promptTokens: %d,\n", c.PromptTokens)
		fmt.Fprintf(&builder, "        completionTokens: %d,\n", c.CompletionTokens)
		fmt.Fprintf(&builder, "        estimatedCostUsd: %s,\n", tsNumber(c.EstimatedCost))
		builder.WriteString("      },\n")
		builder.WriteString("      checks: {\n")
		fmt.Fprintf(&builder, "        passed: %d,\n", c.ChecksPassed)
		fmt.Fprintf(&builder, "        total: %d,\n", c.ChecksTotal)
		builder.WriteString("        items: [\n")
		for _, item := range c.Checks {
			builder.WriteString("          { ")
			fmt.Fprintf(&builder, "name: %s, type: %s, verdict: %s", tsString(item.Name), tsString(item.Type), tsString(item.Verdict))
			if strings.TrimSpace(item.Message) != "" {
				fmt.Fprintf(&builder, ", message: %s", tsString(truncate(item.Message, 4000)))
			}
			builder.WriteString(" },\n")
		}
		builder.WriteString("        ],\n")
		builder.WriteString("      },\n")
		fmt.Fprintf(&builder, "      autopsy: { outcome: %s, confidence: %s, summary: %s },\n",
			tsString(c.AutopsyOutcome), tsString(c.AutopsyConf), tsString(c.AutopsySummary))
		fmt.Fprintf(&builder, "      artifactDir: %s,\n", tsString(c.ArtifactDir))
		builder.WriteString("    },\n")
	}
	builder.WriteString("  ],\n};\n")

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "generate-core-report: mkdir %s: %v\n", filepath.Dir(*out), err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, []byte(builder.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "generate-core-report: write %s: %v\n", *out, err)
		os.Exit(1)
	}

	verdicts := map[string]int{}
	for _, c := range cases {
		verdicts[c.Verdict]++
	}
	fmt.Printf("generate-core-report: %d cases -> %s (%s)\n", len(cases), *out, verdictCounts(verdicts))
}

func durationSeconds(start, end string) float64 {
	s, err := time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return 0
	}
	e, err := time.Parse(time.RFC3339Nano, end)
	if err != nil {
		return 0
	}
	return e.Sub(s).Seconds()
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func verdictCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	return strings.Join(parts, " ")
}

func tsString(value string) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(raw)
}

func tsStrings(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, tsString(v))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func tsNumber(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", value), "0"), ".")
}
