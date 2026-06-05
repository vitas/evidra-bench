package benchsvc

import (
	"encoding/json"
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
const scenarioPatchDiffContentType = "text/x-diff; charset=utf-8"

type scenarioPatchPreviewResponse struct {
	Version      string                     `json:"version"`
	RunID        string                     `json:"run_id"`
	ScenarioID   string                     `json:"scenario_id"`
	ScenarioPath string                     `json:"scenario_path"`
	Changed      bool                       `json:"changed"`
	Diff         string                     `json:"diff"`
	ArtifactURL  string                     `json:"artifact_url"`
	DiffURL      string                     `json:"diff_url"`
	AddedRules   []scenariopatch.RuleChange `json:"added_rules"`
	SkippedRules []scenariopatch.RuleSkip   `json:"skipped_rules"`
}

func handleGetScenarioPatchPreview(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		if !authorizeScenarioPatchRead(w, r, svc, tenantID, id) {
			return
		}
		data, contentType, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedScenarioPatchPreview)
		if err != nil {
			writeArtifactReadError(w, err, "scenario patch preview not found")
			return
		}
		writeRawArtifactResponse(w, data, contentType)
	}
}

func handleGetScenarioPatchDiff(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		if !authorizeScenarioPatchRead(w, r, svc, tenantID, id) {
			return
		}
		preview, err := getStoredScenarioPatchPreview(r, svc, tenantID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "scenario patch preview not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !preview.Changed || strings.TrimSpace(preview.Diff) == "" {
			apiutil.WriteError(w, http.StatusNotFound, "scenario patch diff not found")
			return
		}
		writeRawArtifactResponse(w, []byte(preview.Diff), scenarioPatchDiffContentType)
	}
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

		response := newScenarioPatchPreviewResponse(id, run.ScenarioID, displayPath, preview)
		data, err := json.Marshal(response)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "marshal scenario patch preview: "+err.Error())
			return
		}
		if err := svc.StoreArtifact(r.Context(), id, artifact.HostedScenarioPatchPreview, artifact.ContentTypeJSON, data); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		apiutil.WriteJSON(w, http.StatusOK, response)
	}
}

func newScenarioPatchPreviewResponse(runID, scenarioID, scenarioPath string, preview scenariopatch.Result) scenarioPatchPreviewResponse {
	artifactURL := "/v1/bench/runs/" + runID + "/scenario-patch-preview"
	return scenarioPatchPreviewResponse{
		Version:      scenarioPatchPreviewVersion,
		RunID:        runID,
		ScenarioID:   scenarioID,
		ScenarioPath: scenarioPath,
		Changed:      preview.Changed,
		Diff:         preview.Diff,
		ArtifactURL:  artifactURL,
		DiffURL:      "/v1/bench/runs/" + runID + "/scenario-patch.diff",
		AddedRules:   preview.AddedRules,
		SkippedRules: preview.SkippedRules,
	}
}

func getStoredScenarioPatchPreview(r *http.Request, svc *Service, tenantID, id string) (scenarioPatchPreviewResponse, error) {
	data, _, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedScenarioPatchPreview)
	if err != nil {
		return scenarioPatchPreviewResponse{}, err
	}
	var preview scenarioPatchPreviewResponse
	if err := json.Unmarshal(data, &preview); err != nil {
		return scenarioPatchPreviewResponse{}, err
	}
	return preview, nil
}

func authorizeScenarioPatchRead(w http.ResponseWriter, r *http.Request, svc *Service, tenantID, id string) bool {
	return authorizeScenarioPatchArtifactRead(w, r, svc, tenantID, id, "scenario patch preview not found")
}

func authorizeScenarioPatchArtifactRead(w http.ResponseWriter, r *http.Request, svc *Service, tenantID, id, notFoundMessage string) bool {
	reviewData, _, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedRunReview)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apiutil.WriteError(w, http.StatusNotFound, notFoundMessage)
			return false
		}
		apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
		return false
	}
	review, err := runreview.Decode(reviewData)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "parse run review: "+err.Error())
		return false
	}
	if isAnonymousRead(r) && !runreview.IsPublic(review) {
		apiutil.WriteError(w, http.StatusNotFound, notFoundMessage)
		return false
	}
	return true
}
