package evidrawire

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCoreBundleFixtureReproducesHashes is the anti-drift guarantee for the
// copied protocol surface: every entry of the upstream conformance fixture
// (tests/external_bundle_v1 from samebits.com/evidra, byte-copied here) must
// re-hash to its stored value and verify against its embedded public key with
// THIS package's code. If any copied file drifts from upstream semantics,
// hashes stop matching and this test fails.
func TestCoreBundleFixtureReproducesHashes(t *testing.T) {
	root := filepath.Join("testdata", "core_bundle_v1")

	m, found, err := LoadBundleManifest(root)
	if err != nil || !found {
		t.Fatalf("LoadBundleManifest: found=%v err=%v", found, err)
	}
	pub, err := m.PublicKeyBytes()
	if err != nil {
		t.Fatalf("PublicKeyBytes: %v", err)
	}

	entries, err := readSegmentEntries(root)
	if err != nil {
		t.Fatalf("readSegmentEntries: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("fixture entries = %d, want 4", len(entries))
	}
	if entries[0].PreviousHash != "" {
		t.Fatal("fixture genesis entry must have empty previous_hash")
	}

	for i, e := range entries {
		got, err := computeEntryHash(e)
		if err != nil {
			t.Fatalf("entry %d: computeEntryHash: %v", i, err)
		}
		if got != e.Hash {
			t.Fatalf("entry %d: copied hash logic drifted from upstream:\n stored %s\n recomputed %s", i, e.Hash, got)
		}
		if i > 0 && e.PreviousHash != entries[i-1].Hash {
			t.Fatalf("entry %d: chain link broken", i)
		}
	}
	if err := verifyEntries(entries, pub); err != nil {
		t.Fatalf("verifyEntries: %v", err)
	}

	// Manifest format must match what the core store expects.
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var sm StoreManifest
	if err := json.Unmarshal(raw, &sm); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if sm.Format != manifestFormat {
		t.Fatalf("manifest format = %q, want %q", sm.Format, manifestFormat)
	}
	if sm.RecordsTotal != len(entries) || sm.LastHash != entries[len(entries)-1].Hash {
		t.Fatalf("manifest totals drifted: %+v", sm)
	}
}

// TestWriterRoundTrip verifies the produced bundle passes the same checks the
// core applies: chain links, hash recomputation, signature, manifests.
func TestWriterRoundTrip(t *testing.T) {
	root := filepath.Join(t.TempDir(), "bundle")
	signer, err := NewEphemeralSigner()
	if err != nil {
		t.Fatalf("NewEphemeralSigner: %v", err)
	}
	w, err := NewBundleWriter(root, "01TESTSESSION", "01TESTTRACE", "evidra-bench/test", "test", signer)
	if err != nil {
		t.Fatalf("NewBundleWriter: %v", err)
	}
	actor := Actor{Type: "agent", ID: "tester", Provenance: "evidra-bench"}

	payload := json.RawMessage(`{"report_id":"r1","prescription_id":"p1","exit_code":0,"verdict":"success"}`)
	if _, err := w.Append(EntryBuildParams{Type: EntryTypeReport, Actor: actor, Payload: payload}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if _, err := w.Append(EntryBuildParams{Type: EntryTypeSessionEnd, Actor: actor,
		Payload: json.RawMessage(`{"status":"completed"}`)}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(BundleProducer{Name: "evidra-bench", Version: "test"}, "unit test"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	m, err := VerifyBundle(root)
	if err != nil {
		t.Fatalf("VerifyBundle: %v", err)
	}
	if m.Producer.Name != "evidra-bench" || m.TrustLevel != TrustEphemeral {
		t.Fatalf("unexpected manifest: %+v", m)
	}

	// Tamper detection.
	seg := filepath.Join(root, segmentsDirName, defaultSegmentName)
	raw, err := os.ReadFile(seg)
	if err != nil {
		t.Fatal(err)
	}
	tampered := []byte(string(raw[0 : len(raw)/2])) // truncate mid-record
	if err := os.WriteFile(seg, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyBundle(root); err == nil {
		t.Fatal("tampered bundle passed verification")
	}
}
