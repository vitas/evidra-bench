package tui

import (
	"strings"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

// CatalogItem wraps a scenario for display in the TUI catalog.
type CatalogItem struct {
	Scenario   *scenario.Scenario
	LastResult string // "pass", "fail", or ""
}

// CategoryFilters defines the rotation of category filters.
var CategoryFilters = []string{"", "kubernetes", "terraform", "helm", "argocd", "aws"}

// FilterCatalog returns items matching the given text query and category filter.
func FilterCatalog(items []CatalogItem, query, category string) []CatalogItem {
	if query == "" && category == "" {
		return items
	}
	query = strings.ToLower(query)
	var result []CatalogItem
	for _, item := range items {
		if category != "" && !item.Scenario.HasCategory(category) {
			continue
		}
		if query != "" && !matchesQuery(item.Scenario, query) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func matchesQuery(s *scenario.Scenario, query string) bool {
	if strings.Contains(strings.ToLower(s.ID), query) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Title), query) {
		return true
	}
	for _, cat := range s.ResolvedCategories() {
		if strings.Contains(strings.ToLower(cat), query) {
			return true
		}
	}
	for _, tag := range s.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}

// BuildCatalog creates a catalog from loaded scenarios.
func BuildCatalog(scenarios []*scenario.Scenario) []CatalogItem {
	items := make([]CatalogItem, len(scenarios))
	for i, s := range scenarios {
		items[i] = CatalogItem{Scenario: s}
	}
	return items
}
