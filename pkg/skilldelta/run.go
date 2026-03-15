package skilldelta

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PairSpec identifies one scenario/model/repeat pair.
type PairSpec struct {
	ScenarioID string
	Model      string
	Provider   string
	Repeat     int
	Paths      ArtifactPaths
}

// ArtifactPaths describes the output layout for one paired benchmark case.
type ArtifactPaths struct {
	PairDir             string
	WithoutSkillRunsDir string
	WithSkillRunsDir    string
	PairJSONPath        string
}

// PairArtifactPaths returns the standard directory layout for one pair.
func PairArtifactPaths(root, scenarioID, model string, repeat int) ArtifactPaths {
	pairDir := filepath.Join(
		root,
		"cases",
		safePathPart(scenarioID),
		safePathPart(model),
		fmt.Sprintf("repeat-%d", repeat),
	)
	return ArtifactPaths{
		PairDir:             pairDir,
		WithoutSkillRunsDir: filepath.Join(pairDir, "without_skill"),
		WithSkillRunsDir:    filepath.Join(pairDir, "with_skill"),
		PairJSONPath:        filepath.Join(pairDir, "pair.json"),
	}
}

// BuildDryRunPair returns a placeholder pair result for command and pipeline
// verification when the underlying scenario execution is skipped.
func BuildDryRunPair(spec PairSpec) PairResult {
	without := RunSnapshot{
		Metadata: map[string]string{
			"dry_run": "true",
		},
	}
	with := RunSnapshot{
		Metadata: map[string]string{
			"dry_run": "true",
		},
	}

	return PairResult{
		ScenarioID:   spec.ScenarioID,
		Model:        spec.Model,
		Provider:     spec.Provider,
		Repeat:       spec.Repeat,
		WithoutSkill: without,
		WithSkill:    with,
		VerdictDelta: "same",
		Paths: PairPaths{
			WithoutSkillRunDir: spec.Paths.WithoutSkillRunsDir,
			WithSkillRunDir:    spec.Paths.WithSkillRunsDir,
		},
	}
}

func safePathPart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, string(filepath.Separator), "-")
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, " ", "-")
	if value == "" {
		return "unknown"
	}
	return value
}
