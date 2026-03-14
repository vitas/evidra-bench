package verifier

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DeploymentReadyCheck verifies a deployment has all replicas ready.
type DeploymentReadyCheck struct {
	Namespace string
	Name      string
}

// Validate checks that required fields are set.
func (c *DeploymentReadyCheck) Validate() error {
	if c.Namespace == "" {
		return fmt.Errorf("verifier.DeploymentReadyCheck: namespace is required")
	}
	if c.Name == "" {
		return fmt.Errorf("verifier.DeploymentReadyCheck: name is required")
	}
	return nil
}

// Check runs kubectl to verify the deployment is ready.
func (c *DeploymentReadyCheck) Check(ctx context.Context, kubeconfigPath string) CheckResult {
	name := fmt.Sprintf("deployment-ready/%s/%s", c.Namespace, c.Name)
	out, err := exec.CommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"get", "deployment", c.Name,
		"-n", c.Namespace,
		"-o", "jsonpath={.status.readyReplicas}/{.spec.replicas}",
	).CombinedOutput()
	if err != nil {
		return CheckResult{Name: name, Type: "deployment-ready", Verdict: VerdictFail, Message: string(out)}
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "/")
	if len(parts) != 2 || parts[0] != parts[1] || parts[0] == "" || parts[0] == "0" {
		return CheckResult{Name: name, Type: "deployment-ready", Verdict: VerdictFail, Message: fmt.Sprintf("replicas: %s", out)}
	}
	return CheckResult{Name: name, Type: "deployment-ready", Verdict: VerdictPass}
}

// ServiceEndpointsCheck verifies a service has at least one endpoint.
type ServiceEndpointsCheck struct {
	Namespace string
	Name      string
}

// Validate checks that required fields are set.
func (c *ServiceEndpointsCheck) Validate() error {
	if c.Namespace == "" {
		return fmt.Errorf("verifier.ServiceEndpointsCheck: namespace is required")
	}
	if c.Name == "" {
		return fmt.Errorf("verifier.ServiceEndpointsCheck: name is required")
	}
	return nil
}

// Check runs kubectl to verify the service has endpoints.
func (c *ServiceEndpointsCheck) Check(ctx context.Context, kubeconfigPath string) CheckResult {
	name := fmt.Sprintf("service-endpoints/%s/%s", c.Namespace, c.Name)
	out, err := exec.CommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"get", "endpoints", c.Name,
		"-n", c.Namespace,
		"-o", "jsonpath={.subsets[*].addresses[*].ip}",
	).CombinedOutput()
	if err != nil {
		return CheckResult{Name: name, Type: "service-endpoints", Verdict: VerdictFail, Message: string(out)}
	}
	if strings.TrimSpace(string(out)) == "" {
		return CheckResult{Name: name, Type: "service-endpoints", Verdict: VerdictFail, Message: "no endpoints"}
	}
	return CheckResult{Name: name, Type: "service-endpoints", Verdict: VerdictPass}
}

// HelmReleaseCheck verifies a Helm release is deployed.
type HelmReleaseCheck struct {
	Namespace string
	Name      string
}

// Validate checks that required fields are set.
func (c *HelmReleaseCheck) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("verifier.HelmReleaseCheck: name is required")
	}
	return nil
}

// Check runs helm to verify the release exists and is deployed.
func (c *HelmReleaseCheck) Check(ctx context.Context, kubeconfigPath string) CheckResult {
	name := fmt.Sprintf("helm-release/%s", c.Name)
	args := []string{"--kubeconfig", kubeconfigPath, "status", c.Name, "-o", "json"}
	if c.Namespace != "" {
		args = append(args, "-n", c.Namespace)
	}
	out, err := exec.CommandContext(ctx, "helm", args...).CombinedOutput()
	if err != nil {
		return CheckResult{Name: name, Type: "helm-release", Verdict: VerdictFail, Message: string(out)}
	}
	if strings.Contains(string(out), `"status":"deployed"`) {
		return CheckResult{Name: name, Type: "helm-release", Verdict: VerdictPass}
	}
	return CheckResult{Name: name, Type: "helm-release", Verdict: VerdictFail, Message: "release not in deployed state"}
}

// ArgoCDAppHealthyCheck verifies an Argo CD application is healthy and synced.
type ArgoCDAppHealthyCheck struct {
	Name string
}

// Validate checks that required fields are set.
func (c *ArgoCDAppHealthyCheck) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("verifier.ArgoCDAppHealthyCheck: name is required")
	}
	return nil
}

// Check runs argocd or kubectl to verify app health.
func (c *ArgoCDAppHealthyCheck) Check(ctx context.Context, kubeconfigPath string) CheckResult {
	name := fmt.Sprintf("argocd-app-healthy/%s", c.Name)
	out, err := exec.CommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"get", "application", c.Name,
		"-n", "argocd",
		"-o", "jsonpath={.status.health.status}/{.status.sync.status}",
	).CombinedOutput()
	if err != nil {
		return CheckResult{Name: name, Type: "argocd-app-healthy", Verdict: VerdictFail, Message: string(out)}
	}
	result := strings.TrimSpace(string(out))
	if result == "Healthy/Synced" {
		return CheckResult{Name: name, Type: "argocd-app-healthy", Verdict: VerdictPass}
	}
	return CheckResult{Name: name, Type: "argocd-app-healthy", Verdict: VerdictFail, Message: fmt.Sprintf("status: %s", result)}
}

// RunChecks executes all checkers and returns the aggregate result.
func RunChecks(ctx context.Context, kubeconfigPath string, checkers []Checker) *VerifyResult {
	result := &VerifyResult{Passed: true}
	for _, c := range checkers {
		cr := c.Check(ctx, kubeconfigPath)
		result.Checks = append(result.Checks, cr)
		if cr.Verdict == VerdictFail {
			result.Passed = false
		}
	}
	return result
}

// CheckDef is the check definition accepted by BuildCheckers.
// It mirrors scenario.Check to avoid a package dependency cycle.
type CheckDef struct {
	Type      string
	Namespace string
	Name      string
	Condition string
}

// BuildCheckers creates Checker instances from check definitions.
func BuildCheckers(checks []CheckDef) ([]Checker, error) {
	var checkers []Checker
	for _, cd := range checks {
		switch cd.Type {
		case "deployment-ready":
			c := &DeploymentReadyCheck{Namespace: cd.Namespace, Name: cd.Name}
			if err := c.Validate(); err != nil {
				return nil, err
			}
			checkers = append(checkers, c)
		case "service-endpoints":
			c := &ServiceEndpointsCheck{Namespace: cd.Namespace, Name: cd.Name}
			if err := c.Validate(); err != nil {
				return nil, err
			}
			checkers = append(checkers, c)
		case "helm-release":
			c := &HelmReleaseCheck{Namespace: cd.Namespace, Name: cd.Name}
			if err := c.Validate(); err != nil {
				return nil, err
			}
			checkers = append(checkers, c)
		case "argocd-app-healthy":
			c := &ArgoCDAppHealthyCheck{Name: cd.Name}
			if err := c.Validate(); err != nil {
				return nil, err
			}
			checkers = append(checkers, c)
		default:
			return nil, fmt.Errorf("verifier.BuildCheckers: unknown check type: %s", cd.Type)
		}
	}
	return checkers, nil
}
