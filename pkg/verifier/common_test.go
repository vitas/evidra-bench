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

func TestDeploymentReadyMessage_Pass(t *testing.T) {
	t.Parallel()
	deployment := deploymentStatus{}
	deployment.Metadata.Generation = 4
	deployment.Spec.Replicas = 2
	deployment.Status.ObservedGeneration = 4
	deployment.Status.UpdatedReplicas = 2
	deployment.Status.ReadyReplicas = 2
	deployment.Status.AvailableReplicas = 2
	if msg := deploymentReadyMessage(deployment); msg != "" {
		t.Fatalf("expected deployment to be ready, got %q", msg)
	}
}

func TestDeploymentReadyMessage_FailsWhenUpdateNotRolledOut(t *testing.T) {
	t.Parallel()
	deployment := deploymentStatus{}
	deployment.Metadata.Generation = 5
	deployment.Spec.Replicas = 2
	deployment.Status.ObservedGeneration = 5
	deployment.Status.UpdatedReplicas = 1
	deployment.Status.ReadyReplicas = 2
	deployment.Status.AvailableReplicas = 2
	if msg := deploymentReadyMessage(deployment); msg == "" || msg != "updated replicas: 1/2" {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestDeploymentReadyMessage_FailsWhenControllerHasNotObservedGeneration(t *testing.T) {
	t.Parallel()
	deployment := deploymentStatus{}
	deployment.Metadata.Generation = 3
	deployment.Spec.Replicas = 1
	deployment.Status.ObservedGeneration = 2
	deployment.Status.UpdatedReplicas = 1
	deployment.Status.ReadyReplicas = 1
	deployment.Status.AvailableReplicas = 1
	if msg := deploymentReadyMessage(deployment); msg == "" {
		t.Fatal("expected generation mismatch failure")
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

func TestServiceReachableCheck_Validate(t *testing.T) {
	t.Parallel()
	c := &ServiceReachableCheck{Namespace: "bench", Name: "web", SourcePod: "net-client"}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestServiceReachableCheck_Validate_DefaultProbePod(t *testing.T) {
	t.Parallel()
	c := &ServiceReachableCheck{Namespace: "bench", Name: "web"}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if c.SourcePod != "net-client" {
		t.Fatalf("unexpected default source pod: %s", c.SourcePod)
	}
}

func TestBuildCheckers_ValidDefs(t *testing.T) {
	t.Parallel()
	defs := []CheckDef{
		{Type: "deployment-ready", Namespace: "bench", Name: "web"},
		{Type: "service-endpoints", Namespace: "bench", Name: "web"},
		{Type: "service-reachable", Namespace: "bench", Name: "web", Condition: "net-client"},
		{Type: "helm-release", Name: "my-release"},
		{Type: "argocd-app-healthy", Name: "my-app"},
	}
	checkers, err := BuildCheckers(defs)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(checkers) != 5 {
		t.Fatalf("expected 5 checkers, got %d", len(checkers))
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

func TestServiceReachableCheck_ImplementsChecker(t *testing.T) {
	t.Parallel()
	var _ Checker = (*ServiceReachableCheck)(nil)
}

func TestHelmReleaseCheck_ImplementsChecker(t *testing.T) {
	t.Parallel()
	var _ Checker = (*HelmReleaseCheck)(nil)
}

func TestArgoCDAppHealthyCheck_ImplementsChecker(t *testing.T) {
	t.Parallel()
	var _ Checker = (*ArgoCDAppHealthyCheck)(nil)
}
