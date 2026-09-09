package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vitas/evidra-bench/pkg/agent"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/modelconfig"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/suite"
)

const defaultTestCaseTimeout = 5 * time.Minute

type preparedTestEvaluation struct {
	Plan          evaluation.Plan
	Suite         *suite.Loaded
	Config        config.Config
	ModelProvider agent.Provider
}

func prepareTestEvaluation(req testRequest, lookupEnv func(string) string) (*preparedTestEvaluation, error) {
	if req.Environment != "kind" && req.Environment != "k3d" {
		return nil, fmt.Errorf("test: environment must be kind or k3d, got %q", req.Environment)
	}
	if req.Timeout <= 0 {
		return nil, fmt.Errorf("test: timeout must be positive")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("test: resolve working directory: %w", err)
	}
	root, err := resolveTestAssetsRoot(req.ProjectRoot, lookupEnv, cwd, "/opt/evidra")
	if err != nil {
		return nil, err
	}
	manifestPath, err := testSuiteManifest(req.Suite)
	if err != nil {
		return nil, err
	}
	loaded, err := suite.Load(filepath.Join(root, manifestPath), root)
	if err != nil {
		return nil, fmt.Errorf("test: load suite: %w", err)
	}
	if !containsString(loaded.Manifest.Environment.Providers, req.Environment) {
		return nil, fmt.Errorf("test: suite %s does not support environment %s", loaded.Identity, req.Environment)
	}

	cfg := config.Default()
	cfg.EnvironmentProvider = req.Environment
	cfg.ScenariosDir = filepath.Join(root, "scenarios")
	cfg.RunsDir = filepath.Join(req.OutputDir, "runs")
	cfg.Timeout = req.Timeout
	cfg.ClusterName = oneCommandClusterName(os.Getpid(), time.Now())
	cfg.Adapter = "cli"

	prepared := &preparedTestEvaluation{Suite: loaded, Config: cfg}
	var target evaluation.TargetPlan
	if strings.TrimSpace(req.Agent) != "" {
		cfg.AgentCommand = req.Agent
		digest := sha256.Sum256([]byte(req.Agent))
		target = evaluation.TargetPlan{
			Kind:            evaluation.TargetAgent,
			Adapter:         "cli",
			CommandIdentity: "sha256:" + hex.EncodeToString(digest[:]),
		}
	} else {
		resolved, resolveErr := modelconfig.Resolve(modelconfig.Input{Model: req.Model, Endpoint: req.Endpoint, LookupEnv: lookupEnv})
		if resolveErr != nil {
			return nil, resolveErr
		}
		cfg.Provider = resolved.Provider
		cfg.Model = resolved.Model
		target = resolved.EvaluationTarget()
		prepared.ModelProvider, err = agent.ResolveProviderWithConfig(resolved.Provider, agent.OpenAICompatibleConfig{
			Name:    resolved.Provider,
			BaseURL: resolved.Endpoint,
			APIKey:  resolved.Credential,
			Retry:   agent.DefaultRetryConfig(),
		})
		if err != nil {
			return nil, fmt.Errorf("test: configure model provider: %w", err)
		}
	}
	prepared.Config = cfg
	prepared.Plan = evaluation.Plan{
		Version:     evaluation.PlanVersion,
		Suite:       loaded.EvaluationSuite(),
		Environment: evaluation.EnvironmentPlan{Provider: req.Environment, Profile: string(loaded.Manifest.Environment.Profile)},
		Target:      target,
		Limits:      evaluation.Limits{CaseTimeout: req.Timeout, MaxTurns: 25},
		Attempts:    1,
		Output: evaluation.OutputPolicy{
			Directory: req.OutputDir,
			Formats:   []evaluation.OutputFormat{evaluation.OutputTerminal, evaluation.OutputHTML, evaluation.OutputJSON},
		},
	}
	return prepared, nil
}

func oneCommandClusterName(processID int, now time.Time) string {
	return fmt.Sprintf("evidra-%d-%s", processID, strconv.FormatInt(now.UnixNano(), 36))
}

func resolveTestAssetsRoot(explicit string, lookupEnv func(string) string, cwd, imageRoot string) (string, error) {
	if lookupEnv == nil {
		lookupEnv = os.Getenv
	}
	if root := strings.TrimSpace(explicit); root != "" {
		return validateTestAssetsRoot(root)
	}
	if root := strings.TrimSpace(lookupEnv("EVIDRA_ASSETS_DIR")); root != "" {
		return validateTestAssetsRoot(root)
	}
	if root := strings.TrimSpace(imageRoot); root != "" && hasTestSuiteAssets(root) {
		return filepath.Abs(root)
	}
	if root := strings.TrimSpace(cwd); root != "" && hasTestSuiteAssets(root) {
		return filepath.Abs(root)
	}
	return "", fmt.Errorf("test: suite assets not found; run from the Evidra source tree or set EVIDRA_ASSETS_DIR")
}

func validateTestAssetsRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("test: resolve suite assets: %w", err)
	}
	if !hasTestSuiteAssets(abs) {
		return "", fmt.Errorf("test: suite assets not found under %s", abs)
	}
	return abs, nil
}

func hasTestSuiteAssets(root string) bool {
	info, err := os.Stat(filepath.Join(root, "suites", "kubernetes-demo-v1.yaml"))
	return err == nil && info.Mode().IsRegular()
}

func testSuiteManifest(id string) (string, error) {
	switch strings.TrimSpace(id) {
	case "", "kubernetes-demo", "kubernetes-demo@1":
		return "suites/kubernetes-demo-v1.yaml", nil
	default:
		return "", fmt.Errorf("test: unknown suite %q (available: kubernetes-demo@1)", id)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func runOneCommandEvaluation(ctx context.Context, req testRequest) (evaluation.Result, error) {
	prepared, err := prepareTestEvaluation(req, os.Getenv)
	if err != nil {
		return evaluation.Result{}, err
	}
	service := evaluation.Service{
		Preflights: []evaluation.Preflight{localTestPreflight{Environment: req.Environment}},
		Provisioner: suiteEvaluationProvisioner{
			Config: prepared.Config,
			Suite:  prepared.Suite,
		},
		Executor: suiteEvaluationExecutor{
			Config:        prepared.Config,
			Suite:         prepared.Suite,
			ModelProvider: prepared.ModelProvider,
		},
		CleanupTimeout: config.GracefulStopTimeout,
	}
	return service.Run(ctx, prepared.Plan)
}

type localTestPreflight struct {
	Environment string
}

func (p localTestPreflight) Check(ctx context.Context, _ evaluation.Plan) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is required: install Docker and retry")
	}
	if _, err := exec.LookPath(p.Environment); err != nil {
		return fmt.Errorf("%s is required for the selected environment", p.Environment)
	}
	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker is installed but unavailable: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

type suiteEvaluationProvisioner struct {
	Config config.Config
	Suite  *suite.Loaded
}

func (p suiteEvaluationProvisioner) Acquire(ctx context.Context, _ evaluation.Plan) (evaluation.Lease, error) {
	if p.Suite == nil || len(p.Suite.Scenarios) == 0 {
		return nil, fmt.Errorf("test: suite has no scenarios")
	}
	first := p.Suite.Scenarios[0]
	lease, err := newLocalProvisioner(p.Config).Acquire(ctx, environment.ProvisionRequest{
		Scenario:           first,
		Profile:            p.Suite.Manifest.Environment.Profile,
		ProviderName:       p.Config.EnvironmentProvider,
		ClusterName:        p.Config.ClusterName,
		ReuseCluster:       p.Config.ReuseCluster,
		ExistingKubeconfig: p.Config.KubeconfigPath,
		Shared:             true,
	})
	if err != nil {
		return nil, err
	}
	return &legacyEvaluationLease{lease: lease}, nil
}

type suiteEvaluationExecutor struct {
	Config        config.Config
	Suite         *suite.Loaded
	ModelProvider agent.Provider
}

func (e suiteEvaluationExecutor) Execute(ctx context.Context, _ evaluation.Plan, c evaluation.CasePlan, lease evaluation.Lease) (evaluation.CaseResult, error) {
	s := e.findScenario(c.ID)
	if s == nil {
		return evaluation.CaseResult{}, fmt.Errorf("test: suite contains no case %q", c.ID)
	}
	localLease, ok := lease.(*legacyEvaluationLease)
	if !ok || localLease.lease == nil {
		return evaluation.CaseResult{}, fmt.Errorf("test: unexpected environment lease %T", lease)
	}
	if err := s.ProviderCompatibilityError(e.Config.EnvironmentProvider); err != nil {
		return evaluation.CaseResult{}, err
	}
	runtime, err := buildLocalHarnessRuntimeWithProvider(e.Config, localLease.lease.Provider, nil, e.ModelProvider)
	if err != nil {
		return evaluation.CaseResult{}, err
	}
	defer runtime.Close()
	result, runErr := harness.New(runtime.Deps).Run(ctx, harness.RunRequest{
		Config:         e.Config,
		Scenario:       s,
		KubeconfigPath: localLease.lease.KubeconfigPath,
		ExtraEnv:       localLease.lease.ExtraEnv,
	})
	if result != nil && result.Case != nil {
		return *result.Case, runErr
	}
	if runErr != nil {
		return evaluation.CaseResult{}, runErr
	}
	return evaluation.CaseResult{}, fmt.Errorf("test: harness returned no canonical case result")
}

func (e suiteEvaluationExecutor) findScenario(id string) *scenario.Scenario {
	if e.Suite == nil {
		return nil
	}
	for _, s := range e.Suite.Scenarios {
		if s.ID == id {
			return s
		}
	}
	return nil
}
