package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
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

	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/certify", authMiddleware(apiToken, handleCertifyAPI(cfg)))
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

func handleCertifyAPI(baseCfg config.Config) http.HandlerFunc {
	var mu sync.Mutex
	running := false

	// Load valid scenario IDs for validation.
	validScenarios := make(map[string]bool)
	if allScenarios, loadErr := scenario.LoadAll(baseCfg.ScenariosDir); loadErr == nil {
		for _, s := range allScenarios {
			validScenarios[s.ID] = true
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Limit request body to 1MB.
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

		// Validate scenario IDs against known scenarios.
		for _, sid := range req.Scenarios {
			if !validScenarios[sid] {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown scenario: %q", sid)})
				return
			}
		}

		mu.Lock()
		if running {
			mu.Unlock()
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a certification run is already in progress"})
			return
		}
		running = true
		mu.Unlock()

		// Build config for this run.
		runCfg := baseCfg
		runCfg.Model = req.Model
		runCfg.Provider = req.Provider
		if runCfg.Provider == "" {
			runCfg.Provider = "bifrost"
		}
		if req.Callback.EvidraURL != "" {
			runCfg.EvidraURL = req.Callback.EvidraURL
		}
		if req.Callback.EvidraAPIKey != "" {
			runCfg.EvidraAPIKey = req.Callback.EvidraAPIKey
		}

		jobID := req.JobID
		progressURL := req.Callback.ProgressURL
		scenarios := req.Scenarios

		// Start async with timeout (15min per scenario).
		go func() {
			defer func() {
				mu.Lock()
				running = false
				mu.Unlock()
			}()

			timeout := time.Duration(len(scenarios)) * 15 * time.Minute
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			completed := 0
			total := len(scenarios)

			for _, scenarioID := range scenarios {
				// Notify: running.
				sendProgress(progressURL, ProgressCallback{
					ContractVersion: "v1.0.0",
					JobID:           jobID,
					Scenario:        scenarioID,
					Status:          "running",
					Completed:       completed,
					Total:           total,
				})

				// Run the scenario using existing certify infrastructure.
				cert, err := runCertifyScenario(ctx, runCfg, scenarioID)

				completed++
				status := "passed"
				runID := ""
				if err != nil || cert == nil || cert.Passed < cert.Total {
					status = "failed"
				}
				if cert != nil {
					runID = fmt.Sprintf("%s-%s-%s",
						time.Now().UTC().Format("20060102-150405"),
						scenarioID, runCfg.Model)
				}

				// Submit bench run to Evidra.
				if runCfg.EvidraURL != "" && cert != nil {
					submitBenchRun(runCfg, scenarioID, runID, cert)
				}

				// Notify: done.
				sendProgress(progressURL, ProgressCallback{
					ContractVersion: "v1.0.0",
					JobID:           jobID,
					Scenario:        scenarioID,
					Status:          status,
					RunID:           runID,
					Completed:       completed,
					Total:           total,
				})
			}
		}()

		writeJSON(w, http.StatusAccepted, map[string]string{
			"job_id": jobID,
			"status": "accepted",
		})
	}
}

// runCertifyScenario runs a single scenario by ID.
func runCertifyScenario(ctx context.Context, cfg config.Config, scenarioID string) (*CertResult, error) {
	return runCertifySingleScenario(ctx, cfg, scenarioID)
}

// runCertifySingleScenario runs exactly one scenario by ID.
func runCertifySingleScenario(ctx context.Context, cfg config.Config, scenarioID string) (*CertResult, error) {
	scenariosDir := cfg.ScenariosDir
	if scenariosDir == "" {
		scenariosDir = "scenarios"
	}

	allScenarios, err := scenario.LoadAll(scenariosDir)
	if err != nil {
		return nil, fmt.Errorf("load scenarios: %w", err)
	}

	var target *scenario.Scenario
	for _, s := range allScenarios {
		if s.ID == scenarioID {
			target = s
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("scenario %q not found", scenarioID)
	}

	startTime := time.Now()
	runDir := fmt.Sprintf("/tmp/bench-service/%s_%s", scenarioID, cfg.Model)

	runCfg := cfg
	runCfg.Scenario = target.Path
	runCfg.RunsDir = runDir
	runCfg.EvidraEvidenceDir = runDir + "/evidence"

	if cfg.ReuseCluster {
		cleanBenchNamespace(ctx, cfg.ClusterName, target)
	}

	runResult, runErr := runScenarioOnce(ctx, runCfg, target)

	passed := false
	if runErr == nil && runResult != nil {
		passed = runResult.Passed
	}

	passedCount := 0
	if passed {
		passedCount = 1
	}

	return &CertResult{
		Track:       target.Track,
		Model:       cfg.Model,
		Provider:    cfg.Provider,
		Grade:       "single",
		Total:       1,
		Passed:      passedCount,
		Duration:    time.Since(startTime),
		CertifiedAt: time.Now().UTC(),
	}, nil
}

func sendProgress(progressURL string, payload ProgressCallback) {
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
		req.Header.Set("Authorization", "Bearer "+cfg.EvidraAPIKey)
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
