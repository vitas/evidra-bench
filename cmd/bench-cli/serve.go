package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/jobqueue"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/workspace"
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

// ProgressCallback sends a progress update to the Evidra trigger webhook.
type ProgressCallback struct {
	ContractVersion string `json:"contract_version"`
	JobID           string `json:"job_id"`
	Scenario        string `json:"scenario"`
	Status          string `json:"status"`
	RunID           string `json:"run_id,omitempty"`
	Completed       int    `json:"completed"`
	Total           int    `json:"total"`
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

	// Build the run function that River workers will call.
	runFn := buildServeRunFunc(cfg)

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
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	// Load valid scenario IDs for validation.
	validScenarios := make(map[string]bool)
	if allScenarios, loadErr := scenario.LoadAll(baseCfg.ScenariosDir); loadErr == nil {
		for _, s := range allScenarios {
			validScenarios[s.ID] = true
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

		for _, sid := range req.Scenarios {
			if !validScenarios[sid] {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown scenario: %q", sid)})
				return
			}
		}

		jobID := req.JobID
		if jobID == "" {
			jobID = fmt.Sprintf("certify-%s", time.Now().UTC().Format("20060102-150405"))
		}

		parallel := baseCfg.Parallel
		if parallel < 1 {
			parallel = 1
		}

		// Resolve scenario paths for enqueue.
		var scenarioPaths []string
		allScenarios, _ := scenario.LoadAll(baseCfg.ScenariosDir)
		pathMap := make(map[string]string)
		for _, s := range allScenarios {
			pathMap[s.ID] = s.Path
		}
		for _, sid := range req.Scenarios {
			if p, ok := pathMap[sid]; ok {
				scenarioPaths = append(scenarioPaths, p)
			}
		}

		provider := req.Provider
		if provider == "" {
			provider = "bifrost"
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

// buildServeRunFunc creates the RunFunc for the serve command.
func buildServeRunFunc(cfg config.Config) jobqueue.RunFunc {
	return func(ctx context.Context, args jobqueue.BenchJobArgs, ns string) error {
		ws, err := workspace.New(
			fmt.Sprintf("%s-%s-%d", args.ScenarioID, args.Model, time.Now().UnixNano()),
			cfg.ScenariosDir,
		)
		if err != nil {
			return fmt.Errorf("workspace: %w", err)
		}
		defer ws.Cleanup()

		scenarioDir := filepath.Join(ws.ScenariosDir, filepath.Dir(args.ScenarioID))
		if err := workspace.RewriteNamespace(scenarioDir, "bench", ns); err != nil {
			log.Printf("[serve-worker] namespace rewrite warning: %v", err)
		}

		workerCfg := cfg
		workerCfg.Scenario = args.ScenarioID
		workerCfg.ScenariosDir = ws.ScenariosDir
		workerCfg.RunsDir = ws.RunsDir
		workerCfg.EvidraEvidenceDir = ws.EvidenceDir
		workerCfg.Model = args.Model
		workerCfg.Provider = args.Provider

		scenarioPath := filepath.Join(ws.ScenariosDir, args.ScenarioID)
		s, loadErr := scenario.Load(scenarioPath)
		if loadErr != nil {
			return fmt.Errorf("load scenario: %w", loadErr)
		}

		runResult, runErr := runScenarioOnce(ctx, workerCfg, s)

		if runErr != nil {
			log.Printf("[serve-worker] FAIL %s/%s: %v", args.ScenarioID, args.Model, runErr)
		} else if runResult != nil && runResult.Passed {
			log.Printf("[serve-worker] PASS %s/%s (%s)", args.ScenarioID, args.Model, runResult.Duration)
		} else {
			log.Printf("[serve-worker] FAIL %s/%s", args.ScenarioID, args.Model)
		}

		// Submit to Evidra API if configured.
		if workerCfg.EvidraURL != "" && runResult != nil {
			passedCount := 0
			if runResult.Passed {
				passedCount = 1
			}
			submitBenchRun(workerCfg, args.ScenarioID,
				fmt.Sprintf("%s-%s-%s", time.Now().UTC().Format("20060102-150405"), args.ScenarioID, args.Model),
				&CertResult{
					Model:    args.Model,
					Provider: args.Provider,
					Total:    1,
					Passed:   passedCount,
					Duration: runResult.Duration,
				})
		}

		return nil // don't fail River job on scenario failure
	}
}

func sendProgress(progressURL, authToken string, payload ProgressCallback) {
	if progressURL == "" {
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[bench-service] marshal progress: %v", err)
		return
	}
	req, err := http.NewRequest("POST", progressURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[bench-service] create progress request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[bench-service] send progress: %v", err)
		return
	}
	resp.Body.Close()
}

func submitBenchRun(cfg config.Config, scenarioID, runID string, cert *CertResult) {
	if cfg.EvidraURL == "" {
		return
	}
	run := map[string]any{
		"id":               runID,
		"scenario_id":      scenarioID,
		"model":            cfg.Model,
		"provider":         cfg.Provider,
		"adapter":          "kagent",
		"evidence_mode":    "direct",
		"passed":           cert.Passed > 0,
		"duration_seconds": cert.Duration.Seconds(),
		"checks_passed":    cert.Passed,
		"checks_total":     cert.Total,
	}
	body, err := json.Marshal(run)
	if err != nil {
		log.Printf("[bench-service] marshal bench run: %v", err)
		return
	}
	url := cfg.EvidraURL + "/v1/bench/runs"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[bench-service] create bench run request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.EvidraAPIKey != "" {
		auth := cfg.EvidraAPIKey
		if !strings.HasPrefix(auth, "Bearer ") {
			auth = "Bearer " + auth
		}
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[bench-service] submit bench run: %v", err)
		return
	}
	resp.Body.Close()
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
