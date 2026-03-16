package api

import (
	"net/http"

	"samebits.com/evidra-infra-bench/pkg/store"
)

func (s *Server) handleSignals(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.RunFilters{
		ScenarioID: q.Get("scenario"),
		Model:      q.Get("model"),
		Provider:   q.Get("provider"),
		Since:      q.Get("since"),
	}

	agg, err := s.store.SignalSummary(r.Context(), f)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, agg)
}
