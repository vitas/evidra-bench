package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/vitas/evidra-bench/internal/auth"
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
	ReviewDraftMode  string
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	benchService := benchsvc.NewService(benchRepo, benchsvc.ServiceConfig{
		PublicTenant:      publicTenant,
		ScenariosDir:      cfg.ScenariosDir,
		ReviewDraftMode:   opts.ReviewDraftMode,
		TriggerStore:      triggerStore,
		Dispatcher:        &benchsvc.PoolDispatcher{},
		BackgroundContext: ctx,
	})

	runner, teardown, err := prepareServeRunner(ctx, cfg, opts, defaultServeOrchestrator)
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	defer func() {
		teardownCtx, cancel := context.WithTimeout(context.Background(), config.GracefulStopTimeout)
		defer cancel()
		teardown(teardownCtx)
	}()
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
		mux.HandleFunc("POST /v1/certify", authMiddleware(apiToken, handleCertifyAPI(ctx, cfg, runner, dbURL)))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), config.GracefulStopTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("[bench-service] shutdown failed: %v", err)
		}
	}()

	log.Printf("bench service listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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
		auth.StaticKeyMiddleware(token, "")(next).ServeHTTP(w, r)
	}
}

func handleCertifyAPI(serviceCtx context.Context, baseCfg config.Config, runner parallelRunner, dbURL string) http.HandlerFunc {
	if serviceCtx == nil {
		serviceCtx = context.Background()
	}
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
		if authToken == "" {
			authToken = baseCfg.BenchAPIKey
		}
		runCfg.BenchURL = benchURL
		runCfg.BenchAPIKey = normalizeBenchAPIKey(authToken)

		reporter := &benchReporter{
			progressURL: progressURL,
			authToken:   authToken,
		}

		go func() {
			result, err := runner.RunParallel(serviceCtx, runCfg, reporter, scenarioPaths, []string{req.Model}, 1, parallel, dbURL)
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

// benchReporter implements orchestrator.ProgressReporter by sending progress
// webhooks to the Bench API. Run ingestion belongs to the harness, which has
// the artifact-backed localstore record.
type benchReporter struct {
	progressURL string // POST progress updates here
	authToken   string // Bearer token for progress endpoint
}

// OnScenario sends a progress webhook.
func (r *benchReporter) OnScenario(_ context.Context, ev orchestrator.ScenarioEvent) {
	log.Printf("[bench-reporter] %s %s/%s (completed=%d/%d, progressURL=%q)",
		ev.Status, ev.ScenarioID, ev.Model, ev.Completed, ev.Total, r.progressURL)

	r.sendProgress(ev)
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
	if authHeader := benchBearerHeader(r.authToken); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
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

func normalizeBenchAPIKey(authToken string) string {
	authToken = strings.TrimSpace(authToken)
	parts := strings.Fields(authToken)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return authToken
}

func benchBearerHeader(authToken string) string {
	apiKey := normalizeBenchAPIKey(authToken)
	if apiKey == "" {
		return ""
	}
	return "Bearer " + apiKey
}
