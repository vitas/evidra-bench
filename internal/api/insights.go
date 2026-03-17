package api

import "net/http"

func (s *Server) handleFailureAnalysis(w http.ResponseWriter, r *http.Request) {
	scenario := r.URL.Query().Get("scenario")
	if scenario == "" {
		respondError(w, http.StatusBadRequest, "query param 'scenario' is required")
		return
	}

	insights, err := s.store.FailureAnalysis(r.Context(), scenario)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, insights)
}
