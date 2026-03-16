package api

import (
	"net/http"

	"samebits.com/evidra-infra-bench/pkg/store"
)

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.RunFilters{
		ScenarioID: q.Get("scenario"),
		Model:      q.Get("model"),
		Provider:   q.Get("provider"),
	}

	st, err := s.store.FilteredStats(r.Context(), f)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, st)
}
