package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vitas/evidra-bench/pkg/qualification"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

// ComputeQualification assembles the per-run ledger inputs, loads the
// scenario's qualification.json (if any) and returns the verification
// verdict. Runs ONLY when an authority profile exists: without one the
// case has no qualified path anyway, and we avoid pointless hashing + a
// kubectl round-trip on every legacy run.
func ComputeQualification(ctx context.Context, s *scenario.Scenario, adminKubeconfig, provider, providerVersion string) (*qualification.Verdict, qualification.Inputs) {
	absent := &qualification.Verdict{Missing: true, Reasons: []string{"no authority profile"}}
	if s == nil || s.AuthorityProfile == nil {
		return absent, qualification.Inputs{}
	}
	scenarioFile := scenarioFileFor(s)
	in := qualification.Inputs{
		Scenario:       mustHash(qualification.HashFile(scenarioFile)),
		Fixtures:       hashFixtures(s),
		ComponentRevis: qualification.BuildRevision(),
		Providers:      []string{provider + "@" + versionOrProbe(ctx, adminKubeconfig, providerVersion)},
	}
	if profileYAML, err := yaml.Marshal(s.AuthorityProfile); err == nil {
		in.AuthorityPolicy = qualification.HashBytes(profileYAML)
	} else {
		in.AuthorityPolicy = "unmarshal-error"
	}
	// The verifier contract is compiled into the harness binary (assert-v2
	// templates); the component revision pins it. Scenario check params
	// live in the scenario/fixture hashes.
	in.VerifierScripts = in.ComponentRevis
	entry, err := qualification.Load(s.Dir)
	if err != nil {
		return &qualification.Verdict{Reasons: []string{"corrupt ledger: " + err.Error()}}, in
	}
	v := qualification.Verify(entry, in)
	return &v, in
}

// hashFixtures digests the case data on disk: the scenario directory tree
// minus its yaml and any ledger itself (both carry their own pins). The
// agent bundle is deliberately NOT part of fixtures — the bundle is the
// component UNDER test; pinning it would make every behavior-matrix run
// (and every real user agent) demote its own cases.
func hashFixtures(s *scenario.Scenario) string {
	var parts []string
	extras := []string{qualification.FileName, scenarioFileFor(s)}
	if tree, err := hashTreeExcept(s.Dir, extras...); err == nil {
		parts = append(parts, tree)
	}
	if len(parts) == 0 {
		return "sha256:none"
	}
	return qualification.HashBytes([]byte(joinSemi(parts[0], parts[1:]...)))
}

func hashTreeExcept(root string, except ...string) (string, error) {
	tmp, err := os.MkdirTemp("", "evidra-hash-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	skip := map[string]bool{}
	for _, e := range except {
		// Entries may be bare file names rooted at the walk dir or full
		// paths; resolve BOTH to absolute so the exclusion actually hits.
		for _, cand := range []string{e, filepath.Join(root, e)} {
			abs, err := filepath.Abs(cand)
			if err == nil {
				skip[abs] = true
			}
		}
	}
	skipRoot := tmp
	err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		abs, _ := filepath.Abs(p)
		if skip[abs] {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		dest := filepath.Join(skipRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o600)
	})
	if err != nil {
		return "", err
	}
	return qualification.HashTree(skipRoot)
}

func versionOrProbe(ctx context.Context, kubeconfig, explicit string) string {
	if explicit != "" {
		return explicit
	}
	return serverGitVersion(ctx, kubeconfig)
}

// serverGitVersion asks the live cluster what it is (provider version input
// to the ledger). Best-effort: failure yields "unknown", which simply won't
// match any ledger and demotes to preview.
func serverGitVersion(ctx context.Context, kubeconfig string) string {
	if kubeconfig == "" {
		return "unknown"
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	//nolint:gosec // fixed argv.
	cmd := exec.CommandContext(cctx, "kubectl", "--kubeconfig", kubeconfig, "version", "-o", "json")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "unknown"
	}
	var doc struct {
		ServerVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"serverVersion"`
	}
	if json.Unmarshal(out.Bytes(), &doc) != nil || doc.ServerVersion.GitVersion == "" {
		return "unknown"
	}
	return doc.ServerVersion.GitVersion
}

func mustHash(h string, err error) string {
	if err != nil {
		return fmt.Sprintf("hash-error:%v", err)
	}
	return h
}

func ledgerEventKind(v *qualification.Verdict) string {
	switch {
	case v == nil:
		return "skipped"
	case v.Authorized:
		return "authorized"
	case v.Missing:
		return "no-ledger"
	default:
		return "demoted"
	}
}

func ledgerEventDetail(v *qualification.Verdict, in qualification.Inputs) string {
	if v == nil {
		return ""
	}
	// Always name the live pins: `bench-cli qualify record` on a second
	// provider leg must pass EXACTLY these strings, and operators
	// diagnosing a demotion need to see what the run thought it was on.
	detail := "providers=" + strings.Join(in.Providers, "+")
	if v.Authorized {
		return detail + "; revision=" + in.ComponentRevis
	}
	return detail + "; " + joinSemi("", v.Reasons...)
}

// scenarioFileFor: scenario.Path is a SUITE-RELATIVE ref after
// scenario.Resolve (not a filesystem path!) while loaders that go
// straight through scenario.Load leave Path empty. The ledger must
// derive the same digest from either provenance — Dir is canonical on
// both ends (repo checkout on the host, /opt/evidra in the image).
func scenarioFileFor(s *scenario.Scenario) string {
	if s.Dir != "" {
		return filepath.Join(s.Dir, "scenario.yaml")
	}
	return s.Path
}
