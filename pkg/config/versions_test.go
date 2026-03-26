package config

import (
	"testing"

	promptdata "samebits.com/evidra/prompts"
)

func TestCollectVersions_UsesCanonicalPromptMetadata(t *testing.T) {
	t.Parallel()

	cfg := Default()
	// Use the embedded prompt path — no filesystem dependency on parent repo.
	cfg.SystemPromptFile = promptdata.MCPAgentContractPath

	got := CollectVersions("dev", "test-commit", cfg)
	if got.ContractVersion != promptdata.DefaultContractVersion {
		t.Fatalf("contract_version = %q, want %q", got.ContractVersion, promptdata.DefaultContractVersion)
	}
	expectedSkill := promptdata.DefaultContractSkillVersion
	if got.SkillVersion != expectedSkill {
		t.Fatalf("skill_version = %q, want %q", got.SkillVersion, expectedSkill)
	}
	if got.PromptVersion == "" {
		t.Fatalf("prompt_version is empty")
	}
}
