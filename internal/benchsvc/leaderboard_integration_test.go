//go:build integration

package benchsvc

import (
	"context"
	"math"
	"testing"
	"time"

	bench "github.com/vitas/evidra-bench/pkg/bench"
)

func TestLeaderboard_PassKMath(t *testing.T) {
	pool := setupTestDB(t)
	store := NewPgStore(pool)
	ctx := context.Background()

	tenantID := testID("tnt")
	seedTenant(t, pool, tenantID)

	model := "passk-test-model"

	// Scenario A: 3/3 pass -> pass_rate=1.0, p^3=1.0
	for i := 0; i < 3; i++ {
		err := store.InsertRun(ctx, tenantID, bench.RunRecord{
			ID:         testID("run"),
			ScenarioID: "scenario-a",
			Model:      model,
			Provider:   "test",
			Passed:     true,
			Duration:   10.0,
			CreatedAt:  time.Now(),
		})
		if err != nil {
			t.Fatalf("insert run A-%d: %v", i, err)
		}
	}

	// Scenario B: 1/3 pass -> pass_rate=0.333, p^3=0.037
	for i := 0; i < 3; i++ {
		passed := i == 0
		err := store.InsertRun(ctx, tenantID, bench.RunRecord{
			ID:         testID("run"),
			ScenarioID: "scenario-b",
			Model:      model,
			Provider:   "test",
			Passed:     passed,
			Duration:   10.0,
			CreatedAt:  time.Now(),
		})
		if err != nil {
			t.Fatalf("insert run B-%d: %v", i, err)
		}
	}

	entries, err := store.Leaderboard(ctx, tenantID, "", 3, nil)
	if err != nil {
		t.Fatalf("Leaderboard: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]

	// pass_rate is run-weighted: 4/6 = 66.67%
	if math.Abs(e.PassRate-66.67) > 0.5 {
		t.Errorf("PassRate = %.2f, want ~66.67 (run-weighted)", e.PassRate)
	}

	// pass^3: avg(1.0^3, (1/3)^3) * 100 = avg(1.0, 0.037) * 100 = 51.85
	if math.Abs(e.PassK-51.85) > 1.0 {
		t.Errorf("PassK = %.2f, want ~51.85 (scenario-weighted power)", e.PassK)
	}

	if e.PassKTrials != 3 {
		t.Errorf("PassKTrials = %d, want 3", e.PassKTrials)
	}

	if e.SufficientScenarios != 2 {
		t.Errorf("SufficientScenarios = %d, want 2", e.SufficientScenarios)
	}

	if e.Runs != 6 {
		t.Errorf("Runs = %d, want 6", e.Runs)
	}

	if e.Scenarios != 2 {
		t.Errorf("Scenarios = %d, want 2", e.Scenarios)
	}
}

func TestLeaderboard_PassKInsufficientTrials(t *testing.T) {
	pool := setupTestDB(t)
	store := NewPgStore(pool)
	ctx := context.Background()

	tenantID := testID("tnt")
	seedTenant(t, pool, tenantID)

	model := "passk-insufficient-model"

	// Only 2 runs per scenario, but k=3 -> no qualifying scenarios.
	for _, scenario := range []string{"scenario-x", "scenario-y"} {
		for i := 0; i < 2; i++ {
			err := store.InsertRun(ctx, tenantID, bench.RunRecord{
				ID:         testID("run"),
				ScenarioID: scenario,
				Model:      model,
				Provider:   "test",
				Passed:     true,
				Duration:   5.0,
				CreatedAt:  time.Now(),
			})
			if err != nil {
				t.Fatalf("insert run: %v", err)
			}
		}
	}

	entries, err := store.Leaderboard(ctx, tenantID, "", 3, nil)
	if err != nil {
		t.Fatalf("Leaderboard: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]

	// pass_rate should still be computed (run-weighted).
	if e.PassRate != 100.0 {
		t.Errorf("PassRate = %.2f, want 100.0", e.PassRate)
	}

	// pass^k should be 0 because no scenarios have >= 3 trials.
	if e.PassK != 0.0 {
		t.Errorf("PassK = %.2f, want 0.0 (insufficient trials)", e.PassK)
	}

	if e.SufficientScenarios != 0 {
		t.Errorf("SufficientScenarios = %d, want 0", e.SufficientScenarios)
	}
}
