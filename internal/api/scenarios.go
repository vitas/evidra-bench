package api

import (
	"net/http"

	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func (s *Server) handleListScenarios(w http.ResponseWriter, r *http.Request) {
	scenarios, err := scenario.LoadAll(s.scenarios)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type scenarioResponse struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		Description string   `json:"description,omitempty"`
		Category    string   `json:"category"`
		Tags        []string `json:"tags"`
		Chaos       bool     `json:"chaos"`
		Evidra      bool     `json:"evidra"`
	}

	items := make([]scenarioResponse, 0, len(scenarios))
	for _, sc := range scenarios {
		tags := sc.Tags
		if tags == nil {
			tags = []string{}
		}
		items = append(items, scenarioResponse{
			ID:          sc.ID,
			Title:       sc.Title,
			Description: sc.Description,
			Category:    sc.Category,
			Tags:        tags,
			Chaos:       len(sc.Chaos.Steps) > 0,
			Evidra:      sc.Evidra.Enabled,
		})
	}

	respondJSON(w, http.StatusOK, listResponse{
		Items:  items,
		Total:  len(items),
		Limit:  len(items),
		Offset: 0,
	})
}
