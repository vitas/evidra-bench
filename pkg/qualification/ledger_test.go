package qualification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func goodInputs() Inputs {
	return Inputs{
		Scenario: "sha256:aa", AuthorityPolicy: "sha256:bb", Fixtures: "sha256:cc",
		VerifierScripts: "sha256:dd", ComponentRevis: "123456789012", Providers: []string{"kind@v1.31.2"},
	}
}

func goodEntry(in Inputs) *Entry {
	return &Entry{
		Schema: SchemaID, ScenarioID: "broken-deployment", Qualified: true,
		GrantedAt: time.Now().UTC(), Inputs: in,
		Matrix: MatrixEvidence{
			KnownGoodPasses: 5, KnownGoodRequired: 5, NoOpFailsOutcome: true,
			ShortcutViolates: true, ForbiddenUnsafe: true, Forbidden403Safe: true,
			EvidenceLossIncomp: true, VerifierFaultIncomp: true,
			ProvidersEquivalent: true, FlakeBudgetRespected: true,
		},
	}
}

func TestVerifyHappyAndDrift(t *testing.T) {
	in := goodInputs()
	if v := Verify(goodEntry(in), in); !v.Authorized {
		t.Fatalf("clean ledger must authorize: %+v", v)
	}
	// Every field must independently demote (the P10 negative test).
	mut := func(f func(*Inputs)) Inputs { c := in; f(&c); return c }
	for name, changed := range map[string]Inputs{
		"scenario":           mut(func(i *Inputs) { i.Scenario = "sha256:zz" }),
		"authority_policy":   mut(func(i *Inputs) { i.AuthorityPolicy = "sha256:zz" }),
		"fixtures":           mut(func(i *Inputs) { i.Fixtures = "sha256:zz" }),
		"verifier_scripts":   mut(func(i *Inputs) { i.VerifierScripts = "sha256:zz" }),
		"component_revision": mut(func(i *Inputs) { i.ComponentRevis = "other" }),
		"providers":          mut(func(i *Inputs) { i.Providers = []string{"k3d@v1.31.5+k3s1"} }),
	} {
		v := Verify(goodEntry(in), changed)
		if v.Authorized || !strings.Contains(strings.Join(v.Reasons, "|"), name) {
			t.Fatalf("%s drift not caught: %+v", name, v)
		}
	}
}

func TestVerifyRefusesBadShapes(t *testing.T) {
	in := goodInputs()
	if v := Verify(nil, in); !v.Missing || v.Authorized {
		t.Fatalf("missing = %+v", v)
	}
	wrong := goodEntry(in)
	wrong.Schema = "v0"
	if v := Verify(wrong, in); v.Authorized {
		t.Fatal("foreign schema must not authorize")
	}
	off := goodEntry(in)
	off.Qualified = false
	if v := Verify(off, in); v.Authorized {
		t.Fatal("non-attested entry must not authorize")
	}
	flake := goodEntry(in)
	flake.Matrix.KnownGoodPasses = 4
	if v := Verify(flake, in); v.Authorized {
		t.Fatal("flake budget shortfall must not authorize")
	}
	hollow := goodEntry(in)
	hollow.Matrix.ShortcutViolates = false
	if v := Verify(hollow, in); v.Authorized {
		t.Fatal("missing behavior attestation must not authorize")
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	e := goodEntry(goodInputs())
	if err := Write(dir, e); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil || got == nil {
		t.Fatalf("load: %v %v", got, err)
	}
	if got.Schema != SchemaID || !got.Matrix.Complete() {
		t.Fatalf("roundtrip lost fields: %+v", got)
	}
	// Absent = nil,nil (preview path), corrupt = error (never silent OK).
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("corrupt ledger must error, not demote silently")
	}
	empty := t.TempDir()
	if got, err := Load(empty); got != nil || err != nil {
		t.Fatal("absent must be (nil,nil)")
	}
}

func TestHashTreeSemantics(t *testing.T) {
	dir := t.TempDir()
	mk := func(p, content string) {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("run", "#!/bin/sh\necho hi\n")
	mk("inputs/prompt.md", "task")
	base, err := HashTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Content change flips.
	mk("run", "#!/bin/sh\necho evil\n")
	changed, _ := HashTree(dir)
	if base == changed {
		t.Fatal("content change invisible")
	}
	// Rename flips (names are hashed too).
	mk("run2", "#!/bin/sh\necho evil\n")
	if err := os.Remove(filepath.Join(dir, "run")); err != nil {
		t.Fatal(err)
	}
	renamed, _ := HashTree(dir)
	if renamed == changed {
		t.Fatal("rename invisible")
	}
	// Rebuild identical tree => identical digest (ordering irrelevant).
	dir2 := t.TempDir()
	mk2 := func(p, content string) {
		full := filepath.Join(dir2, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk2("inputs/prompt.md", "task")
	mk2("run2", "#!/bin/sh\necho evil\n")
	again, err := HashTree(dir2)
	if err != nil || again != renamed {
		t.Fatalf("stable rebuild unstable: %v %v vs %v", again, renamed, err)
	}
}

func TestBuildRevisionNonEmpty(t *testing.T) {
	if r := BuildRevision(); r == "" {
		t.Fatal("revision must identify the build")
	}
}

func TestVerifyProviderSetMembership(t *testing.T) {
	in := goodInputs() // live: kind@v1.31.2
	rec := goodInputs()
	rec.Providers = []string{"kind@v1.31.2", "k3d@v1.31.5+k3s1"}
	if v := Verify(goodEntry(rec), in); !v.Authorized {
		t.Fatalf("recorded set must qualify its members: %+v", v.Reasons)
	}
	liveBoth := goodInputs()
	liveBoth.Providers = []string{"kind@v1.31.2", "azure@v1.32"}
	if v := Verify(goodEntry(rec), liveBoth); v.Authorized {
		t.Fatal("unrecorded member must demote")
	}
}

// The revision is a compile-time fact. Review #4 killed the runtime
// environment override; this test proves nothing at runtime can move it.
func TestBuildRevisionIsImmutable(t *testing.T) {
	t.Setenv("EVIDRA_BUILD_REVISION", "git-abc123")
	if got := BuildRevision(); got == "git-abc123" {
		t.Fatal("environment must NOT be able to pin the revision")
	}
	// Injecting the linker stamp (same mechanism -X uses) does win.
	old := buildRevision
	buildRevision = "stamped-rev"
	defer func() { buildRevision = old }()
	if got := BuildRevision(); got != "stamped-rev" {
		t.Fatalf("linked stamp must win: %q", got)
	}
	// Unstamped: honest fallback detection, never empty.
	buildRevision = ""
	if got := BuildRevision(); got == "" {
		t.Fatal("fallback must still report something")
	}
}
