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
	"samebits.com/evidra-infra-bench/pkg/environment"
	"samebits.com/evidra-infra-bench/pkg/jobqueue"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/store"
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

	// Open shared results store for parallel workers.
	sharedStore, storeErr := store.Open(cfg.RunsDir)
	if storeErr != nil {
		log.Printf("[bench-service] warning: could not open shared store: %v", storeErr)
	}
	if sharedStore != nil {
		defer sharedStore.Close()
	}

	// Provision cluster once for all workers.
	var envProvider environment.Provider
	switch cfg.EnvironmentProvider {
	case "k3d":
		p := environment.NewK3dProvider()
		p.ReuseExisting = cfg.ReuseCluster
		envProvider = p
	default:
		p := environment.NewKindProvider()
		p.ReuseExisting = cfg.ReuseCluster
		envProvider = p
	}
	handle, provErr := envProvider.Create(ctx, cfg.ClusterName)
	if provErr != nil {
		return fmt.Errorf("serve: provision cluster: %w", provErr)
	}
	kubeconfigPath := handle.KubeconfigPath
	log.Printf("[bench-service] cluster %s ready, kubeconfig: %s", cfg.ClusterName, kubeconfigPath)
	if !cfg.ReuseCluster {
		defer func() {
			if err := envProvider.Destroy(ctx, handle); err != nil {
				log.Printf("[bench-service] warning: destroy cluster: %v", err)
			}
		}()
	}

	// Build the run function that River workers will call.
	var completed, passed, failed int64
	runFn := buildParallelRunFunc(cfg, &completed, &passed, &failed, sharedStore, kubeconfigPath)

	parallel := cfg.Parallel
	if parallel < 1 {
		parallel = 1
	}

	jqClient, err := jobqueue.NewClient(ctx, dbURL, parallel, runFn)
	if err != nil {
		return fmt.Errorf("serve: job queue: %w", err)
	}

	if err := jqClient.Migrate(ctx); err != nil {
		return fmt.Errorf("serve: river migrate: %w", err)
	}

	// Start River workers in background.
	go func() {
		if err := jqClient.Start(ctx); err != nil {
			log.Printf("[bench-service] river stopped: %v", err)
		}
	}()
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), config.GracefulStopTimeout)
		defer cancel()
		jqClient.Stop(stopCtx)
	}()

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

	mux.HandleFunc("POST /v1/certify", authMiddleware(apiToken, handleCertifyAPI(cfg, jqClient)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	log.Printf("bench service listening on %s (parallel=%d)", addr, parallel)
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

func handleCertifyAPI(baseCfg config.Config, jqClient *jobqueue.Client) http.HandlerFunc {
	// Load scenarios once at handler construction — used for validation and path lookup.
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

		// Validate and resolve scenario paths in one pass.
		var scenarioPaths []string
		for _, sid := range req.Scenarios {
			p, ok := scenarioPathMap[sid]
			if !ok {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown scenario: %q", sid)})
				return
			}
			scenarioPaths = append(scenarioPaths, p)
		}

		jobID := req.JobID
		if jobID == "" {
			jobID = fmt.Sprintf("certify-%s", time.Now().UTC().Format("20060102-150405"))
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

		if err := jqClient.InsertBatch(r.Context(), scenarioPaths, req.Model, provider,
			baseCfg.MCPServer, jobID, "", parallel); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

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
