//go:build integration

package benchsvc

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/vitas/evidra-bench/internal/benchdb"
	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	pool, err := db.Connect(databaseURL)
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	resetModelConfigTables(t, pool)
	return pool
}

func resetModelConfigTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(context.Background(), `DELETE FROM bench_tenant_providers`); err != nil {
		t.Fatalf("clear bench_tenant_providers: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM bench_models`); err != nil {
		t.Fatalf("clear bench_models: %v", err)
	}
}

func testID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ToLower(ulid.Make().String()))
}

func seedTenant(t *testing.T, pool *pgxpool.Pool, tenantID string) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenants (id, label) VALUES ($1, $2)
		 ON CONFLICT (id) DO NOTHING`,
		tenantID, "Benchsvc Integration Tenant")
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
}

func seedModel(t *testing.T, pool *pgxpool.Pool, modelID string) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO bench_models (id, display_name, provider, input_cost_per_mtok, output_cost_per_mtok)
		 VALUES ($1, $2, $3, $4, $5)`,
		modelID, "Test Model", "custom", 0.5, 1.0)
	if err != nil {
		t.Fatalf("insert model: %v", err)
	}
}

func seedModelWithEnv(t *testing.T, pool *pgxpool.Pool, modelID, apiKeyEnv string) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO bench_models (id, display_name, provider, api_key_env, input_cost_per_mtok, output_cost_per_mtok)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		modelID, "Platform Model", "google", apiKeyEnv, 0.15, 0.60)
	if err != nil {
		t.Fatalf("insert model with env: %v", err)
	}
}

func TestPgStore_ListEnabledModels(t *testing.T) {
	pool := setupTestDB(t)
	store := NewPgStore(pool)
	tenantID := testID("tnt")
	platformModelID := testID("gemini")
	customModelID := testID("custom")

	seedTenant(t, pool, tenantID)
	seedModelWithEnv(t, pool, platformModelID, "GEMINI_API_KEY")
	seedModel(t, pool, customModelID)

	_, err := pool.Exec(context.Background(),
		`INSERT INTO bench_tenant_providers (tenant_id, model_id, api_key_enc, enabled)
		 VALUES ($1, $2, $3, true)`,
		tenantID, customModelID, "sk-secret")
	if err != nil {
		t.Fatalf("insert tenant provider: %v", err)
	}

	models, err := store.ListEnabledModels(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListEnabledModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
}

func TestPgStore_UpsertTenantProvider(t *testing.T) {
	pool := setupTestDB(t)
	store := NewPgStore(pool)
	tenantID := testID("tnt")
	modelID := testID("model")

	seedTenant(t, pool, tenantID)
	seedModel(t, pool, modelID)

	err := store.UpsertTenantProvider(context.Background(), tenantID, modelID, TenantProviderConfig{
		APIKeyEnc:  "sk-first",
		APIBaseURL: "https://custom.example.com",
		RateLimit:  10,
	})
	if err != nil {
		t.Fatalf("UpsertTenantProvider insert: %v", err)
	}

	err = store.UpsertTenantProvider(context.Background(), tenantID, modelID, TenantProviderConfig{
		APIKeyEnc: "sk-updated",
	})
	if err != nil {
		t.Fatalf("UpsertTenantProvider update: %v", err)
	}

	models, err := store.ListEnabledModels(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListEnabledModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}

	var count int
	var apiKey string
	err = pool.QueryRow(context.Background(),
		`SELECT COUNT(*), MAX(api_key_enc)
		 FROM bench_tenant_providers
		 WHERE tenant_id = $1 AND model_id = $2`,
		tenantID, modelID,
	).Scan(&count, &apiKey)
	if err != nil {
		t.Fatalf("select tenant provider: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if apiKey != "sk-updated" {
		t.Fatalf("api_key_enc = %q, want sk-updated", apiKey)
	}
}

func TestPgStore_DeleteTenantProvider(t *testing.T) {
	pool := setupTestDB(t)
	store := NewPgStore(pool)
	tenantID := testID("tnt")
	modelID := testID("model")

	seedTenant(t, pool, tenantID)
	seedModel(t, pool, modelID)

	if err := store.UpsertTenantProvider(context.Background(), tenantID, modelID, TenantProviderConfig{
		APIKeyEnc: "sk-delete-me",
	}); err != nil {
		t.Fatalf("UpsertTenantProvider: %v", err)
	}

	if err := store.DeleteTenantProvider(context.Background(), tenantID, modelID); err != nil {
		t.Fatalf("DeleteTenantProvider: %v", err)
	}

	models, err := store.ListEnabledModels(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListEnabledModels: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("len(models) = %d, want 0", len(models))
	}
}

func TestPgStore_UpdateGlobalModel(t *testing.T) {
	pool := setupTestDB(t)
	store := NewPgStore(pool)
	modelID := testID("model")

	seedModel(t, pool, modelID)

	err := store.UpdateGlobalModel(context.Background(), modelID, GlobalModelConfig{
		APIBaseURL: "https://gateway.example.com/v1",
		APIKeyEnv:  "CUSTOM_API_KEY",
	})
	if err != nil {
		t.Fatalf("UpdateGlobalModel: %v", err)
	}

	models, err := store.ListEnabledModels(context.Background(), testID("tenant"))
	if err != nil {
		t.Fatalf("ListEnabledModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}

	var apiBaseURL, apiKeyEnv string
	err = pool.QueryRow(context.Background(),
		`SELECT api_base_url, api_key_env
		 FROM bench_models
		 WHERE id = $1`,
		modelID,
	).Scan(&apiBaseURL, &apiKeyEnv)
	if err != nil {
		t.Fatalf("select updated model: %v", err)
	}
	if apiBaseURL != "https://gateway.example.com/v1" {
		t.Fatalf("api_base_url = %q, want https://gateway.example.com/v1", apiBaseURL)
	}
	if apiKeyEnv != "CUSTOM_API_KEY" {
		t.Fatalf("api_key_env = %q, want CUSTOM_API_KEY", apiKeyEnv)
	}
}

func TestPgStore_ModelMatrix_AggregatesAllRuns(t *testing.T) {
	pool := setupTestDB(t)
	store := NewPgStore(pool)
	tenantID := testID("tnt")

	seedTenant(t, pool, tenantID)

	runs := []bench.RunRecord{
		{
			ID:            testID("run"),
			TenantID:      tenantID,
			ScenarioID:    "broken-deployment",
			Model:         "sonnet",
			Passed:        true,
			Duration:      10,
			EstimatedCost: 1.0,
			CreatedAt:     time.Now(),
		},
		{
			ID:            testID("run"),
			TenantID:      tenantID,
			ScenarioID:    "broken-deployment",
			Model:         "sonnet",
			Passed:        false,
			Duration:      20,
			EstimatedCost: 2.0,
			CreatedAt:     time.Now(),
		},
		{
			ID:            testID("run"),
			TenantID:      tenantID,
			ScenarioID:    "broken-deployment",
			Model:         "opus",
			Passed:        true,
			Duration:      30,
			EstimatedCost: 3.0,
			CreatedAt:     time.Now(),
		},
	}

	for _, run := range runs {
		if err := store.InsertRun(context.Background(), tenantID, run); err != nil {
			t.Fatalf("InsertRun %s: %v", run.ID, err)
		}
	}

	matrix, err := store.ModelMatrix(context.Background(), tenantID, nil, nil)
	if err != nil {
		t.Fatalf("ModelMatrix: %v", err)
	}
	if len(matrix.Models) != 2 {
		t.Fatalf("models = %v, want 2 models", matrix.Models)
	}
	if cell := matrix.Cells["broken-deployment"]["sonnet"]; cell.Runs != 2 || cell.Passed != 1 || cell.PassRate != 50 {
		t.Fatalf("sonnet cell = %+v, want two runs with one pass", cell)
	}
	if cell := matrix.Cells["broken-deployment"]["opus"]; cell.Runs != 1 || cell.Passed != 1 {
		t.Fatalf("opus cell = %+v, want one passed run", cell)
	}
}
