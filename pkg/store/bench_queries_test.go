package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestListRuns_Pagination(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for i := 0; i < 10; i++ {
		s.Insert(RunRecord{ID: fmt.Sprintf("r%02d", i), ScenarioID: "s1", Model: "sonnet", CreatedAt: now.Add(time.Duration(i) * time.Second)})
	}

	runs, total, err := s.ListRuns(ctx, RunFilters{Limit: 3, Offset: 0})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if total != 10 {
		t.Fatalf("expected total=10, got %d", total)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}

	// Second page
	runs2, total2, err := s.ListRuns(ctx, RunFilters{Limit: 3, Offset: 3})
	if err != nil {
		t.Fatalf("ListRuns page 2: %v", err)
	}
	if total2 != 10 {
		t.Fatalf("expected total=10 on page 2, got %d", total2)
	}
	if len(runs2) != 3 {
		t.Fatalf("expected 3 runs on page 2, got %d", len(runs2))
	}
	// Pages should not overlap
	if runs[0].ID == runs2[0].ID {
		t.Fatal("pages overlap")
	}
}

func TestListRuns_Filters(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	s.Insert(RunRecord{ID: "r1", ScenarioID: "s1", Model: "sonnet", Provider: "claude", Passed: true, CreatedAt: now})
	s.Insert(RunRecord{ID: "r2", ScenarioID: "s1", Model: "haiku", Provider: "claude", Passed: false, CreatedAt: now})
	s.Insert(RunRecord{ID: "r3", ScenarioID: "s2", Model: "sonnet", Provider: "bifrost", Passed: true, CreatedAt: now})

	tests := []struct {
		name    string
		filters RunFilters
		want    int
	}{
		{"by scenario", RunFilters{ScenarioID: "s1"}, 2},
		{"by model", RunFilters{Model: "sonnet"}, 2},
		{"by provider", RunFilters{Provider: "claude"}, 2},
		{"passed only", RunFilters{PassedOnly: true}, 2},
		{"failed only", RunFilters{FailedOnly: true}, 1},
		{"combined", RunFilters{ScenarioID: "s1", Model: "sonnet"}, 1},
		{"no filter", RunFilters{}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs, total, err := s.ListRuns(ctx, tt.filters)
			if err != nil {
				t.Fatalf("ListRuns: %v", err)
			}
			if len(runs) != tt.want {
				t.Fatalf("expected %d runs, got %d", tt.want, len(runs))
			}
			if total != tt.want {
				t.Fatalf("expected total=%d, got %d", tt.want, total)
			}
		})
	}
}

func TestGetRun(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()

	s.Insert(RunRecord{ID: "run-123", ScenarioID: "s1", Model: "sonnet", Passed: true, CreatedAt: time.Now().UTC()})

	r, err := s.GetRun(ctx, "run-123")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if r.Model != "sonnet" {
		t.Fatalf("expected sonnet, got %s", r.Model)
	}

	_, err = s.GetRun(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestCompareRuns(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	s.Insert(RunRecord{ID: "a", ScenarioID: "s1", Model: "sonnet", Passed: true, ChecksJSON: `{"passed":true,"checks":[{"name":"nginx","type":"deployment-ready","verdict":"pass"}]}`, CreatedAt: now})
	s.Insert(RunRecord{ID: "b", ScenarioID: "s1", Model: "haiku", Passed: false, ChecksJSON: `{"passed":false,"checks":[{"name":"nginx","type":"deployment-ready","verdict":"fail"}]}`, CreatedAt: now})

	cmp, err := s.CompareRuns(ctx, "a", "b")
	if err != nil {
		t.Fatalf("CompareRuns: %v", err)
	}
	if cmp.RunA.ID != "a" || cmp.RunB.ID != "b" {
		t.Fatal("wrong runs")
	}
	if len(cmp.CheckDiffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(cmp.CheckDiffs))
	}
	if cmp.CheckDiffs[0].Change != "regressed" {
		t.Fatalf("expected regressed, got %s", cmp.CheckDiffs[0].Change)
	}
}

func TestModelMatrix(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	s.Insert(RunRecord{ID: "r1", ScenarioID: "s1", Model: "sonnet", Passed: true, Duration: 10, EstimatedCost: 0.01, CreatedAt: now})
	s.Insert(RunRecord{ID: "r2", ScenarioID: "s1", Model: "sonnet", Passed: false, Duration: 20, EstimatedCost: 0.02, CreatedAt: now})
	s.Insert(RunRecord{ID: "r3", ScenarioID: "s1", Model: "haiku", Passed: true, Duration: 5, EstimatedCost: 0.001, CreatedAt: now})
	s.Insert(RunRecord{ID: "r4", ScenarioID: "s2", Model: "sonnet", Passed: true, Duration: 15, EstimatedCost: 0.015, CreatedAt: now})

	matrix, err := s.ModelMatrix(ctx, []string{"sonnet", "haiku"}, nil)
	if err != nil {
		t.Fatalf("ModelMatrix: %v", err)
	}
	if len(matrix.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(matrix.Models))
	}
	if len(matrix.Scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(matrix.Scenarios))
	}

	cell := matrix.Cells["s1"]["sonnet"]
	if cell.Runs != 2 {
		t.Fatalf("expected 2 runs for s1/sonnet, got %d", cell.Runs)
	}
	if cell.Passed != 1 {
		t.Fatalf("expected 1 passed for s1/sonnet, got %d", cell.Passed)
	}
	if cell.PassRate != 50 {
		t.Fatalf("expected 50%% pass rate, got %.0f%%", cell.PassRate)
	}
}

func TestFilteredStats(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	s.Insert(RunRecord{ID: "r1", ScenarioID: "s1", Model: "sonnet", Passed: true, CreatedAt: now})
	s.Insert(RunRecord{ID: "r2", ScenarioID: "s1", Model: "haiku", Passed: false, CreatedAt: now})
	s.Insert(RunRecord{ID: "r3", ScenarioID: "s2", Model: "sonnet", Passed: true, CreatedAt: now})

	st, err := s.FilteredStats(ctx, RunFilters{Model: "sonnet"})
	if err != nil {
		t.Fatalf("FilteredStats: %v", err)
	}
	if st.TotalRuns != 2 {
		t.Fatalf("expected 2 total, got %d", st.TotalRuns)
	}
	if st.PassCount != 2 {
		t.Fatalf("expected 2 pass, got %d", st.PassCount)
	}
}
