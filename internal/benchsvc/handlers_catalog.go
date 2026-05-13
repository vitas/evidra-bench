package benchsvc

import (
	"encoding/json"
	"net/http"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func handleCatalog(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		cat, err := svc.Catalog(r.Context(), tenantID)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, "catalog query failed")
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, cat)
	}
}

func handleListScenarios(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scenarios, err := svc.ListScenarios(r.Context())
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if scenarios == nil {
			scenarios = []bench.ScenarioSummary{}
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"scenarios": scenarios,
		})
	}
}

func handleSyncScenarios(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Scenarios []bench.ScenarioSummary `json:"scenarios"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if len(req.Scenarios) == 0 {
			apiutil.WriteError(w, http.StatusBadRequest, "scenarios array is required")
			return
		}
		upserted, err := svc.UpsertScenarios(r.Context(), req.Scenarios)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"upserted": upserted,
			"total":    len(req.Scenarios),
		})
	}
}
