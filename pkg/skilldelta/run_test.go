package skilldelta

import (
	"path/filepath"
	"testing"
)

func TestPairArtifactPaths(t *testing.T) {
	t.Parallel()

	paths := PairArtifactPaths("/tmp/skill-delta", "broken-deployment", "openai/gpt-4o", 2)

	wantBase := filepath.Join("/tmp/skill-delta", "cases", "broken-deployment", "openai-gpt-4o", "repeat-2")
	if paths.PairDir != wantBase {
		t.Fatalf("PairDir = %q, want %q", paths.PairDir, wantBase)
	}
	if paths.WithoutSkillRunsDir != filepath.Join(wantBase, "without_skill") {
		t.Fatalf("WithoutSkillRunsDir = %q", paths.WithoutSkillRunsDir)
	}
	if paths.WithSkillRunsDir != filepath.Join(wantBase, "with_skill") {
		t.Fatalf("WithSkillRunsDir = %q", paths.WithSkillRunsDir)
	}
	if paths.PairJSONPath != filepath.Join(wantBase, "pair.json") {
		t.Fatalf("PairJSONPath = %q", paths.PairJSONPath)
	}
}

func TestBuildDryRunPair(t *testing.T) {
	t.Parallel()

	paths := PairArtifactPaths("/tmp/skill-delta", "broken-deployment", "sonnet", 1)
	pair := BuildDryRunPair(PairSpec{
		ScenarioID: "broken-deployment",
		Model:      "sonnet",
		Provider:   "claude",
		Repeat:     1,
		Paths:      paths,
	})

	if pair.ScenarioID != "broken-deployment" {
		t.Fatalf("ScenarioID = %q", pair.ScenarioID)
	}
	if pair.Paths.WithoutSkillRunDir != paths.WithoutSkillRunsDir {
		t.Fatalf("WithoutSkillRunDir = %q", pair.Paths.WithoutSkillRunDir)
	}
	if pair.WithoutSkill.Metadata["dry_run"] != "true" {
		t.Fatalf("WithoutSkill.Metadata[dry_run] = %q", pair.WithoutSkill.Metadata["dry_run"])
	}
	if pair.WithSkill.Metadata["dry_run"] != "true" {
		t.Fatalf("WithSkill.Metadata[dry_run] = %q", pair.WithSkill.Metadata["dry_run"])
	}
}
