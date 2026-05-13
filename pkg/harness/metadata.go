package harness

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/verifier"
)

func countChecks(vr *verifier.VerifyResult) (passed, total int) {
	if vr == nil {
		return 0, 0
	}
	for _, c := range vr.Checks {
		total++
		if c.Verdict == verifier.VerdictPass {
			passed++
		}
	}
	return
}

func parseIntMeta(meta map[string]string, key string) int {
	v, ok := meta[key]
	if !ok {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

func parseFloatMeta(meta map[string]string, key string) float64 {
	v, ok := meta[key]
	if !ok {
		return 0
	}
	n, err := strconv.ParseFloat(v, 64)
	if err == nil {
		return n
	}
	if idx := strings.Index(v, "$"); idx >= 0 {
		num := v[idx+1:]
		for i, c := range num {
			if c == ' ' || c == '(' {
				num = num[:i]
				break
			}
		}
		n, _ = strconv.ParseFloat(num, 64)
	}
	return n
}

func resolveToolServerIdentity(cfg config.Config) (string, string) {
	if config.IsSupportedEvidenceMode(cfg.EvidenceMode) {
		cfg = config.ApplyEvidenceMode(cfg, cfg.EvidenceMode)
	}

	id := strings.TrimSpace(cfg.ToolServerID)
	if id == "" {
		id = inferToolServerID(cfg.MCPServer)
	}

	version := strings.TrimSpace(cfg.ToolServerVersion)
	if version == "" {
		version = mcpServerVersion(cfg.MCPServer)
	}
	return id, version
}

func inferToolServerID(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	first := filepath.Base(parts[0])
	switch first {
	case "npx", "pnpm", "bunx", "yarn":
		if pkg := inferPackageRunnerTarget(parts[1:]); pkg != "" {
			return pkg
		}
	}
	return first
}

func inferPackageRunnerTarget(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-y", "--yes", "--stdio", "--":
			continue
		case "-p", "--package":
			if i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--package=") {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

// mcpServerVersion queries the MCP server binary for its version string.
func mcpServerVersion(cmd string) string {
	if cmd == "" {
		return ""
	}
	parts := strings.Fields(cmd)
	out, err := exec.Command(parts[0], "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
