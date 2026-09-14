package audit

import (
	"encoding/json"
	"strconv"
	"strings"
)

// This file implements field-level transition matching over audit request
// bodies (authority-profile `forbidden_changes`). The audit body is the
// evidence: a patch that removes a field, or sets it to a forbidden value,
// indicts even when a later patch restores the object - transient hostile
// edits are exactly what final-state checks cannot see (owner review P0-2).
//
// Supported request-body shapes are the ones kubectl emits and the arms
// use: JSONPatch arrays ({"op":..,"path":..,"value":..}) and merge /
// strategic-merge objects (lists merged by their "name" key).

// FieldSegment is one step of a dotted field path; Pred is set for list
// steps written containers[name=api] and matches the "name" key of a list
// entry (JSONPatch index steps normalize onto the same "*" identity).
type FieldSegment struct {
	Key  string
	Pred string // "" = unrestricted step (any list element / any name)
}

// ParseFieldPath splits "spec.template.spec.containers[name=api].readinessProbe".
func ParseFieldPath(field string) []FieldSegment {
	var segs []FieldSegment
	for _, part := range strings.Split(field, ".") {
		if part == "" {
			continue
		}
		key := part
		pred := ""
		if i := strings.IndexByte(part, '['); i >= 0 && strings.HasSuffix(part, "]") {
			key = part[:i]
			expr := part[i+1 : len(part)-1]
			if kv := strings.SplitN(expr, "=", 2); len(kv) == 2 && kv[0] == "name" {
				pred = kv[1]
			}
		}
		segs = append(segs, FieldSegment{Key: key, Pred: pred})
	}
	return segs
}

// rulePath yields the normalized form of a parsed path. withIndex=true
// models JSONPointer: a predicated list step occupies TWO positions
// (list key + index), indices normalize to "*". withIndex=false models
// merge-object walking, where the selector happens inside the list.
func rulePath(segs []FieldSegment, withIndex bool) []string {
	out := make([]string, 0, len(segs)+2)
	for _, s := range segs {
		out = append(out, s.Key)
		if withIndex && s.Pred != "" {
			out = append(out, "*")
		}
	}
	return out
}

// normalizePointer unescapes an RFC6901 path segment list and folds numeric
// array indices to "*".
func normalizePointer(p string) []string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return nil
	}
	raw := strings.Split(p, "/")
	out := make([]string, 0, len(raw))
	for _, seg := range raw {
		seg = strings.ReplaceAll(seg, "~1", "/")
		seg = strings.ReplaceAll(seg, "~0", "~")
		if _, err := strconv.Atoi(seg); err == nil {
			seg = "*"
		}
		out = append(out, seg)
	}
	return out
}

// ForbiddenChangeMatches reports whether body (an audit requestObject for a
// patch/update on a matching object) performs the declared transition on
// field. change is "removed" or "set" (value compared with JSON
// semantics).
//
// fullReplace marks verb=update (PUT): the body is the ENTIRE object, so a
// forbidden field that is simply ABSENT from it is deleted from the live
// object - omission counts as removal (owner review round-3: a plain
// update must not bypass field rules). Merge/strategic-merge patches keep
// the opposite semantics: absent means "unchanged".
//
// The second result reports whether the body could be interpreted at all:
// empty, non-JSON, or unshape-matching bodies are UNVERIFIABLE, never
// silently "no match" - callers must degrade to INCOMPLETE, not PASS
// (owner review round-3: a missing requestObject must not launder a
// dangerous mutation).
func ForbiddenChangeMatches(body json.RawMessage, field, change string, value any, fullReplace bool) (matched, ok bool) {
	segs := ParseFieldPath(field)
	if len(segs) == 0 {
		return false, false
	}
	trimmed := bytes0(body)
	switch {
	case strings.HasPrefix(trimmed, "["):
		var ops []jsonPatchOp
		if err := json.Unmarshal(body, &ops); err != nil {
			return false, false
		}
		return jsonPatchMatches(body, segs, change, value), true
	case strings.HasPrefix(trimmed, "{"):
		var root any
		if err := json.Unmarshal(body, &root); err != nil {
			return false, false
		}
		if _, isMap := root.(map[string]any); !isMap {
			return false, false
		}
		if mergeMatches(body, segs, change, value) {
			return true, true
		}
		if fullReplace && change == "removed" && mergeAbsent(body, segs) {
			return true, true
		}
		return false, true
	}
	return false, false
}

// mergeAbsent reports that the field path does not terminate in the body:
// an object key, predicated list entry, or any list element is missing at
// some level. Only meaningful for full-replace bodies, where absence IS
// deletion.
func mergeAbsent(body json.RawMessage, segs []FieldSegment) bool {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return false
	}
	return absentWalk([]any{root}, segs, 0)
}

func absentWalk(nodes []any, segs []FieldSegment, i int) bool {
	if i == len(segs) {
		return false // the path terminated: present, not absent
	}
	seg := segs[i]
	anyMap := false
	for _, n := range nodes {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		anyMap = true
		v, present := m[seg.Key]
		if !present {
			return true
		}
		switch child := v.(type) {
		case map[string]any:
			if absentWalk([]any{child}, segs, i+1) {
				return true
			}
		case []any:
			var cand []any
			for _, e := range child {
				em, isMap := e.(map[string]any)
				if !isMap {
					continue
				}
				if seg.Pred != "" {
					if name, _ := em["name"].(string); name != seg.Pred {
						continue
					}
				}
				cand = append(cand, em)
			}
			if len(cand) == 0 {
				return true // predicated (or any) entry vanished
			}
			if absentWalk(cand, segs, i+1) {
				return true
			}
		default:
			// scalar before the path ends: nothing to descend into
			return i+1 < len(segs)
		}
	}
	return !anyMap && i < len(segs)
}

func bytes0(b json.RawMessage) string { return strings.TrimSpace(string(b)) }

type jsonPatchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

func jsonPatchMatches(body json.RawMessage, segs []FieldSegment, change string, value any) bool {
	var ops []jsonPatchOp
	if err := json.Unmarshal(body, &ops); err != nil {
		return false
	}
	rp := rulePath(segs, true)
	for _, o := range ops {
		np := normalizePointer(o.Path)
		if len(np) == 0 {
			continue
		}
		switch change {
		case "removed":
			if o.Op != "remove" {
				continue
			}
			// Removing the field itself or any ancestor of it removes it.
			if len(np) <= len(rp) && equalPrefix(rp, np) {
				return true
			}
		case "set":
			if o.Op != "replace" && o.Op != "add" {
				continue
			}
			if len(np) == len(rp) && equalPrefix(rp, np) && jsonValueEquals(o.Value, value) {
				return true
			}
			// Ancestor write: the patch replaces a PARENT of the rule
			// field; the forbidden value may ride inside the new subtree
			// (e.g. replacing securityContext wholesale).
			if len(np) < len(rp) && equalPrefix(rp[:len(np)], np) {
				if v, ok := walkJSONValue(o.Value, rp[len(np):]); ok && jsonValueEquals(marshalLossy(v), value) {
					return true
				}
			}
		}
	}
	return false
}

func equalPrefix(rule, got []string) bool {
	for i := range got {
		if rule[i] != got[i] && rule[i] != "*" && got[i] != "*" {
			return false
		}
	}
	return true
}

func jsonValueEquals(raw json.RawMessage, value any) bool {
	if value == nil {
		return strings.TrimSpace(string(raw)) == "null"
	}
	var got any
	if err := json.Unmarshal(raw, &got); err != nil {
		return false
	}
	want, err := json.Marshal(value)
	if err != nil {
		return false
	}
	var wantV any
	if err := json.Unmarshal(want, &wantV); err != nil {
		return false
	}
	return jsonDeepEqual(got, wantV)
}

func jsonDeepEqual(a, b any) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ab) == string(bb)
}

// mergeMatches walks a merge/strategic-merge object. Predicated list steps
// select the entry whose "name" equals the predicate; unpredicated steps
// descend into any object or any element of a list (backtracking).
func mergeMatches(body json.RawMessage, segs []FieldSegment, change string, value any) bool {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return false
	}
	return walkMerge([]any{root}, segs, 0, change, value)
}

func walkMerge(nodes []any, segs []FieldSegment, i int, change string, value any) bool {
	if i == len(segs) {
		return false
	}
	seg := segs[i]
	leaf := i == len(segs)-1
	for _, n := range nodes {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		v, present := m[seg.Key]
		if !present {
			continue
		}
		if leaf {
			switch change {
			case "removed":
				if v == nil {
					return true // explicit null deletes in merge semantics
				}
			case "set":
				if jsonValueEquals(marshalLossy(v), value) {
					return true
				}
			}
			continue
		}
		switch child := v.(type) {
		case map[string]any:
			if walkMerge([]any{child}, segs, i+1, change, value) {
				return true
			}
		case []any:
			// Select entries: by name predicate when given, else any
			// object element (strategic-merge lists merge by name; an
			// index-patched list arrives as a JSONPatch instead).
			var cand []any
			for _, e := range child {
				em, ok := e.(map[string]any)
				if !ok {
					continue
				}
				if seg.Pred != "" {
					if name, _ := em["name"].(string); name != seg.Pred {
						continue
					}
				}
				cand = append(cand, em)
			}
			if walkMerge(cand, segs, i+1, change, value) {
				return true
			}
		}
	}
	return false
}

func marshalLossy(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// walkJSONValue follows plain key segments into a patch value, descending
// into ANY element of encountered lists (strategic-merge name predicates
// were already resolved by the object-exact rule match).
func walkJSONValue(raw json.RawMessage, keys []string) (any, bool) {
	var cur any
	if err := json.Unmarshal(raw, &cur); err != nil {
		return nil, false
	}
	for _, k := range keys {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[k]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			var found any
			ok := false
			for _, e := range node {
				em, isMap := e.(map[string]any)
				if !isMap {
					continue
				}
				if v, has := em[k]; has {
					found, ok = v, true
					break
				}
			}
			if !ok {
				return nil, false
			}
			cur = found
		default:
			return nil, false
		}
	}
	return cur, true
}
