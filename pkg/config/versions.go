package config

import (
	"os"
	"strings"
)

// VersionInfo holds all version metadata for reproducible benchmarks.
// Every run must record these so results can be compared across time,
// models, and evidra releases.
type VersionInfo struct {
	// infra-bench
	InfraBenchVersion string `json:"infra_bench_version"`
	InfraBenchCommit  string `json:"infra_bench_commit,omitempty"`

	// prompt/contract
	ContractVersion string `json:"contract_version,omitempty"`
	SkillVersion    string `json:"skill_version,omitempty"`
	PromptVersion   string `json:"prompt_version,omitempty"`
	PromptFile      string `json:"prompt_file,omitempty"`
	Role            string `json:"role,omitempty"`
}

// CollectVersions gathers version information from the prompt file and the
// infra-bench build.
func CollectVersions(infraBenchVersion, infraBenchCommit string, cfg Config) VersionInfo {
	vi := VersionInfo{
		InfraBenchVersion: infraBenchVersion,
		InfraBenchCommit:  infraBenchCommit,
		ContractVersion:   cfg.ContractVersion,
		PromptFile:        cfg.ResolveSystemPromptFile(),
		Role:              cfg.Role,
	}

	if vi.ContractVersion != "" {
		vi.SkillVersion = parseSkillVersionFromContractVersion(vi.ContractVersion)
	}

	if vi.PromptFile != "" {
		if vi.ContractVersion == "" {
			vi.ContractVersion = extractContractVersion(vi.PromptFile)
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
	if v.Role != "" {
		m["role"] = v.Role
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
