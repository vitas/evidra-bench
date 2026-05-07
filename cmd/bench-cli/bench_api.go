package main

import (
	"net/http"

	"samebits.com/evidra-infra-bench/internal/auth"
	"samebits.com/evidra-infra-bench/internal/benchsvc"
)

func registerBenchAPIRoutes(mux *http.ServeMux, svc *benchsvc.Service, apiKey string, tenantOpt ...string) {
	tenantID := "default"
	if len(tenantOpt) > 0 && tenantOpt[0] != "" {
		tenantID = tenantOpt[0]
	}
	authMw := auth.StaticKeyMiddleware(apiKey, tenantID)
	benchsvc.RegisterRoutes(mux, svc, authMw)
}
