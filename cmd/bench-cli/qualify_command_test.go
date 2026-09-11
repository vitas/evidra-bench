package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/qualification"
)

// copyStarterCase stages a throwaway copy of a starter scenario: qualify
// tests must NEVER mutate (or delete) the committed ledgers under
// scenarios/ — an earlier revision of this file wiped a granted
// qualification.json from the working tree.
func copyStarterCase(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("..", "..", "scenarios", "kubernetes", name)
	if _, err := os.Stat(filepath.Join(src, "scenario.yaml")); err != nil {
		t.Skip("starter scenario not present")
	}
	dst := filepath.Join(t.TempDir(), name)
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func TestQualifyVerifyRecordRoundTrip(t *testing.T) {
	dir := copyStarterCase(t, "broken-deployment")
	if err := os.Remove(filepath.Join(dir, qualification.FileName)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	run := func(args ...string) (string, error) {
		cmd := newQualifyCommand()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs(args)
		err := cmd.ExecuteContext(context.Background())
		return buf.String(), err
	}

	// Absent ledger: verify must refuse (non-nil sentinel error).
	out, err := run("verify", "--scenario-dir", dir, "--provider", "kind", "--provider-version", "v1.31.2-test")
	if err == nil {
		t.Fatalf("expected refusal without ledger; out=%s", out)
	}
	if !strings.Contains(err.Error(), "preview") {
		t.Fatalf("sentinel wrong: %v", err)
	}

	// Record with an incomplete matrix: writes but warns; verify still refuses.
	_, err = run("record", "--scenario-dir", dir, "--provider", "kind", "--provider-version", "v1.31.2-test", "--known-good-passes", "1")
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, err := run("verify", "--scenario-dir", dir, "--provider", "kind", "--provider-version", "v1.31.2-test"); err == nil {
		t.Fatal("incomplete matrix must not authorize")
	}

	// Full attestation through --matrix-file authorizes (same inputs).
	matrix := qualification.MatrixEvidence{
		KnownGoodPasses: 5, KnownGoodRequired: 5, NoOpFailsOutcome: true,
		ShortcutViolates: true, ForbiddenUnsafe: true, Forbidden403Safe: true,
		EvidenceLossIncomp: true, VerifierFaultIncomp: true,
		ProvidersEquivalent: true, FlakeBudgetRespected: true,
	}
	mf := filepath.Join(t.TempDir(), "matrix.json")
	data, _ := json.Marshal(matrix)
	if err := os.WriteFile(mf, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, qualification.FileName)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := run("record", "--scenario-dir", dir, "--provider", "kind", "--provider-version", "v1.31.2-test", "--matrix-file", mf); err != nil {
		t.Fatalf("full record: %v", err)
	}
	if _, err := run("verify", "--scenario-dir", dir, "--provider", "kind", "--provider-version", "v1.31.2-test"); err != nil {
		t.Fatalf("verify after full grant: %v", err)
	}
}

func TestQualifyRecordUnionsSecondLeg(t *testing.T) {
	dir := copyStarterCase(t, "broken-deployment")
	if err := os.Remove(filepath.Join(dir, qualification.FileName)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	run := func(args ...string) error {
		cmd := newQualifyCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		return cmd.ExecuteContext(context.Background())
	}
	matrix := qualification.MatrixEvidence{
		KnownGoodPasses: 15, KnownGoodRequired: 5, NoOpFailsOutcome: true,
		ShortcutViolates: true, ForbiddenUnsafe: true, Forbidden403Safe: true,
		EvidenceLossIncomp: true, VerifierFaultIncomp: true,
		ProvidersEquivalent: true, FlakeBudgetRespected: true,
	}
	mf := filepath.Join(t.TempDir(), "matrix.json")
	data, _ := json.Marshal(matrix)
	if err := os.WriteFile(mf, data, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := run("record", "--scenario-dir", dir, "--provider", "kind", "--provider-version", "vA", "--matrix-file", mf); err != nil {
		t.Fatalf("leg 1: %v", err)
	}
	if err := run("record", "--scenario-dir", dir, "--provider", "k3d", "--provider-version", "vB", "--matrix-file", mf); err != nil {
		t.Fatalf("leg 2: %v", err)
	}
	if err := run("verify", "--scenario-dir", dir, "--provider", "kind", "--provider-version", "vA"); err != nil {
		t.Fatalf("kind leg must stay authorized: %v", err)
	}
	if err := run("verify", "--scenario-dir", dir, "--provider", "k3d", "--provider-version", "vB"); err != nil {
		t.Fatalf("k3d leg must authorize: %v", err)
	}
	if err := run("verify", "--scenario-dir", dir, "--provider", "azure", "--provider-version", "vA"); err == nil {
		t.Fatal("ungranted provider must demote")
	}
	// Re-recording an authorized leg is a no-op refusal, not a clobber.
	if err := run("record", "--scenario-dir", dir, "--provider", "k3d", "--provider-version", "vB", "--matrix-file", mf); err == nil ||
		!strings.Contains(err.Error(), "nothing to do") {
		t.Fatalf("authorized re-record must refuse gently: %v", err)
	}
	e, err := qualification.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Inputs.Providers) != 2 || e.Matrix.KnownGoodPasses != 30 {
		t.Fatalf("union expected (2 pins, 30 passes), got %v / %+v", e.Inputs.Providers, e.Matrix)
	}
}
