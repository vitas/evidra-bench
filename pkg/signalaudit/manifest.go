package signalaudit

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadManifest reads a signal-audit manifest from disk and validates its entries.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("decode manifest: empty document")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("decode manifest: root must be a mapping")
	}

	manifest := make(Manifest, len(root.Content)/2)
	for i := 0; i < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valueNode := root.Content[i+1]

		scenarioID := strings.TrimSpace(keyNode.Value)
		if scenarioID == "" {
			return nil, fmt.Errorf("manifest line %d: scenario id is required", keyNode.Line)
		}
		if _, exists := manifest[scenarioID]; exists {
			return nil, fmt.Errorf("manifest line %d: duplicate scenario %q", keyNode.Line, scenarioID)
		}

		var entry Expectation
		if err := valueNode.Decode(&entry); err != nil {
			return nil, fmt.Errorf("manifest %q: decode entry: %w", scenarioID, err)
		}
		entry.normalize()
		if err := entry.validate(scenarioID); err != nil {
			return nil, err
		}

		manifest[scenarioID] = entry
	}

	return manifest, nil
}

func (e *Expectation) normalize() {
	e.PrimarySignal = strings.TrimSpace(e.PrimarySignal)
	e.ExpectedSignals = normalizeSignalList(e.ExpectedSignals)
	e.AllowedSecondarySignals = normalizeSignalList(e.AllowedSecondarySignals)
	e.ForbiddenSignals = normalizeSignalList(e.ForbiddenSignals)
}

func (e Expectation) validate(scenarioID string) error {
	if e.PrimarySignal == "" {
		return fmt.Errorf("manifest %q: primary_signal is required", scenarioID)
	}
	for _, signal := range e.ForbiddenSignals {
		if signal == e.PrimarySignal {
			return fmt.Errorf("manifest %q: forbidden_signals cannot contain primary_signal %q", scenarioID, signal)
		}
	}
	return nil
}

func normalizeSignalList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
