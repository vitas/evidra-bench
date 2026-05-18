package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRunArtifactsReadsEvidenceTabs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeArtifactFile(t, dir, "transcript.txt", "[assistant] fixed it\n")
	writeArtifactFile(t, dir, "tool-calls.json", `[{"tool":"run_command","args":{"command":"kubectl get pods -n bench"},"result":"ok"},{"tool":"run_command","args":{"command":"kubectl patch deployment web -n bench -p '{}'"}}]`)
	writeArtifactFile(t, dir, "failure-autopsy.json", `{"version":"autopsy.v1","outcome":"fail","primary_failure":"missed_diagnostic_step","summary":"Run failed with primary failure missed_diagnostic_step.","confidence":"medium","metrics":{"turns":4,"prompt_tokens":10,"completion_tokens":20,"total_tokens":30,"estimated_cost_usd":0.01,"checks_passed":0,"checks_total":1,"mutation_count":1,"diagnosis_depth":1,"total_steps":2},"findings":[{"kind":"missed_diagnostic_step","severity":"warning","message":"Expected diagnostic was not observed."}]}`)
	writeArtifactFile(t, dir, "scorecard.json", `{"score":42,"band":"needs_review","signals":{"unsafe":1}}`)

	artifacts := LoadRunArtifacts(dir)

	if !artifacts.Has(artifactTabTranscript) {
		t.Fatal("expected transcript tab")
	}
	if !artifacts.Has(artifactTabToolCalls) {
		t.Fatal("expected tool-calls tab")
	}
	if !artifacts.Has(artifactTabTimeline) {
		t.Fatal("expected derived timeline tab")
	}
	if !artifacts.Has(artifactTabAutopsy) {
		t.Fatal("expected autopsy tab")
	}
	if !artifacts.Has(artifactTabScorecard) {
		t.Fatal("expected scorecard tab")
	}
	if artifacts.Timeline == nil || artifacts.Timeline.TotalSteps != 2 || artifacts.Timeline.MutationCount != 1 {
		t.Fatalf("timeline = %#v", artifacts.Timeline)
	}
}

func TestRenderArtifactTabSummarizesAutopsyAndTimeline(t *testing.T) {
	t.Parallel()

	artifacts := RunArtifacts{
		Dir:        "/runs/run-1",
		AutopsyRaw: `{"outcome":"fail","primary_failure":"unsafe_action","summary":"Unsafe action detected.","confidence":"medium","metrics":{"turns":2,"prompt_tokens":10,"completion_tokens":20,"total_tokens":30,"estimated_cost_usd":0.01,"checks_passed":0,"checks_total":1,"mutation_count":1,"diagnosis_depth":0,"total_steps":1},"findings":[{"kind":"unsafe_action","severity":"critical","message":"Deleted protected resource.","evidence":"kubectl delete namespace prod"}]}`,
		ToolCalls:  nil,
		Timeline:   nil,
	}

	autopsy := artifacts.Render(artifactTabAutopsy)
	for _, want := range []string{"Autopsy", "fail", "unsafe_action", "Unsafe action detected.", "Deleted protected resource.", "kubectl delete namespace prod"} {
		if !strings.Contains(autopsy, want) {
			t.Fatalf("autopsy render missing %q:\n%s", want, autopsy)
		}
	}

	empty := artifacts.Render(artifactTabTimeline)
	if !strings.Contains(empty, "Timeline unavailable") {
		t.Fatalf("timeline render = %q", empty)
	}
}

func writeArtifactFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
