package harness

import (
	"testing"

	"github.com/vitas/evidra-bench/pkg/scenario"
)

// TestChecksToCheckDefsCarriesAssertV2 pins Phase 2 scope: the assert-v2
// type parses through the full schema->CheckDef pipeline unchanged (its
// runtime protocol lands in Phase 3; unknown types already error in
// verifier.BuildCheckers).
func TestChecksToCheckDefsCarriesAssertV2(t *testing.T) {
	defs := checksToCheckDefs([]scenario.Check{{
		Type:      scenario.CheckTypeAssertV2,
		Name:      "web-image-repaired",
		Namespace: "bench",
		Condition: "fixtures/assert_web.sh",
	}})
	if len(defs) != 1 {
		t.Fatalf("defs = %d", len(defs))
	}
	d := defs[0]
	if d.Type != "assert-v2" || d.Name != "web-image-repaired" || d.Namespace != "bench" || d.Condition != "fixtures/assert_web.sh" {
		t.Fatalf("check fields drifted: %+v", d)
	}
}
