package main

import (
	"net/http"

	"samebits.com/evidra-infra-bench/internal/auth"
	"samebits.com/evidra-infra-bench/internal/benchsvc"
)

func registerBenchAPIRoutes(mux *http.ServeMux, svc *benchsvc.Service, apiKey string) {
	authMw := auth.StaticKeyMiddleware(apiKey, "default")
	benchsvc.RegisterRoutes(mux, svc, authMw)
}
