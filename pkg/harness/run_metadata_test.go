package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/agent"
	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/scenario"
	"github.com/vitas/evidra-bench/pkg/store"
)

func TestBuildRunMetadata_UsesPromptFileMetadata(t *testing.T) {
	t.Parallel()

	promptPath := writePromptMetadataFile(t, "v1.2.3", "p7")
	cfg := config.Default()
	cfg.Provider = "claude"
	cfg.Model = "sonnet"
	cfg.SystemPromptFile = promptPath

	meta := buildRunMetadata(cfg, &agent.LoopResult{
		Turns:        4,
		MemoryWindow: 12,
		TotalUsage: agent.Usage{
			PromptTokens:     120,
			CompletionTokens: 45,
		},
	}, "/tmp/evidence")

	if meta["contract_version"] != "v1.2.3" {
		t.Fatalf("contract_version = %q, want v1.2.3", meta["contract_version"])
	}
	if meta["skill_version"] != "1.2" {
		t.Fatalf("skill_version = %q, want 1.2", meta["skill_version"])
	}
	if meta["prompt_version"] != "p7" {
		t.Fatalf("prompt_version = %q, want p7", meta["prompt_version"])
	}
}

func TestBuildRunMetadata_IncludesToolServerIdentity(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Provider = "claude"
	cfg.Model = "sonnet"
	cfg.MCPServer = "npx -y @vendor/kubernetes-mcp --stdio"
	cfg.ToolServerID = "kubernetes-mcp"
	cfg.ToolServerVersion = "1.2.3"

	meta := buildRunMetadata(cfg, &agent.LoopResult{}, "/tmp/evidence")

	if meta["tool_server"] != "kubernetes-mcp" {
		t.Fatalf("tool_server = %q, want kubernetes-mcp", meta["tool_server"])
	}
	if meta["tool_server_version"] != "1.2.3" {
		t.Fatalf("tool_server_version = %q, want 1.2.3", meta["tool_server_version"])
	}
	if meta["tool_server_cmd"] != "npx -y @vendor/kubernetes-mcp --stdio" {
		t.Fatalf("tool_server_cmd = %q, want command", meta["tool_server_cmd"])
	}
}

func TestBuildRunMetadata_IncludesReportID(t *testing.T) {
	t.Parallel()

	cfg := config.Config{ReportID: "kubernetes-mcp-readiness-2026-05"}
	meta := buildRunMetadata(cfg, &agent.LoopResult{}, "/tmp/evidence")
	if meta["report_id"] != "kubernetes-mcp-readiness-2026-05" {
		t.Fatalf("report_id metadata = %q", meta["report_id"])
	}
}

func TestParseFloatMetaReadsFormattedCost(t *testing.T) {
	t.Parallel()

	got := parseFloatMeta(map[string]string{
		"estimated_cost": "$0.0336 (in: $0.0316/225443T, out: $0.0020/7294T)",
	}, "estimated_cost")
	if got != 0.0336 {
		t.Fatalf("estimated_cost = %v, want 0.0336", got)
	}
}

func TestResolveToolServerIdentity_PrefersExplicitLabels(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		MCPServer:         "npx -y @vendor/kubernetes-mcp --stdio",
		ToolServerID:      "kubernetes-mcp",
		ToolServerVersion: "1.2.3",
	}

	id, version := resolveToolServerIdentity(cfg)
	if id != "kubernetes-mcp" {
		t.Fatalf("tool server id = %q, want kubernetes-mcp", id)
	}
	if version != "1.2.3" {
		t.Fatalf("tool server version = %q, want 1.2.3", version)
	}
}

func TestResolveToolServerIdentity_InfersNpxPackageName(t *testing.T) {
	t.Parallel()

	cfg := config.Config{MCPServer: "npx -y @vendor/kubernetes-mcp --stdio"}

	id, _ := resolveToolServerIdentity(cfg)
	if id != "@vendor/kubernetes-mcp" {
		t.Fatalf("tool server id = %q, want @vendor/kubernetes-mcp", id)
	}
}

func TestHarness_StoreUsesNativeToolBaseline(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	fa := &fakeAdapter{}
	storeDir := t.TempDir()
	rs, err := store.Open(storeDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := rs.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	h := New(Deps{
		EnvProvider: fp,
		Adapter:     fa,
		Store:       rs,
	})

	cfg := config.Default()
	cfg.Scenario = "broken-deployment"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")
	cfg.SystemPromptFile = writePromptMetadataFile(t, "v1.2.3", "p7")

	if _, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		Scenario: &scenario.Scenario{
			ID:       "broken-deployment",
			Title:    "Fix broken deployment",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		},
	}); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(storeDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 stored record, got %d", len(lines))
	}
	var rec store.RunRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("unmarshal stored record: %v", err)
	}
	if rec.ToolServer != "" {
		t.Fatalf("stored tool_server = %q, want empty baseline", rec.ToolServer)
	}
}

func TestHarness_StoreUsesExplicitToolServerIdentity(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	fa := &fakeAdapter{}
	storeDir := t.TempDir()
	rs, err := store.Open(storeDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := rs.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	h := New(Deps{
		EnvProvider: fp,
		Adapter:     fa,
		Store:       rs,
	})

	cfg := config.Default()
	cfg.Scenario = "broken-deployment"
	cfg.RunsDir = filepath.Join(t.TempDir(), "runs")
	cfg.MCPServer = "npx -y @vendor/kubernetes-mcp --stdio"
	cfg.ToolServerID = "kubernetes-mcp"
	cfg.ToolServerVersion = "1.2.3"

	if _, err := h.Run(context.Background(), RunRequest{
		Config:         cfg,
		KubeconfigPath: fakeKubeconfig(t),
		Scenario: &scenario.Scenario{
			ID:       "broken-deployment",
			Title:    "Fix broken deployment",
			Category: "kubernetes",
			Checks:   []scenario.Check{{Type: "deployment-ready", Namespace: "bench", Name: "web"}},
		},
	}); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(storeDir, "results.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 stored record, got %d", len(lines))
	}
	var rec store.RunRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("unmarshal stored record: %v", err)
	}
	if rec.ToolServer != "kubernetes-mcp" {
		t.Fatalf("stored tool_server = %q, want kubernetes-mcp", rec.ToolServer)
	}
	if rec.ToolServerVersion != "1.2.3" {
		t.Fatalf("stored tool_server_version = %q, want 1.2.3", rec.ToolServerVersion)
	}
	if !strings.Contains(rec.MetadataJSON, `"tool_server_cmd":"npx -y @vendor/kubernetes-mcp --stdio"`) {
		t.Fatalf("metadata_json = %q, want tool_server_cmd", rec.MetadataJSON)
	}
}
