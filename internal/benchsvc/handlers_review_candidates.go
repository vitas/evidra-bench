package benchsvc

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func handleListReviewCandidates(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		f := runFiltersFromRequest(r)
		f.ReviewState = "unreviewed"
		f.ReviewIncludePrivate = !isAnonymousRead(r)

		runs, total, err := svc.ListRuns(r.Context(), tenantID, f)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		candidates, err := reviewCandidatesForRuns(r, svc, tenantID, runs)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if candidates == nil {
			candidates = []bench.ReviewCandidate{}
		}
		apiutil.WriteJSON(w, http.StatusOK, bench.ReviewCandidatesResponse{
			Candidates: candidates,
			Total:      total,
			Limit:      f.Limit,
			Offset:     f.Offset,
		})
	}
}

func reviewCandidatesForRuns(r *http.Request, svc *Service, tenantID string, runs []bench.RunRecord) ([]bench.ReviewCandidate, error) {
	candidates := make([]bench.ReviewCandidate, 0, len(runs))
	for i := range runs {
		candidate, err := reviewCandidateForRun(r, svc, tenantID, runs[i])
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
	})
	return candidates, nil
}

func reviewCandidateForRun(r *http.Request, svc *Service, tenantID string, run bench.RunRecord) (bench.ReviewCandidate, error) {
	coverage, autopsyData, err := reviewCandidateArtifactCoverage(r, svc, tenantID, run.ID)
	if err != nil {
		return bench.ReviewCandidate{}, err
	}
	signals := reviewCandidateSignals(autopsyData)
	priority := reviewCandidatePriority(run, coverage, signals)
	reason := reviewCandidateReason(run, coverage, signals)
	runURL := "/v1/bench/runs/" + run.ID
	draftURL := ""
	if svc.reviewDraftsEnabled() {
		draftURL = "/v1/bench/review-candidates/" + run.ID + "/draft"
	}
	return bench.ReviewCandidate{
		RunID:            run.ID,
		ScenarioID:       run.ScenarioID,
		Model:            run.Model,
		Provider:         run.Provider,
		Passed:           run.Passed,
		CreatedAt:        run.CreatedAt,
		Priority:         priority,
		Reason:           reason,
		Signals:          signals,
		ArtifactCoverage: coverage,
		RunURL:           runURL,
		ReviewURL:        runURL + "/review",
		DraftURL:         draftURL,
	}, nil
}

func reviewCandidateArtifactCoverage(r *http.Request, svc *Service, tenantID, runID string) (bench.ReviewCandidateArtifactCoverage, []byte, error) {
	var coverage bench.ReviewCandidateArtifactCoverage
	var autopsyData []byte
	if _, ok, err := artifactExists(r, svc, tenantID, runID, artifact.HostedToolCalls); err != nil {
		return coverage, nil, err
	} else if ok {
		coverage.ToolCalls = true
	}
	if _, ok, err := artifactExists(r, svc, tenantID, runID, artifact.HostedTimeline); err != nil {
		return coverage, nil, err
	} else if ok {
		coverage.Timeline = true
	}
	if data, ok, err := artifactExists(r, svc, tenantID, runID, artifact.HostedFailureAutopsy); err != nil {
		return coverage, nil, err
	} else if ok {
		coverage.FailureAutopsy = true
		autopsyData = data
	}
	if _, ok, err := artifactExists(r, svc, tenantID, runID, artifact.HostedRunError); err != nil {
		return coverage, nil, err
	} else if ok {
		coverage.RunError = true
	}
	if _, ok, err := artifactExists(r, svc, tenantID, runID, artifact.HostedRunEvents); err != nil {
		return coverage, nil, err
	} else if ok {
		coverage.RunEvents = true
	}
	return coverage, autopsyData, nil
}

func artifactExists(r *http.Request, svc *Service, tenantID, runID, artifactType string) ([]byte, bool, error) {
	data, _, err := svc.GetArtifact(r.Context(), tenantID, runID, artifactType)
	if errors.Is(err, ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, false, nil
	}
	return data, true, nil
}

type reviewCandidateAutopsy struct {
	PrimaryFailure string `json:"primary_failure"`
	Findings       []struct {
		Kind string `json:"kind"`
	} `json:"findings"`
}

func reviewCandidateSignals(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	var report reviewCandidateAutopsy
	if err := json.Unmarshal(data, &report); err != nil {
		return []string{}
	}
	signals := make([]string, 0, 1+len(report.Findings))
	seen := map[string]struct{}{}
	addSignal := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		signals = append(signals, value)
	}
	addSignal(report.PrimaryFailure)
	for _, finding := range report.Findings {
		addSignal(finding.Kind)
	}
	return signals
}

func reviewCandidatePriority(run bench.RunRecord, coverage bench.ReviewCandidateArtifactCoverage, signals []string) int {
	priority := 10
	if !run.Passed {
		priority += 20
	}
	if coverage.FailureAutopsy {
		priority += 70
	}
	if run.Passed && coverage.FailureAutopsy {
		priority += 15
	}
	if len(signals) > 0 {
		priority += 10
	}
	if coverage.Timeline {
		priority += 8
	}
	if coverage.ToolCalls {
		priority += 5
	}
	if coverage.RunError {
		priority += 5
	}
	if coverage.RunEvents {
		priority += 2
	}
	return priority
}

func reviewCandidateReason(run bench.RunRecord, coverage bench.ReviewCandidateArtifactCoverage, signals []string) string {
	if len(signals) > 0 {
		return "Autopsy flagged " + signals[0]
	}
	if coverage.FailureAutopsy {
		return "Failure autopsy available"
	}
	if !run.Passed && coverage.Timeline {
		return "Failed run has timeline evidence"
	}
	if !run.Passed {
		return "Failed run needs final review"
	}
	if coverage.Timeline || coverage.ToolCalls {
		return "Run has artifact evidence"
	}
	return "Run needs final review"
}
