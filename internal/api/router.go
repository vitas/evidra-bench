// Package api provides HTTP handlers for the bench API.
package api

import (
	"net/http"

	"samebits.com/evidra-infra-bench/internal/executor"
	"samebits.com/evidra-infra-bench/pkg/store"
)

// Server holds dependencies for all API handlers.
type Server struct {
	store     store.BenchStore
	exec      *executor.Executor
	scenarios string // path to scenarios directory
}

// NewServer creates a Server with the given dependencies.
func NewServer(s store.BenchStore, exec *executor.Executor, scenariosDir string) *Server {
	return &Server{store: s, exec: exec, scenarios: scenariosDir}
}

// Handler returns the top-level HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealthz)

	// Runs
	mux.HandleFunc("GET /v1/bench/runs", s.handleListRuns)
	mux.HandleFunc("GET /v1/bench/runs/{id}", s.handleGetRun)
	mux.HandleFunc("GET /v1/bench/runs/{id}/transcript", s.handleGetTranscript)
	mux.HandleFunc("GET /v1/bench/runs/{id}/tool-calls", s.handleGetToolCalls)
	mux.HandleFunc("GET /v1/bench/runs/{id}/scorecard", s.handleGetScorecard)

	// Compare
	mux.HandleFunc("GET /v1/bench/compare/runs", s.handleCompareRuns)
	mux.HandleFunc("GET /v1/bench/compare/models", s.handleCompareModels)

	// Stats
	mux.HandleFunc("GET /v1/bench/stats", s.handleStats)

	// Execute
	mux.HandleFunc("POST /v1/bench/execute", s.handleExecute)
	mux.HandleFunc("GET /v1/bench/execute/{id}/status", s.handleExecuteStatus)

	// Scenarios
	mux.HandleFunc("GET /v1/bench/scenarios", s.handleListScenarios)

	// CORS middleware for future UI
	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
