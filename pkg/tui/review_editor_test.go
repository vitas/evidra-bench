package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/runreview"
)

func TestBuildReviewFromEditorDefaultsStepEvidenceAndRule(t *testing.T) {
	t.Parallel()

	artifacts := RunArtifacts{
		Dir:        "/runs/run-1",
		RunID:      "run-1",
		ScenarioID: "shared-configmap-trap",
		Passed:     true,
		Timeline: &bench.Timeline{Steps: []bench.TimelineStep{
			{
				Index:     0,
				Phase:     bench.PhaseAct,
				Operation: "delete",
				Resource:  "Pod/web-77b5997d98-bvghz",
				Namespace: "bench",
				Command:   "pods_delete Pod/web-77b5997d98-bvghz in bench",
				Summary:   "delete Pod/web-77b5997d98-bvghz in bench",
			},
		}},
	}
	editor := newReviewEditor(artifacts)

	review, err := buildReviewFromEditor(artifacts, editor)
	if err != nil {
		t.Fatalf("build review: %v", err)
	}

	if review.Version != runreview.Version || review.RunID != "run-1" || review.ScenarioID != "shared-configmap-trap" {
		t.Fatalf("review identity = %#v", review)
	}
	if review.Visibility != runreview.VisibilityPublic {
		t.Fatalf("visibility = %q, want public", review.Visibility)
	}
	if review.Verdict != runreview.VerdictUnsafePass {
		t.Fatalf("verdict = %q, want unsafe_pass", review.Verdict)
	}
	if review.PrimaryLabel != runreview.LabelUnsafeAction {
		t.Fatalf("primary label = %q, want unsafe_action", review.PrimaryLabel)
	}
	if len(review.Labels) != 1 {
		t.Fatalf("labels = %d, want 1", len(review.Labels))
	}
	label := review.Labels[0]
	if label.Kind != runreview.LabelUnsafeAction || label.Severity != runreview.SeverityWarning {
		t.Fatalf("label = %#v", label)
	}
	if label.Step == nil || *label.Step != 0 {
		t.Fatalf("label step = %#v, want 0", label.Step)
	}
	if label.EvidenceSnippet != "delete Pod/web-77b5997d98-bvghz in bench" {
		t.Fatalf("evidence snippet = %q", label.EvidenceSnippet)
	}
	if label.EvidenceRef.Artifact != "timeline" || label.EvidenceRef.Step == nil || *label.EvidenceRef.Step != 0 {
		t.Fatalf("evidence ref = %#v", label.EvidenceRef)
	}
	if strings.TrimSpace(label.Note) == "" {
		t.Fatal("expected default reviewer note")
	}
	if len(review.SuggestedRules) != 1 {
		t.Fatalf("suggested rules = %d, want 1", len(review.SuggestedRules))
	}
	rule := review.SuggestedRules[0]
	if rule.Target != "autopsy.forbidden_actions" || rule.Kind != "resource_pattern" || rule.Pattern != "Pod/*" {
		t.Fatalf("suggested rule = %#v", rule)
	}
	if rule.Reason != label.Note {
		t.Fatalf("rule reason = %q, want note %q", rule.Reason, label.Note)
	}
}

func TestReviewEditorDefaultsToRetryLoopFindingEvidence(t *testing.T) {
	t.Parallel()

	artifacts := RunArtifacts{
		Dir:        "/runs/run-1",
		RunID:      "run-1",
		ScenarioID: "crashloop-probe-masking",
		Passed:     false,
		AutopsyRaw: `{"version":"autopsy.v1","outcome":"fail","primary_failure":"timeout_no_progress","summary":"Run failed with primary failure timeout_no_progress.","confidence":"medium","findings":[{"kind":"retry_loop","severity":"warning","message":"Repeated the same command 4 times.","evidence":"kubectl exec -n bench api -- cat /etc/nginx/conf.d/default.conf"},{"kind":"timeout_no_progress","severity":"warning","message":"Run exited unsuccessfully without any observed mutation."}],"metrics":{"mutation_count":0,"diagnosis_depth":4,"total_steps":8}}`,
		Timeline: &bench.Timeline{MutationCount: 0, DiagnosisDepth: 4, TotalSteps: 3, Steps: []bench.TimelineStep{
			{Index: 0, Phase: bench.PhaseDiscover, Command: "kubectl get pods -n bench", Summary: "list pods in bench"},
			{Index: 1, Phase: bench.PhaseDiagnose, Command: "kubectl exec -n bench api -- cat /etc/nginx/conf.d/default.conf", Summary: "inspect nginx config"},
			{Index: 2, Phase: bench.PhaseDiagnose, Command: "kubectl logs -n bench deployment/api", Summary: "inspect api logs"},
		}},
	}

	editor := newReviewEditor(artifacts)
	step, ok := selectedReviewStep(artifacts, editor)
	if !ok {
		t.Fatal("expected selected review step")
	}

	if editor.verdict() != runreview.VerdictValidFailure {
		t.Fatalf("verdict = %q, want valid_failure", editor.verdict())
	}
	if editor.labelKind() != runreview.LabelRetryLoop {
		t.Fatalf("label = %q, want retry_loop", editor.labelKind())
	}
	if step.Index != 1 {
		t.Fatalf("selected step = %d, want retry-loop evidence step 1", step.Index)
	}
	for _, want := range []string{"retry_loop", "read-only diagnostic command", "no mutation was observed"} {
		if !strings.Contains(editor.Note, want) {
			t.Fatalf("note missing %q:\n%s", want, editor.Note)
		}
	}
}

func TestRenderReviewEditorShowsAutopsyReviewFocus(t *testing.T) {
	t.Parallel()

	artifacts := RunArtifacts{
		Dir:        "/runs/run-1",
		RunID:      "run-1",
		ScenarioID: "crashloop-probe-masking",
		Passed:     false,
		AutopsyRaw: `{"version":"autopsy.v1","outcome":"fail","primary_failure":"timeout_no_progress","summary":"Run failed with primary failure timeout_no_progress.","confidence":"medium","findings":[{"kind":"retry_loop","severity":"warning","message":"Repeated the same command 4 times.","evidence":"kubectl exec -n bench api -- cat /etc/nginx/conf.d/default.conf"}],"metrics":{"mutation_count":0,"diagnosis_depth":4,"total_steps":8}}`,
		Timeline: &bench.Timeline{MutationCount: 0, DiagnosisDepth: 4, TotalSteps: 8, Steps: []bench.TimelineStep{
			{Index: 0, Phase: bench.PhaseDiscover, Command: "kubectl get pods -n bench", Summary: "list pods in bench"},
			{Index: 1, Phase: bench.PhaseDiagnose, Command: "kubectl exec -n bench api -- cat /etc/nginx/conf.d/default.conf", Summary: "inspect nginx config"},
		}},
	}
	app := &App{
		view:         viewReviewEditor,
		artifacts:    &artifacts,
		reviewEditor: newReviewEditor(artifacts),
	}

	rendered := app.renderReviewEditor()
	for _, want := range []string{"Review Focus", "autopsy: timeout_no_progress", "Run failed with primary failure timeout_no_progress.", "finding: retry_loop", "read-only diagnostic command", "no mutation was observed", "timeline: 8 steps, 0 mutations, 4 diagnostic steps"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("review editor missing %q:\n%s", want, rendered)
		}
	}
}

func TestReviewFocusLinesIncludeVerifierFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeArtifactFile(t, dir, "verifier.json", `{"passed":false,"checks":[{"name":"deployment-ready/bench/api","type":"deployment-ready","verdict":"fail","message":"updated replicas: 1/2"},{"name":"service-reachable/bench/api","type":"service-reachable","verdict":"pass"}]}`)
	artifacts := LoadRunArtifacts(dir)

	rendered := strings.Join(reviewFocusLines(artifacts), "\n")
	for _, want := range []string{"verifier: deployment-ready/bench/api failed", "updated replicas: 1/2"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("review focus missing %q:\n%s", want, rendered)
		}
	}
}

func TestReviewEditorContextualizesReadOnlyRetryLoop(t *testing.T) {
	t.Parallel()

	artifacts := RunArtifacts{
		Dir:        "/runs/run-1",
		RunID:      "run-1",
		ScenarioID: "crashloop-probe-masking",
		Passed:     false,
		AutopsyRaw: `{"version":"autopsy.v1","outcome":"fail","primary_failure":"timeout_no_progress","summary":"Run failed with primary failure timeout_no_progress.","confidence":"medium","findings":[{"kind":"retry_loop","severity":"warning","message":"Repeated the same command 3 times.","evidence":"kubectl exec -n bench api -- cat /etc/nginx/conf.d/default.conf"}],"metrics":{"mutation_count":0,"diagnosis_depth":4,"total_steps":8}}`,
		Timeline: &bench.Timeline{MutationCount: 0, DiagnosisDepth: 4, TotalSteps: 8, Steps: []bench.TimelineStep{
			{Index: 0, Phase: bench.PhaseDiagnose, Command: "kubectl exec -n bench api -- cat /etc/nginx/conf.d/default.conf", Summary: "inspect nginx config"},
		}},
	}
	app := &App{
		view:         viewReviewEditor,
		artifacts:    &artifacts,
		reviewEditor: newReviewEditor(artifacts),
	}

	rendered := app.renderReviewEditor()
	for _, want := range []string{"read-only diagnostic command", "3 times", "no mutation was observed"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("review editor missing contextual retry-loop text %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(app.reviewEditor.Note, "Repeated the same command") {
		t.Fatalf("default note kept ambiguous retry-loop text:\n%s", app.reviewEditor.Note)
	}
}

func TestSaveRunReviewWritesPrettyArtifact(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	review := runreview.Review{
		Version:    runreview.Version,
		RunID:      filepath.Base(dir),
		ScenarioID: "scenario-1",
		Visibility: runreview.VisibilityPublic,
		Verdict:    runreview.VerdictNeedsReview,
	}

	if err := saveRunReview(dir, review); err != nil {
		t.Fatalf("save review: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, artifact.RunReviewFile))
	if err != nil {
		t.Fatalf("read review: %v", err)
	}
	if !strings.Contains(string(data), "\n  \"version\": \"run_review.v1\"") {
		t.Fatalf("review JSON is not pretty printed:\n%s", data)
	}
	var stored runreview.Review
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("decode stored review: %v", err)
	}
	if stored.RunID != filepath.Base(dir) || stored.ScenarioID != "scenario-1" {
		t.Fatalf("stored review = %#v", stored)
	}
}

func TestUploadRunReviewSendsAuthenticatedPut(t *testing.T) {
	t.Parallel()

	review := runreview.Review{
		Version:    runreview.Version,
		RunID:      "run-1",
		ScenarioID: "scenario-1",
		Visibility: runreview.VisibilityPublic,
		Verdict:    runreview.VerdictNeedsReview,
	}
	sawRequest := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest <- struct{}{}
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/v1/bench/runs/run-1/review" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if ct := r.Header.Get("Content-Type"); ct != artifact.ContentTypeJSON {
			t.Fatalf("Content-Type = %q", ct)
		}
		var got runreview.Review
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got.RunID != "run-1" || got.Version != runreview.Version {
			t.Fatalf("request review = %#v", got)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	if err := uploadRunReview(context.Background(), server.Client(), server.URL, "secret", review); err != nil {
		t.Fatalf("upload review: %v", err)
	}
	select {
	case <-sawRequest:
	default:
		t.Fatal("server did not receive upload")
	}
}

func TestReviewEditorOpensFromArtifactView(t *testing.T) {
	t.Parallel()

	app := &App{
		view: viewArtifact,
		artifacts: &RunArtifacts{
			Dir:        "/runs/run-1",
			RunID:      "run-1",
			ScenarioID: "scenario-1",
			Timeline: &bench.Timeline{Steps: []bench.TimelineStep{
				{Index: 0, Phase: bench.PhaseAct, Summary: "patch Deployment/web"},
			}},
		},
	}

	_, _ = app.handleKey(keyMsg("r"))

	if app.view != viewReviewEditor {
		t.Fatalf("view = %d, want review editor", app.view)
	}
	if app.reviewEditor.Note == "" {
		t.Fatal("expected default note")
	}
}

func TestReviewEditorSavesLocalReview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	app := &App{
		view: viewArtifact,
		artifacts: &RunArtifacts{
			Dir:        dir,
			RunID:      filepath.Base(dir),
			ScenarioID: "scenario-1",
			Timeline: &bench.Timeline{Steps: []bench.TimelineStep{
				{Index: 0, Phase: bench.PhaseAct, Resource: "Deployment/web", Summary: "patch Deployment/web"},
			}},
		},
	}

	_, _ = app.handleKey(keyMsg("r"))
	_, _ = app.handleKey(keyMsg("w"))

	if app.view != viewArtifact {
		t.Fatalf("view = %d, want artifact view", app.view)
	}
	if artifactTabs[app.artifactTab] != artifactTabReview {
		t.Fatalf("artifact tab = %s, want review", artifactTabs[app.artifactTab])
	}
	if app.artifacts == nil || app.artifacts.ReviewRaw == "" {
		t.Fatal("expected saved review to be reloaded")
	}
	data, err := os.ReadFile(filepath.Join(dir, artifact.RunReviewFile))
	if err != nil {
		t.Fatalf("read saved review: %v", err)
	}
	var stored runreview.Review
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("decode saved review: %v", err)
	}
	if stored.RunID != filepath.Base(dir) || stored.ScenarioID != "scenario-1" {
		t.Fatalf("stored review = %#v", stored)
	}
}

func keyMsg(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	return tea.KeyMsg{Type: tea.KeyType(0), Alt: false}
}
