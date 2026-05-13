package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vitas/evidra-bench/internal/benchdb"
	"github.com/vitas/evidra-bench/internal/benchsvc"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/orchestrator"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

type parallelRunner interface {
	RunParallel(ctx context.Context, runCfg config.Config, reporter orchestrator.ProgressReporter, scenarios []string, models []string, repeats, parallel int, dbURL string) (*orchestrator.RunResult, error)
}

type serveOptions struct {
	ControlPlaneOnly bool
}

type serveOrchestrator interface {
	parallelRunner
	Provision(ctx context.Context) (string, error)
	Teardown(ctx context.Context)
}

type serveOrchestratorFactory func(config.Config) serveOrchestrator

// CertifyRequest matches the Bench executor contract v1.0.0.
type CertifyRequest struct {
	ContractVersion string   `json:"contract_version"`
	JobID           string   `json:"job_id"`
	Model           string   `json:"model"`
	Provider        string   `json:"provider"`
	Scenarios       []string `json:"scenarios"`
	Config          struct {
		TimeoutPerScenario int    `json:"timeout_per_scenario,omitempty"`
		Adapter            string `json:"adapter,omitempty"`
		A2AAgentURL        string `json:"a2a_agent_url,omitempty"`
		MCPServer          string `json:"mcp_server,omitempty"`
		ToolServer         string `json:"tool_server,omitempty"`
		ToolServerVersion  string `json:"tool_server_version,omitempty"`
		SkillFile          string `json:"skill_file,omitempty"`
		SkillID            string `json:"skill_id,omitempty"`
		SkillVersion       string `json:"skill_version,omitempty"`
		SkillSource        string `json:"skill_source,omitempty"`
		SkillSHA256        string `json:"skill_sha256,omitempty"`
	} `json:"config"`
	Callback struct {
		ProgressURL string `json:"progress_url"`
		BenchURL    string `json:"bench_url"`
		BenchAPIKey string `json:"bench_api_key"`
	} `json:"callback"`
}

func serveAPI(cfg config.Config, addr string, optList ...serveOptions) error {
	opts := serveOptions{}
	if len(optList) > 0 {
		opts = optList[0]
	}

	apiToken := cfg.BenchAPIKey
	if apiToken == "" {
		return fmt.Errorf("serve: --bench-api-key required for bench service authentication")
	}

	dbURL := cfg.ResolveDatabaseURL()
	if dbURL == "" {
		return fmt.Errorf("serve: --database-url required for bench service")
	}

	pool, err := benchdb.Connect(dbURL)
	if err != nil {
		return fmt.Errorf("serve: bench db: %w", err)
	}
	defer pool.Close()

	benchRepo := benchsvc.NewPgStore(pool)
	triggerStore := benchsvc.NewTriggerStore()
	defaultTenant, publicTenant := resolveServeTenants()
	benchService := benchsvc.NewService(benchRepo, benchsvc.ServiceConfig{
		PublicTenant: publicTenant,
		TriggerStore: triggerStore,
		Dispatcher:   &benchsvc.PoolDispatcher{},
	})

	ctx := context.Background()

	runner, teardown, err := prepareServeRunner(ctx, cfg, opts, defaultServeOrchestrator)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	defer teardown(ctx)
	go benchsvc.StartRunnerJanitor(ctx, benchRepo, 10*time.Second)

	// Sync scenarios to the configured bench API on startup.
	if cfg.BenchURL != "" {
		go func() {
			if err := pushScenarios(cfg.ScenariosDir, cfg.BenchURL, cfg.BenchAPIKey); err != nil {
				log.Printf("[bench-service] scenario sync failed (non-fatal): %v", err)
			} else {
				log.Printf("[bench-service] scenarios synced to %s", cfg.BenchURL)
			}
		}()
	}

	mux := http.NewServeMux()

	registerBenchAPIRoutes(mux, benchService, apiToken, defaultTenant)
	if runner == nil {
		mux.HandleFunc("POST /v1/certify", authMiddleware(apiToken, handleCertifyDisabled()))
	} else {
		mux.HandleFunc("POST /v1/certify", authMiddleware(apiToken, handleCertifyAPI(cfg, runner, dbURL)))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	log.Printf("bench service listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func resolveServeTenants() (defaultTenant string, publicTenant string) {
	defaultTenant = strings.TrimSpace(os.Getenv("BENCH_DEFAULT_TENANT"))
	publicTenant = strings.TrimSpace(os.Getenv("BENCH_PUBLIC_TENANT"))
	if defaultTenant == "" {
		defaultTenant = publicTenant
	}
	if defaultTenant == "" {
		defaultTenant = "default"
	}
	if publicTenant == "" {
		publicTenant = defaultTenant
	}
	return defaultTenant, publicTenant
}

func defaultServeOrchestrator(cfg config.Config) serveOrchestrator {
	return orchestrator.New(cfg, makeScenarioRunFunc())
}

func prepareServeRunner(ctx context.Context, cfg config.Config, opts serveOptions, factory serveOrchestratorFactory) (parallelRunner, func(context.Context), error) {
	if opts.ControlPlaneOnly {
		log.Printf("[bench-service] control-plane-only mode enabled; direct executor disabled")
		return nil, func(context.Context) {}, nil
	}

	orch := factory(cfg)
	if _, err := orch.Provision(ctx); err != nil {
		return nil, nil, err
	}
	return orch, orch.Teardown, nil
}

func handleCertifyDisabled() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "direct executor disabled in control-plane-only mode",
		})
	}
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
	// Load scenarios once at handler construction — cache full objects for filtering.
	scenarioMap := make(map[string]*scenario.Scenario) // ID → Scenario
	if allScenarios, loadErr := scenario.LoadAll(baseCfg.ScenariosDir); loadErr == nil {
		for _, s := range allScenarios {
			scenarioMap[s.ID] = s
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

		// Validate, resolve, and filter incompatible scenarios.
		var scenarioPaths []string
		skippedCount := 0
		for _, sid := range req.Scenarios {
			s, ok := scenarioMap[sid]
			if !ok {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown scenario: %q", sid)})
				return
			}
			if !s.IsProviderCompatible(baseCfg.EnvironmentProvider) {
				skippedCount++
				log.Printf("[bench-service] SKIP %s — requires %v, running on %s",
					sid, s.Environment.Providers, baseCfg.EnvironmentProvider)
				continue
			}
			scenarioPaths = append(scenarioPaths, s.Path)
		}

		if len(scenarioPaths) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("all %d scenarios incompatible with provider %s",
					len(req.Scenarios), baseCfg.EnvironmentProvider),
			})
			return
		}

		// Shared-cluster parallel mode only supports the default profile.
		// Collect the resolved scenario objects for profile validation.
		var selectedScenarios []*scenario.Scenario
		for _, sid := range req.Scenarios {
			if s, ok := scenarioMap[sid]; ok && s.IsProviderCompatible(baseCfg.EnvironmentProvider) {
				selectedScenarios = append(selectedScenarios, s)
			}
		}
		if err := orchestrator.ValidateParallelProfiles(selectedScenarios); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
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
		benchURL := req.Callback.BenchURL
		authToken := req.Callback.BenchAPIKey
		if benchURL == "" {
			benchURL = baseCfg.BenchURL
		}

		runMeta := config.CollectVersions(version, commit, runCfg).ToMetadata()
		reporter := &benchReporter{
			progressURL:       progressURL,
			benchURL:          benchURL,
			authToken:         authToken,
			adapter:           runCfg.Adapter,
			toolServer:        runCfg.ToolServerID,
			toolServerVersion: runCfg.ToolServerVersion,
			skillID:           runMeta["skill_id"],
			skillVersion:      runMeta["skill_version"],
			skillSource:       runMeta["skill_source"],
			skillSHA256:       runMeta["skill_sha256"],
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

		writeJSON(w, http.StatusAccepted, map[string]any{
			"job_id":  jobID,
			"status":  "accepted",
			"total":   fmt.Sprintf("%d", len(scenarioPaths)),
			"skipped": fmt.Sprintf("%d", skippedCount),
		})
	}
}

func buildCertifyRunConfig(baseCfg config.Config, req CertifyRequest) config.Config {
	runCfg := baseCfg
	// Only override provider if bench-cli natively supports it;
	// unknown providers (deepseek, openai, google) fall through to baseCfg default (bifrost).
	switch req.Provider {
	case "bifrost", "claude", "anthropic":
		runCfg.Provider = req.Provider
	}
	if req.Config.A2AAgentURL != "" {
		runCfg.A2AAgentURL = req.Config.A2AAgentURL
	}
	if req.Config.Adapter == "a2a" {
		runCfg.Adapter = "a2a"
	} else if runCfg.Provider == "" {
		runCfg.Provider = "bifrost"
	}
	if req.Config.TimeoutPerScenario > 0 {
		runCfg.Timeout = time.Duration(req.Config.TimeoutPerScenario) * time.Second
	}
	if req.Config.MCPServer != "" {
		runCfg.MCPServer = req.Config.MCPServer
	}
	if req.Config.ToolServer != "" {
		runCfg.ToolServerID = req.Config.ToolServer
	}
	if req.Config.ToolServerVersion != "" {
		runCfg.ToolServerVersion = req.Config.ToolServerVersion
	}
	if req.Config.SkillFile != "" {
		runCfg.SkillFile = req.Config.SkillFile
	}
	if req.Config.SkillID != "" {
		runCfg.SkillID = req.Config.SkillID
	}
	if req.Config.SkillVersion != "" {
		runCfg.SkillVersion = req.Config.SkillVersion
	}
	if req.Config.SkillSource != "" {
		runCfg.SkillSource = req.Config.SkillSource
	}
	if req.Config.SkillSHA256 != "" {
		runCfg.SkillSHA256 = req.Config.SkillSHA256
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

// benchReporter implements orchestrator.ProgressReporter by sending
// progress webhooks and bench run submissions to the Bench API.
type benchReporter struct {
	progressURL       string // POST progress updates here
	benchURL          string // POST bench runs here
	authToken         string // Bearer token for both endpoints
	adapter           string // configured bench execution mode
	toolServer        string // stable MCP server identity for run submissions
	toolServerVersion string // stable MCP server version for run submissions
	skillID           string // stable skill identity for run submissions
	skillVersion      string // stable skill version for run submissions
	skillSource       string // skill source label for run submissions
	skillSHA256       string // skill prompt digest for run submissions
}

// OnScenario sends a progress webhook and (on completion) submits the bench run.
func (r *benchReporter) OnScenario(_ context.Context, ev orchestrator.ScenarioEvent) {
	log.Printf("[bench-reporter] %s %s/%s (completed=%d/%d, progressURL=%q)",
		ev.Status, ev.ScenarioID, ev.Model, ev.Completed, ev.Total, r.progressURL)

	// Send progress webhook.
	r.sendProgress(ev)

	// Submit bench run on terminal status.
	if ev.Status == "passed" || ev.Status == "failed" || ev.Status == "error" {
		r.submitBenchRun(ev)
	}
}

func (r *benchReporter) sendProgress(ev orchestrator.ScenarioEvent) {
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
		log.Printf("[bench-reporter] marshal progress: %v", err)
		return
	}
	req, err := http.NewRequest("POST", r.progressURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[bench-reporter] create progress request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if r.authToken != "" {
		req.Header.Set("Authorization", r.authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[bench-reporter] send progress: %v", err)
		return
	}
	log.Printf("[bench-reporter] progress POST %s → %d", r.progressURL, resp.StatusCode)
	if err := resp.Body.Close(); err != nil {
		log.Printf("[bench-reporter] close progress response: %v", err)
	}
}

func (r *benchReporter) submitBenchRun(ev orchestrator.ScenarioEvent) {
	if r.benchURL == "" {
		return
	}
	adapterName := "bench-cli"
	if r.adapter == "a2a" {
		adapterName = "a2a"
	}
	run := map[string]any{
		"id":               ev.RunID,
		"scenario_id":      ev.ScenarioID,
		"model":            ev.Model,
		"provider":         ev.Provider,
		"adapter":          adapterName,
		"passed":           ev.Passed,
		"exit_code":        ev.ExitCode,
		"duration_seconds": ev.Duration.Seconds(),
		"checks_passed":    boolToInt(ev.Passed),
		"checks_total":     1,
	}
	if r.toolServer != "" {
		run["tool_server"] = r.toolServer
	}
	if r.toolServerVersion != "" {
		run["tool_server_version"] = r.toolServerVersion
	}
	if r.skillID != "" {
		run["skill_id"] = r.skillID
	}
	if r.skillVersion != "" {
		run["skill_version"] = r.skillVersion
	}
	if r.skillSource != "" {
		run["skill_source"] = r.skillSource
	}
	if r.skillSHA256 != "" {
		run["skill_sha256"] = r.skillSHA256
	}
	body, err := json.Marshal(run)
	if err != nil {
		log.Printf("[bench-reporter] marshal bench run: %v", err)
		return
	}
	url := r.benchURL + "/v1/bench/runs"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[bench-reporter] create bench run request: %v", err)
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
		log.Printf("[bench-reporter] submit bench run: %v", err)
		return
	}
	log.Printf("[bench-reporter] bench run POST %s → %d", url, resp.StatusCode)
	if err := resp.Body.Close(); err != nil {
		log.Printf("[bench-reporter] close bench run response: %v", err)
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
