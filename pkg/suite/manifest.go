// Package suite loads versioned, ordered collections of existing Evidra
// scenarios without introducing a second scenario representation.
package suite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"gopkg.in/yaml.v3"
)

const SchemaV1 = "evidra-suite/v1"

type Manifest struct {
	Schema      string              `yaml:"schema"`
	ID          string              `yaml:"id"`
	Revision    int                 `yaml:"revision"`
	Environment EnvironmentContract `yaml:"environment"`
	Cases       []string            `yaml:"cases"`
	Includes    []string            `yaml:"includes"`
	Limitations []string            `yaml:"limitations"`
}

type EnvironmentContract struct {
	Profile   scenario.ExecutionProfile `yaml:"profile"`
	Providers []string                  `yaml:"providers"`
}

type Loaded struct {
	Manifest  Manifest
	Identity  string
	Digest    string
	Scenarios []*scenario.Scenario
}

// EvaluationSuite returns the canonical, credential-free suite portion of an
// evaluation plan while keeping scenario execution on the shared loader path.
func (l *Loaded) EvaluationSuite() evaluation.SuitePlan {
	cases := make([]evaluation.CasePlan, 0, len(l.Scenarios))
	for _, s := range l.Scenarios {
		cases = append(cases, evaluation.CasePlan{ID: s.ID})
	}
	return evaluation.SuitePlan{ID: l.Identity, Digest: l.Digest, Cases: cases}
}

// Load validates a suite manifest, resolves its cases through the shared
// scenario loader, and fingerprints every declared executable input.
func Load(manifestPath, projectRoot string) (*Loaded, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("suite.Load: resolve project root: %w", err)
	}
	manifestPath, err = secureProjectPath(root, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("suite.Load: manifest: %w", err)
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("suite.Load: read manifest: %w", err)
	}
	var manifest Manifest
	decoder := yaml.NewDecoder(bytes.NewReader(manifestData))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("suite.Load: parse manifest: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return nil, fmt.Errorf("suite.Load: %w", err)
	}

	loaded := &Loaded{
		Manifest: manifest,
		Identity: fmt.Sprintf("%s@%d", manifest.ID, manifest.Revision),
	}
	seenCases := make(map[string]struct{}, len(manifest.Cases))
	seenScenarioIDs := make(map[string]string, len(manifest.Cases))
	scenariosRoot := filepath.Join(root, "scenarios")
	for _, ref := range manifest.Cases {
		if _, ok := seenCases[ref]; ok {
			return nil, fmt.Errorf("suite.Load: duplicate case %q", ref)
		}
		seenCases[ref] = struct{}{}
		s, err := scenario.Resolve(scenariosRoot, ref)
		if err != nil {
			return nil, fmt.Errorf("suite.Load: resolve case %q: %w", ref, err)
		}
		if previousRef, ok := seenScenarioIDs[s.ID]; ok {
			return nil, fmt.Errorf("suite.Load: duplicate scenario id %q from cases %q and %q", s.ID, previousRef, ref)
		}
		seenScenarioIDs[s.ID] = ref
		if s.ResolvedProfile() != manifest.Environment.Profile {
			return nil, fmt.Errorf("suite.Load: case %q uses profile %q, suite requires %q", ref, s.ResolvedProfile(), manifest.Environment.Profile)
		}
		for _, provider := range manifest.Environment.Providers {
			if !s.IsProviderCompatible(provider) {
				return nil, fmt.Errorf("suite.Load: case %q is not compatible with provider %q", ref, provider)
			}
		}
		loaded.Scenarios = append(loaded.Scenarios, s)
	}

	loaded.Digest, err = digestInputs(root, manifestPath, loaded)
	if err != nil {
		return nil, fmt.Errorf("suite.Load: digest: %w", err)
	}
	return loaded, nil
}

func validateManifest(manifest Manifest) error {
	if manifest.Schema != SchemaV1 {
		return fmt.Errorf("unsupported schema %q", manifest.Schema)
	}
	if strings.TrimSpace(manifest.ID) == "" {
		return fmt.Errorf("suite id is required")
	}
	if manifest.Revision <= 0 {
		return fmt.Errorf("positive suite revision is required")
	}
	if !scenario.IsSupportedExecutionProfile(manifest.Environment.Profile) {
		return fmt.Errorf("unsupported environment profile %q", manifest.Environment.Profile)
	}
	if len(manifest.Environment.Providers) == 0 {
		return fmt.Errorf("at least one environment provider is required")
	}
	providers := make(map[string]struct{}, len(manifest.Environment.Providers))
	for _, provider := range manifest.Environment.Providers {
		if provider != "kind" && provider != "k3d" {
			return fmt.Errorf("unsupported environment provider %q", provider)
		}
		if _, ok := providers[provider]; ok {
			return fmt.Errorf("duplicate environment provider %q", provider)
		}
		providers[provider] = struct{}{}
	}
	if len(manifest.Cases) == 0 {
		return fmt.Errorf("at least one case is required")
	}
	if len(manifest.Includes) == 0 {
		return fmt.Errorf("at least one shared asset include is required")
	}
	return nil
}

func secureProjectPath(root, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !pathWithinRoot(root, abs) {
		return "", fmt.Errorf("path %q is outside project root", path)
	}
	return abs, nil
}

func pathWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
