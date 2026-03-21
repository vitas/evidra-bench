package scenario

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type resourceRef struct {
	kind      string
	namespace string
	name      string
}

func runtimeProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func TestImplementedScenarios_RuntimeContracts(t *testing.T) {
	t.Parallel()

	root := runtimeProjectRoot()
	scenariosDir := filepath.Join(root, "scenarios")
	scenarios, err := LoadAll(scenariosDir)
	if err != nil {
		t.Fatalf("load scenarios: %v", err)
	}

	for _, s := range scenarios {
		s := s
		t.Run(s.Path, func(t *testing.T) {
			t.Parallel()
			if err := validateRuntimeContract(s); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateRuntimeContract_ChaosKubectlApply(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "fixtures"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "task.md"), []byte("Fix it."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixtures", "baseline.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: bench
spec:
  template:
    spec:
      containers:
        - name: web
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: bench
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixtures", "broken.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: bench
spec:
  template:
    spec:
      containers:
        - name: web
          image: broken:v1
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixtures", "chaos.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: bench
spec:
  template:
    spec:
      containers:
        - name: web
          image: noisy:v2
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte(`id: chaos-runtime
title: Chaos runtime contract
category: kubernetes
prompt: prompts/task.md
bootstrap:
  - type: kubectl-apply
    path: fixtures/baseline.yaml
break:
  type: apply
  path: fixtures/broken.yaml
chaos:
  steps:
    - at: 20s
      name: mutate-web
      type: kubectl-apply
      path: fixtures/chaos.yaml
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	if got := s.Chaos.Steps[0].Path; !filepath.IsAbs(got) {
		t.Fatalf("chaos path not resolved: %s", got)
	}
	if err := validateRuntimeContract(s); err != nil {
		t.Fatalf("validate runtime contract: %v", err)
	}
}

func TestValidateRuntimeContract_ChaosUnsupportedType(t *testing.T) {
	t.Parallel()

	s := &Scenario{
		ID:       "chaos-unsupported",
		Title:    "Chaos unsupported",
		Category: "kubernetes",
		Prompt:   "ignored",
		Bootstrap: []BootstrapStep{
			{Name: "baseline", Type: "kubectl-apply", Path: filepath.Join(runtimeProjectRoot(), "manifests", "baseline", "deployment.yaml")},
		},
		Break: Break{
			Type: "apply",
			Path: filepath.Join(runtimeProjectRoot(), "scenarios", "kubernetes", "broken-deployment", "fixtures", "broken.yaml"),
		},
		Chaos: ChaosConfig{
			Steps: []ChaosStep{
				{
					Name: "unsupported",
					Type: "explode",
					At:   Duration{Duration: time.Second, Set: true},
				},
			},
		},
		Checks: []Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
	}

	err := validateRuntimeContract(s)
	if err == nil {
		t.Fatal("expected unsupported chaos step type to fail")
	}
	if !strings.Contains(err.Error(), `unsupported step type "explode"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func validateRuntimeContract(s *Scenario) error {
	known := map[resourceRef]bool{
		{kind: "Namespace", name: "bench"}: true,
	}
	knownDeploymentContainers := map[resourceRef]map[string]bool{}

	// If any bootstrap step is a shell script, we can't statically analyze
	// what resources it creates. Pre-register resources from checks so that
	// later kubectl wait steps don't trigger false "unknown resource" errors.
	hasShellBootstrap := false
	for _, step := range s.Bootstrap {
		if step.Type == "shell" {
			hasShellBootstrap = true
			break
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
		default:
			return fmt.Errorf("scenario %s: unsupported check type %q", s.ID, check.Type)
		}
	}
	return nil
}

func manifestResources(path string) ([]resourceRef, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if strings.Contains(path, "argo-cd") && strings.Contains(path, "install.yaml") {
			return []resourceRef{
				{kind: "Namespace", name: "argocd"},
				{kind: "Deployment", namespace: "argocd", name: "argocd-server"},
				{kind: "Deployment", namespace: "argocd", name: "argocd-repo-server"},
				{kind: "StatefulSet", namespace: "argocd", name: "argocd-application-controller"},
				{kind: "CustomResourceDefinition", name: "applications.argoproj.io"},
			}, nil
		}
		return nil, fmt.Errorf("unsupported remote manifest path %q", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var refs []resourceRef
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			childRefs, err := manifestResources(filepath.Join(path, entry.Name()))
			if err != nil {
				return nil, err
			}
			refs = append(refs, childRefs...)
		}
		return refs, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(doc) == 0 {
			continue
		}
		kind, _ := doc["kind"].(string)
		metadata, _ := doc["metadata"].(map[string]any)
		name, _ := metadata["name"].(string)
		namespace, _ := metadata["namespace"].(string)
		if kind == "" || name == "" {
			continue
		}
		refs = append(refs, resourceRef{
			kind:      canonicalKind(kind),
			namespace: defaultNamespace(namespace),
			name:      name,
		})
	}
	return refs, nil
}

func deploymentContainerSets(path string) map[resourceRef]map[string]bool {
	result := map[resourceRef]map[string]bool{}
	objects, err := manifestObjects(path)
	if err != nil {
		return result
	}
	for _, obj := range objects {
		if obj.kind != "Deployment" {
			continue
		}
		if len(obj.containers) == 0 {
			continue
		}
		result[resourceRef{kind: obj.kind, namespace: defaultNamespace(obj.namespace), name: obj.name}] = obj.containers
	}
	return result
}

type manifestObject struct {
	kind       string
	namespace  string
	name       string
	containers map[string]bool
}

func manifestObjects(path string) ([]manifestObject, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return nil, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	var objects []manifestObject
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			child, err := manifestObjects(filepath.Join(path, entry.Name()))
			if err != nil {
				return nil, err
			}
			objects = append(objects, child...)
		}
		return objects, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(doc) == 0 {
			continue
		}
		kind, _ := doc["kind"].(string)
		metadata, _ := doc["metadata"].(map[string]any)
		name, _ := metadata["name"].(string)
		namespace, _ := metadata["namespace"].(string)
		if kind == "" || name == "" {
			continue
		}
		obj := manifestObject{
			kind:       canonicalKind(kind),
			namespace:  defaultNamespace(namespace),
			name:       name,
			containers: containerSetFromDoc(doc),
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

func containerSetFromDoc(doc map[string]any) map[string]bool {
	containers := map[string]bool{}
	spec, _ := doc["spec"].(map[string]any)
	template, _ := spec["template"].(map[string]any)
	templateSpec, _ := template["spec"].(map[string]any)
	rawContainers, _ := templateSpec["containers"].([]any)
	for _, raw := range rawContainers {
		container, _ := raw.(map[string]any)
		name, _ := container["name"].(string)
		if name != "" {
			containers[name] = true
		}
	}
	return containers
}

func containerSetsOverlap(a, b map[string]bool) bool {
	for name := range a {
		if b[name] {
			return true
		}
	}
	return false
}

func splitTypedName(value string) (string, string) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		return canonicalKind(value), ""
	}
	return canonicalKind(parts[0]), parts[1]
}

func valueAfterFlag(args []string, flag, fallback string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return fallback
}

func kubectlWaitTarget(args []string) string {
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if strings.Contains(arg, "/") {
			return arg
		}
	}
	return ""
}

func canonicalKind(kind string) string {
	switch strings.ToLower(kind) {
	case "deployment", "deployments":
		return "Deployment"
	case "service", "services":
		return "Service"
	case "application", "applications":
		return "Application"
	case "statefulset", "statefulsets":
		return "StatefulSet"
	case "crd", "customresourcedefinition", "customresourcedefinitions":
		return "CustomResourceDefinition"
	case "namespace", "namespaces":
		return "Namespace"
	case "pod", "pods":
		return "Pod"
	case "replicaset", "replicasets", "rs":
		return "ReplicaSet"
	default:
		return kind
	}
}

func defaultNamespace(namespace string) string {
	if namespace == "" {
		return "bench"
	}
	return namespace
}

func TestEvidraEnabledScenarios_RuntimeContracts(t *testing.T) {
	t.Parallel()

	root := runtimeProjectRoot()
	scenariosDir := filepath.Join(root, "scenarios")
	scenarios, err := LoadAll(scenariosDir)
	if err != nil {
		t.Fatalf("load scenarios: %v", err)
	}

	for _, s := range scenarios {
		if !s.Evidra.Enabled {
			continue
		}
		s := s
		t.Run(s.Path, func(t *testing.T) {
			t.Parallel()
			if s.Evidra.MinPrescriptions < 1 {
				t.Errorf("evidra.min_prescriptions must be >= 1, got %d", s.Evidra.MinPrescriptions)
			}
			if s.Evidra.MinReports < 1 {
				t.Errorf("evidra.min_reports must be >= 1, got %d", s.Evidra.MinReports)
			}

			// Verify orphaned_prescriptions and protocol_violations are explicitly set
			// by reading the raw YAML and checking the keys exist.
			rawPath := filepath.Join(s.Dir, "scenario.yaml")
			data, err := os.ReadFile(rawPath)
			if err != nil {
				t.Fatalf("read scenario.yaml: %v", err)
			}
			var raw map[string]any
			if err := yaml.Unmarshal(data, &raw); err != nil {
				t.Fatalf("parse scenario.yaml: %v", err)
			}
			evidraRaw, ok := raw["evidra"].(map[string]any)
			if !ok {
				t.Fatal("evidra key not found in scenario.yaml")
			}
			if _, ok := evidraRaw["orphaned_prescriptions"]; !ok {
				t.Error("evidra.orphaned_prescriptions must be explicitly set")
			}
			if _, ok := evidraRaw["protocol_violations"]; !ok {
				t.Error("evidra.protocol_violations must be explicitly set")
			}
		})
	}
}
