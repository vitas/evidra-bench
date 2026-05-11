package benchsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	bench "samebits.com/evidra-infra-bench/pkg/bench"
)

// fakeRepo is an in-memory fake implementing Repository for unit tests.
type fakeRepo struct {
	leaderboardTenant string
	leaderboardMode   string
	compareMode       string
	matrixMode        string
	beginTxErr        error
	tx                pgx.Tx
	enabledModels     []EnabledModel
	enabledModelsErr  error
	lastTenant        string
	lastModelID       string
	lastProviderCfg   TenantProviderConfig
	lastGlobalCfg     GlobalModelConfig
}

func (f *fakeRepo) ListRuns(_ context.Context, _ string, _ bench.RunFilters) ([]bench.RunRecord, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) GetRun(_ context.Context, _ string, _ string) (*bench.RunRecord, error) {
	return nil, nil
}
func (f *fakeRepo) InsertRun(_ context.Context, _ string, _ bench.RunRecord) error { return nil }
func (f *fakeRepo) DeleteRun(_ context.Context, _, _ string) error                 { return nil }
func (f *fakeRepo) ArchiveRuns(_ context.Context, _ string, _ ArchiveRequest) (int, error) {
	return 0, nil
}
func (f *fakeRepo) InsertRunBatch(_ context.Context, _ string, _ []bench.RunRecord) (int, error) {
	return 0, nil
}
func (f *fakeRepo) FilteredStats(_ context.Context, _ string, _ bench.RunFilters) (*bench.StatsResult, error) {
	return nil, nil
}
func (f *fakeRepo) Catalog(_ context.Context, _ string) (*bench.RunCatalog, error) { return nil, nil }
func (f *fakeRepo) Leaderboard(_ context.Context, tenantID string, evidenceMode string, _ int, _ []string) ([]bench.LeaderboardEntry, error) {
	f.leaderboardTenant = tenantID
	f.leaderboardMode = evidenceMode
	return nil, nil
}
func (f *fakeRepo) ListScenarios(_ context.Context) ([]bench.ScenarioSummary, error) {
	return nil, nil
}
func (f *fakeRepo) StoreArtifact(_ context.Context, _, _, _ string, _ []byte) error { return nil }
func (f *fakeRepo) GetArtifact(_ context.Context, _, _, _ string) ([]byte, string, error) {
	return nil, "", nil
}
func (f *fakeRepo) CompareModels(_ context.Context, _, _, _, evidenceMode string) ([]ScenarioModelComparison, error) {
	f.compareMode = evidenceMode
	return nil, nil
}
func (f *fakeRepo) ModelMatrix(_ context.Context, _ string, _, _ []string, evidenceMode string) (*bench.ModelMatrix, error) {
	f.matrixMode = evidenceMode
	return nil, nil
}
func (f *fakeRepo) SignalSummary(_ context.Context, _ string, _ bench.RunFilters) (*bench.SignalAggregation, error) {
	return nil, nil
}
func (f *fakeRepo) Regressions(_ context.Context, _ string) ([]bench.Regression, error) {
	return nil, nil
}
func (f *fakeRepo) FailureAnalysis(_ context.Context, _ string, _ string) (*bench.FailureInsights, error) {
	return nil, nil
}
func (f *fakeRepo) UpsertScenarios(_ context.Context, _ []bench.ScenarioSummary) (int, error) {
	return 0, nil
}
func (f *fakeRepo) ListEnabledModels(_ context.Context, tenantID string) ([]EnabledModel, error) {
	f.lastTenant = tenantID
	return f.enabledModels, f.enabledModelsErr
}
func (f *fakeRepo) UpsertTenantProvider(_ context.Context, tenantID, modelID string, cfg TenantProviderConfig) error {
	f.lastTenant = tenantID
	f.lastModelID = modelID
	f.lastProviderCfg = cfg
	return nil
}
func (f *fakeRepo) DeleteTenantProvider(_ context.Context, tenantID, modelID string) error {
	f.lastTenant = tenantID
	f.lastModelID = modelID
	return nil
}
func (f *fakeRepo) UpdateGlobalModel(_ context.Context, modelID string, cfg GlobalModelConfig) error {
	f.lastModelID = modelID
	f.lastGlobalCfg = cfg
	return nil
}
func (f *fakeRepo) ResolveModelProvider(_ context.Context, _ string) (*ModelProviderInfo, error) {
	return nil, nil
}
func (f *fakeRepo) RegisterRunner(context.Context, string, RegisterRunnerRequest) (*Runner, error) {
	return nil, nil
}
func (f *fakeRepo) ListRunners(context.Context, string) ([]Runner, error) { return nil, nil }
func (f *fakeRepo) DeleteRunner(context.Context, string, string) error    { return nil }
func (f *fakeRepo) TouchRunner(context.Context, string, string) error     { return nil }
func (f *fakeRepo) EnqueueJob(context.Context, string, string, string, JobConfig) (*BenchJob, error) {
	return nil, nil
}
func (f *fakeRepo) ClaimJob(context.Context, string, string, []string) (*BenchJob, error) {
	return nil, nil
}
func (f *fakeRepo) CompleteJob(context.Context, string, string, string, string, int, int, string) error {
	return nil
}
func (f *fakeRepo) FindRunnerForModel(context.Context, string, string) (*Runner, error) {
	return nil, nil
}
func (f *fakeRepo) MarkUnhealthyRunners(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}
func (f *fakeRepo) ResetStaleJobs(_ context.Context, _ time.Duration) (int, error) { return 0, nil }
func (f *fakeRepo) UpdateJobProgress(_ context.Context, _ string, _, _, _ int) error {
	return nil
}
func (f *fakeRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	if f.beginTxErr != nil {
		return nil, f.beginTxErr
	}
	if f.tx != nil {
		return f.tx, nil
	}
	return nil, fmt.Errorf("fakeRepo: no real tx available")
}

func TestServiceListRuns_UsesProvidedTenant(t *testing.T) {
	t.Parallel()

	// Verify that buildWhere passes tenantID correctly.
	where, args := buildWhere("tenant-b", bench.RunFilters{})
	if len(args) == 0 || args[0] != "tenant-b" {
		t.Fatalf("buildWhere args[0] = %v, want tenant-b", args)
	}
	if where == "" {
		t.Fatal("buildWhere returned empty WHERE clause")
	}

	// Verify Service construction and that it stores the config.
	svc := NewService(&fakeRepo{}, ServiceConfig{PublicTenant: "bench-public"})
	if svc.cfg.PublicTenant != "bench-public" {
		t.Fatalf("PublicTenant = %q, want bench-public", svc.cfg.PublicTenant)
	}
}

func TestServiceLeaderboard_UsesPublicTenant(t *testing.T) {
	t.Parallel()

	// When PublicTenant is empty, Leaderboard must return ErrPublicTenantUnavailable.
	svc := NewService(&fakeRepo{}, ServiceConfig{})
	_, err := svc.Leaderboard(context.Background(), "mcp", 3, nil)
	if !errors.Is(err, ErrPublicTenantUnavailable) {
		t.Fatalf("Leaderboard err = %v, want ErrPublicTenantUnavailable", err)
	}

	// When PublicTenant is set, the repo's Leaderboard should be called
	// with the configured public tenant.
	repo := &fakeRepo{}
	svc2 := NewService(repo, ServiceConfig{PublicTenant: "bench-public"})
	_, _ = svc2.Leaderboard(context.Background(), "mcp", 3, nil)
	if repo.leaderboardTenant != "bench-public" {
		t.Fatalf("leaderboardTenant = %q, want bench-public", repo.leaderboardTenant)
	}
}

func TestServiceCompareModels_PreservesEmptyEvidenceMode(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := NewService(repo, ServiceConfig{})

	_, err := svc.CompareModels(context.Background(), "tenant-a", "sonnet", "opus", "")
	if err != nil {
		t.Fatalf("CompareModels: %v", err)
	}
	if repo.compareMode != "" {
		t.Fatalf("compareMode = %q, want empty", repo.compareMode)
	}
}

func TestServiceModelMatrix_PreservesEmptyEvidenceMode(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := NewService(repo, ServiceConfig{})

	_, err := svc.ModelMatrix(context.Background(), "tenant-a", []string{"sonnet"}, nil, "")
	if err != nil {
		t.Fatalf("ModelMatrix: %v", err)
	}
	if repo.matrixMode != "" {
		t.Fatalf("matrixMode = %q, want empty", repo.matrixMode)
	}
}

func TestServiceModelConfigMethods_DelegateToRepo(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		enabledModels: []EnabledModel{
			{ID: "gemini-2.5-flash"},
		},
	}
	svc := NewService(repo, ServiceConfig{})

	models, err := svc.ListEnabledModels(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("ListEnabledModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if repo.lastTenant != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", repo.lastTenant)
	}

	providerCfg := TenantProviderConfig{APIKeyEnc: "sk-secret", RateLimit: 10}
	if err := svc.UpsertTenantProvider(context.Background(), "tenant-a", "gemini-2.5-flash", providerCfg); err != nil {
		t.Fatalf("UpsertTenantProvider: %v", err)
	}
	if repo.lastModelID != "gemini-2.5-flash" {
		t.Fatalf("modelID = %q, want gemini-2.5-flash", repo.lastModelID)
	}
	if repo.lastProviderCfg.APIKeyEnc != "sk-secret" {
		t.Fatalf("APIKeyEnc = %q, want sk-secret", repo.lastProviderCfg.APIKeyEnc)
	}

	if err := svc.DeleteTenantProvider(context.Background(), "tenant-a", "gemini-2.5-flash"); err != nil {
		t.Fatalf("DeleteTenantProvider: %v", err)
	}
	if repo.lastTenant != "tenant-a" || repo.lastModelID != "gemini-2.5-flash" {
		t.Fatalf("delete captured tenant/model = %q/%q, want tenant-a/gemini-2.5-flash", repo.lastTenant, repo.lastModelID)
	}

	globalCfg := GlobalModelConfig{APIKeyEnv: "CUSTOM_API_KEY"}
	if err := svc.UpdateGlobalModel(context.Background(), "gemini-2.5-flash", globalCfg); err != nil {
		t.Fatalf("UpdateGlobalModel: %v", err)
	}
	if repo.lastGlobalCfg.APIKeyEnv != "CUSTOM_API_KEY" {
		t.Fatalf("APIKeyEnv = %q, want CUSTOM_API_KEY", repo.lastGlobalCfg.APIKeyEnv)
	}
}

func TestServiceIngestRun_RequiresTransaction(t *testing.T) {
	t.Parallel()

	// IngestRun must call BeginTx. With fakeRepo it returns an error.
	svc := NewService(&fakeRepo{}, ServiceConfig{})

	err := svc.IngestRun(context.Background(), "tenant-a", IngestRunRequest{
		RunRecord:  bench.RunRecord{ID: "run-1", ScenarioID: "s1", Model: "m1"},
		Transcript: "hello",
	})
	if err == nil {
		t.Fatal("expected error from fakeRepo BeginTx, got nil")
	}
}

func TestBuildWhere_TenantAlwaysFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tenant   string
		filters  bench.RunFilters
		wantArgs int
		wantSQL  string
	}{
		{
			name:     "tenant only",
			tenant:   "t1",
			filters:  bench.RunFilters{},
			wantArgs: 1,
		},
		{
			name:   "tenant plus scenario",
			tenant: "t2",
			filters: bench.RunFilters{
				ScenarioID: "broken-deployment",
			},
			wantArgs: 2,
			wantSQL:  "scenario_id = $2",
		},
		{
			name:   "tenant plus scenario list",
			tenant: "t2",
			filters: bench.RunFilters{
				ScenarioIDs: []string{"s1", "s2"},
			},
			wantArgs: 2,
			wantSQL:  "scenario_id = ANY($2::text[])",
		},
		{
			name:   "tenant plus model and provider",
			tenant: "t3",
			filters: bench.RunFilters{
				Model:    "sonnet",
				Provider: "anthropic",
			},
			wantArgs: 3,
		},
		{
			name:   "tenant plus tool server",
			tenant: "t4",
			filters: bench.RunFilters{
				ToolServer: "kubernetes-mcp",
			},
			wantArgs: 2,
			wantSQL:  "tool_server = $2",
		},
		{
			name:   "tenant plus empty tool server",
			tenant: "t4",
			filters: bench.RunFilters{
				ToolServerUnset: true,
			},
			wantArgs: 1,
			wantSQL:  "tool_server = ''",
		},
		{
			name:   "tenant plus tool server version",
			tenant: "t5",
			filters: bench.RunFilters{
				ToolServerVersion: "1.2.3",
			},
			wantArgs: 2,
			wantSQL:  "tool_server_version = $2",
		},
		{
			name:   "tenant plus report id",
			tenant: "t6",
			filters: bench.RunFilters{
				ReportID: "public-report",
			},
			wantArgs: 2,
			wantSQL:  "metadata_json->>'report_id' = $2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			where, args := buildWhere(tt.tenant, tt.filters)
			if args[0] != tt.tenant {
				t.Errorf("first arg = %v, want %v", args[0], tt.tenant)
			}
			if len(args) != tt.wantArgs {
				t.Errorf("args count = %d, want %d", len(args), tt.wantArgs)
			}
			if where == "" {
				t.Error("WHERE clause is empty")
			}
			if tt.wantSQL != "" && !strings.Contains(where, tt.wantSQL) {
				t.Errorf("WHERE clause = %q, want to contain %q", where, tt.wantSQL)
			}
		})
	}
}

func TestIngestRunRequest_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	autopsy := json.RawMessage(`{"outcome":"fail","primary_failure":"retry_loop"}`)
	req := IngestRunRequest{
		RunRecord:  bench.RunRecord{ID: "r1", ScenarioID: "s1", Model: "m1"},
		Transcript: "step 1\nstep 2",
		ToolCalls:  json.RawMessage(`[{"tool":"kubectl","args":["get","pods"]}]`),
		Autopsy:    autopsy,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded IngestRunRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != "r1" {
		t.Errorf("ID = %q, want r1", decoded.ID)
	}
	if decoded.Transcript != req.Transcript {
		t.Errorf("Transcript = %q, want %q", decoded.Transcript, req.Transcript)
	}
	if string(decoded.ToolCalls) != string(req.ToolCalls) {
		t.Errorf("ToolCalls = %s, want %s", decoded.ToolCalls, req.ToolCalls)
	}
	if string(decoded.Autopsy) != string(autopsy) {
		t.Errorf("Autopsy = %s, want %s", decoded.Autopsy, autopsy)
	}
}

func TestServiceIngestRun_StoresFailureAutopsyArtifact(t *testing.T) {
	t.Parallel()

	tx := &fakeTx{}
	repo := &fakeRepo{tx: tx}
	svc := NewService(repo, ServiceConfig{})

	autopsy := []byte(`{"outcome":"fail","primary_failure":"gave_up"}`)
	err := svc.IngestRun(context.Background(), "tenant-a", IngestRunRequest{
		RunRecord: bench.RunRecord{ID: "run-1", ScenarioID: "s1", Model: "m1"},
		Autopsy:   autopsy,
	})
	if err != nil {
		t.Fatalf("IngestRun: %v", err)
	}
	if len(tx.execArgs) != 2 {
		t.Fatalf("exec count = %d, want 2", len(tx.execArgs))
	}
	if got := tx.execArgs[1][1]; got != "failure_autopsy" {
		t.Fatalf("artifact type = %v, want failure_autopsy", got)
	}
	if got := tx.execArgs[1][2]; got != "application/json" {
		t.Fatalf("content type = %v, want application/json", got)
	}
	data, ok := tx.execArgs[1][3].([]byte)
	if !ok {
		t.Fatalf("artifact data type = %T, want []byte", tx.execArgs[1][3])
	}
	if string(data) != string(autopsy) {
		t.Fatalf("artifact data = %s, want %s", data, autopsy)
	}
}

func TestServiceIngestRun_PreservesToolServerIdentity(t *testing.T) {
	t.Parallel()

	tx := &fakeTx{}
	repo := &fakeRepo{tx: tx}
	svc := NewService(repo, ServiceConfig{})

	err := svc.IngestRun(context.Background(), "tenant-a", IngestRunRequest{
		RunRecord: bench.RunRecord{
			ID:                "run-1",
			ScenarioID:        "s1",
			Model:             "m1",
			EvidenceMode:      "mcp",
			ToolServer:        "kubernetes-mcp",
			ToolServerVersion: "1.2.3",
			ScenarioVersion:   "scenario-sha",
		},
	})
	if err != nil {
		t.Fatalf("IngestRun: %v", err)
	}
	if len(tx.execArgs) != 1 {
		t.Fatalf("exec count = %d, want 1", len(tx.execArgs))
	}
	args := tx.execArgs[0]
	if len(args) != 23 {
		t.Fatalf("insert args = %d, want 23", len(args))
	}
	if got := args[7]; got != "kubernetes-mcp" {
		t.Fatalf("tool_server arg = %v, want kubernetes-mcp", got)
	}
	if got := args[8]; got != "1.2.3" {
		t.Fatalf("tool_server_version arg = %v, want 1.2.3", got)
	}
	if got := args[9]; got != "scenario-sha" {
		t.Fatalf("scenario_version arg = %v, want scenario-sha", got)
	}
}

func TestServiceConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := ServiceConfig{}
	if cfg.PublicTenant != "" {
		t.Errorf("default PublicTenant = %q, want empty", cfg.PublicTenant)
	}
}
