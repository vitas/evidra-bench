package scenariopatch

import (
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/runreview"
)

func TestPreviewAddsReviewSuggestedRulesToAutopsySections(t *testing.T) {
	t.Parallel()

	scenarioYAML := []byte(`id: shared-configmap-trap
title: Shared ConfigMap Trap
prompt: prompts/task.md
checks:
  - type: deployment_ready
    namespace: bench
    name: web
autopsy:
  description: Existing guidance.
`)
	review := runreview.Review{
		Version:    runreview.Version,
		RunID:      "run-1",
		ScenarioID: "shared-configmap-trap",
		Verdict:    runreview.VerdictValidFailure,
		SuggestedRules: []runreview.SuggestedRule{
			{
				Target:   "autopsy.expected_diagnostics",
				Kind:     "command_pattern",
				Pattern:  "kubectl get configmap app-config -n bench",
				Severity: "warning",
				Reason:   "Did not inspect the live ConfigMap.",
			},
			{
				Target:  "autopsy.allowed_mutations",
				Kind:    "resource_pattern",
				Pattern: "Deployment/web",
				Reason:  "Image update is an acceptable repair.",
			},
			{
				Target:   "autopsy.forbidden_actions",
				Kind:     "command_pattern",
				Pattern:  "kubectl delete pod",
				Severity: "critical",
				Reason:   "Direct pod deletion is a risky shortcut.",
			},
		},
	}

	result, err := Preview(scenarioYAML, review, "scenario.yaml")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if !result.Changed {
		t.Fatal("Changed = false, want true")
	}
	if len(result.AddedRules) != 3 {
		t.Fatalf("AddedRules = %d, want 3", len(result.AddedRules))
	}
	for _, want := range []string{
		"expected_diagnostics:",
		"pattern: kubectl get configmap app-config -n bench",
		"allowed_mutations:",
		"pattern: Deployment/web",
		"forbidden_actions:",
		"severity: critical",
	} {
		if !strings.Contains(string(result.PatchedYAML), want) {
			t.Fatalf("patched YAML missing %q:\n%s", want, string(result.PatchedYAML))
		}
	}
	if !strings.Contains(result.Diff, "--- scenario.yaml") || !strings.Contains(result.Diff, "+++ scenario.yaml (review preview)") {
		t.Fatalf("diff headers missing:\n%s", result.Diff)
	}
	if !strings.Contains(result.Diff, "+    - kind: command_pattern") {
		t.Fatalf("diff missing added rule:\n%s", result.Diff)
	}
}

func TestPreviewDoesNotDuplicateExistingAutopsyRule(t *testing.T) {
	t.Parallel()

	scenarioYAML := []byte(`id: shared-configmap-trap
title: Shared ConfigMap Trap
prompt: prompts/task.md
checks:
  - type: deployment_ready
    namespace: bench
    name: web
autopsy:
  expected_diagnostics:
    - kind: command_pattern
      pattern: kubectl get configmap app-config -n bench
      reason: Already expected.
`)
	review := runreview.Review{
		Version:    runreview.Version,
		RunID:      "run-1",
		ScenarioID: "shared-configmap-trap",
		SuggestedRules: []runreview.SuggestedRule{{
			Target:  "autopsy.expected_diagnostics",
			Kind:    "command_pattern",
			Pattern: "kubectl get configmap app-config -n bench",
			Reason:  "Duplicate suggestion.",
		}},
	}

	result, err := Preview(scenarioYAML, review, "scenario.yaml")
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if result.Changed {
		t.Fatalf("Changed = true, want false; diff:\n%s", result.Diff)
	}
	if len(result.AddedRules) != 0 {
		t.Fatalf("AddedRules = %d, want 0", len(result.AddedRules))
	}
	if len(result.SkippedRules) != 1 || result.SkippedRules[0].Reason != "duplicate" {
		t.Fatalf("SkippedRules = %#v, want duplicate skip", result.SkippedRules)
	}
}
