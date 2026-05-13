package benchsvc

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
)

func handleListModels(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		models, err := svc.ListEnabledModels(r.Context(), tenantID)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		type modelResponse struct {
			ID                string  `json:"id"`
			DisplayName       string  `json:"display_name"`
			Provider          string  `json:"provider"`
			APIBaseURL        string  `json:"api_base_url,omitempty"`
			Available         bool    `json:"available"`
			InputCostPerMtok  float64 `json:"input_cost_per_mtok"`
			OutputCostPerMtok float64 `json:"output_cost_per_mtok"`
		}

		result := make([]modelResponse, 0, len(models))
		for _, m := range models {
			result = append(result, modelResponse{
				ID:                m.ID,
				DisplayName:       m.DisplayName,
				Provider:          m.Provider,
				APIBaseURL:        m.APIBaseURL,
				Available:         os.Getenv(m.APIKeyEnv) != "",
				InputCostPerMtok:  m.InputCostPerMtok,
				OutputCostPerMtok: m.OutputCostPerMtok,
			})
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{"models": result})
	}
}

// HandleUpdateGlobalModel updates platform-level defaults for a model.
// This handler is intended to be wrapped by an invite-secret gate in the API router.
func HandleUpdateGlobalModel(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modelID := r.PathValue("model_id")

		var cfg GlobalModelConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			apiutil.WriteError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := svc.UpdateGlobalModel(r.Context(), modelID, cfg); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
