package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/orchestrator"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

// CertifyRequest matches the Evidra executor contract v1.0.0.
type CertifyRequest struct {
	ContractVersion string   `json:"contract_version"`
	JobID           string   `json:"job_id"`
	Model           string   `json:"model"`
	Provider        string   `json:"provider"`
	Scenarios       []string `json:"scenarios"`
	Config          struct {
		TimeoutPerScenario int    `json:"timeout_per_scenario,omitempty"`
		Adapter            string `json:"adapter,omitempty"`
	} `json:"config"`
	Callback struct {
		ProgressURL  string `json:"progress_url"`
		EvidraURL    string `json:"evidra_url"`
		EvidraAPIKey string `json:"evidra_api_key"`
	} `json:"callback"`
}

func serveAPI(cfg config.Config, addr string) error {
	apiToken := cfg.EvidraAPIKey
	if apiToken == "" {
		return fmt.Errorf("serve: --evidra-api-key required for bench service authentication")
	}

	dbURL := cfg.ResolveDatabaseURL()
	if dbURL == "" {
		return fmt.Errorf("serve: --database-url required for bench service")
	}

	ctx := context.Background()

	// Provision cluster once for all workers.
	orch := orchestrator.New(cfg, makeScenarioRunFunc())
	if _, err := orch.Provision(ctx); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	defer orch.Teardown(ctx)

	// Sync scenarios to Evidra on startup.
	if cfg.EvidraURL != "" {
		go func() {
			if err := pushScenarios(cfg.ScenariosDir, cfg.EvidraURL, cfg.EvidraAPIKey); err != nil {
				log.Printf("[bench-service] scenario sync failed (non-fatal): %v", err)
			} else {
				log.Printf("[bench-service] scenarios synced to %s", cfg.EvidraURL)
			}
		}()
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/certify", authMiddleware(apiToken, handleCertifyAPI(cfg, orch, dbURL)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	log.Printf("bench service listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// authMiddleware checks for a valid Bearer token.
func authMiddleware(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func handleCertifyAPI(baseCfg config.Config, orch *orchestrator.Orchestrator, dbURL string) http.HandlerFunc {
	// Load scenarios once at handler construction.
	scenarioPathMap := make(map[string]string) // ID → Path
	if allScenarios, loadErr := scenario.LoadAll(baseCfg.ScenariosDir); loadErr == nil {
		for _, s := range allScenarios {
			scenarioPathMap[s.ID] = s.Path
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req CertifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		if req.Model == "" || len(req.Scenarios) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model and scenarios required"})
			return
		}

		// Validate and resolve scenario paths.
		var scenarioPaths []string
		for _, sid := range req.Scenarios {
			p, ok := scenarioPathMap[sid]
			if !ok {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown scenario: %q", sid)})
				return
			}
			scenarioPaths = append(scenarioPaths, p)
		}

		parallel := baseCfg.Parallel
		if parallel < 1 {
			parallel = 1
		}

		provider := req.Provider
		if provider == "" {
			provider = baseCfg.Provider
			if provider == "" {
				provider = "bifrost"
			}
		}

		// Run async via orchestrator.
		jobID := req.JobID
		if jobID == "" {
			jobID = fmt.Sprintf("certify-%s", time.Now().UTC().Format("20060102-150405"))
		}

		go func() {
			runCtx := context.Background()
			result, err := orch.RunParallel(runCtx, scenarioPaths, []string{req.Model}, 1, parallel, dbURL)
			if err != nil {
				log.Printf("[bench-service] certify job %s failed: %v", jobID, err)
				return
			}
			log.Printf("[bench-service] certify job %s done: %d/%d passed", jobID, result.Passed, result.Total)
		}()

		writeJSON(w, http.StatusAccepted, map[string]string{
			"job_id": jobID,
			"status": "accepted",
			"total":  fmt.Sprintf("%d", len(req.Scenarios)),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
