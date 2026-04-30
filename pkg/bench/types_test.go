package bench

import (
	"encoding/json"
	"testing"
)

func TestLeaderboardEntry_PassKFields(t *testing.T) {
	t.Parallel()

	e := LeaderboardEntry{
		Model:               "sonnet",
		Scenarios:           10,
		Runs:                30,
		PassRate:            80.0,
		AvgDuration:         45.2,
		AvgCost:             0.15,
		TotalCost:           4.50,
		PassK:               51.8,
		PassKTrials:         3,
		SufficientScenarios: 8,
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded LeaderboardEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.PassK != 51.8 {
		t.Errorf("PassK = %v, want 51.8", decoded.PassK)
	}
	if decoded.PassKTrials != 3 {
		t.Errorf("PassKTrials = %v, want 3", decoded.PassKTrials)
	}
	if decoded.SufficientScenarios != 8 {
		t.Errorf("SufficientScenarios = %v, want 8", decoded.SufficientScenarios)
	}
}

func TestLeaderboardEntry_PassKJSONTags(t *testing.T) {
	t.Parallel()

	e := LeaderboardEntry{
		PassK:               42.5,
		PassKTrials:         5,
		SufficientScenarios: 3,
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	if _, ok := m["pass_k"]; !ok {
		t.Error("JSON missing pass_k key")
	}
	if _, ok := m["pass_k_trials"]; !ok {
		t.Error("JSON missing pass_k_trials key")
	}
	if _, ok := m["sufficient_scenarios"]; !ok {
		t.Error("JSON missing sufficient_scenarios key")
	}
}
