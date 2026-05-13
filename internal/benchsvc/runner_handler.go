package benchsvc

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
)

func handleRegisterRunner(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		var req RegisterRunnerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if len(req.Models) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "models is required")
			return
		}
		runner, err := svc.RegisterRunner(r.Context(), tenantID, req)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
			"runner_id":     runner.ID,
			"poll_interval": runner.Config.PollInterval,
		})
	}
}

func handleListRunners(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		runners, err := svc.ListRunners(r.Context(), tenantID)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if runners == nil {
			runners = []Runner{}
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{"runners": runners})
	}
}

func handleDeleteRunner(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		runnerID := r.PathValue("id")
		if err := svc.DeleteRunner(r.Context(), tenantID, runnerID); err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "runner not found")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handlePollJob(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		runnerID := r.URL.Query().Get("runner_id")
		if runnerID == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "runner_id query parameter is required")
			return
		}

		if err := svc.TouchRunner(r.Context(), tenantID, runnerID); err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "runner not found or draining")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		runners, err := svc.ListRunners(r.Context(), tenantID)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var models []string
		for _, runner := range runners {
			if runner.ID == runnerID {
				models = runner.Config.Models
				break
			}
		}
		if len(models) == 0 {
			apiutil.WriteError(w, http.StatusNotFound, "runner not found")
			return
		}

		job, err := svc.ClaimJob(r.Context(), tenantID, runnerID, models)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if job == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var cfg JobConfig
		if len(job.ConfigJSON) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "job config_json is required")
			return
		}
		if err := json.Unmarshal(job.ConfigJSON, &cfg); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid job config_json: "+err.Error())
			return
		}
		if cfg.ExecutionMode == "" {
			cfg.ExecutionMode = "provider" // default for legacy jobs without execution_mode
		}
		if cfg.ToolServer == "" {
			cfg.ToolServer = job.ToolServer
		}
		if cfg.ToolServerVersion == "" {
			cfg.ToolServerVersion = job.ToolServerVersion
		}

		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"job_id":              job.ID,
			"model":               job.Model,
			"provider":            job.Provider,
			"scenarios":           cfg.Scenarios,
			"timeout":             cfg.Timeout,
			"execution_mode":      cfg.ExecutionMode,
			"mcp_server":          cfg.MCPServer,
			"tool_server":         cfg.ToolServer,
			"tool_server_version": cfg.ToolServerVersion,
		})
	}
}

func handleCompleteJob(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		jobID := r.PathValue("id")

		var req struct {
			RunnerID string `json:"runner_id"`
			Status   string `json:"status"`
			Passed   int    `json:"passed"`
			Failed   int    `json:"failed"`
			Message  string `json:"message,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.Status != "completed" && req.Status != "failed" {
			apiutil.WriteError(w, http.StatusBadRequest, "status must be completed or failed")
			return
		}
		if req.RunnerID == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "runner_id is required")
			return
		}

		if err := svc.CompleteJob(r.Context(), tenantID, req.RunnerID, jobID, req.Status, req.Passed, req.Failed, req.Message); err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "job not found or not owned by this runner")
				return
			}
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if svc.cfg.TriggerStore != nil {
			svc.cfg.TriggerStore.Complete(jobID, req.Status, req.Passed, req.Failed)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
