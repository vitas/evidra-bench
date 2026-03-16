package api

import (
	"encoding/json"
	"net/http"

	"samebits.com/evidra-infra-bench/internal/executor"
)

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if s.exec == nil {
		respondError(w, http.StatusServiceUnavailable, "executor not configured")
		return
	}

	var req executor.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ScenarioID == "" {
		respondError(w, http.StatusBadRequest, "scenario_id is required")
		return
	}

	jobID, err := s.exec.Start(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]string{
		"job_id": jobID,
		"status": "pending",
	})
}

func (s *Server) handleExecuteStatus(w http.ResponseWriter, r *http.Request) {
	if s.exec == nil {
		respondError(w, http.StatusServiceUnavailable, "executor not configured")
		return
	}

	id := r.PathValue("id")
	job, err := s.exec.Status(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, job)
}
