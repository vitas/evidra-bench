package benchsvc

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
)

const scenarioPatchValidationVersion = "scenario_patch_validation.v1"

type scenarioPatchValidationRequest struct {
	RunnerID          string `json:"runner_id,omitempty"`
	ExecutionMode     string `json:"execution_mode,omitempty"`
	MCPServer         string `json:"mcp_server,omitempty"`
	ToolServer        string `json:"tool_server,omitempty"`
	ToolServerVersion string `json:"tool_server_version,omitempty"`
	SkillFile         string `json:"skill_file,omitempty"`
	SkillID           string `json:"skill_id,omitempty"`
	SkillVersion      string `json:"skill_version,omitempty"`
	SkillSource       string `json:"skill_source,omitempty"`
	SkillSHA256       string `json:"skill_sha256,omitempty"`
}

type scenarioPatchValidationResponse struct {
	Version          string   `json:"version"`
	SourceRunID      string   `json:"source_run_id"`
	SourceRunURL     string   `json:"source_run_url"`
	ScenarioID       string   `json:"scenario_id"`
	Model            string   `json:"model"`
	Provider         string   `json:"provider"`
	TriggerID        string   `json:"trigger_id"`
	TriggerURL       string   `json:"trigger_url"`
	ValidationURL    string   `json:"validation_url"`
	Status           string   `json:"status"`
	Mode             string   `json:"mode,omitempty"`
	CurrentScenario  string   `json:"current_scenario,omitempty"`
	Total            int      `json:"total"`
	Completed        int      `json:"completed"`
	Passed           int      `json:"passed"`
	Failed           int      `json:"failed"`
	ValidationRunIDs []string `json:"validation_run_ids,omitempty"`
	PatchPreviewURL  string   `json:"patch_preview_url"`
	PatchDiffURL     string   `json:"patch_diff_url"`
}

func handleGetScenarioPatchValidation(svc *Service, store *TriggerStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		if !authorizeScenarioPatchArtifactRead(w, r, svc, tenantID, id, "scenario patch validation not found") {
			return
		}

		validation, err := getStoredScenarioPatchValidation(r, svc, tenantID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "scenario patch validation not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if store != nil {
			if job := store.Get(validation.TriggerID); job != nil {
				validation = enrichScenarioPatchValidationFromTrigger(validation, job)
				if err := storeScenarioPatchValidation(r, svc, id, validation); err != nil {
					apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
					return
				}
			}
		}

		apiutil.WriteJSON(w, http.StatusOK, validation)
	}
}

func handlePostScenarioPatchValidation(svc *Service, store *TriggerStore, executor RunExecutor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			apiutil.WriteError(w, http.StatusBadRequest, "scenario patch validation requires a changed patch preview")
			return
		}

		overrides, ok := decodeScenarioPatchValidationRequest(w, r)
		if !ok {
			return
		}
		triggerReq, ok := triggerRequestForScenarioPatchValidation(w, *run, overrides)
		if !ok {
			return
		}
		result, ok := startTriggerRequest(w, r, svc, store, executor, tenantID, triggerReq)
		if !ok {
			return
		}

		validation := newScenarioPatchValidationResponse(*run, triggerReq, result, preview)
		if job := store.Get(result.ID); job != nil {
			validation = enrichScenarioPatchValidationFromTrigger(validation, job)
		}
		if err := storeScenarioPatchValidation(r, svc, id, validation); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		apiutil.WriteJSON(w, http.StatusAccepted, validation)
	}
}

func decodeScenarioPatchValidationRequest(w http.ResponseWriter, r *http.Request) (scenarioPatchValidationRequest, bool) {
	var req scenarioPatchValidationRequest
	if r.Body == nil {
		return req, true
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return req, true
		}
		apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return scenarioPatchValidationRequest{}, false
	}
	return req, true
}

func triggerRequestForScenarioPatchValidation(w http.ResponseWriter, run bench.RunRecord, overrides scenarioPatchValidationRequest) (TriggerRequest, bool) {
	executionMode := overrides.ExecutionMode
	if executionMode == "" {
		executionMode = executionModeForRun(run)
	}
	var ok bool
	executionMode, ok = normalizeTriggerExecutionMode(executionMode)
	if !ok {
		apiutil.WriteError(w, http.StatusBadRequest, "execution_mode must be provider or a2a")
		return TriggerRequest{}, false
	}
	return TriggerRequest{
		Model:             run.Model,
		Provider:          run.Provider,
		RunnerID:          overrides.RunnerID,
		ExecutionMode:     executionMode,
		MCPServer:         overrides.MCPServer,
		ToolServer:        firstNonEmpty(overrides.ToolServer, run.ToolServer),
		ToolServerVersion: firstNonEmpty(overrides.ToolServerVersion, run.ToolServerVersion),
		SkillFile:         overrides.SkillFile,
		SkillID:           firstNonEmpty(overrides.SkillID, run.SkillID),
		SkillVersion:      firstNonEmpty(overrides.SkillVersion, run.SkillVersion),
		SkillSource:       firstNonEmpty(overrides.SkillSource, run.SkillSource),
		SkillSHA256:       firstNonEmpty(overrides.SkillSHA256, run.SkillSHA256),
		Scenarios:         []string{run.ScenarioID},
	}, true
}

func newScenarioPatchValidationResponse(run bench.RunRecord, req TriggerRequest, result triggerStartResult, preview scenarioPatchPreviewResponse) scenarioPatchValidationResponse {
	return scenarioPatchValidationResponse{
		Version:         scenarioPatchValidationVersion,
		SourceRunID:     run.ID,
		SourceRunURL:    runURLForRun(run.ID),
		ScenarioID:      run.ScenarioID,
		Model:           run.Model,
		Provider:        resultProvider(run.Provider, req.Provider),
		TriggerID:       result.ID,
		TriggerURL:      "/v1/bench/trigger/" + result.ID,
		ValidationURL:   validationURLForRun(run.ID),
		Status:          result.Status,
		Mode:            result.Mode,
		Total:           len(req.Scenarios),
		PatchPreviewURL: firstNonEmpty(preview.ArtifactURL, previewURLForRun(run.ID)),
		PatchDiffURL:    firstNonEmpty(preview.DiffURL, diffURLForRun(run.ID)),
	}
}

func storeScenarioPatchValidation(r *http.Request, svc *Service, id string, validation scenarioPatchValidationResponse) error {
	data, err := json.Marshal(validation)
	if err != nil {
		return err
	}
	return svc.StoreArtifact(r.Context(), id, artifact.HostedScenarioPatchValidation, artifact.ContentTypeJSON, data)
}

func getStoredScenarioPatchValidation(r *http.Request, svc *Service, tenantID, id string) (scenarioPatchValidationResponse, error) {
	data, _, err := svc.GetArtifact(r.Context(), tenantID, id, artifact.HostedScenarioPatchValidation)
	if err != nil {
		return scenarioPatchValidationResponse{}, err
	}
	var validation scenarioPatchValidationResponse
	if err := json.Unmarshal(data, &validation); err != nil {
		return scenarioPatchValidationResponse{}, err
	}
	return normalizeScenarioPatchValidation(validation, id), nil
}

func normalizeScenarioPatchValidation(validation scenarioPatchValidationResponse, runID string) scenarioPatchValidationResponse {
	if validation.Version == "" {
		validation.Version = scenarioPatchValidationVersion
	}
	if validation.SourceRunID == "" {
		validation.SourceRunID = runID
	}
	if validation.SourceRunURL == "" {
		validation.SourceRunURL = runURLForRun(validation.SourceRunID)
	}
	if validation.ValidationURL == "" {
		validation.ValidationURL = validationURLForRun(validation.SourceRunID)
	}
	if validation.PatchPreviewURL == "" {
		validation.PatchPreviewURL = previewURLForRun(validation.SourceRunID)
	}
	if validation.PatchDiffURL == "" {
		validation.PatchDiffURL = diffURLForRun(validation.SourceRunID)
	}
	return validation
}

func enrichScenarioPatchValidationFromTrigger(validation scenarioPatchValidationResponse, job *TriggerJob) scenarioPatchValidationResponse {
	if job == nil {
		return validation
	}
	validation.Status = job.Status
	if job.Model != "" {
		validation.Model = job.Model
	}
	if job.Provider != "" {
		validation.Provider = job.Provider
	}
	validation.Total = job.Total
	validation.Completed = job.Completed
	validation.Passed = job.Passed
	validation.Failed = job.Failed
	validation.CurrentScenario = job.CurrentScenario
	validation.ValidationRunIDs = scenarioPatchValidationRunIDs(job)
	return validation
}

func scenarioPatchValidationRunIDs(job *TriggerJob) []string {
	if job == nil {
		return nil
	}
	seen := make(map[string]struct{})
	ids := make([]string, 0, len(job.RunIDs)+len(job.Progress))
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, id := range job.RunIDs {
		add(id)
	}
	for _, progress := range job.Progress {
		add(progress.RunID)
	}
	return ids
}

func executionModeForRun(run bench.RunRecord) string {
	if run.Adapter == ExecutionModeA2A {
		return ExecutionModeA2A
	}
	return ExecutionModeProvider
}

func resultProvider(sourceProvider, triggerProvider string) string {
	if triggerProvider != "" {
		return triggerProvider
	}
	return sourceProvider
}

func previewURLForRun(runID string) string {
	return "/v1/bench/runs/" + runID + "/scenario-patch-preview"
}

func diffURLForRun(runID string) string {
	return "/v1/bench/runs/" + runID + "/scenario-patch.diff"
}

func runURLForRun(runID string) string {
	return "/v1/bench/runs/" + runID
}

func validationURLForRun(runID string) string {
	return "/v1/bench/runs/" + runID + "/scenario-patch-validation"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
