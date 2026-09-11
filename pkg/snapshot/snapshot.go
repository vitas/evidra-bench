// Package snapshot implements the state-snapshot evidence layer (ADR 0001
// Phase 7): checkpointed, normalized reads of the cluster state a scenario
// claims to touch, taken through the EVIDENCE-READER identity. Snapshots are
// authoritative for EFFECTS: what actually changed in the cluster (as
// opposed to what was attempted, which audit answers).
//
// Discipline (docs/plans/2026-09-09-safety-evidence-foundation-design.md §2/§5):
//   - normalized digests must be byte-stable on a quiesced cluster:
//     volatile fields (resourceVersion, managedFields, leader annotations,
//     timestamps...) are stripped before hashing;
//   - persisted snapshots pass the same redaction pipeline as audit
//     (Secret payloads -> digests, SA token data scrubbed, size caps);
//   - raw (unredacted) snapshot JSON never leaves memory; artifacts always
//     carry the redacted form + the digest of the normalized form.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Target is one namespaced resource kind to read.
type Target struct {
	Namespace string
	Resource  string // kubectl resource name, e.g. "deployments"
	Kind      string // API kind used when items omit their own, e.g. "Deployment"
}

// Key identifies an object inside a snapshot set.
type Key struct {
	Kind      string
	Namespace string
	Name      string
}

func (k Key) String() string { return k.Kind + "/" + k.Namespace + "/" + k.Name }

// ListFunc fetches one target and returns the parsed LIST document.
type ListFunc func(target Target) (map[string]any, error)

// Set is one checkpoint: object key -> normalized JSON bytes.
type Set struct {
	Name       string
	Objects    map[Key][]byte
	Unreadable []string // targets that failed to read (drives coverage)
}

// NewSet builds a normalized snapshot set from a lister over targets.
func NewSet(name string, targets []Target, list ListFunc) *Set {
	s := &Set{Name: name, Objects: map[Key][]byte{}}
	for _, t := range targets {
		doc, err := list(t)
		if err != nil {
			s.Unreadable = append(s.Unreadable, fmt.Sprintf("%s/%s: %v", t.Namespace, t.Resource, err))
			continue
		}
		items, _ := doc["items"].([]any)
		kind := t.Kind
		for _, raw := range items {
			obj, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			key, data := NormalizeObject(obj, kind, t.Namespace)
			s.Objects[key] = data
		}
	}
	return s
}

// volatile metadata fields stripped from every object.
var volatileTop = []string{"resourceVersion", "uid", "creationTimestamp", "deletionTimestamp", "deletionGracePeriodSeconds", "managedFields", "selfLink", "generation"}

var volatileAnnotations = []string{
	"control-plane.alpha.kubernetes.io/leader",
	"kubectl.kubernetes.io/last-applied-configuration",
	"leader.election.kubernetes.io/holder",
}

// NormalizeObject strips volatile fields and redacts payloads, returning the
// canonical key and compact JSON bytes.
func NormalizeObject(obj map[string]any, fallbackKind, fallbackNS string) (Key, []byte) {
	meta, _ := obj["metadata"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
	}
	kind, _ := obj["kind"].(string)
	if kind == "" {
		kind = fallbackKind
	}
	name, _ := meta["name"].(string)
	ns, _ := meta["namespace"].(string)
	if ns == "" {
		ns = fallbackNS
	}
	for _, f := range volatileTop {
		delete(meta, f)
	}
	if anns, ok := meta["annotations"].(map[string]any); ok {
		for _, a := range volatileAnnotations {
			delete(anns, a)
		}
		if len(anns) == 0 {
			delete(meta, "annotations")
		}
	}
	if status, ok := obj["status"].(map[string]any); ok {
		stripVolatile(status)
	}
	secretRedact(kind, obj)
	obj["metadata"] = meta
	data, err := marshalCanonical(obj)
	if err != nil {
		data = []byte("{}")
	}
	return Key{Kind: kind, Namespace: ns, Name: name}, data
}

// stripVolatile recursively removes timestamp-like fields from status maps.
func stripVolatile(m map[string]any) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			if isVolatileKey(k) && looksLikeTimestamp(val) {
				delete(m, k)
			}
			if k == "resourceVersion" || k == "controllerRevisionHash" {
				delete(m, k)
			}
		case map[string]any:
			stripVolatile(val)
		case []any:
			for _, item := range val {
				if im, ok := item.(map[string]any); ok {
					stripVolatile(im)
				}
			}
		}
	}
}

func isVolatileKey(k string) bool {
	for _, s := range []string{"lastTransitionTime", "lastProbeTime", "lastUpdateTime", "startTime", "finishTime", "transitionTime", "lastScheduleTime", "lastSyncTime", "initializationTime", "lastTimestamp", "firstTimestamp", "eventTime"} {
		if k == s {
			return true
		}
	}
	return false
}

func looksLikeTimestamp(v string) bool {
	// RFC3339-ish: contains 'T' and ends with 'Z' or offset, with '-' in date part.
	return len(v) >= 20 && strings.Contains(v, "T") && (strings.HasSuffix(v, "Z") || strings.Count(v, ":") >= 2)
}

// secretRedact replaces Secret payloads with digests (user decision:
// snapshots are redacted too) — the DIGEST keeps change detection intact
// without persisting content.
func secretRedact(kind string, obj map[string]any) {
	if !strings.EqualFold(kind, "Secret") {
		return
	}
	for _, field := range []string{"data", "stringData"} {
		val, ok := obj[field].(map[string]any)
		if !ok {
			continue
		}
		sum := sha256.New()
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			_, _ = fmt.Fprintf(sum, "%s=%v\n", k, val[k])
		}
		obj[field] = map[string]any{"evidra:digest": hex.EncodeToString(sum.Sum(nil))}
	}
}

// marshalCanonical serializes with sorted keys (encoding/json maps already
// marshal key-sorted) and compact separators — byte-stable output.
func marshalCanonical(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Digest hashes the whole set (name-ordered) into one hex digest.
func (s *Set) Digest() string {
	keys := make([]Key, 0, len(s.Objects))
	for k := range s.Objects {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	h := sha256.New()
	for _, k := range keys {
		_, _ = fmt.Fprintf(h, "%s\n", k)
		h.Write(s.Objects[k])
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Empty reports a snapshot set that captured nothing readable.
func (s *Set) Empty() bool { return len(s.Objects) == 0 }
