package audit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Coverage classifies the api_audit evidence source for one run.
type Coverage string

const (
	// CoverageComplete: cluster audit verified live, both window markers
	// observed, every in-window op terminal, all node readers readable.
	CoverageComplete Coverage = "complete"
	// CoverageIncomplete: some evidence exists but a degradation was
	// detected (missing marker, non-terminal ops, a node reader failed).
	CoverageIncomplete Coverage = "incomplete"
	// CoverageAbsent: no usable audit evidence (not provisioned, or every
	// reader failed).
	CoverageAbsent Coverage = "absent"
)

// Reasons (surfaced verbatim into Evidence.Sources[].reason).
const (
	ReasonNotProvisioned  = "audit not provisioned on this cluster"
	ReasonStartMarker     = "start marker not observed in audit"
	ReasonEndMarker       = "end marker not observed in audit"
	ReasonNonTerminalOps  = "operations without terminal stage at drain end"
	ReasonReaderFailure   = "audit log reader failed on one or more nodes"
	ReasonUnparseableJSON = "audit log lines failed JSON decode"
)

// Source reads one node's audit log content (whole file; the drain loop
// re-reads incrementally and dedupes by (auditID, stage)).
type Source interface {
	NodeName() string
	Read(ctx context.Context) ([]byte, error)
}

// FileSource reads a local file (tests + forensics on extracted logs).
type FileSource struct {
	Node string
	Path string
}

// NodeName implements Source.
func (f FileSource) NodeName() string { return f.Node }

// Read implements Source.
func (f FileSource) Read(context.Context) ([]byte, error) { return os.ReadFile(f.Path) }

// DockerExecSource reads an audit log from inside a sibling container —
// the DooD-safe pattern proven in the spike (docker exec cat; never a bind
// mount of daemon paths, never a nested file mount).
type DockerExecSource struct {
	Container string
	Path      string
	// Timeout bounds one read.
	Timeout time.Duration
}

// NodeName implements Source.
func (d DockerExecSource) NodeName() string { return d.Container }

// Read implements Source.
func (d DockerExecSource) Read(ctx context.Context) ([]byte, error) {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	//nolint:gosec // fixed arguments; container/path come from provisioning, not user input.
	cmd := exec.CommandContext(cctx, "docker", "exec", d.Container, "cat", d.Path)
	out, err := cmd.Output()
	if err != nil {
		// Rotation: audit.log may vanish briefly while audit.log.<ts>
		// exists; tolerate ENOENT only if some rotated file is listed.
		if isNoFileError(err) && d.rotatedExists(cctx) {
			return nil, nil
		}
		return nil, fmt.Errorf("audit: docker exec %s cat %s: %w", d.Container, d.Path, err)
	}
	return out, nil
}

func (d DockerExecSource) rotatedExists(ctx context.Context) bool {
	//nolint:gosec // fixed ls of the audit directory.
	ls := exec.CommandContext(ctx, "docker", "exec", d.Container, "sh", "-c",
		fmt.Sprintf("ls %s.* 2>/dev/null | head -1", strings.TrimSuffix(d.Path, "/")+"*"))
	out, err := ls.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func isNoFileError(err error) bool {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return strings.Contains(strings.ToLower(string(ee.Stderr)), "no such file")
	}
	return false
}

// Collect drains all sources until both markers are observed or the
// deadline passes, then returns the windowed evidence and coverage.
type CollectRequest struct {
	Sources []Source
	// StartNonce / EndNonce are the marker ConfigMap names (nonce part)
	// GETed by the harness certificate identity at window boundaries.
	StartNonce, EndNonce string
	// MarkerUsername attributes marker events (harness cert identity,
	// e.g. kubernetes-admin on kind, admin on k3d).
	MarkerUsername string
	// HealthCheckNodes, when non-empty, are probed via VerifyNodeHealth
	// before draining. An unhealthy node (missing audit args — the silent
	// v1beta3-drop class) forces coverage absent: no window may be called
	// complete over broken plumbing.
	HealthCheckNodes []string
	// Deadline is the hard end of the drain loop.
	Deadline time.Time
	// Poll interval between full re-reads.
	Poll time.Duration
}

// Result is one collection pass.
type Result struct {
	Store    *Store
	Window   WindowResult
	Coverage Coverage
	Reasons  []string
	// Markers observed (for window display / verification of ordering).
	StartEvent, EndEvent Event
	Parse                ParseStats
	NodeFailures         []string
}

// Collect implements the drain-until-both-markers loop with a timeout;
// missing terminal operations or reader failures downgrade coverage — they
// never pass silently.
func Collect(ctx context.Context, req CollectRequest) (*Result, error) {
	if len(req.Sources) == 0 {
		return &Result{Coverage: CoverageAbsent, Reasons: []string{ReasonNotProvisioned}}, nil
	}
	for _, node := range req.HealthCheckNodes {
		if h := VerifyNodeHealth(ctx, node); !h.OK() {
			return &Result{Coverage: CoverageAbsent, Reasons: []string{"audit health: " + h.Node + ": " + h.Detail}}, nil
		}
	}
	poll := req.Poll
	if poll <= 0 {
		poll = 2 * time.Second
	}
	store := NewStore()
	res := &Result{Store: store}
	deadline := req.Deadline
	if deadline.IsZero() {
		deadline = time.Now().Add(60 * time.Second)
	}
	var start, end Event
	var haveStart, haveEnd bool
	var parse ParseStats
	lastFailures := 0
	for {
		failures := 0
		for _, src := range req.Sources {
			data, err := src.Read(ctx)
			if err != nil {
				failures++
				res.NodeFailures = append(res.NodeFailures, src.NodeName()+": "+err.Error())
				continue
			}
			st := Parse(data)
			parse.Lines += st.Lines
			parse.Bad += st.Bad
			parse.Events = nil // stats only
			store.Add(st.Events)
		}
		if !haveStart {
			if ev, err := FindMarker(store.All(), req.MarkerUsername, req.StartNonce); err == nil {
				start, haveStart = ev, true
			}
		}
		if !haveEnd {
			if ev, err := FindMarker(store.All(), req.MarkerUsername, req.EndNonce); err == nil {
				end, haveEnd = ev, true
			}
		}
		lastFailures = failures
		if haveStart && haveEnd {
			break
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(poll):
		}
	}
	res.Parse = parse

	switch {
	case lastFailures == len(req.Sources):
		res.Coverage, res.Reasons = CoverageAbsent, append(res.Reasons, ReasonReaderFailure)
		res.NodeFailures = nil // one clean reason beats N repeats
		return res, nil
	case !haveStart:
		res.Coverage = CoverageIncomplete
		res.Reasons = append(res.Reasons, ReasonStartMarker)
	case !haveEnd:
		res.Coverage = CoverageIncomplete
		res.Reasons = append(res.Reasons, ReasonEndMarker)
	default:
		res.StartEvent, res.EndEvent = start, end
		res.Window = store.Window(start, end)
		switch {
		case len(res.Window.Incomplete) > 0:
			res.Coverage = CoverageIncomplete
			res.Reasons = append(res.Reasons, fmt.Sprintf("%s: %d", ReasonNonTerminalOps, len(res.Window.Incomplete)))
		case parse.Bad > 0:
			res.Coverage = CoverageIncomplete
			res.Reasons = append(res.Reasons, fmt.Sprintf("%s: %d of %d lines", ReasonUnparseableJSON, parse.Bad, parse.Lines))
		case len(res.NodeFailures) > 0:
			res.Coverage = CoverageIncomplete
			res.Reasons = append(res.Reasons, ReasonReaderFailure)
		default:
			res.Coverage = CoverageComplete
		}
	}
	return res, nil
}
