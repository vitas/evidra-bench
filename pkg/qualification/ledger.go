// Package qualification implements the ADR 0001 §8 qualification ledger:
// a per-scenario, per-provider attestation that the adversarial behavior
// matrix (known-good ×N, no-op, shortcut, forbidden attempt incl. 403,
// evidence-loss, verifier-fault) passed on the exact inputs the case runs
// against. The ledger is VERIFIED, never trusted: every run recomputes its
// input digests and a fully matching, complete entry is the only thing
// that can flip safety.qualified=true — any mismatch demotes to preview.
//
// Digest inputs (design §8): scenario file, fixtures/agent bundle, verifier
// scripts, authority policy, plus a COMPONENT REVISION (git vcs stamp of
// the binary carrying collector/normalizer/verdict-engine code — source
// trees are not shipped inside the harness image, the revision pin is the
// container-portable equivalent) and the PROVIDER version line.
package qualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

// SchemaID is the ledger document schema tag.
const SchemaID = "evidra-qualification-v1"

// FileName is where an entry lives inside a scenario directory.
const FileName = "qualification.json"

// Inputs are the per-run recomputed digests a ledger entry is pinned to.
type Inputs struct {
	Scenario        string `json:"scenario"`           // scenario.yaml bytes
	AuthorityPolicy string `json:"authority_policy"`   // profile doc bytes (or "-" when inline)
	Fixtures        string `json:"fixtures"`           // agent bundle / input files tree
	VerifierScripts string `json:"verifier_scripts"`   // verifier .sh/.go template bytes
	ComponentRevis  string `json:"component_revision"` // git revision(+dirty) of the harness binary
	// Providers are pinned "provider@gitVersion" pairs the case was
	// qualified for; a live run qualifies iff its pair is in the set.
	// (A single pin would make kind and k3d evict each other — the
	// matrix's ProvidersEquivalent flag is what licenses adding legs.)
	Providers []string `json:"providers"`
}

// Equal compares all digest fields.
func (a Inputs) Equal(b Inputs) bool { return reflect.DeepEqual(a, b) }

// FieldNames/Digests pairs for reporting drift.
func (in Inputs) Fields() map[string]string {
	return map[string]string{
		"scenario": in.Scenario, "authority_policy": in.AuthorityPolicy,
		"fixtures": in.Fixtures, "verifier_scripts": in.VerifierScripts,
		"component_revision": in.ComponentRevis,
		"providers":          strings.Join(sortedStrings(in.Providers), ","),
	}
}

// MatrixEvidence records the adversarial behaviors the ledger attests.
type MatrixEvidence struct {
	KnownGoodPasses      int  `json:"known_good_passes"`
	KnownGoodRequired    int  `json:"known_good_required"`
	NoOpFailsOutcome     bool `json:"no_op_fails_outcome"`
	ShortcutViolates     bool `json:"shortcut_violates_preservation"`
	ForbiddenUnsafe      bool `json:"forbidden_attempt_unsafe"`
	Forbidden403Safe     bool `json:"forbidden_403_still_unsafe"`
	EvidenceLossIncomp   bool `json:"evidence_loss_incomplete"`
	VerifierFaultIncomp  bool `json:"verifier_fault_incomplete"`
	ProvidersEquivalent  bool `json:"kind_k3d_equivalent"`
	FlakeBudgetRespected bool `json:"flake_budget_respected"`
}

// Complete reports every attested behavior present + the flake budget met.
func (m MatrixEvidence) Complete() bool {
	return m.KnownGoodRequired > 0 && m.KnownGoodPasses >= m.KnownGoodRequired &&
		m.NoOpFailsOutcome && m.ShortcutViolates && m.ForbiddenUnsafe &&
		m.Forbidden403Safe && m.EvidenceLossIncomp && m.VerifierFaultIncomp &&
		m.ProvidersEquivalent && m.FlakeBudgetRespected
}

// Entry is one ledger document (qualification.json).
type Entry struct {
	Schema     string         `json:"schema"`
	ScenarioID string         `json:"scenario_id"`
	Qualified  bool           `json:"qualified"`
	GrantedAt  time.Time      `json:"granted_at"`
	Inputs     Inputs         `json:"inputs"`
	Matrix     MatrixEvidence `json:"matrix"`
	Notes      string         `json:"notes,omitempty"`
}

// Verdict explains why an entry does or does not authorize the flip.
type Verdict struct {
	Authorized bool
	Missing    bool     // no ledger file at all
	Reasons    []string // field-level drift / incompleteness
}

// Verify checks schema, qualification flag, digest match and matrix
// completeness. It never panics on junk input — junk = not authorized.
func Verify(e *Entry, in Inputs) Verdict {
	v := Verdict{}
	if e == nil {
		v.Missing = true
		v.Reasons = []string{"no " + FileName + " entry for this case"}
		return v
	}
	if e.Schema != SchemaID {
		v.Reasons = append(v.Reasons, "schema "+e.Schema+" != "+SchemaID)
	}
	if !e.Qualified {
		v.Reasons = append(v.Reasons, "entry not flagged qualified")
	}
	if e.ScenarioID == "" {
		v.Reasons = append(v.Reasons, "entry missing scenario id")
	}
	for _, name := range []string{"scenario", "authority_policy", "fixtures", "verifier_scripts", "component_revision"} {
		want := e.Inputs.Fields()[name]
		got := in.Fields()[name]
		if want != got {
			// FULL values in the message: pseudo-versions share 12-char
			// prefixes, and a truncated drift report made two genuinely
			// different revisions print as identical (P10 field bug).
			v.Reasons = append(v.Reasons, fmt.Sprintf("%s digest drift: ledger %s != run %s", name, want, got))
		}
	}
	// Provider pinning is SET MEMBERSHIP, not equality: each equivalence
	// leg recorded (kind first, k3d after a green cross-provider matrix)
	// qualifies its own runs; anything else demotes to preview.
	if !providersCover(e.Inputs.Providers, in.Providers) {
		v.Reasons = append(v.Reasons, fmt.Sprintf(
			"providers: run pin %s not among qualified [%s]",
			strings.Join(in.Providers, "+"), strings.Join(e.Inputs.Providers, ", ")))
	}
	if !e.Matrix.Complete() {
		v.Reasons = append(v.Reasons, "matrix evidence incomplete (behaviors or flake budget)")
	}
	v.Authorized = len(v.Reasons) == 0
	return v
}

// Load reads + parses an entry; os.IsNotExist maps to (nil, nil) so the
// caller can distinguish "absent" from "corrupt".
func Load(scenarioDir string) (*Entry, error) {
	data, err := os.ReadFile(filepath.Join(scenarioDir, FileName))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	return &e, nil
}

// Write atomically stores an entry in the scenario dir.
func Write(scenarioDir string, e *Entry) error {
	if e.Schema == "" {
		e.Schema = SchemaID
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(scenarioDir, FileName+".tmp")
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(scenarioDir, FileName))
}

// ---- digest helpers -------------------------------------------------------

// HashFile digests one file.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	return hashReader(f)
}

// HashTree digests a file tree: sorted relpaths + per-file hashes, so
// content or NAME changes flip the digest, mtime does not. A single plain
// file behaves like HashFile.
func HashTree(root string) (string, error) {
	st, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return HashFile(root)
	}
	var rels []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rels = append(rels, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(rels)
	h := sha256.New()
	for _, rel := range rels {
		fh, err := os.Open(filepath.Join(root, rel))
		if err != nil {
			return "", err
		}
		d, err := hashReader(fh)
		_ = fh.Close()
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(h, "%s\x00%s\x00", filepath.ToSlash(rel), d)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// HashBytes is the in-memory variant.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// BuildRevision identifies the compiled harness: git commit, dirty flag,
// module version. This is the container-portable stand-in for hashing the
// source of collector/normalizer/verdict-engine packages.
// buildRevision is the ONLY value that may pin this binary's source
// revision for the ledger: it is injected at link time
// (-X pkg/qualification.buildRevision=$(git rev-parse HEAD)) and is
// therefore immutable for the life of the artifact. ADR 0001 review #4
// removed the runtime environment override — a binary must not be able to
// talk its way into believing it is some other build. Release packaging
// (Dockerfile.bench ARG, make release) always supplies it.
var buildRevision string

// LinkedRevision returns the compile-time stamp exactly as injected
// (empty when the binary was built ad hoc without it).
func LinkedRevision() string { return buildRevision }

// BuildRevision reports the revision this binary was built from.
func BuildRevision() string {
	if v := strings.TrimSpace(buildRevision); v != "" {
		return v
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	rev, mod, ver := "", "", ""
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			mod = s.Value
		}
	}
	if info.Main.Version != "" {
		ver = info.Main.Version
	}
	if rev == "" {
		return "no-vcs+" + ver
	}
	short := rev
	if len(short) > 12 {
		short = short[:12]
	}
	if mod == "true" {
		short += "-dirty"
	}
	if ver != "" && ver != "(devel)" {
		short += "+" + strings.TrimPrefix(ver, "v")
	}
	return short
}

// ProviderPin assembles the canonical "<provider>@<version>" ledger pin.
// The run side passes a bare gitVersion, P10-era operators passed the
// already-prefixed form; both must land on one shape, or a correctly
// pinned ledger silently gates (field-caught in re-qualification).
func ProviderPin(provider, version string) string {
	v := strings.TrimSpace(version)
	if stripped, ok := strings.CutPrefix(v, provider+"@"); ok {
		v = stripped
	}
	return provider + "@" + v
}

// providersCover: every live pin must be in the recorded set.
func providersCover(recorded, live []string) bool {
	if len(live) == 0 {
		return false
	}
	for _, l := range live {
		found := false
		for _, r := range recorded {
			if r == l {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func sortedStrings(v []string) []string {
	out := append([]string(nil), v...)
	sort.Strings(out)
	return out
}

// ---- multi-provider granting helpers -------------------------------------

// EqualModuloProviders compares everything except the pin set.
func (a Inputs) EqualModuloProviders(b Inputs) bool {
	x, y := a, b
	x.Providers, y.Providers = nil, nil
	return reflect.DeepEqual(x, y)
}

// UnionProviders merges pin sets, sorted and deduplicated.
func UnionProviders(sets ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, set := range sets {
		for _, p := range set {
			if p != "" && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// MergeMatrices unions two attestation legs: boolean evidence persists,
// known-good counts accumulate (each leg ran its own budget).
func MergeMatrices(a, b MatrixEvidence) MatrixEvidence {
	or := func(x, y bool) bool { return x || y }
	return MatrixEvidence{
		KnownGoodPasses:      a.KnownGoodPasses + b.KnownGoodPasses,
		KnownGoodRequired:    max(a.KnownGoodRequired, b.KnownGoodRequired),
		NoOpFailsOutcome:     or(a.NoOpFailsOutcome, b.NoOpFailsOutcome),
		ShortcutViolates:     or(a.ShortcutViolates, b.ShortcutViolates),
		ForbiddenUnsafe:      or(a.ForbiddenUnsafe, b.ForbiddenUnsafe),
		Forbidden403Safe:     or(a.Forbidden403Safe, b.Forbidden403Safe),
		EvidenceLossIncomp:   or(a.EvidenceLossIncomp, b.EvidenceLossIncomp),
		VerifierFaultIncomp:  or(a.VerifierFaultIncomp, b.VerifierFaultIncomp),
		ProvidersEquivalent:  or(a.ProvidersEquivalent, b.ProvidersEquivalent),
		FlakeBudgetRespected: or(a.FlakeBudgetRespected, b.FlakeBudgetRespected),
	}
}
