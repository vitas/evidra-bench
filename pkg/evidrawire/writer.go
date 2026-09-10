package evidrawire

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Manifest and segment constants mirror the core segmented store layout
// (pkg/evidence manifest.go / segment.go) as of the pinned commit above.
// manifestFileName, segmentsDirName, and defaultSegmentMaxBytes come from the
// copied types.go.
const (
	manifestFormat      = "evidra-evidence-manifest-v0.1"
	defaultSegmentName  = "evidence-000001.jsonl"
	writerSpecVersion   = "v1.1.0" // core pkg/version.SpecVersion at the pinned commit
	writerCanonVersion  = "v1"
	writerAdapterPrefix = "evidra-bench"
)

// BundleWriter emits a single-segment external evidence bundle. It performs no
// locking and no segment rotation: bundles are complete-at-export artifacts
// sized far below the segment limit. Entries must be appended in chain order;
// the writer tracks previous_hash automatically.
type BundleWriter struct {
	root      string
	sessionID string
	traceID   string
	adapter   string
	signer    *EphemeralSigner

	semantics string
	prev      string
	count     int
	segPath   string
	segFile   *os.File
	encoder   *json.Encoder
	created   time.Time
}

// NewBundleWriter creates the bundle directory tree under root (which must not
// exist yet) and prepares it for appends. sessionID/traceID group every entry;
// adapter names the producing run context (e.g. "evidra-bench/kubernetes").
// BundleWriterOption tunes the produced bundle.json (Phase 11 cohort stamp).
type BundleWriterOption func(*BundleWriter)

// WithSemanticsVersion stamps the result-semantics cohort into bundle.json.
func WithSemanticsVersion(v string) BundleWriterOption {
	return func(w *BundleWriter) { w.semantics = strings.TrimSpace(v) }
}

func NewBundleWriter(root, sessionID, traceID, adapter, producerVersion string, signer *EphemeralSigner, opts ...BundleWriterOption) (*BundleWriter, error) {
	if sessionID == "" || traceID == "" {
		return nil, fmt.Errorf("evidrawire.NewBundleWriter: sessionID and traceID are required")
	}
	if _, err := os.Stat(BundleManifestPath(root)); !os.IsNotExist(err) {
		return nil, fmt.Errorf("evidrawire.NewBundleWriter: %s already exists", BundleFileName)
	}
	if err := os.MkdirAll(filepath.Join(root, segmentsDirName), 0o755); err != nil {
		return nil, fmt.Errorf("evidrawire.NewBundleWriter: mkdir: %w", err)
	}
	segPath := filepath.Join(root, segmentsDirName, defaultSegmentName)
	f, err := os.Create(segPath)
	if err != nil {
		return nil, fmt.Errorf("evidrawire.NewBundleWriter: create segment: %w", err)
	}
	now := time.Now().UTC()
	w := &BundleWriter{
		root:      root,
		sessionID: sessionID,
		traceID:   traceID,
		adapter:   adapter,
		signer:    signer,
		segPath:   segPath,
		segFile:   f,
		encoder:   json.NewEncoder(f),
		created:   now,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w, nil
}

// Append builds one entry from params (Type, Actor, Payload are required;
// PreviousHash and chain-level defaults are owned by the writer) and writes it
// to the segment.
func (w *BundleWriter) Append(params EntryBuildParams) (EvidenceEntry, error) {
	if params.SessionID == "" {
		params.SessionID = w.sessionID
	}
	if params.TraceID == "" {
		params.TraceID = w.traceID
	}
	if params.SpecVersion == "" {
		params.SpecVersion = writerSpecVersion
	}
	if params.AdapterVersion == "" {
		params.AdapterVersion = w.adapter
	}
	if params.CanonVersion == "" {
		params.CanonVersion = writerCanonVersion
	}
	params.PreviousHash = w.prev
	params.Signer = w.signer

	entry, err := BuildEntry(params)
	if err != nil {
		return EvidenceEntry{}, fmt.Errorf("evidrawire.BundleWriter.Append: %w", err)
	}
	if err := w.encoder.Encode(entry); err != nil {
		return EvidenceEntry{}, fmt.Errorf("evidrawire.BundleWriter.Append: write: %w", err)
	}
	w.prev = entry.Hash
	w.count++
	return entry, nil
}

// Close finalizes the store: writes manifest.json and bundle.json. The
// ephemeral signer key becomes unreproducible afterwards by design.
func (w *BundleWriter) Close(producer BundleProducer, notes string) error {
	if w.segFile != nil {
		if err := w.segFile.Close(); err != nil {
			return fmt.Errorf("evidrawire.BundleWriter.Close: %w", err)
		}
		w.segFile = nil
	}
	info, err := os.Stat(w.segPath)
	if err != nil {
		return fmt.Errorf("evidrawire.BundleWriter.Close: stat segment: %w", err)
	}
	if info.Size() > defaultSegmentMaxBytes {
		return fmt.Errorf("evidrawire.BundleWriter.Close: segment exceeds %d bytes; rotation is not implemented", defaultSegmentMaxBytes)
	}
	m := StoreManifest{
		Format:          manifestFormat,
		CreatedAt:       w.created.Format(time.RFC3339),
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
		SegmentsDir:     segmentsDirName,
		CurrentSegment:  defaultSegmentName,
		SealedSegments:  []string{},
		SegmentMaxBytes: defaultSegmentMaxBytes,
		RecordsTotal:    w.count,
		LastHash:        w.prev,
	}
	data, err := json.MarshalIndent(&m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(w.root, manifestFileName), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return SaveBundleManifest(w.root, BundleManifest{
		Spec:             BundleSpecV1,
		Producer:         producer,
		TrustLevel:       TrustEphemeral,
		PublicKey:        w.signer.PublicKeyBase64(),
		CreatedAt:        w.created.Format(time.RFC3339),
		Notes:            notes,
		SemanticsVersion: w.semantics,
	})
}
