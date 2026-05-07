package harness

import (
	"os/exec"
	"strconv"
	"strings"

	"samebits.com/evidra-infra-bench/pkg/verifier"
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
	n, _ := strconv.ParseFloat(v, 64)
	return n
}

// mcpServerName extracts the binary name from a full MCP server command.
// "my-mcp-server --stdio" -> "my-mcp-server"
func mcpServerName(cmd string) string {
	if cmd == "" {
		return ""
	}
	parts := strings.Fields(cmd)
	return parts[0]
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
