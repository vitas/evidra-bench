package benchsvc

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
)

func handleCompareRuns(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		a := r.URL.Query().Get("a")
		b := r.URL.Query().Get("b")
		if a == "" || b == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "query params 'a' and 'b' (run IDs) are required")
			return
		}
		result, err := svc.CompareRuns(r.Context(), tenantID, a, b)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "one or both runs not found")
			} else {
				apiutil.WriteError(w, http.StatusInternalServerError, "comparison failed")
			}
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, result)
	}
}

func handleCompareModels(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()

		// Support both ?models=X,Y,Z (multi-model matrix) and legacy ?a=X&b=Y (pairwise).
		modelsStr := q.Get("models")
		if modelsStr != "" {
			models := strings.Split(modelsStr, ",")
			var scenarios []string
			if scenariosStr := q.Get("scenarios"); scenariosStr != "" {
				scenarios = strings.Split(scenariosStr, ",")
			}
			mode := q.Get("evidence_mode")
			matrix, err := svc.ModelMatrix(r.Context(), tenantID, models, scenarios, mode)
			if err != nil {
				apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			apiutil.WriteJSON(w, http.StatusOK, matrix)
			return
		}

		// Legacy pairwise comparison.
		modelA := q.Get("a")
		modelB := q.Get("b")
		if modelA == "" || modelB == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "query param 'models' (comma-separated) or 'a' and 'b' are required")
			return
		}
		mode := q.Get("evidence_mode")
		result, err := svc.CompareModels(r.Context(), tenantID, modelA, modelB, mode)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, result)
	}
}

func handleCompareToolServer(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		q := r.URL.Query()
		model := q.Get("model")
		toolServer := q.Get("tool_server")
		if model == "" || toolServer == "" {
			apiutil.WriteError(w, http.StatusBadRequest, "query params 'model' and 'tool_server' are required")
			return
		}

		scenarios := parseCSVQuery(q.Get("scenarios"))
		if scenario := q.Get("scenario"); scenario != "" {
			scenarios = parseCSVQuery(scenario)
		}

		result, err := svc.CompareToolServer(r.Context(), tenantID, ToolServerCompareRequest{
			Model:             model,
			ToolServer:        toolServer,
			ToolServerVersion: q.Get("tool_server_version"),
			ReportID:          q.Get("report_id"),
			ScenarioIDs:       scenarios,
		})
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, result)
	}
}
