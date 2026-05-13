package tui

import (
	"testing"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

func testScenarios() []CatalogItem {
	return []CatalogItem{
		{Scenario: &scenario.Scenario{ID: "broken-deployment", Title: "Fix broken deployment", Category: "kubernetes", Tags: []string{"deployment", "image"}}},
		{Scenario: &scenario.Scenario{ID: "helm-failed-upgrade", Title: "Fix failed Helm upgrade", Category: "helm", Tags: []string{"helm", "rollback"}}},
		{Scenario: &scenario.Scenario{ID: "argocd-out-of-sync", Title: "Fix ArgoCD drift", Category: "argocd", Tags: []string{"argocd", "drift"}}},
		{Scenario: &scenario.Scenario{ID: "privileged-pod-review", Title: "Privileged pod review", Category: "kubernetes", Tags: []string{"security", "protocol"}}},
	}
}

func TestFilterCatalog_NoFilter(t *testing.T) {
	t.Parallel()
	items := testScenarios()
	result := FilterCatalog(items, "", "")
	if len(result) != 4 {
		t.Fatalf("expected 4 items, got %d", len(result))
	}
}

func TestFilterCatalog_ByCategory(t *testing.T) {
	t.Parallel()
	items := testScenarios()
	result := FilterCatalog(items, "", "kubernetes")
	if len(result) != 2 {
		t.Fatalf("expected 2 kubernetes items, got %d", len(result))
	}
}

func TestFilterCatalog_ByQuery(t *testing.T) {
	t.Parallel()
	items := testScenarios()
	result := FilterCatalog(items, "helm", "")
	if len(result) != 1 {
		t.Fatalf("expected 1 helm item, got %d", len(result))
	}
	if result[0].Scenario.ID != "helm-failed-upgrade" {
		t.Fatalf("expected helm-failed-upgrade, got %s", result[0].Scenario.ID)
	}
}

func TestFilterCatalog_ByTag(t *testing.T) {
	t.Parallel()
	items := testScenarios()
	result := FilterCatalog(items, "security", "")
	if len(result) != 1 {
		t.Fatalf("expected 1 security item, got %d", len(result))
	}
}

func TestFilterCatalog_ByQueryAndCategory(t *testing.T) {
	t.Parallel()
	items := testScenarios()
	result := FilterCatalog(items, "deployment", "kubernetes")
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
}

func TestFilterCatalog_CaseInsensitive(t *testing.T) {
	t.Parallel()
	items := testScenarios()
	result := FilterCatalog(items, "HELM", "")
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
}

func TestFilterCatalog_NoMatch(t *testing.T) {
	t.Parallel()
	items := testScenarios()
	result := FilterCatalog(items, "nonexistent", "")
	if len(result) != 0 {
		t.Fatalf("expected 0 items, got %d", len(result))
	}
}

func TestBuildCatalog(t *testing.T) {
	t.Parallel()
	scenarios := []*scenario.Scenario{
		{ID: "a"},
		{ID: "b"},
	}
	items := BuildCatalog(scenarios)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Scenario.ID != "a" {
		t.Fatalf("expected id=a, got %s", items[0].Scenario.ID)
	}
}
