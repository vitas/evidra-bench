package verifier

import (
	"context"
	"encoding/json"
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
		"-o", "json",
	).CombinedOutput()
	if err != nil {
		return CheckResult{Name: name, Type: "deployment-ready", Verdict: VerdictFail, Message: string(out)}
	}
	var deployment deploymentStatus
	if err := json.Unmarshal(out, &deployment); err != nil {
		return CheckResult{Name: name, Type: "deployment-ready", Verdict: VerdictFail, Message: fmt.Sprintf("parse deployment status: %v", err)}
	}
	if message := deploymentReadyMessage(deployment); message != "" {
		return CheckResult{Name: name, Type: "deployment-ready", Verdict: VerdictFail, Message: message}
	}
	return CheckResult{Name: name, Type: "deployment-ready", Verdict: VerdictPass}
}

type deploymentStatus struct {
	Metadata struct {
		Generation int64 `json:"generation"`
	} `json:"metadata"`
	Spec struct {
		Replicas int32 `json:"replicas"`
	} `json:"spec"`
	Status struct {
		ObservedGeneration  int64 `json:"observedGeneration"`
		UpdatedReplicas     int32 `json:"updatedReplicas"`
		ReadyReplicas       int32 `json:"readyReplicas"`
		AvailableReplicas   int32 `json:"availableReplicas"`
		UnavailableReplicas int32 `json:"unavailableReplicas"`
	} `json:"status"`
}

func deploymentReadyMessage(d deploymentStatus) string {
	if d.Spec.Replicas <= 0 {
		return fmt.Sprintf("replicas: spec=%d", d.Spec.Replicas)
	}
	if d.Status.ObservedGeneration < d.Metadata.Generation {
		return fmt.Sprintf("deployment generation not observed yet: observed=%d generation=%d", d.Status.ObservedGeneration, d.Metadata.Generation)
	}
	if d.Status.UpdatedReplicas != d.Spec.Replicas {
		return fmt.Sprintf("updated replicas: %d/%d", d.Status.UpdatedReplicas, d.Spec.Replicas)
	}
	if d.Status.ReadyReplicas != d.Spec.Replicas {
		return fmt.Sprintf("ready replicas: %d/%d", d.Status.ReadyReplicas, d.Spec.Replicas)
	}
	if d.Status.AvailableReplicas != d.Spec.Replicas {
		return fmt.Sprintf("available replicas: %d/%d", d.Status.AvailableReplicas, d.Spec.Replicas)
	}
	if d.Status.UnavailableReplicas != 0 {
		return fmt.Sprintf("unavailable replicas: %d", d.Status.UnavailableReplicas)
	}
	return ""
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

// ServiceReachableCheck verifies a service is reachable from a probe pod inside the cluster.
type ServiceReachableCheck struct {
	Namespace string
	Name      string
	SourcePod string
}

// Validate checks that required fields are set.
func (c *ServiceReachableCheck) Validate() error {
	if c.Namespace == "" {
		return fmt.Errorf("verifier.ServiceReachableCheck: namespace is required")
	}
	if c.Name == "" {
		return fmt.Errorf("verifier.ServiceReachableCheck: name is required")
	}
	if c.SourcePod == "" {
		c.SourcePod = "net-client"
	}
	return nil
}

// Check runs kubectl exec from the probe pod and verifies an HTTP request succeeds.
func (c *ServiceReachableCheck) Check(ctx context.Context, kubeconfigPath string) CheckResult {
	name := fmt.Sprintf("service-reachable/%s/%s", c.Namespace, c.Name)
	target := fmt.Sprintf("http://%s.%s.svc.cluster.local", c.Name, c.Namespace)
	out, err := exec.CommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"exec",
		"-n", c.Namespace,
		c.SourcePod,
		"--",
		"wget", "-q", "-O", "-", "-T", "5", target,
	).CombinedOutput()
	if err != nil {
		return CheckResult{Name: name, Type: "service-reachable", Verdict: VerdictFail, Message: string(out)}
	}
	if strings.TrimSpace(string(out)) == "" {
		return CheckResult{Name: name, Type: "service-reachable", Verdict: VerdictFail, Message: "empty response"}
	}
	return CheckResult{Name: name, Type: "service-reachable", Verdict: VerdictPass}
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

// ResourceExistsCheck verifies a specific Kubernetes resource still exists.
type ResourceExistsCheck struct {
	Namespace string
	Name      string
	Kind      string // e.g. "NetworkPolicy", "PodDisruptionBudget"
}

// Validate checks that required fields are set.
func (c *ResourceExistsCheck) Validate() error {
	if c.Namespace == "" {
		return fmt.Errorf("verifier.ResourceExistsCheck: namespace is required")
	}
	if c.Name == "" {
		return fmt.Errorf("verifier.ResourceExistsCheck: name is required")
	}
	if c.Kind == "" {
		return fmt.Errorf("verifier.ResourceExistsCheck: kind is required (use condition field)")
	}
	return nil
}

// Check runs kubectl to verify the resource exists.
func (c *ResourceExistsCheck) Check(ctx context.Context, kubeconfigPath string) CheckResult {
	name := fmt.Sprintf("resource-exists/%s/%s/%s", c.Kind, c.Namespace, c.Name)
	args := []string{"--kubeconfig", kubeconfigPath, "get", c.Kind, c.Name}
	// Cluster-scoped resources (Namespace, Node, etc.) don't use -n.
	if !isClusterScoped(c.Kind) {
		args = append(args, "-n", c.Namespace)
	}
	args = append(args, "-o", "name")
	out, err := exec.CommandContext(ctx, "kubectl", args...).CombinedOutput()
	if err != nil {
		return CheckResult{Name: name, Type: "resource-exists", Verdict: VerdictFail, Message: strings.TrimSpace(string(out))}
	}
	if strings.TrimSpace(string(out)) == "" {
		return CheckResult{Name: name, Type: "resource-exists", Verdict: VerdictFail, Message: "resource not found"}
	}
	return CheckResult{Name: name, Type: "resource-exists", Verdict: VerdictPass}
}

func isClusterScoped(kind string) bool {
	switch strings.ToLower(kind) {
	case "namespace", "node", "persistentvolume", "clusterrole", "clusterrolebinding":
		return true
	}
	return false
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
		case "service-reachable":
			c := &ServiceReachableCheck{Namespace: cd.Namespace, Name: cd.Name, SourcePod: cd.Condition}
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
		case "resource-exists":
			c := &ResourceExistsCheck{Namespace: cd.Namespace, Name: cd.Name, Kind: cd.Condition}
			if err := c.Validate(); err != nil {
				return nil, err
			}
			checkers = append(checkers, c)
		case "command-succeeds":
			c := &CommandSucceedsCheck{Name: cd.Name, Command: cd.Condition}
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
