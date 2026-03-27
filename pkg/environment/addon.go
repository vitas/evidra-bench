package environment

import (
	"context"
	"fmt"
	"log"
	"os/exec"
)

// Addon describes a cluster addon that can be installed via bootstrap.
type Addon struct {
	Name  string
	Steps []BootstrapStep
}

// AddonRegistry maps addon names to their installation steps.
var AddonRegistry = map[string]Addon{
	"argocd": {
		Name: "argocd",
		Steps: []BootstrapStep{
			{
				Name:    "create-argocd-namespace",
				Type:    StepKubectl,
				Feature: "addon-argocd",
				Args:    []string{"create", "namespace", "argocd", "--dry-run=client", "-o", "yaml", "|", "kubectl", "apply", "-f", "-"},
			},
			{
				Name:      "install-argocd",
				Type:      StepKubectlApply,
				Path:      "https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml",
				Feature:   "addon-argocd",
				Namespace: "argocd",
			},
			{
				Name:    "wait-argocd-crds",
				Type:    StepKubectl,
				Feature: "addon-argocd",
				Args:    []string{"wait", "--for=condition=Established", "crd", "applications.argoproj.io", "--timeout=60s"},
			},
			{
				Name:    "wait-argocd-server",
				Type:    StepKubectl,
				Feature: "addon-argocd",
				Args:    []string{"-n", "argocd", "rollout", "status", "deployment/argocd-server", "--timeout=120s"},
			},
			{
				Name:    "wait-argocd-repo-server",
				Type:    StepKubectl,
				Feature: "addon-argocd",
				Args:    []string{"-n", "argocd", "rollout", "status", "deployment/argocd-repo-server", "--timeout=120s"},
			},
			{
				Name:    "wait-argocd-application-controller",
				Type:    StepKubectl,
				Feature: "addon-argocd",
				Args:    []string{"-n", "argocd", "rollout", "status", "statefulset/argocd-application-controller", "--timeout=120s"},
			},
		},
	},
	"falco": {
		Name: "falco",
		Steps: []BootstrapStep{
			{
				Name:      "add-falco-helm-repo",
				Type:      StepShell,
				Path:      "", // inline via Args
				Feature:   "addon-falco",
				Args:      []string{},
				Namespace: "",
			},
			{
				Name:      "install-falco",
				Type:      StepHelmInstall,
				Release:   "falco",
				Path:      "falcosecurity/falco",
				Namespace: "falco",
				Feature:   "addon-falco",
				Args:      []string{"--create-namespace", "--set", "driver.kind=ebpf", "--set", "falcosidekick.enabled=false"},
			},
		},
	},
	"gatekeeper": {
		Name: "gatekeeper",
		Steps: []BootstrapStep{
			{
				Name:      "install-gatekeeper",
				Type:      StepHelmInstall,
				Release:   "gatekeeper",
				Path:      "gatekeeper/gatekeeper",
				Namespace: "gatekeeper-system",
				Feature:   "addon-gatekeeper",
				Args:      []string{"--create-namespace"},
			},
		},
	},
	"trivy-operator": {
		Name: "trivy-operator",
		Steps: []BootstrapStep{
			{
				Name:      "install-trivy-operator",
				Type:      StepHelmInstall,
				Release:   "trivy-operator",
				Path:      "aqua/trivy-operator",
				Namespace: "trivy-system",
				Feature:   "addon-trivy",
				Args:      []string{"--create-namespace"},
			},
		},
	},
	"cilium": {
		Name: "cilium",
		Steps: []BootstrapStep{
			{
				Name:      "install-cilium",
				Type:      StepHelmInstall,
				Release:   "cilium",
				Path:      "cilium/cilium",
				Namespace: "kube-system",
				Feature:   "addon-cilium",
				Args:      []string{"--set", "hubble.enabled=false"},
			},
		},
	},
	"calico": {
		Name: "calico",
		Steps: []BootstrapStep{
			{
				Name:    "install-calico",
				Type:    StepKubectlApply,
				Path:    "https://raw.githubusercontent.com/projectcalico/calico/v3.28.0/manifests/calico.yaml",
				Feature: "addon-calico",
			},
		},
	},
}

// AddonSteps returns the bootstrap steps needed to install the requested addons
// and CNI. Unknown addon names are returned as warnings.
func AddonSteps(cni string, addons []string) ([]BootstrapStep, []string) {
	var steps []BootstrapStep
	var warnings []string

	// CNI addon (must come first — pods won't schedule without networking).
	if cni != "" {
		if addon, ok := AddonRegistry[cni]; ok {
			steps = append(steps, addon.Steps...)
		} else {
			warnings = append(warnings, fmt.Sprintf("unknown CNI addon: %q", cni))
		}
	}

	for _, name := range addons {
		if addon, ok := AddonRegistry[name]; ok {
			steps = append(steps, addon.Steps...)
		} else {
			warnings = append(warnings, fmt.Sprintf("unknown addon: %q", name))
		}
	}

	return steps, warnings
}

// AddHelmRepos ensures the Helm repositories needed by the registered addons
// are available. Call this once before running addon bootstrap steps.
func AddHelmRepos(ctx context.Context, runner CommandRunner) error {
	repos := map[string]string{
		"falcosecurity": "https://falcosecurity.github.io/charts",
		"gatekeeper":    "https://open-policy-agent.github.io/gatekeeper/charts",
		"aqua":          "https://aquasecurity.github.io/helm-charts",
		"cilium":        "https://helm.cilium.io",
	}

	for name, url := range repos {
		cmd := exec.CommandContext(ctx, "helm", "repo", "add", name, url, "--force-update")
		if out, err := runner.Run(ctx, cmd); err != nil {
			log.Printf("[addon] warning: helm repo add %s: %v: %s", name, err, string(out))
			// Non-fatal — repo may already exist.
		}
	}

	cmd := exec.CommandContext(ctx, "helm", "repo", "update")
	if _, err := runner.Run(ctx, cmd); err != nil {
		return fmt.Errorf("addon.AddHelmRepos: helm repo update: %w", err)
	}
	return nil
}
