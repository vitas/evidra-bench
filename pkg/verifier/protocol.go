package verifier

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Protocol versions of the structured verifier contract (design §2).
// assert-v2 scripts print exactly one JSON object on stdout:
//
//	{"status":"pass|fail|error",
//	 "assertions":[{"name":"...","passed":true,"observed":"..."}],
//	 "error":{"kind":"transport|timeout|parse|rbac","message":"..."}}
//
// The status field is authoritative whenever the output parses: the script's
// own exit code must not contradict it (and is ignored by the parser on
// purpose, so wrappers that mask exit codes stay honest).

// ProtocolStatus* are the legal values of the protocol status field.
const (
	ProtocolStatusPass  = "pass"
	ProtocolStatusFail  = "fail"
	ProtocolStatusError = "error"
)

// ProtocolError* are the legal error kinds.
const (
	ErrorKindTransport = "transport"
	ErrorKindTimeout   = "timeout"
	ErrorKindParse     = "parse"
	ErrorKindRBAC      = "rbac"
	// ErrorKindExitCode is reserved for legacy command-succeeds checks
	// that fail with exit >= 2; scripts cannot emit it.
	ErrorKindExitCode = "exit-code"
)

// ProtocolAssertion is one observed assertion inside an assert-v2 document.
type ProtocolAssertion struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Observed string `json:"observed,omitempty"`
}

// ProtocolDocument is the assert-v2 stdout contract.
type ProtocolDocument struct {
	Status     string              `json:"status"`
	Assertions []ProtocolAssertion `json:"assertions,omitempty"`
	Error      *ProtocolError      `json:"error,omitempty"`
}

// ProtocolError is the error member of the contract.
type ProtocolError struct {
	Kind    string `json:"kind"`
	Message string `json:"message,omitempty"`
}

// ParseProtocolOutput extracts the last valid protocol JSON object in the
// output (scripts may log noise before printing the document). ok=false
// means no parseable protocol object was found.
func ParseProtocolOutput(out []byte) (doc ProtocolDocument, ok bool) {
	lines := strings.Split(string(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" || !strings.HasPrefix(t, "{") {
			continue
		}
		var candidate ProtocolDocument
		if err := json.Unmarshal([]byte(t), &candidate); err != nil {
			continue
		}
		if candidate.Status == "" {
			continue
		}
		if normalized, err := normalizeProtocol(candidate); err == nil {
			return normalized, true
		}
	}
	return ProtocolDocument{}, false
}

// normalizeProtocol validates the document shape; invalid documents are
// treated as unparseable (protocol_violation) by callers.
func normalizeProtocol(doc ProtocolDocument) (ProtocolDocument, error) {
	switch doc.Status {
	case ProtocolStatusPass, ProtocolStatusFail:
		if doc.Error != nil {
			return ProtocolDocument{}, fmt.Errorf("status %q with error member", doc.Status)
		}
		if doc.Status == ProtocolStatusPass && len(doc.Assertions) == 0 {
			return ProtocolDocument{}, fmt.Errorf("pass with zero assertions")
		}
		for _, a := range doc.Assertions {
			if strings.TrimSpace(a.Name) == "" {
				return ProtocolDocument{}, fmt.Errorf("assertion without name")
			}
		}
		if doc.Status == ProtocolStatusFail {
			anyFailed := false
			for _, a := range doc.Assertions {
				if !a.Passed {
					anyFailed = true
				}
			}
			if !anyFailed {
				return ProtocolDocument{}, fmt.Errorf("fail status with no failed assertion")
			}
		} else {
			for _, a := range doc.Assertions {
				if !a.Passed {
					return ProtocolDocument{}, fmt.Errorf("pass status with failed assertion %q", a.Name)
				}
			}
		}
		return doc, nil
	case ProtocolStatusError:
		if doc.Error == nil {
			return ProtocolDocument{}, fmt.Errorf("error status without error member")
		}
		switch doc.Error.Kind {
		case ErrorKindTransport, ErrorKindTimeout, ErrorKindParse, ErrorKindRBAC:
		default:
			return ProtocolDocument{}, fmt.Errorf("unknown error kind %q", doc.Error.Kind)
		}
		return doc, nil
	default:
		return ProtocolDocument{}, fmt.Errorf("unknown status %q", doc.Status)
	}
}
