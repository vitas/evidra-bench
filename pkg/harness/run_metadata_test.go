package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-infra-bench/pkg/agent"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/scenario"
	"samebits.com/evidra-infra-bench/pkg/store"
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

func TestBuildRunMetadata_PrefersExplicitEvidenceMode(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Provider = "claude"
	cfg.Model = "sonnet"
	cfg.EvidenceMode = "none"
	cfg.MCPServer = "sample-mcp --stdio"
	cfg.SystemPromptFile = writePromptMetadataFile(t, "v1.2.3", "p7")
	cfg.ContractVersion = "v9.9.9"

	meta := buildRunMetadata(cfg, &agent.LoopResult{}, "/tmp/evidence")

	if meta["evidence_mode"] != "none" {
		t.Fatalf("evidence_mode = %q, want none", meta["evidence_mode"])
	}
	if _, ok := meta["contract_version"]; ok {
		t.Fatalf("contract_version unexpectedly present: %q", meta["contract_version"])
	}
}

func TestHarness_StoreUsesExplicitEvidenceMode(t *testing.T) {
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
	cfg.EvidenceMode = "none"
	cfg.MCPServer = "sample-mcp --stdio"
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
	if rec.EvidenceMode != "none" {
		t.Fatalf("stored evidence_mode = %q, want none", rec.EvidenceMode)
	}
}
