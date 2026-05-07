package benchsvc

import (
	"encoding/json"
	"errors"
	"net/http"

	"samebits.com/evidra-infra-bench/internal/apiutil"
	"samebits.com/evidra-infra-bench/internal/auth"
)

func handleIngestRun(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var req IngestRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.ID == "" || req.ScenarioID == "" || req.Model == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "id, scenario_id, and model are required")
			return
		}
		req.TenantID = tenantID
		if err := svc.IngestRun(r.Context(), tenantID, req); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "insert: "+err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": req.ID})
	}
}

func handleIngestBatch(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var req struct {
			Runs []IngestRunRequest `json:"runs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if len(req.Runs) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "runs array is empty")
			return
		}
		for i := range req.Runs {
			if req.Runs[i].ID == "" || req.Runs[i].ScenarioID == "" || req.Runs[i].Model == "" {
				apiutil.WriteError(w, http.StatusBadRequest, "each run requires id, scenario_id, and model")
				return
			}
			req.Runs[i].TenantID = tenantID
		}
		count, err := svc.IngestRunBatch(r.Context(), tenantID, req.Runs)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "batch insert: "+err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
			"ok":       true,
			"imported": count,
			"total":    len(req.Runs),
		})
	}
}

func handleDeleteRun(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		err := svc.DeleteRun(r.Context(), tenantID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "run not found")
			} else {
				apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleArchiveRuns(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())

		var req ArchiveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.Before == nil && len(req.IDs) == 0 && req.Model == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "at least one filter is required: before, ids, or model")
			return
		}

		count, err := svc.ArchiveRuns(r.Context(), tenantID, req)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{"archived": count})
	}
}
