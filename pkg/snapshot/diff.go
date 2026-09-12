package snapshot

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Violation is one out-of-scope state change detected between two
// checkpoints (ADR 0001 Phase 7: snapshots are authoritative for EFFECTS;
// anything the agent was not granted to touch that nonetheless persistently
// changed is a violation entry attributed to source "snapshot").
type Violation struct {
	Source string `json:"source"` // always "snapshot"
	Object string `json:"object"` // Key string form
	Kind   string `json:"kind"`   // modified | created | deleted
	From   string `json:"from_digest,omitempty"`
	To     string `json:"to_digest,omitempty"`
}

func (v Violation) String() string {
	return fmt.Sprintf("%s %s (%s -> %s)", v.Kind, v.Object, trunc(v.From), trunc(v.To))
}

func trunc(d string) string {
	if len(d) > 12 {
		return d[:12]
	}
	if d == "" {
		return "-"
	}
	return d
}

// Diff compares before/after sets. Allowed reports whether a key may change
// (derived from the authority profile's agent grants). Absent keys count as
// deleted/created. Unreadable targets on either side make the diff
// INCOMPLETE (caller must not claim full coverage).
func Diff(before, after *Set, allowed func(Key) bool) (violations []Violation, allowedChanges int) {
	all := map[Key]bool{}
	for k := range before.Objects {
		all[k] = true
	}
	for k := range after.Objects {
		all[k] = true
	}
	keys := make([]Key, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	for _, k := range keys {
		b, bin := before.Objects[k]
		a, ain := after.Objects[k]
		// Only the WRITABLE surface (spec+metadata) is persistent state:
		// status is controller output. A workload recovering from a
		// crash-loop is not an agent violation; writes that reached the
		// status subresource are still attributed through the audit.
		b, a = comparable(b, bin), comparable(a, ain)
		if bin && ain && string(b) == string(a) {
			continue
		}
		if allowed != nil && allowed(k) {
			allowedChanges++
			continue
		}
		v := Violation{Source: "snapshot", Object: k.String(), Kind: "modified"}
		if !bin {
			v.Kind = "created"
		} else if !ain {
			v.Kind = "deleted"
		}
		if bin {
			v.From = digestOf(b)
		}
		if ain {
			v.To = digestOf(a)
		}
		violations = append(violations, v)
	}
	return violations, allowedChanges
}

// comparable returns the bytes used for persistence comparison: the
// normalized object with its top-level status stripped (absent objects
// compare as absent).
func comparable(data []byte, present bool) []byte {
	if !present || len(data) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return data
	}
	delete(obj, "status")
	out, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return out
}

func digestOf(data []byte) string {
	sum := sha256Short(data)
	return sum
}
