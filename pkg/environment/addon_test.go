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

func TestAddonSteps_ArgocdRemovedFromRegistry(t *testing.T) {
	t.Parallel()
	_, ok := AddonRegistry["argocd"]
	if ok {
		t.Fatal("argocd should not be in AddonRegistry — it is realized through profile hooks")
	}
}

func TestAddonSteps_ArgocdViaAddonSteps_ReturnsWarning(t *testing.T) {
	t.Parallel()
	_, warnings := AddonSteps("", []string{"argocd"})
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for removed argocd addon, got %d", len(warnings))
	}
}

func TestAddonSteps_Empty(t *testing.T) {
	t.Parallel()
	steps, warnings := AddonSteps("", nil)
	if len(steps) != 0 || len(warnings) != 0 {
		t.Fatalf("expected empty results, got %d steps, %d warnings", len(steps), len(warnings))
	}
}
