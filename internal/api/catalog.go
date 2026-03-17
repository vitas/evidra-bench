package api

import (
	"net/http"

	"samebits.com/evidra-infra-bench/pkg/store"
)

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := s.store.Catalog(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if catalog == nil {
		catalog = &emptyCatalog
	}
	respondJSON(w, http.StatusOK, catalog)
}

var emptyCatalog = store.RunCatalog{
	Models:    []string{},
	Providers: []string{},
}
