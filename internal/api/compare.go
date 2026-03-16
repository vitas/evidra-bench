package api

import (
	"net/http"
	"strings"
)

func (s *Server) handleCompareRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	idA := q.Get("a")
	idB := q.Get("b")
	if idA == "" || idB == "" {
		respondError(w, http.StatusBadRequest, "query params 'a' and 'b' are required")
		return
	}

	cmp, err := s.store.CompareRuns(r.Context(), idA, idB)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, cmp)
}

func (s *Server) handleCompareModels(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	modelsStr := q.Get("models")
	if modelsStr == "" {
		respondError(w, http.StatusBadRequest, "query param 'models' is required (comma-separated)")
		return
	}
	models := strings.Split(modelsStr, ",")

	var scenarios []string
	if scenariosStr := q.Get("scenarios"); scenariosStr != "" {
		scenarios = strings.Split(scenariosStr, ",")
	}

	matrix, err := s.store.ModelMatrix(r.Context(), models, scenarios)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, matrix)
}
