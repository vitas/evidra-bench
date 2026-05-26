package benchsvc

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/runreview"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/scenariopatch"
)

const scenarioPatchPreviewVersion = "scenario_patch_preview.v1"

type scenarioPatchPreviewResponse struct {
	Version      string                     `json:"version"`
	RunID        string                     `json:"run_id"`
	ScenarioID   string                     `json:"scenario_id"`
	ScenarioPath string                     `json:"scenario_path"`
	Changed      bool                       `json:"changed"`
	Diff         string                     `json:"diff"`
	AddedRules   []scenariopatch.RuleChange `json:"added_rules"`
	SkippedRules []scenariopatch.RuleSkip   `json:"skipped_rules"`
}

func handlePostScenarioPatchPreview(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(svc.cfg.ScenariosDir) == "" {
			apiutil.WriteError(w, http.StatusServiceUnavailable, "scenario patch preview requires scenarios directory")
			return
		}

		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		run, err := svc.GetRun(r.Context(), tenantID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "run not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if run == nil {
			apiutil.WriteError(w, http.StatusNotFound, "run not found")
			return
		}

		reviewData, _, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedRunReview)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "run review not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		review, err := runreview.Decode(reviewData)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "parse run review: "+err.Error())
			return
		}
		review, err = runreview.NormalizeForRun(review, id, run.ScenarioID, runreview.VisibilityPrivate)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "normalize run review: "+err.Error())
			return
		}

		s, err := scenario.Resolve(svc.cfg.ScenariosDir, run.ScenarioID)
		if err != nil {
			apiutil.WriteError(w, http.StatusNotFound, "scenario not found")
			return
		}
		scenarioYAMLPath := filepath.Join(s.Dir, "scenario.yaml")
		scenarioYAML, err := os.ReadFile(scenarioYAMLPath)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "read scenario YAML: "+err.Error())
			return
		}

		displayPath := filepath.ToSlash(filepath.Join(s.Path, "scenario.yaml"))
		preview, err := scenariopatch.Preview(scenarioYAML, review, displayPath)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, scenarioPatchPreviewResponse{
			Version:      scenarioPatchPreviewVersion,
			RunID:        id,
			ScenarioID:   run.ScenarioID,
			ScenarioPath: displayPath,
			Changed:      preview.Changed,
			Diff:         preview.Diff,
			AddedRules:   preview.AddedRules,
			SkippedRules: preview.SkippedRules,
		})
	}
}
