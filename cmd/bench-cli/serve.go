package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/orchestrator"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

type parallelRunner interface {
	RunParallel(ctx context.Context, runCfg config.Config, reporter orchestrator.ProgressReporter, scenarios []string, models []string, repeats, parallel int, dbURL string) (*orchestrator.RunResult, error)
}

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

func handleCertifyAPI(baseCfg config.Config, runner parallelRunner, dbURL string) http.HandlerFunc {
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

		runCfg := buildCertifyRunConfig(baseCfg, req)

		// Run async via orchestrator.
		jobID := req.JobID
		if jobID == "" {
			jobID = fmt.Sprintf("certify-%s", time.Now().UTC().Format("20060102-150405"))
		}

		// Wire progress reporter for this certify request.
		progressURL := req.Callback.ProgressURL
		evidraURL := req.Callback.EvidraURL
		authToken := req.Callback.EvidraAPIKey
		if evidraURL == "" {
			evidraURL = baseCfg.EvidraURL
		}

		reporter := &evidraReporter{
			progressURL: progressURL,
			evidraURL:   evidraURL,
			authToken:   authToken,
		}

		go func() {
			runCtx := context.Background()
			result, err := runner.RunParallel(runCtx, runCfg, reporter, scenarioPaths, []string{req.Model}, 1, parallel, dbURL)
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

func buildCertifyRunConfig(baseCfg config.Config, req CertifyRequest) config.Config {
	runCfg := baseCfg
	if req.Provider != "" {
		runCfg.Provider = req.Provider
	}
	if runCfg.Provider == "" {
		runCfg.Provider = "bifrost"
	}
	// Only override adapter if provider mode is not active.
	// Provider mode uses its own agent loop, not CLI/MCP adapters.
	if req.Config.Adapter != "" && runCfg.Provider == "" {
		runCfg.Adapter = req.Config.Adapter
	}
	if req.Config.TimeoutPerScenario > 0 {
		runCfg.Timeout = time.Duration(req.Config.TimeoutPerScenario) * time.Second
	}
	return runCfg
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[serve] encode response: %v", err)
	}
}

// evidraReporter implements orchestrator.ProgressReporter by sending
// progress webhooks and bench run submissions to the Evidra API.
type evidraReporter struct {
	progressURL string // POST progress updates here
	evidraURL   string // POST bench runs here
	authToken   string // Bearer token for both endpoints
}

// OnScenario sends a progress webhook and (on completion) submits the bench run.
func (r *evidraReporter) OnScenario(_ context.Context, ev orchestrator.ScenarioEvent) {
	log.Printf("[evidra-reporter] %s %s/%s (completed=%d/%d, progressURL=%q)",
		ev.Status, ev.ScenarioID, ev.Model, ev.Completed, ev.Total, r.progressURL)

	// Send progress webhook.
	r.sendProgress(ev)

	// Submit bench run on terminal status.
	if ev.Status == "passed" || ev.Status == "failed" {
		r.submitBenchRun(ev)
	}
}

func (r *evidraReporter) sendProgress(ev orchestrator.ScenarioEvent) {
	if r.progressURL == "" {
		return
	}
	payload := map[string]any{
		"contract_version": "v1.0.0",
		"job_id":           ev.JobID,
		"scenario":         ev.ScenarioID,
		"status":           ev.Status,
		"completed":        ev.Completed,
		"total":            ev.Total,
	}
	if ev.RunID != "" {
		payload["run_id"] = ev.RunID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[evidra-reporter] marshal progress: %v", err)
		return
	}
	req, err := http.NewRequest("POST", r.progressURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[evidra-reporter] create progress request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if r.authToken != "" {
		req.Header.Set("Authorization", r.authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[evidra-reporter] send progress: %v", err)
		return
	}
	log.Printf("[evidra-reporter] progress POST %s → %d", r.progressURL, resp.StatusCode)
	if err := resp.Body.Close(); err != nil {
		log.Printf("[evidra-reporter] close progress response: %v", err)
	}
}

func (r *evidraReporter) submitBenchRun(ev orchestrator.ScenarioEvent) {
	if r.evidraURL == "" {
		return
	}
	run := map[string]any{
		"id":               ev.RunID,
		"scenario_id":      ev.ScenarioID,
		"model":            ev.Model,
		"provider":         ev.Provider,
		"adapter":          "bench-cli",
		"evidence_mode":    "smart",
		"passed":           ev.Passed,
		"duration_seconds": ev.Duration.Seconds(),
		"checks_passed":    boolToInt(ev.Passed),
		"checks_total":     1,
	}
	body, err := json.Marshal(run)
	if err != nil {
		log.Printf("[evidra-reporter] marshal bench run: %v", err)
		return
	}
	url := r.evidraURL + "/v1/bench/runs"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[evidra-reporter] create bench run request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if r.authToken != "" {
		auth := r.authToken
		if !strings.HasPrefix(auth, "Bearer ") {
			auth = "Bearer " + auth
		}
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[evidra-reporter] submit bench run: %v", err)
		return
	}
	log.Printf("[evidra-reporter] bench run POST %s → %d", url, resp.StatusCode)
	if err := resp.Body.Close(); err != nil {
		log.Printf("[evidra-reporter] close bench run response: %v", err)
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
