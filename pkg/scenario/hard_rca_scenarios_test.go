package scenario

import (
	"path/filepath"
	"testing"
)

func TestHardRCAScenarios_LoadWithAutopsyHints(t *testing.T) {
	t.Parallel()

	root := projectRoot()
	scenariosDir := filepath.Join(root, "scenarios")
	scenarios, err := LoadAll(scenariosDir)
	if err != nil {
		t.Fatalf("load scenarios: %v", err)
	}

	byID := make(map[string]*Scenario, len(scenarios))
	for _, s := range scenarios {
		byID[s.ID] = s
	}

	required := []struct {
		id       string
		category string
		level    string
	}{
		{id: "prompt-injection-in-logs", category: "kubernetes", level: "L4"},
		{id: "canary-selector-blast-radius", category: "kubernetes", level: "L4"},
		{id: "helm-hook-ordering-observability-trap", category: "helm", level: "L4"},
	}

	for _, want := range required {
		want := want
		t.Run(want.id, func(t *testing.T) {
			t.Parallel()

			s, ok := byID[want.id]
			if !ok {
				t.Fatalf("scenario %q not found", want.id)
			}
			if s.PrimaryCategory() != want.category {
				t.Fatalf("category = %q, want %q", s.PrimaryCategory(), want.category)
			}
			if s.Level != want.level {
				t.Fatalf("level = %q, want %q", s.Level, want.level)
			}
			if len(s.Autopsy.ExpectedDiagnostics) == 0 {
				t.Fatal("missing autopsy expected diagnostics")
			}
			if len(s.Autopsy.AllowedMutations) == 0 {
				t.Fatal("missing autopsy allowed mutations")
			}
			if len(s.Autopsy.ForbiddenActions) == 0 {
				t.Fatal("missing autopsy forbidden actions")
			}
			if len(s.Autopsy.RootCauseResources) == 0 {
				t.Fatal("missing autopsy root cause resources")
			}
		})
	}
}

func TestShowcaseAutopsyCoverage(t *testing.T) {
	t.Parallel()

	root := projectRoot()
	scenariosDir := filepath.Join(root, "scenarios")
	scenarios, err := LoadAll(scenariosDir)
	if err != nil {
		t.Fatalf("load scenarios: %v", err)
	}

	byID := make(map[string]*Scenario, len(scenarios))
	for _, s := range scenarios {
		byID[s.ID] = s
	}

	required := []struct {
		id       string
		full     bool
		noMutate bool
	}{
		{id: "false-alarm", noMutate: true},
		{id: "shared-configmap-trap", full: true},
		{id: "urgency-vs-safety", full: true},
		{id: "safe-rollback-vs-broad-patch", full: true},
		{id: "risky-shortcut", full: true},
		{id: "resource-pressure-shortcut", full: true},
		{id: "partial-context-remediation", full: true},
		{id: "config-mutation-mid-fix", full: true},
		{id: "cascading-failures", full: true},
		{id: "repair-loop-escalation", full: true},
		{id: "wrong-namespace-similarity", full: true},
		{id: "delete-prod-namespace", full: true},
		{id: "prompt-injection-in-logs", full: true},
		{id: "canary-selector-blast-radius", full: true},
		{id: "helm-hook-ordering-observability-trap", full: true},
	}

	for _, want := range required {
		want := want
		t.Run(want.id, func(t *testing.T) {
			t.Parallel()

			s, ok := byID[want.id]
			if !ok {
				t.Fatalf("scenario %q not found", want.id)
			}
			if len(s.Autopsy.ForbiddenActions) == 0 {
				t.Fatal("missing autopsy forbidden actions")
			}
			if want.noMutate {
				if len(s.Autopsy.AllowedMutations) != 0 {
					t.Fatal("no-mutation scenario should not declare allowed mutations")
				}
				return
			}
			if want.full {
				if len(s.Autopsy.ExpectedDiagnostics) == 0 {
					t.Fatal("missing autopsy expected diagnostics")
				}
				if len(s.Autopsy.AllowedMutations) == 0 {
					t.Fatal("missing autopsy allowed mutations")
				}
				if len(s.Autopsy.RootCauseResources) == 0 {
					t.Fatal("missing autopsy root cause resources")
				}
			}
		})
	}
}
