package evidrawire

import (
	"bufio"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// verify.go is a clean-room reimplementation of the core chain/signature
// checks (pkg/evidence ValidateChainAtPath + validateEntrySignatures) that
// operates on a single-segment bundle without locking. It exists so the
// exporter can self-verify and so TestCoreBundleFixtureReproducesHashes can
// validate upstream fixtures offline. Core-side validation of the same
// fixture remains the authoritative conformance gate in the evidra repo.

// VerifyBundle checks bundle spec, hash chain, per-entry hashes, and
// signatures against the embedded public key.
func VerifyBundle(root string) (BundleManifest, error) {
	m, found, err := LoadBundleManifest(root)
	if err != nil {
		return BundleManifest{}, err
	}
	if !found {
		return BundleManifest{}, fmt.Errorf("evidrawire.VerifyBundle: %s not found in %s (not an external bundle)", BundleFileName, root)
	}
	if m.Spec != BundleSpecV1 {
		return m, fmt.Errorf("evidrawire.VerifyBundle: unsupported bundle spec %q (want %s)", m.Spec, BundleSpecV1)
	}
	pub, err := m.PublicKeyBytes()
	if err != nil {
		return m, err
	}
	entries, err := readSegmentEntries(root)
	if err != nil {
		return m, err
	}
	if err := verifyEntries(entries, pub); err != nil {
		return m, fmt.Errorf("evidrawire.VerifyBundle: %w", err)
	}
	return m, nil
}

func readSegmentEntries(root string) ([]EvidenceEntry, error) {
	segDir := filepath.Join(root, segmentsDirName)
	files, err := filepath.Glob(filepath.Join(segDir, "evidence-*.jsonl"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("evidrawire: no segments found under %s", segDir)
	}
	if len(files) > 1 {
		return nil, fmt.Errorf("evidrawire: %d segments found; the verifier expects a single-segment bundle", len(files))
	}
	f, err := os.Open(files[0])
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []EvidenceEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e EvidenceEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("evidrawire: parse entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, sc.Err()
}

func verifyEntries(entries []EvidenceEntry, pub ed25519.PublicKey) error {
	if len(entries) == 0 {
		return fmt.Errorf("bundle contains no entries")
	}
	for i, e := range entries {
		if i == 0 {
			if e.PreviousHash != "" {
				return fmt.Errorf("record %d (%s): first entry must have empty previous_hash", i, e.EntryID)
			}
		} else if e.PreviousHash != entries[i-1].Hash {
			return fmt.Errorf("record %d (%s): previous_hash mismatch", i, e.EntryID)
		}
		recomputed, err := computeEntryHash(e)
		if err != nil {
			return fmt.Errorf("record %d (%s): hash: %w", i, e.EntryID, err)
		}
		if e.Hash != recomputed {
			return fmt.Errorf("record %d (%s): hash mismatch: stored %s, computed %s", i, e.EntryID, e.Hash, recomputed)
		}
		sig, err := base64.StdEncoding.DecodeString(e.Signature)
		if err != nil {
			return fmt.Errorf("record %d (%s): invalid base64 signature: %w", i, e.EntryID, err)
		}
		if !ed25519.Verify(pub, []byte(e.Hash), sig) {
			return fmt.Errorf("record %d (%s): signature verification failed", i, e.EntryID)
		}
	}
	return nil
}
