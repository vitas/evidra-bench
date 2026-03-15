package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
)

// VersionInfo holds all version metadata for reproducible benchmarks.
// Every run must record these so results can be compared across time,
// models, and evidra releases.
type VersionInfo struct {
	// infra-bench
	InfraBenchVersion string `json:"infra_bench_version"`
	InfraBenchCommit  string `json:"infra_bench_commit,omitempty"`

	// evidra binary
	EvidraVersion string `json:"evidra_version,omitempty"`

	// evidra signal/scoring specification
	SpecVersion      string `json:"spec_version,omitempty"`
	ScoringVersion   string `json:"scoring_version,omitempty"`
	ScoringProfileID string `json:"scoring_profile_id,omitempty"`

	// prompt/contract
	ContractVersion string `json:"contract_version,omitempty"`
	SkillVersion    string `json:"skill_version,omitempty"`
	PromptFile      string `json:"prompt_file,omitempty"`
}

// CollectVersions gathers version information from the evidra binary,
// the prompt file, and the infra-bench build.
func CollectVersions(infraBenchVersion, infraBenchCommit string, cfg Config) VersionInfo {
	vi := VersionInfo{
		InfraBenchVersion: infraBenchVersion,
		InfraBenchCommit:  infraBenchCommit,
		ContractVersion:   cfg.ContractVersion,
		SkillVersion:      cfg.ContractVersion, // skill_version matches contract_version
		PromptFile:        cfg.ResolveSystemPromptFile(),
	}

	evidraBin := cfg.ResolveEvidraBin()
	if evidraBin != "" {
		// Get evidra version
		if out, err := exec.Command(evidraBin, "--version").CombinedOutput(); err == nil {
			vi.EvidraVersion = strings.TrimSpace(string(out))
		}
		// Get spec/scoring versions from evidra scorecard --help or dry run
		vi.SpecVersion, vi.ScoringVersion, vi.ScoringProfileID = probeEvidraVersions(evidraBin)
	}

	// Extract contract version from prompt file header if not set
	if vi.ContractVersion == "" && vi.PromptFile != "" {
		vi.ContractVersion = extractContractVersion(vi.PromptFile)
		vi.SkillVersion = vi.ContractVersion
	}

	return vi
}

// probeEvidraVersions runs a minimal evidra scorecard to extract spec/scoring metadata.
func probeEvidraVersions(evidraBin string) (specVersion, scoringVersion, profileID string) {
	// Create a temp dir with empty evidence to get a scorecard with version fields
	tmpDir, err := os.MkdirTemp("", "evidra-version-probe")
	if err != nil {
		return
	}
	defer os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir+"/segments", 0755)

	out, err := exec.Command(evidraBin, "scorecard", "--evidence-dir", tmpDir, "--ttl", "1s").CombinedOutput()
	if err != nil {
		return
	}

	var sc struct {
		SpecVersion      string `json:"spec_version"`
		ScoringVersion   string `json:"scoring_version"`
		ScoringProfileID string `json:"scoring_profile_id"`
	}
	if json.Unmarshal(out, &sc) == nil {
		specVersion = sc.SpecVersion
		scoringVersion = sc.ScoringVersion
		profileID = sc.ScoringProfileID
	}
	return
}

// ToMetadata converts VersionInfo to a flat string map for run metadata.
func (v VersionInfo) ToMetadata() map[string]string {
	m := map[string]string{}
	if v.InfraBenchVersion != "" {
		m["infra_bench_version"] = v.InfraBenchVersion
	}
	if v.InfraBenchCommit != "" {
		m["infra_bench_commit"] = v.InfraBenchCommit
	}
	if v.EvidraVersion != "" {
		m["evidra_version"] = v.EvidraVersion
	}
	if v.SpecVersion != "" {
		m["spec_version"] = v.SpecVersion
	}
	if v.ScoringVersion != "" {
		m["scoring_version"] = v.ScoringVersion
	}
	if v.ScoringProfileID != "" {
		m["scoring_profile_id"] = v.ScoringProfileID
	}
	if v.ContractVersion != "" {
		m["contract_version"] = v.ContractVersion
	}
	if v.SkillVersion != "" {
		m["skill_version"] = v.SkillVersion
	}
	if v.PromptFile != "" {
		m["system_prompt_file"] = v.PromptFile
	}
	return m
}

// extractContractVersion reads the <!-- contract: vX.Y.Z --> header from a prompt file.
func extractContractVersion(path string) string {
	data, err := readFileHead(path, 512)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "<!-- contract:") {
			v := strings.TrimPrefix(line, "<!-- contract:")
			v = strings.TrimSuffix(v, "-->")
			return strings.TrimSpace(v)
		}
		if strings.HasPrefix(line, "# contract:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# contract:"))
		}
	}
	return ""
}

func readFileHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	nr, err := f.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:nr], nil
}
