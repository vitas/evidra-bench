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
	if s.Break.Path != "" && !filepath.IsAbs(s.Break.Path) {
		s.Break.Path = filepath.Join(dir, s.Break.Path)
	}
	if s.Break.Chart != "" && !filepath.IsAbs(s.Break.Chart) {
		s.Break.Chart = filepath.Join(dir, s.Break.Chart)
	}
	for i := range s.Bootstrap {
		if s.Bootstrap[i].Path != "" && !filepath.IsAbs(s.Bootstrap[i].Path) {
			s.Bootstrap[i].Path = filepath.Join(dir, s.Bootstrap[i].Path)
		}
	}

	return &s, nil
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
	if s.Category == "" {
		return fmt.Errorf("scenario %s: missing category", s.ID)
	}
	if s.Prompt == "" {
		return fmt.Errorf("scenario %s: missing prompt", s.ID)
	}
	if len(s.Checks) == 0 {
		return fmt.Errorf("scenario %s: at least one check is required", s.ID)
	}
	return nil
}
