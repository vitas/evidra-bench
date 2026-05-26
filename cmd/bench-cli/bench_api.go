package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/internal/benchsvc"
)

const benchBrowserSessionTTL = 12 * time.Hour

func registerBenchAPIRoutes(mux *http.ServeMux, svc *benchsvc.Service, apiKey string, tenantOpt ...string) {
	tenantID := "default"
	if len(tenantOpt) > 0 && tenantOpt[0] != "" {
		tenantID = tenantOpt[0]
	}
	mux.HandleFunc("GET /v1/bench/info", handleBenchInfo())
	mux.HandleFunc("GET /v1/bench/session", handleBenchSessionStatus(apiKey, tenantID))
	mux.HandleFunc("POST /v1/bench/session", handleBenchSessionLogin(apiKey, tenantID))
	mux.HandleFunc("DELETE /v1/bench/session", handleBenchSessionLogout())
	authMw := auth.StaticKeyMiddleware(apiKey, tenantID)
	benchsvc.RegisterRoutes(mux, svc, authMw)
}

func handleBenchInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"readonly": false,
			"version":  buildVersionString(),
		})
	}
}

func handleBenchSessionStatus(apiKey, tenantID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authTenant, ok := auth.AuthenticateStaticRequest(r, apiKey, tenantID)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"tenant_id":     authTenant,
		})
	}
}

func handleBenchSessionLogin(apiKey, tenantID string) http.HandlerFunc {
	type loginRequest struct {
		APIKey string `json:"api_key"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if !auth.StaticKeyMatches(apiKey, req.APIKey) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		cookie, err := auth.NewSessionCookie(r, apiKey, tenantID, benchBrowserSessionTTL)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		http.SetCookie(w, cookie)
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"tenant_id":     tenantID,
		})
	}
}

func handleBenchSessionLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, auth.ClearSessionCookie(r))
		w.WriteHeader(http.StatusNoContent)
	}
}
