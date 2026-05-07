package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectVersions_UsesPromptHeaders(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(promptPath, []byte("<!-- contract: v1.2.3 -->\n<!-- prompt: p4 -->\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Default()
	cfg.SystemPromptFile = promptPath

	got := CollectVersions("dev", "test-commit", cfg)
	if got.ContractVersion != "v1.2.3" {
		t.Fatalf("contract_version = %q, want v1.2.3", got.ContractVersion)
	}
	if got.SkillVersion != "1.2" {
		t.Fatalf("skill_version = %q, want 1.2", got.SkillVersion)
	}
	if got.PromptVersion != "p4" {
		t.Fatalf("prompt_version = %q, want p4", got.PromptVersion)
	}
}
