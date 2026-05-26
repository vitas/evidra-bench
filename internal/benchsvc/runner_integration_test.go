//go:build integration

package benchsvc

import (
	"context"
	"encoding/json"
	"testing"
)

func TestPgStore_RegisterAndListRunners(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	store := NewPgStore(db)
	tenantID := testID("tnt")
	seedTenant(t, db, tenantID)

	runner, err := store.RegisterRunner(context.Background(), tenantID, RegisterRunnerRequest{
		Name:        "test-runner",
		Models:      []string{"deepseek-chat", "qwen-plus"},
		Provider:    "bifrost",
		Region:      "eu-west",
		MaxParallel: 2,
	})
	if err != nil {
		t.Fatalf("RegisterRunner: %v", err)
	}
	if runner.ID == "" {
		t.Fatal("runner ID is empty")
	}
	if runner.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", runner.Status)
	}

	runners, err := store.ListRunners(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListRunners: %v", err)
	}
	if len(runners) != 1 {
		t.Fatalf("len = %d, want 1", len(runners))
	}
	if len(runners[0].Config.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(runners[0].Config.Models))
	}
}

func TestPgStore_DeleteRunner(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	store := NewPgStore(db)
	tenantID := testID("tnt")
	seedTenant(t, db, tenantID)

	runner, _ := store.RegisterRunner(context.Background(), tenantID, RegisterRunnerRequest{
		Name:   "delete-me",
		Models: []string{"test-model"},
	})

	if err := store.DeleteRunner(context.Background(), tenantID, runner.ID); err != nil {
		t.Fatalf("DeleteRunner: %v", err)
	}

	runners, _ := store.ListRunners(context.Background(), tenantID)
	if len(runners) != 0 {
		t.Fatalf("len = %d, want 0", len(runners))
	}
}

func TestPgStore_EnqueueAndClaimJob(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	store := NewPgStore(db)
	tenantID := testID("tnt")
	seedTenant(t, db, tenantID)

	job, err := store.EnqueueJob(context.Background(), tenantID, "deepseek-chat", "bifrost", JobConfig{
		Scenarios: []string{"broken-deployment", "privileged-pod"},
	})
	if err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}
	if job.Status != "queued" {
		t.Fatalf("status = %q, want queued", job.Status)
	}

	// Claim with matching model.
	claimed, err := store.ClaimJob(context.Background(), tenantID, "runner-1", []string{"deepseek-chat"})
	if err != nil {
		t.Fatalf("ClaimJob: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected a job, got nil")
		return
	}
	if claimed.Status != "claimed" {
		t.Fatalf("status = %q, want claimed", claimed.Status)
	}

	// Second claim returns nil (no more queued jobs).
	second, err := store.ClaimJob(context.Background(), tenantID, "runner-2", []string{"deepseek-chat"})
	if err != nil {
		t.Fatalf("ClaimJob 2: %v", err)
	}
	if second != nil {
		t.Fatal("expected nil, got a job")
	}

	// Claim with non-matching model returns nil.
	third, err := store.ClaimJob(context.Background(), tenantID, "runner-3", []string{"gpt-4.1"})
	if err != nil {
		t.Fatalf("ClaimJob 3: %v", err)
	}
	if third != nil {
		t.Fatal("expected nil for non-matching model")
	}
}

func TestPgStore_EnqueueAndClaimJob_PreservesScenarios(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	store := NewPgStore(db)
	tenantID := testID("tnt")
	seedTenant(t, db, tenantID)

	_, err := store.EnqueueJob(context.Background(), tenantID, "deepseek-chat", "bifrost", JobConfig{
		Scenarios: []string{"broken-deployment"},
	})
	if err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}

	claimed, err := store.ClaimJob(context.Background(), tenantID, "runner-1", []string{"deepseek-chat"})
	if err != nil {
		t.Fatalf("ClaimJob: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected a job, got nil")
		return
	}

	var cfg JobConfig
	if err := json.Unmarshal(claimed.ConfigJSON, &cfg); err != nil {
		t.Fatalf("unmarshal config_json: %v", err)
	}
	if len(cfg.Scenarios) != 1 || cfg.Scenarios[0] != "broken-deployment" {
		t.Fatalf("scenarios = %v, want [broken-deployment]", cfg.Scenarios)
	}
}

func TestPgStore_EnqueueAndClaimJob_PreservesExecutionMode(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	store := NewPgStore(db)
	tenantID := testID("tnt")
	seedTenant(t, db, tenantID)

	_, err := store.EnqueueJob(context.Background(), tenantID, "deepseek-chat", "bifrost", JobConfig{
		Scenarios:     []string{"broken-deployment"},
		ExecutionMode: "a2a",
	})
	if err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}

	claimed, err := store.ClaimJob(context.Background(), tenantID, "runner-1", []string{"deepseek-chat"})
	if err != nil {
		t.Fatalf("ClaimJob: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected a job, got nil")
		return
	}

	var cfg JobConfig
	if err := json.Unmarshal(claimed.ConfigJSON, &cfg); err != nil {
		t.Fatalf("unmarshal config_json: %v", err)
	}
	if cfg.ExecutionMode != "a2a" {
		t.Fatalf("execution_mode = %q, want a2a", cfg.ExecutionMode)
	}
}
