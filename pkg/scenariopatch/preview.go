// Package scenariopatch turns saved review suggestions into scenario YAML diffs.
package scenariopatch

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"

	"github.com/vitas/evidra-bench/pkg/runreview"
)

type Result struct {
	Changed      bool
	PatchedYAML  []byte
	Diff         string
	AddedRules   []RuleChange
	SkippedRules []RuleSkip
}

type RuleChange struct {
	Target  string `json:"target"`
	Section string `json:"section"`
	Kind    string `json:"kind"`
	Pattern string `json:"pattern"`
}

type RuleSkip struct {
	Target  string `json:"target"`
	Kind    string `json:"kind"`
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

func Preview(scenarioYAML []byte, review runreview.Review, scenarioPath string) (Result, error) {
	if strings.TrimSpace(scenarioPath) == "" {
		scenarioPath = "scenario.yaml"
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(scenarioYAML, &doc); err != nil {
		return Result{}, fmt.Errorf("parse scenario YAML: %w", err)
	}
	root, err := rootMapping(&doc)
	if err != nil {
		return Result{}, err
	}

	result := Result{PatchedYAML: append([]byte(nil), scenarioYAML...)}
	for _, rule := range review.SuggestedRules {
		section, ok := autopsySection(rule.Target)
		if !ok {
			result.SkippedRules = append(result.SkippedRules, skip(rule, "unsupported_target"))
			continue
		}
		kind := strings.TrimSpace(rule.Kind)
		pattern := strings.TrimSpace(rule.Pattern)
		if kind == "" {
			result.SkippedRules = append(result.SkippedRules, skip(rule, "missing_kind"))
			continue
		}
		if pattern == "" {
			result.SkippedRules = append(result.SkippedRules, skip(rule, "missing_pattern"))
			continue
		}

		autopsy, err := ensureMapping(root, "autopsy")
		if err != nil {
			return Result{}, err
		}
		seq, err := ensureSequence(autopsy, section)
		if err != nil {
			return Result{}, err
		}
		if sequenceHasRule(seq, kind, pattern) {
			result.SkippedRules = append(result.SkippedRules, skip(rule, "duplicate"))
			continue
		}
		seq.Content = append(seq.Content, ruleNode(rule, kind, pattern))
		result.AddedRules = append(result.AddedRules, RuleChange{
			Target:  rule.Target,
			Section: section,
			Kind:    kind,
			Pattern: pattern,
		})
	}

	if len(result.AddedRules) == 0 {
		return result, nil
	}

	patched, err := marshalYAML(&doc)
	if err != nil {
		return Result{}, err
	}
	diff, err := unifiedDiff(scenarioYAML, patched, scenarioPath)
	if err != nil {
		return Result{}, err
	}
	result.Changed = true
	result.PatchedYAML = patched
	result.Diff = diff
	return result, nil
}

func autopsySection(target string) (string, bool) {
	switch strings.TrimSpace(target) {
	case "autopsy.expected_diagnostics":
		return "expected_diagnostics", true
	case "autopsy.allowed_mutations":
		return "allowed_mutations", true
	case "autopsy.forbidden_actions":
		return "forbidden_actions", true
	default:
		return "", false
	}
}

func rootMapping(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("scenario YAML root must be a mapping")
	}
	return doc, nil
}

func ensureMapping(parent *yaml.Node, key string) (*yaml.Node, error) {
	value := mappingValue(parent, key)
	if value == nil {
		value = &yaml.Node{Kind: yaml.MappingNode}
		parent.Content = append(parent.Content, scalarNode(key), value)
		return value, nil
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a mapping", key)
	}
	return value, nil
}

func ensureSequence(parent *yaml.Node, key string) (*yaml.Node, error) {
	value := mappingValue(parent, key)
	if value == nil {
		value = &yaml.Node{Kind: yaml.SequenceNode}
		parent.Content = append(parent.Content, scalarNode(key), value)
		return value, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s must be a sequence", key)
	}
	return value, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func sequenceHasRule(seq *yaml.Node, kind, pattern string) bool {
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		if mappingScalar(item, "kind") == kind && mappingScalar(item, "pattern") == pattern {
			return true
		}
	}
	return false
}

func mappingScalar(mapping *yaml.Node, key string) string {
	value := mappingValue(mapping, key)
	if value == nil || value.Kind != yaml.ScalarNode {
		return ""
	}
	return strings.TrimSpace(value.Value)
}

func ruleNode(rule runreview.SuggestedRule, kind, pattern string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	node.Content = append(node.Content,
		scalarNode("kind"), scalarNode(kind),
		scalarNode("pattern"), scalarNode(pattern),
	)
	if reason := strings.TrimSpace(rule.Reason); reason != "" {
		node.Content = append(node.Content, scalarNode("reason"), scalarNode(reason))
	}
	if severity := strings.TrimSpace(rule.Severity); severity != "" {
		node.Content = append(node.Content, scalarNode("severity"), scalarNode(severity))
	}
	return node
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value}
}

func skip(rule runreview.SuggestedRule, reason string) RuleSkip {
	return RuleSkip{
		Target:  rule.Target,
		Kind:    rule.Kind,
		Pattern: rule.Pattern,
		Reason:  reason,
	}
}

func marshalYAML(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("marshal scenario YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("marshal scenario YAML: %w", err)
	}
	return buf.Bytes(), nil
}

func unifiedDiff(before, after []byte, scenarioPath string) (string, error) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(before)),
		B:        difflib.SplitLines(string(after)),
		FromFile: scenarioPath,
		ToFile:   scenarioPath + " (review preview)",
		Context:  3,
	}
	out, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return "", fmt.Errorf("build scenario patch diff: %w", err)
	}
	return out, nil
}
