package qualification

import (
	"os/exec"
	"strings"
	"testing"
)

// The ledger's component identity is the digest of the EXECUTABLE build
// inputs (tools/code-revision.sh), not the repo commit sha — commit shas
// made the ledger structurally unpassable, because the commit carrying a
// ledger is always later than the commit its ledger records (reviewer
// round-2 blocker #2). This test pins the contract: the stamp format is
// the script's output format, and the script is deterministic.
func TestRevisionIdentityIsCodeDigest(t *testing.T) {
	out, err := exec.Command("bash", "../../tools/code-revision.sh").Output()
	if err != nil {
		t.Fatalf("code-revision.sh: %v", err)
	}
	rev := strings.TrimSpace(string(out))
	if len(rev) != len("code-")+40 || !strings.HasPrefix(rev, "code-") {
		t.Fatalf("want code-<sha256:40>, got %q", rev)
	}
	out2, err := exec.Command("bash", "../../tools/code-revision.sh").Output()
	if err != nil || strings.TrimSpace(string(out2)) != rev {
		t.Fatal("code revision must be deterministic across invocations")
	}
	// The fallback path (no stamp, no build-info) must never look like a
	// code digest, so a mis-built binary can never masquerade as certified.
	if strings.HasPrefix(fallbackRevisionForTest(), "code-") {
		t.Fatal("fallback revision must not impersonate the digest form")
	}
}

func fallbackRevisionForTest() string { return fallbackRevision() }
