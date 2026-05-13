package config

import (
	"crypto/sha256"
	"fmt"
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

func TestCollectVersions_UsesSkillIdentityAndHash(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillPath := filepath.Join(dir, "k8s-admin.md")
	body := []byte("diagnose before mutate\n")
	if err := os.WriteFile(skillPath, body, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Default()
	cfg.SkillFile = skillPath
	cfg.SkillID = "k8s-admin"
	cfg.SkillVersion = "2026-05-13"
	cfg.SkillSource = "local-temp"

	got := CollectVersions("dev", "test-commit", cfg)
	wantHash := fmt.Sprintf("%x", sha256.Sum256(body))

	if got.PromptFile != skillPath {
		t.Fatalf("prompt_file = %q, want %q", got.PromptFile, skillPath)
	}
	if got.SkillID != "k8s-admin" {
		t.Fatalf("skill_id = %q, want k8s-admin", got.SkillID)
	}
	if got.SkillVersion != "2026-05-13" {
		t.Fatalf("skill_version = %q, want 2026-05-13", got.SkillVersion)
	}
	if got.SkillSource != "local-temp" {
		t.Fatalf("skill_source = %q, want local-temp", got.SkillSource)
	}
	if got.SkillSHA256 != wantHash {
		t.Fatalf("skill_sha256 = %q, want %q", got.SkillSHA256, wantHash)
	}
}

func TestCollectVersions_DoesNotLabelSystemPromptAsSkill(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	promptPath := filepath.Join(dir, "custom.md")
	if err := os.WriteFile(promptPath, []byte("custom prompt\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Default()
	cfg.SystemPromptFile = promptPath

	got := CollectVersions("dev", "test-commit", cfg)
	if got.PromptFile != promptPath {
		t.Fatalf("prompt_file = %q, want %q", got.PromptFile, promptPath)
	}
	if got.SkillID != "" {
		t.Fatalf("skill_id = %q, want empty", got.SkillID)
	}
	if got.SkillFile != "" {
		t.Fatalf("skill_file = %q, want empty", got.SkillFile)
	}
}
