package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VersionInfo holds all version metadata for reproducible benchmarks.
// Every run must record these so results can be compared across time,
// models, prompts, and skill releases.
type VersionInfo struct {
	// infra-bench
	InfraBenchVersion string `json:"infra_bench_version"`
	InfraBenchCommit  string `json:"infra_bench_commit,omitempty"`

	// prompt/contract
	ContractVersion string `json:"contract_version,omitempty"`
	SkillVersion    string `json:"skill_version,omitempty"`
	PromptVersion   string `json:"prompt_version,omitempty"`
	PromptFile      string `json:"prompt_file,omitempty"`
	SkillFile       string `json:"skill_file,omitempty"`
	SkillID         string `json:"skill_id,omitempty"`
	SkillSource     string `json:"skill_source,omitempty"`
	SkillSHA256     string `json:"skill_sha256,omitempty"`
}

// CollectVersions gathers version information from the prompt file and the
// infra-bench build.
func CollectVersions(infraBenchVersion, infraBenchCommit string, cfg Config) VersionInfo {
	skillFile := cfg.ResolveSkillFile()
	promptFile := cfg.ResolvePromptFile()

	vi := VersionInfo{
		InfraBenchVersion: infraBenchVersion,
		InfraBenchCommit:  infraBenchCommit,
		ContractVersion:   cfg.ContractVersion,
		SkillVersion:      strings.TrimSpace(cfg.SkillVersion),
		PromptFile:        promptFile,
		SkillFile:         skillFile,
		SkillID:           strings.TrimSpace(cfg.SkillID),
		SkillSource:       strings.TrimSpace(cfg.SkillSource),
		SkillSHA256:       strings.TrimSpace(cfg.SkillSHA256),
	}

	if vi.SkillID == "" {
		vi.SkillID = inferSkillID(skillFile)
	}
	if vi.SkillSource == "" {
		vi.SkillSource = inferSkillSource(skillFile)
	}
	if vi.SkillSHA256 == "" && skillFile != "" {
		vi.SkillSHA256 = fileSHA256(skillFile)
	}

	if vi.ContractVersion != "" && vi.SkillVersion == "" {
		vi.SkillVersion = parseSkillVersionFromContractVersion(vi.ContractVersion)
	}

	if vi.PromptFile != "" {
		if vi.ContractVersion == "" {
			vi.ContractVersion = extractContractVersion(vi.PromptFile)
		}
		if vi.SkillVersion == "" {
			vi.SkillVersion = parseSkillVersionFromContractVersion(vi.ContractVersion)
		}
		vi.PromptVersion = extractPromptVersion(vi.PromptFile)
	}

	return vi
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
	if v.ContractVersion != "" {
		m["contract_version"] = v.ContractVersion
	}
	if v.SkillVersion != "" {
		m["skill_version"] = v.SkillVersion
	}
	if v.PromptVersion != "" {
		m["prompt_version"] = v.PromptVersion
	}
	if v.PromptFile != "" {
		m["system_prompt_file"] = v.PromptFile
	}
	if v.SkillFile != "" {
		m["skill_file"] = v.SkillFile
	}
	if v.SkillID != "" {
		m["skill_id"] = v.SkillID
	}
	if v.SkillSource != "" {
		m["skill_source"] = v.SkillSource
	}
	if v.SkillSHA256 != "" {
		m["skill_sha256"] = v.SkillSHA256
	}
	return m
}

func parseSkillVersionFromContractVersion(contractVersion string) string {
	v := strings.TrimPrefix(strings.TrimSpace(contractVersion), "v")
	if v == "" {
		return ""
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
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

// extractPromptVersion reads an optional prompt version header from a prompt file.
func extractPromptVersion(path string) string {
	data, err := readFileHead(path, 512)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "<!-- prompt:") {
			v := strings.TrimPrefix(line, "<!-- prompt:")
			v = strings.TrimSuffix(v, "-->")
			return strings.TrimSpace(v)
		}
		if strings.HasPrefix(line, "# prompt:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# prompt:"))
		}
	}
	return ""
}

func inferSkillID(skillFile string) string {
	if skillFile == "" {
		return ""
	}
	base := filepath.Base(skillFile)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func inferSkillSource(skillFile string) string {
	if skillFile != "" {
		return "local-file"
	}
	return ""
}

func fileSHA256(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func readFileHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, n)
	nr, err := f.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:nr], nil
}
