package environment

import (
	"testing"
)

func TestAddonSteps_CNI(t *testing.T) {
	t.Parallel()
	steps, warnings := AddonSteps("cilium", nil)
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(steps) == 0 {
		t.Fatal("expected addon steps for cilium CNI")
	}
	if steps[0].Feature != "addon-cilium" {
		t.Fatalf("expected addon-cilium feature, got %q", steps[0].Feature)
	}
}

func TestAddonSteps_Addons(t *testing.T) {
	t.Parallel()
	steps, warnings := AddonSteps("", []string{"falco", "gatekeeper"})
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(steps) < 2 {
		t.Fatalf("expected at least 2 addon steps, got %d", len(steps))
	}
}

func TestAddonSteps_UnknownAddon(t *testing.T) {
	t.Parallel()
	steps, warnings := AddonSteps("", []string{"nonexistent"})
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if len(steps) != 0 {
		t.Fatalf("expected 0 steps for unknown addon, got %d", len(steps))
	}
}

func TestAddonSteps_CNIFirst(t *testing.T) {
	t.Parallel()
	steps, _ := AddonSteps("cilium", []string{"falco"})
	if len(steps) < 2 {
		t.Fatalf("expected at least 2 steps, got %d", len(steps))
	}
	// CNI must come before other addons.
	if steps[0].Feature != "addon-cilium" {
		t.Fatalf("expected CNI step first, got %q", steps[0].Feature)
	}
}

func TestAddonSteps_Argocd(t *testing.T) {
	t.Parallel()
	addon, ok := AddonRegistry["argocd"]
	if !ok {
		t.Fatal("argocd addon not found in registry")
	}
	if addon.Name != "argocd" {
		t.Fatalf("expected addon name argocd, got %q", addon.Name)
	}
	if len(addon.Steps) < 4 {
		t.Fatalf("expected at least 4 argocd steps (namespace, install, crd-wait, server-wait), got %d", len(addon.Steps))
	}

	// Verify all steps have the addon-argocd feature.
	for _, step := range addon.Steps {
		if step.Feature != "addon-argocd" {
			t.Fatalf("expected feature addon-argocd on step %q, got %q", step.Name, step.Feature)
		}
	}

	// Verify key steps are present.
	stepNames := make(map[string]bool)
	for _, step := range addon.Steps {
		stepNames[step.Name] = true
	}
	required := []string{
		"create-argocd-namespace",
		"install-argocd",
		"wait-argocd-crds",
		"wait-argocd-server",
		"wait-argocd-repo-server",
		"wait-argocd-application-controller",
	}
	for _, name := range required {
		if !stepNames[name] {
			t.Fatalf("missing required argocd step: %s", name)
		}
	}
}

func TestAddonSteps_ArgocdViaAddonSteps(t *testing.T) {
	t.Parallel()
	steps, warnings := AddonSteps("", []string{"argocd"})
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(steps) < 4 {
		t.Fatalf("expected at least 4 argocd steps, got %d", len(steps))
	}
}

func TestAddonSteps_Empty(t *testing.T) {
	t.Parallel()
	steps, warnings := AddonSteps("", nil)
	if len(steps) != 0 || len(warnings) != 0 {
		t.Fatalf("expected empty results, got %d steps, %d warnings", len(steps), len(warnings))
	}
}
