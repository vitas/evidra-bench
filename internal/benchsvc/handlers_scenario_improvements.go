package benchsvc

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/runreview"
)

func handleListScenarioImprovements(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		f := runFiltersFromRequest(r)
		f.ReviewState = "reviewed"
		f.ReviewHasSuggestedRules = true
		f.ReviewIncludePrivate = !isAnonymousRead(r)

		runs, total, err := svc.ListRuns(r.Context(), tenantID, f)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		improvements, err := scenarioImprovementsForRuns(r, svc, tenantID, runs)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if improvements == nil {
			improvements = []bench.ScenarioImprovement{}
		}
		apiutil.WriteJSON(w, http.StatusOK, bench.ScenarioImprovementsResponse{
			Improvements: improvements,
			Total:        total,
			Limit:        f.Limit,
			Offset:       f.Offset,
		})
	}
}

func scenarioImprovementsForRuns(r *http.Request, svc *Service, tenantID string, runs []bench.RunRecord) ([]bench.ScenarioImprovement, error) {
	improvements := make([]bench.ScenarioImprovement, 0, len(runs))
	for i := range runs {
		improvement, ok, err := scenarioImprovementForRun(r, svc, tenantID, runs[i])
		if err != nil {
			return nil, err
		}
		if ok {
			improvements = append(improvements, improvement)
		}
	}
	return improvements, nil
}

func scenarioImprovementForRun(r *http.Request, svc *Service, tenantID string, run bench.RunRecord) (bench.ScenarioImprovement, bool, error) {
	if strings.TrimSpace(run.ID) == "" {
		return bench.ScenarioImprovement{}, false, nil
	}
	data, _, err := svc.GetArtifact(r.Context(), tenantID, run.ID, artifact.HostedRunReview)
	if errors.Is(err, ErrNotFound) {
		return bench.ScenarioImprovement{}, false, nil
	}
	if err != nil {
		return bench.ScenarioImprovement{}, false, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return bench.ScenarioImprovement{}, false, nil
	}
	review, err := runreview.Decode(data)
	if err != nil {
		return bench.ScenarioImprovement{}, false, err
	}
	if isAnonymousRead(r) && !runreview.IsPublic(review) {
		return bench.ScenarioImprovement{}, false, nil
	}
	if len(review.SuggestedRules) == 0 {
		return bench.ScenarioImprovement{}, false, nil
	}

	primaryLabel := review.PrimaryLabel
	if primaryLabel == "" && len(review.Labels) > 0 {
		primaryLabel = review.Labels[0].Kind
	}
	runURL := "/v1/bench/runs/" + run.ID
	return bench.ScenarioImprovement{
		RunID:                  run.ID,
		ScenarioID:             run.ScenarioID,
		Model:                  run.Model,
		Provider:               run.Provider,
		Passed:                 run.Passed,
		CreatedAt:              run.CreatedAt,
		Verdict:                review.Verdict,
		PrimaryLabel:           primaryLabel,
		Visibility:             review.Visibility,
		MaxSeverity:            maxReviewSeverity(review.Labels),
		SuggestedRuleCount:     len(review.SuggestedRules),
		PrimaryEvidenceSnippet: primaryEvidenceSnippet(review, primaryLabel),
		ReviewerNote:           primaryReviewerNote(review, primaryLabel),
		PatchPreviewAvailable:  strings.TrimSpace(svc.cfg.ScenariosDir) != "",
		RunURL:                 runURL,
		ReviewURL:              runURL + "/review",
		PatchPreviewURL:        runURL + "/scenario-patch-preview",
	}, true, nil
}
