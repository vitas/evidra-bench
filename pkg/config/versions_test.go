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
	if got.ContractVersion != "v1.0.1" {
		t.Fatalf("contract_version = %q", got.ContractVersion)
	}
	if got.SkillVersion != "1.0.1" {
		t.Fatalf("skill_version = %q", got.SkillVersion)
	}
	if got.PromptVersion != "sha256:a79fc218d2d69f402fd200de808617de9b770adc95c064d69c6ab22511ad5aef" {
		t.Fatalf("prompt_version = %q", got.PromptVersion)
	}
}
