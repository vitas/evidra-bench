package scenario

import (
	"fmt"
	"strings"
)

func validateRuntimeContract(s *Scenario) error {
	known := map[resourceRef]bool{
		{kind: "Namespace", name: "bench"}: true,
	}
	knownDeploymentContainers := map[resourceRef]map[string]bool{}

	// If any bootstrap step is a shell script, we can't statically analyze
	// what resources it creates. Pre-register resources from checks so that
	// later kubectl wait steps don't trigger false "unknown resource" errors.
	hasShellBootstrap := s.Break.Type == "shell"
	for _, step := range s.Bootstrap {
		if step.Type == "shell" {
			hasShellBootstrap = true
			break
		}
	}
	// Pre-register resources from environment addons (Falco, etc.)
	for _, addon := range s.Environment.Kubernetes.Addons {
		switch addon {
		case "falco":
			known[resourceRef{kind: "DaemonSet", namespace: "falco", name: "falco"}] = true
			known[resourceRef{kind: "Namespace", name: "falco"}] = true
		case "gatekeeper":
			known[resourceRef{kind: "Deployment", namespace: "gatekeeper-system", name: "gatekeeper-controller-manager"}] = true
		case "trivy-operator":
			known[resourceRef{kind: "Deployment", namespace: "trivy-system", name: "trivy-operator"}] = true
		}
	}

	if hasShellBootstrap {
		for _, c := range s.Checks {
			ns := c.Namespace
			if ns == "" {
				ns = "bench"
			}
			switch c.Type {
			case "deployment-ready":
				known[resourceRef{kind: "Deployment", namespace: ns, name: c.Name}] = true
			case "service-endpoints":
				known[resourceRef{kind: "Service", namespace: ns, name: c.Name}] = true
			}
		}
	}

	for _, step := range s.Bootstrap {
		if err := applyStepContract("bootstrap", step, known, knownDeploymentContainers); err != nil {
			return err
		}
	}

	// Multi-stage: validate each stage's break and checks independently.
	if len(s.Stages) > 0 {
		for i, stage := range s.Stages {
			stageSc := *s
			stageSc.Break = stage.Break
			stageSc.Checks = stage.Checks
			if stage.Break.Type != "" {
				if err := validateBreakContract(&stageSc, known, knownDeploymentContainers); err != nil {
					return fmt.Errorf("stage[%d] %q: %w", i, stage.Name, err)
				}
			}
			for _, step := range stage.AfterBreak {
				if err := applyStepContract(fmt.Sprintf("stage[%d].after_break", i), step, known, knownDeploymentContainers); err != nil {
					return err
				}
			}
			if err := validateChecks(&stageSc, known); err != nil {
				return fmt.Errorf("stage[%d] %q: %w", i, stage.Name, err)
			}
		}
	} else {
		if err := validateBreakContract(s, known, knownDeploymentContainers); err != nil {
			return err
		}
		for _, step := range s.AfterBreak {
			if err := applyStepContract("after_break", step, known, knownDeploymentContainers); err != nil {
				return err
			}
		}
		if err := validateChecks(s, known); err != nil {
			return err
		}
	}
	for _, step := range s.Chaos.Steps {
		if err := applyChaosStepContract(step, known, knownDeploymentContainers); err != nil {
			return err
		}
	}
	return nil
}

func applyChaosStepContract(step ChaosStep, known map[resourceRef]bool, knownDeploymentContainers map[resourceRef]map[string]bool) error {
	if !step.At.Set {
		return fmt.Errorf("chaos step %q: missing at", step.Name)
	}
	return applyStepContract("chaos", BootstrapStep{
		Name:      step.Name,
		Type:      step.Type,
		Path:      step.Path,
		Release:   step.Release,
		Namespace: step.Namespace,
		Duration:  step.Duration,
		Args:      step.Args,
	}, known, knownDeploymentContainers)
}

func applyStepContract(phase string, step BootstrapStep, known map[resourceRef]bool, knownDeploymentContainers map[resourceRef]map[string]bool) error {
	switch step.Type {
	case "kubectl-apply":
		resources, err := manifestResources(step.Path)
		if err != nil {
			return fmt.Errorf("%s step %q: %w", phase, step.Name, err)
		}
		for _, ref := range resources {
			if step.Namespace != "" && ref.namespace == "" && ref.kind != "CustomResourceDefinition" && ref.kind != "Namespace" {
				ref.namespace = step.Namespace
			}
			known[ref] = true
			if ref.kind == "Deployment" || ref.kind == "Service" || ref.kind == "Pod" {
				known[resourceRef{kind: ref.kind, namespace: defaultNamespace(ref.namespace), name: ref.name}] = true
			}
		}
		for ref, containers := range deploymentContainerSets(step.Path) {
			if step.Namespace != "" && ref.namespace == "" {
				ref.namespace = step.Namespace
			}
			knownDeploymentContainers[ref] = containers
		}
		if strings.Contains(step.Path, "argocd") && strings.Contains(step.Path, "install.yaml") {
			ns := defaultNamespace(step.Namespace)
			known[resourceRef{kind: "Deployment", namespace: ns, name: "argocd-server"}] = true
			known[resourceRef{kind: "Deployment", namespace: ns, name: "argocd-repo-server"}] = true
			known[resourceRef{kind: "StatefulSet", namespace: ns, name: "argocd-application-controller"}] = true
			known[resourceRef{kind: "CustomResourceDefinition", name: "applications.argoproj.io"}] = true
		}
	case "helm-install":
		if step.Release == "" {
			return fmt.Errorf("%s step %q: helm-install is missing release", phase, step.Name)
		}
		ns := defaultNamespace(step.Namespace)
		known[resourceRef{kind: "HelmRelease", namespace: ns, name: step.Release}] = true
		known[resourceRef{kind: "Deployment", namespace: ns, name: step.Release}] = true
		known[resourceRef{kind: "Service", namespace: ns, name: step.Release}] = true
		knownDeploymentContainers[resourceRef{kind: "Deployment", namespace: ns, name: step.Release}] = map[string]bool{"web": true}
	case "kubectl":
		if err := validateKubectlStep(step, known); err != nil {
			return fmt.Errorf("%s step %q: %w", phase, step.Name, err)
		}
	case "shell":
		// Shell script step — may create resources not statically analyzable.
		// Mark all check-referenced resources as known to avoid false negatives.
		return nil
	case "sleep":
		if step.Duration == "" {
			return fmt.Errorf("%s step %q: sleep is missing duration", phase, step.Name)
		}
	default:
		return fmt.Errorf("%s step %q: unsupported step type %q", phase, step.Name, step.Type)
	}
	return nil
}

func validateKubectlStep(step BootstrapStep, known map[resourceRef]bool) error {
	if len(step.Args) < 2 {
		return nil
	}
	switch {
	case step.Args[0] == "rollout" && step.Args[1] == "status" && len(step.Args) >= 3:
		kind, name := splitTypedName(step.Args[2])
		ns := valueAfterFlag(step.Args, "-n", "")
		ref := resourceRef{kind: kind, namespace: defaultNamespace(ns), name: name}
		if !known[ref] {
			return fmt.Errorf("bootstrap step %q waits for unknown %s/%s in namespace %q", step.Name, kind, name, defaultNamespace(ns))
		}
	case step.Args[0] == "wait" && len(step.Args) >= 2:
		target := kubectlWaitTarget(step.Args)
		if target == "" {
			return nil
		}
		kind, name := splitTypedName(target)
		ns := valueAfterFlag(step.Args, "-n", "")
		ref := resourceRef{kind: kind, namespace: defaultNamespace(ns), name: name}
		if kind == "CustomResourceDefinition" {
			ref = resourceRef{kind: kind, name: name}
		}
		if !known[ref] {
			return fmt.Errorf("bootstrap step %q waits for unknown %s/%s in namespace %q", step.Name, kind, name, defaultNamespace(ns))
		}
	}
	return nil
}

func validateBreakContract(s *Scenario, known map[resourceRef]bool, knownDeploymentContainers map[resourceRef]map[string]bool) error {
	switch s.Break.Type {
	case "", "apply", "kubectl-apply":
		if s.Break.Path == "" {
			return fmt.Errorf("scenario %s: break path is required", s.ID)
		}
		resources, err := manifestResources(s.Break.Path)
		if err != nil {
			return fmt.Errorf("scenario %s: parse break path: %w", s.ID, err)
		}
		for _, ref := range resources {
			known[ref] = true
		}
		for ref, containers := range deploymentContainerSets(s.Break.Path) {
			if baselineContainers, ok := knownDeploymentContainers[ref]; ok && len(containers) > 0 && !containerSetsOverlap(baselineContainers, containers) {
				return fmt.Errorf("scenario %s: break deployment %q patches container names that do not match the baseline deployment", s.ID, ref.name)
			}
			knownDeploymentContainers[ref] = containers
		}
	case "helm-upgrade":
		ns := defaultNamespace(s.Break.Namespace)
		if s.Break.Name == "" {
			return fmt.Errorf("scenario %s: helm-upgrade break is missing release name", s.ID)
		}
		if !known[resourceRef{kind: "HelmRelease", namespace: ns, name: s.Break.Name}] {
			return fmt.Errorf("scenario %s: helm-upgrade break targets unknown release %q", s.ID, s.Break.Name)
		}
		known[resourceRef{kind: "Deployment", namespace: ns, name: s.Break.Name}] = true
		known[resourceRef{kind: "Service", namespace: ns, name: s.Break.Name}] = true
	case "kubectl":
		// Raw kubectl break — uses args, no fixture validation needed
	case "shell":
		// Shell script break — uses path + args, no fixture validation needed
	default:
		return fmt.Errorf("scenario %s: unsupported break type %q", s.ID, s.Break.Type)
	}
	return nil
}

func validateChecks(s *Scenario, known map[resourceRef]bool) error {
	for _, check := range s.Checks {
		switch check.Type {
		case "deployment-ready":
			ref := resourceRef{kind: "Deployment", namespace: defaultNamespace(check.Namespace), name: check.Name}
			if !known[ref] {
				return fmt.Errorf("scenario %s: deployment-ready check references unknown deployment %q", s.ID, check.Name)
			}
		case "service-endpoints":
			ref := resourceRef{kind: "Service", namespace: defaultNamespace(check.Namespace), name: check.Name}
			if !known[ref] {
				return fmt.Errorf("scenario %s: service-endpoints check references unknown service %q", s.ID, check.Name)
			}
		case "service-reachable":
			ref := resourceRef{kind: "Service", namespace: defaultNamespace(check.Namespace), name: check.Name}
			if !known[ref] {
				return fmt.Errorf("scenario %s: service-reachable check references unknown service %q", s.ID, check.Name)
			}
			sourcePod := check.Condition
			if sourcePod == "" {
				sourcePod = "net-client"
			}
			podRef := resourceRef{kind: "Pod", namespace: defaultNamespace(check.Namespace), name: sourcePod}
			if !known[podRef] {
				return fmt.Errorf("scenario %s: service-reachable check references unknown probe pod %q", s.ID, sourcePod)
			}
		case "helm-release":
			ref := resourceRef{kind: "HelmRelease", namespace: defaultNamespace(check.Namespace), name: check.Name}
			if !known[ref] {
				return fmt.Errorf("scenario %s: helm-release check references unknown release %q", s.ID, check.Name)
			}
		case "argocd-app-healthy":
			ref := resourceRef{kind: "Application", namespace: "argocd", name: check.Name}
			if !known[ref] {
				return fmt.Errorf("scenario %s: argocd-app-healthy check references unknown application %q", s.ID, check.Name)
			}
		case "resource-exists":
			if check.Condition == "" {
				return fmt.Errorf("scenario %s: resource-exists check requires condition (kind) field", s.ID)
			}
			// resource-exists checks verify a resource still exists; we trust the kind/name/namespace are valid
		case "command-succeeds":
			if check.Condition == "" {
				return fmt.Errorf("scenario %s: command-succeeds check requires condition (command) field", s.ID)
			}
			// command-succeeds runs an arbitrary script — no static resource validation
		default:
			return fmt.Errorf("scenario %s: unsupported check type %q", s.ID, check.Type)
		}
	}
	return nil
}
