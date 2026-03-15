package config

import (
	"os"
	"os/exec"
	"strings"
)

// VersionInfo holds version metadata for reproducibility.
type VersionInfo struct {
	InfraBenchVersion string `json:"infra_bench_version"`
	EvidraVersion     string `json:"evidra_version,omitempty"`
	ContractVersion   string `json:"contract_version,omitempty"`
	PromptFile        string `json:"prompt_file,omitempty"`
}

// CollectVersions gathers version information from the environment.
func CollectVersions(infraBenchVersion string, cfg Config) VersionInfo {
	vi := VersionInfo{
		InfraBenchVersion: infraBenchVersion,
		ContractVersion:   cfg.ContractVersion,
		PromptFile:        cfg.ResolveSystemPromptFile(),
	}

	// Try to get evidra version
	evidraBin := cfg.ResolveEvidraBin()
	if evidraBin != "" {
		if out, err := exec.Command(evidraBin, "--version").CombinedOutput(); err == nil {
			vi.EvidraVersion = strings.TrimSpace(string(out))
		}
	}

	// Extract contract version from prompt file header if not set
	if vi.ContractVersion == "" && vi.PromptFile != "" {
		vi.ContractVersion = extractContractVersion(vi.PromptFile)
	}

	return vi
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
