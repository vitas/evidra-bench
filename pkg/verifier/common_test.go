package verifier

import (
	"testing"
)

func TestDeploymentReadyCheck_Validate(t *testing.T) {
	t.Parallel()
	c := &DeploymentReadyCheck{Namespace: "default", Name: "web"}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestDeploymentReadyCheck_Validate_MissingNamespace(t *testing.T) {
	t.Parallel()
	c := &DeploymentReadyCheck{Name: "web"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing namespace")
	}
}

func TestDeploymentReadyCheck_Validate_MissingName(t *testing.T) {
	t.Parallel()
	c := &DeploymentReadyCheck{Namespace: "default"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestServiceEndpointsCheck_Validate(t *testing.T) {
	t.Parallel()
	c := &ServiceEndpointsCheck{Namespace: "default", Name: "web"}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestHelmReleaseCheck_Validate(t *testing.T) {
	t.Parallel()
	c := &HelmReleaseCheck{Name: "my-release"}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestHelmReleaseCheck_Validate_MissingName(t *testing.T) {
	t.Parallel()
	c := &HelmReleaseCheck{}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestArgoCDAppHealthyCheck_Validate(t *testing.T) {
	t.Parallel()
	c := &ArgoCDAppHealthyCheck{Name: "my-app"}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestArgoCDAppHealthyCheck_Validate_MissingName(t *testing.T) {
	t.Parallel()
	c := &ArgoCDAppHealthyCheck{}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestBuildCheckers_ValidDefs(t *testing.T) {
	t.Parallel()
	defs := []CheckDef{
		{Type: "deployment-ready", Namespace: "bench", Name: "web"},
		{Type: "service-endpoints", Namespace: "bench", Name: "web"},
		{Type: "helm-release", Name: "my-release"},
		{Type: "argocd-app-healthy", Name: "my-app"},
	}
	checkers, err := BuildCheckers(defs)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(checkers) != 4 {
		t.Fatalf("expected 4 checkers, got %d", len(checkers))
	}
}

func TestBuildCheckers_UnknownType(t *testing.T) {
	t.Parallel()
	defs := []CheckDef{
		{Type: "unknown-check", Name: "x"},
	}
	if _, err := BuildCheckers(defs); err == nil {
		t.Fatal("expected error for unknown check type")
	}
}

func TestDeploymentReadyCheck_ImplementsChecker(t *testing.T) {
	t.Parallel()
	var _ Checker = (*DeploymentReadyCheck)(nil)
}

func TestServiceEndpointsCheck_ImplementsChecker(t *testing.T) {
	t.Parallel()
	var _ Checker = (*ServiceEndpointsCheck)(nil)
}

func TestHelmReleaseCheck_ImplementsChecker(t *testing.T) {
	t.Parallel()
	var _ Checker = (*HelmReleaseCheck)(nil)
}

func TestArgoCDAppHealthyCheck_ImplementsChecker(t *testing.T) {
	t.Parallel()
	var _ Checker = (*ArgoCDAppHealthyCheck)(nil)
}
