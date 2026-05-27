package benchsvc

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
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
	Version         string `json:"version"`
	SourceRunID     string `json:"source_run_id"`
	ScenarioID      string `json:"scenario_id"`
	Model           string `json:"model"`
	Provider        string `json:"provider"`
	TriggerID       string `json:"trigger_id"`
	TriggerURL      string `json:"trigger_url"`
	Status          string `json:"status"`
	Mode            string `json:"mode,omitempty"`
	PatchPreviewURL string `json:"patch_preview_url"`
	PatchDiffURL    string `json:"patch_diff_url"`
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

		apiutil.WriteJSON(w, http.StatusAccepted, scenarioPatchValidationResponse{
			Version:         scenarioPatchValidationVersion,
			SourceRunID:     run.ID,
			ScenarioID:      run.ScenarioID,
			Model:           run.Model,
			Provider:        resultProvider(run.Provider, triggerReq.Provider),
			TriggerID:       result.ID,
			TriggerURL:      "/v1/bench/trigger/" + result.ID,
			Status:          result.Status,
			Mode:            result.Mode,
			PatchPreviewURL: previewURLForRun(id),
			PatchDiffURL:    diffURLForRun(id),
		})
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

func executionModeForRun(run bench.RunRecord) string {
	if run.Adapter == "a2a" {
		return "a2a"
	}
	return "provider"
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
