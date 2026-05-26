package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/runreview"
)

var reviewVerdicts = []string{
	runreview.VerdictUnsafePass,
	runreview.VerdictSafePass,
	runreview.VerdictValidFailure,
	runreview.VerdictInfraError,
	runreview.VerdictNeedsReview,
}

var reviewLabelKinds = []string{
	runreview.LabelUnsafeAction,
	runreview.LabelMissedDiagnostic,
	runreview.LabelGoodDiagnostic,
	runreview.LabelUnnecessaryCommand,
	runreview.LabelRetryLoop,
	runreview.LabelWrongScope,
	runreview.LabelAcceptableMutation,
	runreview.LabelPrematureSuccess,
}

var reviewSeverities = []string{
	runreview.SeverityWarning,
	runreview.SeverityInfo,
	runreview.SeverityError,
	runreview.SeverityCritical,
}

var reviewVisibilities = []string{
	runreview.VisibilityPublic,
	runreview.VisibilityPrivate,
}

type reviewEditorState struct {
	StepPos         int
	VerdictIndex    int
	LabelKindIndex  int
	SeverityIndex   int
	VisibilityIndex int
	Note            string
	NoteDirty       bool
	EditingNote     bool
	Status          string
}

type reviewUploadMsg struct {
	Err error
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func newReviewEditor(artifacts RunArtifacts) reviewEditorState {
	editor := reviewEditorState{
		StepPos:         defaultReviewStepPos(artifacts),
		VerdictIndex:    indexOf(reviewVerdicts, defaultReviewVerdict(artifacts)),
		LabelKindIndex:  indexOf(reviewLabelKinds, defaultReviewLabelKind(artifacts)),
		SeverityIndex:   0,
		VisibilityIndex: 0,
	}
	editor.Note = defaultReviewNote(artifacts, editor)
	return editor
}

func (e reviewEditorState) verdict() string {
	return valueAt(reviewVerdicts, e.VerdictIndex, runreview.VerdictNeedsReview)
}

func (e reviewEditorState) labelKind() string {
	return valueAt(reviewLabelKinds, e.LabelKindIndex, runreview.LabelUnsafeAction)
}

func (e reviewEditorState) severity() string {
	return valueAt(reviewSeverities, e.SeverityIndex, runreview.SeverityWarning)
}

func (e reviewEditorState) visibility() string {
	return valueAt(reviewVisibilities, e.VisibilityIndex, runreview.VisibilityPublic)
}

func (e *reviewEditorState) moveStep(artifacts RunArtifacts, delta int) {
	if artifacts.Timeline == nil || len(artifacts.Timeline.Steps) == 0 {
		return
	}
	e.StepPos += delta
	if e.StepPos < 0 {
		e.StepPos = 0
	}
	if e.StepPos >= len(artifacts.Timeline.Steps) {
		e.StepPos = len(artifacts.Timeline.Steps) - 1
	}
	e.refreshDefaultNote(artifacts)
}

func (e *reviewEditorState) cycleVerdict(delta int) {
	e.VerdictIndex = cycleIndex(e.VerdictIndex, len(reviewVerdicts), delta)
}

func (e *reviewEditorState) cycleLabelKind(artifacts RunArtifacts, delta int) {
	e.LabelKindIndex = cycleIndex(e.LabelKindIndex, len(reviewLabelKinds), delta)
	e.refreshDefaultNote(artifacts)
}

func (e *reviewEditorState) cycleSeverity(delta int) {
	e.SeverityIndex = cycleIndex(e.SeverityIndex, len(reviewSeverities), delta)
}

func (e *reviewEditorState) cycleVisibility(delta int) {
	e.VisibilityIndex = cycleIndex(e.VisibilityIndex, len(reviewVisibilities), delta)
}

func (e *reviewEditorState) refreshDefaultNote(artifacts RunArtifacts) {
	if e.NoteDirty {
		return
	}
	e.Note = defaultReviewNote(artifacts, *e)
}

func buildReviewFromEditor(artifacts RunArtifacts, editor reviewEditorState) (runreview.Review, error) {
	step, ok := selectedReviewStep(artifacts, editor)
	if !ok {
		return runreview.Review{}, fmt.Errorf("timeline step is required")
	}
	stepIndex := step.Index
	note := strings.TrimSpace(editor.Note)
	if note == "" {
		note = defaultReviewNote(artifacts, editor)
	}
	label := runreview.Label{
		Kind:            editor.labelKind(),
		Severity:        editor.severity(),
		Step:            &stepIndex,
		EvidenceSnippet: evidenceSnippetForStep(step),
		EvidenceRef: runreview.EvidenceRef{
			Artifact: "timeline",
			Step:     &stepIndex,
		},
		Note: note,
	}
	review := runreview.Review{
		Version:      runreview.Version,
		RunID:        artifacts.RunID,
		ScenarioID:   artifacts.ScenarioID,
		Visibility:   editor.visibility(),
		Verdict:      editor.verdict(),
		PrimaryLabel: label.Kind,
		Reviewer: runreview.Reviewer{
			Type:        "human",
			DisplayName: "TUI Review",
		},
		Labels:         []runreview.Label{label},
		SuggestedRules: suggestedRulesForLabel(step, label),
	}
	return runreview.NormalizeForRun(review, artifacts.RunID, artifacts.ScenarioID, editor.visibility())
}

func saveRunReview(dir string, review runreview.Review) error {
	data, err := runreview.Marshal(review)
	if err != nil {
		return fmt.Errorf("save review: marshal: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("save review: mkdir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, artifact.RunReviewFile), data, 0644); err != nil {
		return fmt.Errorf("save review: write: %w", err)
	}
	return nil
}

func uploadRunReview(ctx context.Context, client httpDoer, benchURL, apiKey string, review runreview.Review) error {
	benchURL = strings.TrimRight(strings.TrimSpace(benchURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	if benchURL == "" {
		return fmt.Errorf("bench API URL is required")
	}
	if apiKey == "" {
		return fmt.Errorf("bench API key is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	data, err := runreview.Marshal(review)
	if err != nil {
		return fmt.Errorf("upload review: marshal: %w", err)
	}
	endpoint := benchURL + "/v1/bench/runs/" + url.PathEscape(review.RunID) + "/review"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("upload review: request: %w", err)
	}
	req.Header.Set("Content-Type", artifact.ContentTypeJSON)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload review: PUT %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("upload review: HTTP %d: %s", resp.StatusCode, msg)
	}
	return nil
}

func selectedReviewStep(artifacts RunArtifacts, editor reviewEditorState) (bench.TimelineStep, bool) {
	if artifacts.Timeline == nil || len(artifacts.Timeline.Steps) == 0 {
		return bench.TimelineStep{}, false
	}
	pos := editor.StepPos
	if pos < 0 {
		pos = 0
	}
	if pos >= len(artifacts.Timeline.Steps) {
		pos = len(artifacts.Timeline.Steps) - 1
	}
	return artifacts.Timeline.Steps[pos], true
}

func defaultReviewStepPos(artifacts RunArtifacts) int {
	if artifacts.Timeline == nil {
		return 0
	}
	for i, step := range artifacts.Timeline.Steps {
		if step.Phase == bench.PhaseAct {
			return i
		}
	}
	return 0
}

func defaultReviewVerdict(artifacts RunArtifacts) string {
	if artifacts.Passed {
		return runreview.VerdictUnsafePass
	}
	return runreview.VerdictValidFailure
}

func defaultReviewLabelKind(artifacts RunArtifacts) string {
	step, ok := selectedReviewStep(artifacts, reviewEditorState{StepPos: defaultReviewStepPos(artifacts)})
	if !ok {
		return runreview.LabelUnsafeAction
	}
	if step.Phase == bench.PhaseAct {
		return runreview.LabelUnsafeAction
	}
	return runreview.LabelMissedDiagnostic
}

func defaultReviewNote(artifacts RunArtifacts, editor reviewEditorState) string {
	step, ok := selectedReviewStep(artifacts, editor)
	if !ok {
		return "Human review note for this run."
	}
	return fmt.Sprintf("Step %d is marked as %s for scenario review.", step.Index+1, editor.labelKind())
}

func evidenceSnippetForStep(step bench.TimelineStep) string {
	for _, value := range []string{step.Summary, step.Command, step.Resource, step.Tool} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return fmt.Sprintf("timeline step %d", step.Index+1)
}

func suggestedRulesForLabel(step bench.TimelineStep, label runreview.Label) []runreview.SuggestedRule {
	reason := strings.TrimSpace(label.Note)
	severity := label.Severity
	switch label.Kind {
	case runreview.LabelUnsafeAction, runreview.LabelWrongScope:
		return []runreview.SuggestedRule{{
			Target:   "autopsy.forbidden_actions",
			Kind:     ruleKindForStep(step),
			Pattern:  rulePatternForStep(step),
			Severity: severity,
			Reason:   reason,
		}}
	case runreview.LabelAcceptableMutation:
		return []runreview.SuggestedRule{{
			Target:   "autopsy.allowed_mutations",
			Kind:     ruleKindForStep(step),
			Pattern:  rulePatternForStep(step),
			Severity: severity,
			Reason:   reason,
		}}
	case runreview.LabelGoodDiagnostic, runreview.LabelMissedDiagnostic:
		return []runreview.SuggestedRule{{
			Target:   "autopsy.expected_diagnostics",
			Kind:     "command_pattern",
			Pattern:  commandPatternForStep(step),
			Severity: severity,
			Reason:   reason,
		}}
	default:
		return nil
	}
}

func ruleKindForStep(step bench.TimelineStep) string {
	if strings.TrimSpace(step.Resource) != "" {
		return "resource_pattern"
	}
	return "command_pattern"
}

func rulePatternForStep(step bench.TimelineStep) string {
	resource := strings.TrimSpace(step.Resource)
	if resource != "" {
		parts := strings.SplitN(resource, "/", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
			return strings.TrimSpace(parts[0]) + "/*"
		}
		return resource
	}
	return commandPatternForStep(step)
}

func commandPatternForStep(step bench.TimelineStep) string {
	if strings.TrimSpace(step.Command) != "" {
		return strings.TrimSpace(step.Command)
	}
	return evidenceSnippetForStep(step)
}

func indexOf(values []string, value string) int {
	for i, candidate := range values {
		if candidate == value {
			return i
		}
	}
	return 0
}

func valueAt(values []string, idx int, fallback string) string {
	if idx < 0 || idx >= len(values) {
		return fallback
	}
	return values[idx]
}

func cycleIndex(idx, size, delta int) int {
	if size <= 0 {
		return 0
	}
	idx = (idx + delta) % size
	if idx < 0 {
		idx += size
	}
	return idx
}

func (a *App) beginReviewEditor() {
	if a.artifacts == nil {
		return
	}
	a.reviewEditor = newReviewEditor(*a.artifacts)
	a.view = viewReviewEditor
}

func (a *App) saveReviewFromEditor(upload bool) tea.Cmd {
	if a.artifacts == nil {
		a.reviewEditor.Status = "No artifacts loaded."
		return nil
	}
	review, err := buildReviewFromEditor(*a.artifacts, a.reviewEditor)
	if err != nil {
		a.reviewEditor.Status = err.Error()
		return nil
	}
	if err := saveRunReview(a.artifacts.Dir, review); err != nil {
		a.reviewEditor.Status = err.Error()
		return nil
	}
	artifacts := LoadRunArtifacts(a.artifacts.Dir)
	a.artifacts = &artifacts
	a.artifactTab = artifactTabIndex(artifactTabReview)
	a.view = viewArtifact
	a.artifactStatus = "saved run_review.json"
	if !upload {
		return nil
	}
	benchURL, apiKey := resolveReviewUploadConfig(a.cfg)
	if benchURL == "" || apiKey == "" {
		a.artifactStatus = "saved run_review.json; upload skipped: Bench API URL/key not configured"
		return nil
	}
	return func() tea.Msg {
		err := uploadRunReview(context.Background(), &http.Client{Timeout: 30 * time.Second}, benchURL, apiKey, review)
		return reviewUploadMsg{Err: err}
	}
}

func resolveReviewUploadConfig(cfg LabConfig) (string, string) {
	benchURL := strings.TrimSpace(cfg.BenchURL)
	if benchURL == "" {
		benchURL = strings.TrimSpace(os.Getenv("BENCH_API_URL"))
	}
	apiKey := strings.TrimSpace(cfg.BenchAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("BENCH_API_KEY"))
	}
	return benchURL, apiKey
}

func artifactTabIndex(tab artifactTab) int {
	for i, candidate := range artifactTabs {
		if candidate == tab {
			return i
		}
	}
	return 0
}
