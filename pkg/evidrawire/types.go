// Code copied from samebits.com/evidra pkg/evidence@94f2f72 (external evidence
// bundle v1 protocol surface). Keep byte-identical to upstream except the
// package clause; drift is caught by TestCoreBundleFixtureReproducesHashes.
// Producer spec: docs/EVIDENCE_BUNDLE_EXPORT.md.
package evidrawire

import (
	"errors"
	"fmt"
)

// StoreManifest describes the state of a segmented evidence store.
type StoreManifest struct {
	Format          string   `json:"format"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	SegmentsDir     string   `json:"segments_dir"`
	CurrentSegment  string   `json:"current_segment"`
	SealedSegments  []string `json:"sealed_segments"`
	SegmentMaxBytes int64    `json:"segment_max_bytes"`
	RecordsTotal    int      `json:"records_total"`
	LastHash        string   `json:"last_hash"`
	Notes           string   `json:"notes"`
}

const (
	defaultSegmentMaxBytes int64 = 5_000_000
	manifestFileName             = "manifest.json"
	segmentsDirName              = "segments"
	lockFileName                 = ".evidra.lock"
	defaultLockTimeoutMS         = 2000
)

var ErrChainInvalid = errors.New("evidence_chain_invalid")

const (
	ErrorCodeStoreBusy               = "evidence_store_busy"
	ErrorCodeLockNotSupportedWindows = "evidence_lock_not_supported_on_windows"
)

// StoreError represents an error from the evidence store with an error code.
type StoreError struct {
	Code    string
	Message string
	Err     error
}

func (e *StoreError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *StoreError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ErrorCode extracts the error code from a StoreError, or returns empty string.
func ErrorCode(err error) string {
	var se *StoreError
	if errors.As(err, &se) {
		return se.Code
	}
	return ""
}

// ChainValidationError describes a hash-chain validation failure at a specific record index.
type ChainValidationError struct {
	Index   int
	EventID string
	Message string
}

func (e *ChainValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.EventID != "" {
		return fmt.Sprintf("record %d (%s): %s", e.Index, e.EventID, e.Message)
	}
	return fmt.Sprintf("record %d: %s", e.Index, e.Message)
}
