package main

import (
	"net/http"

	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/internal/benchsvc"
)

func registerBenchAPIRoutes(mux *http.ServeMux, svc *benchsvc.Service, apiKey string, tenantOpt ...string) {
	tenantID := "default"
	if len(tenantOpt) > 0 && tenantOpt[0] != "" {
		tenantID = tenantOpt[0]
	}
	mux.HandleFunc("GET /v1/bench/info", handleBenchInfo())
	authMw := auth.StaticKeyMiddleware(apiKey, tenantID)
	benchsvc.RegisterRoutes(mux, svc, authMw)
}

func handleBenchInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"readonly": true,
			"version":  buildVersionString(),
		})
	}
}
