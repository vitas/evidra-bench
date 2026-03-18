package config

import (
	"path/filepath"
	"testing"
)

func TestCollectVersions_UsesCanonicalPromptMetadata(t *testing.T) {
	t.Parallel()

	cfg := Default()
	cfg.SystemPromptFile = filepath.Clean("../evidra-benchmark/prompts/experiments/runtime/agent_contract_v1.md")

	got := CollectVersions("dev", "test-commit", cfg)
	if got.ContractVersion != "v1.1.0" {
		t.Fatalf("contract_version = %q", got.ContractVersion)
	}
	if got.SkillVersion != "1.1.0" {
		t.Fatalf("skill_version = %q", got.SkillVersion)
	}
	if got.PromptVersion != "sha256:6d94c115a8d5c5641be5be89a526f3b27f7a54f9fdd5b8e96f16905696dc100e" {
		t.Fatalf("prompt_version = %q", got.PromptVersion)
	}
}
