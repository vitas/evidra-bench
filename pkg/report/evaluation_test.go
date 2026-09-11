package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/evaluation"
)

func sampleEvaluationResult() evaluation.Result {
	return evaluation.Result{
		Version:     evaluation.ResultVersion,
		ID:          "eval-1",
		Suite:       evaluation.SuitePlan{ID: "kubernetes-demo@1"},
		Target:      evaluation.TargetPlan{Kind: evaluation.TargetModel, Provider: "openai-compatible", Model: "test-model"},
		StartedAt:   time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC),
		EndedAt:     time.Date(2026, 9, 9, 10, 1, 30, 0, time.UTC),
		Termination: evaluation.Termination{Kind: evaluation.TerminationComplete},
		Cleanup:     evaluation.CleanupResult{Attempted: true, Succeeded: true},
		Cases: []evaluation.CaseResult{
			{ScenarioID: "repair", Verdict: evaluation.VerdictPass, Duration: 42 * time.Second, ChecksPassed: 2, ChecksTotal: 2, Usage: evaluation.Usage{Known: true, PromptTokens: 10, CompletionTokens: 5}},
			{ScenarioID: "scope-<script>alert(1)</script>", Verdict: evaluation.VerdictUnsafe, Duration: 37 * time.Second, Usage: evaluation.Usage{Known: false}, Findings: []evaluation.SafetyFinding{{Kind: "scope", Severity: evaluation.SeverityCritical, Measured: true, Message: "neighbor changed"}}},
			{ScenarioID: "timeout", Verdict: evaluation.VerdictIncomplete, Duration: 11 * time.Second, Usage: evaluation.Usage{Known: false}},
		},
		Summary: evaluation.Summary{Total: 3, Passed: 1, Unsafe: 1, Incomplete: 1},
	}
}

func TestRenderEvaluationJSON_RoundTripsCanonicalResult(t *testing.T) {
	var out bytes.Buffer
	want := sampleEvaluationResult()
	if err := RenderEvaluationJSON(&out, want); err != nil {
		t.Fatalf("RenderEvaluationJSON() error = %v", err)
	}
	var got evaluation.Result
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("rendered JSON is invalid: %v", err)
	}
	if got.ID != want.ID || got.Summary != want.Summary || len(got.Cases) != len(want.Cases) {
		t.Fatalf("round trip changed canonical result: %#v", got)
	}
}

func TestRenderEvaluationTerminal_UsesCanonicalVerdictsAndUnknownUsage(t *testing.T) {
	var out bytes.Buffer
	if err := RenderEvaluationTerminal(&out, sampleEvaluationResult()); err != nil {
		t.Fatalf("RenderEvaluationTerminal() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"repair", "PASS", "UNSAFE", "INCOMPLETE", "1 passed", "1 unsafe", "1 incomplete", "Usage: unknown"} {
		if !strings.Contains(got, want) {
			t.Errorf("terminal output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderEvaluationHTML_IsStandaloneEscapedAndHonest(t *testing.T) {
	var out bytes.Buffer
	err := RenderEvaluationHTML(&out, sampleEvaluationResult(), EvaluationReportOptions{
		Title:       "Evidra Infrastructure Agent Tests",
		Limitations: []string{"Starter coverage only; not production certification."},
	})
	if err != nil {
		t.Fatalf("RenderEvaluationHTML() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{"<!doctype html>", "UNSAFE", "INCOMPLETE", "Usage unknown", "Starter coverage only", "&lt;script&gt;"} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	if strings.Contains(got, "<script>alert(1)</script>") {
		t.Fatal("HTML contains unescaped scenario content")
	}
	if strings.Contains(got, "http://") || strings.Contains(got, "https://") {
		t.Fatal("HTML report must not depend on network assets")
	}
}

