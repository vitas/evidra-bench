package api

import (
	"net/http"

	"samebits.com/evidra-infra-bench/pkg/store"
)

func (s *Server) handleRegressions(w http.ResponseWriter, r *http.Request) {
	regs, err := s.store.Regressions(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if regs == nil {
		regs = []store.Regression{}
	}
	respondJSON(w, http.StatusOK, regs)
}
