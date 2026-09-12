package scenario

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and validates a scenario from a directory containing scenario.yaml.
// authorityRuleShape mirrors every PolicyRule field name. It exists so a
// MIS-spelled key (e.g. camelCase resourceNames for resource_names) is a
// load error instead of a silently dropped field that widens the agent's
// grants to a wildcard. Keep in lockstep with PolicyRule; the unit test
// TestAuthorityRuleShadowMirrorsPolicyRule pins the correspondence.
type authorityRuleShape struct {
	APIGroups     []string `yaml:"apiGroups,omitempty"`
	Resources     []string `yaml:"resources,omitempty"`
	ResourceNames []string `yaml:"resource_names,omitempty"`
	Subresources  []string `yaml:"subresources,omitempty"`
	Namespaces    []string `yaml:"namespaces,omitempty"`
	Verbs         []string `yaml:"verbs"`
}

type authorityAgentShape struct {
	Namespaces         []string             `yaml:"namespaces,omitempty"`
	Rules              []authorityRuleShape `yaml:"rules,omitempty"`
	ClusterScopedRules []authorityRuleShape `yaml:"cluster_scoped_rules,omitempty"`
	OnDenied           string               `yaml:"on_denied,omitempty"`
}

type authorityProtectedShape struct {
	APIGroup  string `yaml:"apiGroup,omitempty"`
	Resource  string `yaml:"resource,omitempty"`
	Namespace string `yaml:"namespace,omitempty"`
	Name      string `yaml:"name,omitempty"`
}

type authorityReaderShape struct {
	Namespaces []string `yaml:"namespaces,omitempty"`
	Resources  []string `yaml:"resources,omitempty"`
	Extra      []string `yaml:"extra,omitempty"`
}

type authorityProfileShape struct {
	Agent            authorityAgentShape       `yaml:"agent"`
	Protected        []authorityProtectedShape `yaml:"protected,omitempty"`
	EvidenceReader   authorityReaderShape      `yaml:"evidence_reader,omitempty"`
	AllowImpersonate bool                      `yaml:"allow_impersonation,omitempty"`
}

func validateAuthorityRuleKeys(raw []byte) error {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil // the main unmarshal reports document errors
	}
	sub, ok := doc["authority_profile"]
	if !ok {
		return nil // no profile: nothing to pin
	}
	// Strict-decode ONLY the authority_profile subtree: every field there
	// must exist in the mirror; unknown keys would otherwise be dropped
	// silently and widen grants.
	var probe struct {
		Profile authorityProfileShape `yaml:"profile"`
	}
	wrapped, err := yaml.Marshal(map[string]any{"profile": sub})
	if err != nil {
		return nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(wrapped))
	dec.KnownFields(true)
	if err := dec.Decode(&probe); err != nil {
		return fmt.Errorf("authority_profile: %w (field names must match the schema exactly)", err)
	}
	return nil
}

func Load(dir string) (*Scenario, error) {
	path := filepath.Join(dir, "scenario.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenario.Load: %w", err)
	}

	var s Scenario
	if err := validateAuthorityRuleKeys(data); err != nil {
		return nil, fmt.Errorf("scenario %s: %w", filepath.Base(dir), err)
	}
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("scenario.Load: parse %s: %w", path, err)
	}
	s.Dir = dir

	if err := validate(&s); err != nil {
		return nil, fmt.Errorf("scenario.Load: %w", err)
	}

	// Resolve relative prompt path.
	if s.Prompt != "" && !filepath.IsAbs(s.Prompt) {
		s.Prompt = filepath.Join(dir, s.Prompt)
	}

	// Resolve relative break path.
	if s.Break.Path != "" && !filepath.IsAbs(s.Break.Path) &&
		!strings.HasPrefix(s.Break.Path, "http://") && !strings.HasPrefix(s.Break.Path, "https://") {
		s.Break.Path = filepath.Join(dir, s.Break.Path)
	}
	if s.Break.Chart != "" && !filepath.IsAbs(s.Break.Chart) {
		s.Break.Chart = filepath.Join(dir, s.Break.Chart)
	}
	resolveStepPaths(dir, s.Bootstrap)
	resolveStepPaths(dir, s.AfterBreak)
	resolveChaosStepPaths(dir, s.Chaos.Steps)

	// Resolve relative paths in stages.
	for i := range s.Stages {
		if s.Stages[i].Break.Path != "" && !filepath.IsAbs(s.Stages[i].Break.Path) &&
			!strings.HasPrefix(s.Stages[i].Break.Path, "http://") && !strings.HasPrefix(s.Stages[i].Break.Path, "https://") {
			s.Stages[i].Break.Path = filepath.Join(dir, s.Stages[i].Break.Path)
		}
		if s.Stages[i].Break.Chart != "" && !filepath.IsAbs(s.Stages[i].Break.Chart) {
			s.Stages[i].Break.Chart = filepath.Join(dir, s.Stages[i].Break.Chart)
		}
		resolveStepPaths(dir, s.Stages[i].AfterBreak)
	}

	// Resolve relative cloud setup/teardown paths.
	if s.Environment.Cloud.Setup != "" && !filepath.IsAbs(s.Environment.Cloud.Setup) {
		s.Environment.Cloud.Setup = filepath.Join(dir, s.Environment.Cloud.Setup)
	}
	if s.Environment.Cloud.Teardown != "" && !filepath.IsAbs(s.Environment.Cloud.Teardown) {
		s.Environment.Cloud.Teardown = filepath.Join(dir, s.Environment.Cloud.Teardown)
	}

	// Resolve relative check condition paths for command-succeeds checks.
	for i := range s.Checks {
		if (s.Checks[i].Type == "command-succeeds" || s.Checks[i].Type == CheckTypeAssertV2) && s.Checks[i].Condition != "" && !filepath.IsAbs(s.Checks[i].Condition) {
			s.Checks[i].Condition = filepath.Join(dir, s.Checks[i].Condition)
		}
	}

	return &s, nil
}

func resolveStepPaths(dir string, steps []BootstrapStep) {
	for i := range steps {
		if steps[i].Path == "" || filepath.IsAbs(steps[i].Path) {
			continue
		}
		if strings.HasPrefix(steps[i].Path, "http://") || strings.HasPrefix(steps[i].Path, "https://") {
			continue
		}
		steps[i].Path = filepath.Join(dir, steps[i].Path)
	}
}

func resolveChaosStepPaths(dir string, steps []ChaosStep) {
	for i := range steps {
		if steps[i].Path != "" && !filepath.IsAbs(steps[i].Path) &&
			!strings.HasPrefix(steps[i].Path, "http://") && !strings.HasPrefix(steps[i].Path, "https://") {
			steps[i].Path = filepath.Join(dir, steps[i].Path)
		}
	}
}

// LoadAll loads all scenarios under a base directory by walking subdirectories.
func LoadAll(baseDir string) ([]*Scenario, error) {
	var scenarios []*Scenario
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("scenario.LoadAll: %w", err)
	}

	for _, category := range entries {
		if !category.IsDir() {
			continue
		}
		categoryDir := filepath.Join(baseDir, category.Name())
		subEntries, err := os.ReadDir(categoryDir)
		if err != nil {
			continue
		}
		for _, entry := range subEntries {
			if !entry.IsDir() {
				continue
			}
			scenarioDir := filepath.Join(categoryDir, entry.Name())
			yamlPath := filepath.Join(scenarioDir, "scenario.yaml")
			if _, err := os.Stat(yamlPath); err != nil {
				continue
			}
			s, err := Load(scenarioDir)
			if err != nil {
				return nil, err
			}
			s.Path = filepath.ToSlash(filepath.Join(category.Name(), entry.Name()))
			scenarios = append(scenarios, s)
		}
	}
	return scenarios, nil
}

// Resolve loads a scenario by relative path or by scenario id.
func Resolve(baseDir, ref string) (*Scenario, error) {
	directDir, err := safeScenarioDir(baseDir, ref)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(directDir, "scenario.yaml")); err == nil {
		s, err := Load(directDir)
		if err != nil {
			return nil, err
		}
		s.Path = filepath.ToSlash(ref)
		return s, nil
	}

	scenarios, err := LoadAll(baseDir)
	if err != nil {
		return nil, err
	}
	for _, s := range scenarios {
		if s.ID == ref || strings.EqualFold(s.ID, ref) || s.Path == filepath.ToSlash(ref) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("scenario.Resolve: scenario %q not found", ref)
}

func safeScenarioDir(baseDir, ref string) (string, error) {
	if filepath.IsAbs(ref) {
		return "", fmt.Errorf("scenario.Resolve: reference %q is outside scenario root", ref)
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("scenario.Resolve: resolve scenario root: %w", err)
	}
	candidate := filepath.Join(baseAbs, filepath.FromSlash(ref))
	if !pathWithinRoot(baseAbs, candidate) {
		return "", fmt.Errorf("scenario.Resolve: reference %q is outside scenario root", ref)
	}

	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err == nil {
		realBase, baseErr := filepath.EvalSymlinks(baseAbs)
		if baseErr != nil {
			return "", fmt.Errorf("scenario.Resolve: resolve scenario root symlinks: %w", baseErr)
		}
		if !pathWithinRoot(realBase, realCandidate) {
			return "", fmt.Errorf("scenario.Resolve: reference %q resolves outside scenario root", ref)
		}
	}
	return candidate, nil
}

func pathWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func validate(s *Scenario) error {
	if s.ID == "" {
		return fmt.Errorf("scenario: missing id")
	}
	if s.Title == "" {
		return fmt.Errorf("scenario %s: missing title", s.ID)
	}
	if s.Category == "" && len(s.Categories) == 0 {
		return fmt.Errorf("scenario %s: missing category (set category or categories)", s.ID)
	}
	if s.Prompt == "" {
		return fmt.Errorf("scenario %s: missing prompt", s.ID)
	}

	// Stages vs single-stage mutual exclusion.
	hasStages := len(s.Stages) > 0
	hasBreak := s.Break.Type != ""
	hasChecks := len(s.Checks) > 0

	if hasStages && (hasBreak || hasChecks) {
		return fmt.Errorf("scenario %s: cannot have both 'stages' and top-level 'break'/'checks'", s.ID)
	}

	if hasStages {
		for i, st := range s.Stages {
			if st.Name == "" {
				return fmt.Errorf("scenario %s: stage[%d] missing name", s.ID, i)
			}
			if len(st.Checks) == 0 {
				return fmt.Errorf("scenario %s: stage %q has no verify checks", s.ID, st.Name)
			}
			if st.Break.Memory != "" && st.Break.Memory != "compact" && st.Break.Memory != "reset" {
				return fmt.Errorf("scenario %s: stage %q has invalid break.memory %q (must be compact or reset)", s.ID, st.Name, st.Break.Memory)
			}
			if st.OnFail != "" && st.OnFail != "stop" && st.OnFail != "continue" {
				return fmt.Errorf("scenario %s: stage %q has invalid on_fail %q", s.ID, st.Name, st.OnFail)
			}
		}
	} else if !hasBreak {
		return fmt.Errorf("scenario %s: must have either 'break' or 'stages'", s.ID)
	}

	if !hasStages && !hasChecks {
		return fmt.Errorf("scenario %s: at least one check is required", s.ID)
	}

	if err := validateExecutionProfile(s); err != nil {
		return err
	}
	if err := validateEnvironmentProviders(s); err != nil {
		return err
	}
	if err := validateChaos(s); err != nil {
		return err
	}
	if err := validateAutopsyHints(s); err != nil {
		return err
	}
	if err := validateAuthorityProfile(s); err != nil {
		return err
	}
	if err := ValidateAgentInputs(s); err != nil {
		return err
	}
	return nil
}

// ValidateAgentInputs rejects scenario agent_inputs entries that could
// escape the scenario directory (the sandbox whitelist is only meaningful
// if entries are plain relative paths).
func ValidateAgentInputs(s *Scenario) error {
	for _, in := range s.AgentInputs {
		clean := filepath.Clean(in)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("scenario %s: agent_inputs entry %q must be a relative path inside the scenario directory", s.ID, in)
		}
	}
	return nil
}

func validateExecutionProfile(s *Scenario) error {
	if s.Environment.Profile != "" && !IsSupportedExecutionProfile(s.Environment.Profile) {
		return fmt.Errorf("scenario %s: unsupported execution profile %q (valid: default, argocd, aws-localstack, multi-node)", s.ID, s.Environment.Profile)
	}
	return nil
}

func validateEnvironmentProviders(s *Scenario) error {
	for _, p := range s.Environment.Providers {
		if !supportedEnvironmentProviders[p] {
			return fmt.Errorf("scenario %s: unsupported environment provider %q (valid: kind, k3d)", s.ID, p)
		}
	}
	return nil
}

func validateChaos(s *Scenario) error {
	if s.Chaos.Mode != "" && s.Chaos.Mode != "once" && s.Chaos.Mode != "repeat" {
		return fmt.Errorf("scenario %s: unsupported chaos mode %q", s.ID, s.Chaos.Mode)
	}
	for i, step := range s.Chaos.Steps {
		if step.Type == "" {
			return fmt.Errorf("scenario %s: chaos step %d missing type", s.ID, i)
		}
		switch {
		case step.AfterChange != nil && step.At.Set:
			return fmt.Errorf("scenario %s: chaos step %d sets both at and after_change", s.ID, i)
		case step.AfterChange != nil:
			ac := step.AfterChange
			if ac.APIVersion == "" || ac.Resource == "" || ac.Namespace == "" || ac.Name == "" {
				return fmt.Errorf("scenario %s: chaos step %d after_change requires api_version, resource, namespace and name", s.ID, i)
			}
			if strings.ToLower(ac.Resource) == "secrets" {
				return fmt.Errorf("scenario %s: chaos step %d watches secrets: refused, trigger reads must never touch secret payloads", s.ID, i)
			}
			if clusterScopedResources[strings.ToLower(ac.Resource)] {
				return fmt.Errorf("scenario %s: chaos step %d watches cluster-scoped resource %q", s.ID, i, ac.Resource)
			}
			if !containsNamespace(s.Scope.Namespaces, ac.Namespace) {
				return fmt.Errorf("scenario %s: chaos step %d watches namespace %q outside the scenario scope", s.ID, i, ac.Namespace)
			}
		case !step.At.Set:
			return fmt.Errorf("scenario %s: chaos step %d needs at or after_change", s.ID, i)
		}
	}
	return nil
}

// clusterScopedResources are refused as change triggers: watching them
// (or firing writes because of them) escapes the scenario's blast radius.
var clusterScopedResources = map[string]bool{
	"nodes": true, "namespaces": true, "persistentvolumes": true,
	"clusterroles": true, "clusterrolebindings": true, "storageclasses": true,
	"priorityclasses": true, "customresourcedefinitions": true,
	"mutatingwebhookconfigurations": true, "validatingwebhookconfigurations": true,
	"apiservices": true, "csidrivers": true,
}

// IsClusterScopedResource reports whether an RBAC resource name addresses a
// cluster-scoped API (nodes, namespaces, clusterrolebindings...). Used by
// identity materialization to route evidence reads to a ClusterRole.
func IsClusterScopedResource(name string) bool {
	return clusterScopedResource(name)
}

func clusterScopedResource(name string) bool {
	return clusterScopedResources[name]
}

func containsNamespace(list []string, want string) bool {
	for _, n := range list {
		if n == want {
			return true
		}
	}
	return false
}

func validateAutopsyHints(s *Scenario) error {
	groups := []struct {
		name     string
		patterns []AutopsyPattern
	}{
		{name: "expected_diagnostics", patterns: s.Autopsy.ExpectedDiagnostics},
		{name: "allowed_mutations", patterns: s.Autopsy.AllowedMutations},
		{name: "forbidden_actions", patterns: s.Autopsy.ForbiddenActions},
	}

	for _, group := range groups {
		for i, pattern := range group.patterns {
			if pattern.Kind == "" {
				return fmt.Errorf("scenario %s: autopsy.%s[%d] missing kind", s.ID, group.name, i)
			}
			if pattern.Pattern == "" {
				return fmt.Errorf("scenario %s: autopsy.%s[%d] missing pattern", s.ID, group.name, i)
			}
		}
	}
	return nil
}
