package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and validates a scenario from a directory containing scenario.yaml.
func Load(dir string) (*Scenario, error) {
	path := filepath.Join(dir, "scenario.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenario.Load: %w", err)
	}

	var s Scenario
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
		if s.Checks[i].Type == "command-succeeds" && s.Checks[i].Condition != "" && !filepath.IsAbs(s.Checks[i].Condition) {
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
	directDir := filepath.Join(baseDir, ref)
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
	return nil
}

func validateExecutionProfile(s *Scenario) error {
	if s.Environment.Profile != "" && !IsSupportedExecutionProfile(s.Environment.Profile) {
		return fmt.Errorf("scenario %s: unsupported execution profile %q (valid: default, argocd, aws-localstack)", s.ID, s.Environment.Profile)
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
		if !step.At.Set {
			return fmt.Errorf("scenario %s: chaos step %d missing at", s.ID, i)
		}
	}
	return nil
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
