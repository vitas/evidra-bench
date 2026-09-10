package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/audit"
	"github.com/vitas/evidra-bench/pkg/environment"
	"github.com/vitas/evidra-bench/pkg/evaluation"
)

// auditWindowCollector brackets one case with API-audit window markers and
// drains the cluster's audit log for the window. Markers are the
// cert-identity GETs proven in the spike (nonce-named ConfigMaps; the
// authenticated 404 event IS the boundary); the drain loop waits for both
// markers to appear in the log before declaring the window sealed.
type auditWindowCollector struct {
	disabled        bool
	provisioner     *environment.IdentityProvisioner
	adminKubeconfig string
	access          *environment.AuditAccess
	startNonce      string
	endNonce        string
}

func (h *Harness) startAuditWindow(ctx context.Context, req RunRequest, adminKubeconfig string, recorder *runArtifactRecorder) *auditWindowCollector {
	w := &auditWindowCollector{adminKubeconfig: adminKubeconfig, access: req.Audit}
	if req.Audit == nil || len(req.Audit.NodeContainers) == 0 {
		w.disabled = true
		return w
	}
	w.provisioner = environment.NewIdentityProvisioner(&environment.ExecRunner{})
	nonce, err := w.provisioner.EmitMarker(ctx, adminKubeconfig)
	if err != nil {
		// A failed start marker means the window cannot be sealed:
		// coverage will honestly report it; the run itself proceeds
		// (evaluation stays INCOMPLETE-capable, never fake-complete).
		recorder.Event("audit_window", "start_marker_failed", err.Error())
		log.Printf("harness: audit start marker: %v", err)
	}
	w.startNonce = nonce
	return w
}

// close emits the end marker and collects the window. It returns the
// redacted newline-delimited evidence plus its digest; every failure mode
// degrades coverage, never the run.
func (w *auditWindowCollector) close(ctx context.Context, recorder *runArtifactRecorder) (*audit.Result, []byte, string) {
	if w == nil || w.disabled {
		return nil, nil, ""
	}
	deadlineCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 150*time.Second)
	defer cancel()

	nonce, err := w.provisioner.EmitMarker(deadlineCtx, w.adminKubeconfig)
	if err != nil {
		recorder.Event("audit_window", "end_marker_failed", err.Error())
		log.Printf("harness: audit end marker: %v", err)
	}
	w.endNonce = nonce

	sources := make([]audit.Source, 0, len(w.access.NodeContainers))
	for _, node := range w.access.NodeContainers {
		sources = append(sources, audit.DockerExecSource{Container: node, Path: w.access.LogPath})
	}
	res, err := audit.Collect(deadlineCtx, audit.CollectRequest{
		Sources:          sources,
		StartNonce:       w.startNonce,
		EndNonce:         w.endNonce,
		HealthCheckNodes: w.access.NodeContainers,
		MarkerUsername:   w.access.MarkerUsername,
		Deadline:         time.Now().Add(60 * time.Second),
		Poll:             2 * time.Second,
	})
	if err != nil {
		recorder.Event("audit_window", "collect_failed", err.Error())
		return nil, nil, ""
	}
	if res.Coverage != audit.CoverageComplete {
		recorder.Event("audit_window", "coverage_"+string(res.Coverage), joinReasons(res.Reasons))
	}

	redacted := audit.Redacted(res.Window.Ops)
	data, err := marshalJSONL(redacted)
	if err != nil {
		return res, nil, ""
	}
	sum := sha256.Sum256(data)
	return res, data, hex.EncodeToString(sum[:])
}

func marshalJSONL(events []audit.Event) ([]byte, error) {
	out := make([]byte, 0, len(events)*256)
	for _, e := range events {
		line, err := json.Marshal(e)
		if err != nil {
			return nil, fmt.Errorf("marshal audit event: %w", err)
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out, nil
}

func joinReasons(rs []string) string {
	out := ""
	for i, r := range rs {
		if i > 0 {
			out += "; "
		}
		out += r
	}
	return out
}

// AuditWindowInfo is the sealed-window payload handed to artifact writing
// and case evaluation.
type AuditWindowInfo struct {
	Result     *audit.Result
	JSONL      []byte
	Digest     string
	StartNonce string
	EndNonce   string
}

// BundleSummary renders the artifact-run.json view.
func (a *AuditWindowInfo) BundleSummary() *artifact.AuditSummary {
	if a == nil || a.Result == nil {
		return nil
	}
	return &artifact.AuditSummary{
		Coverage:     string(a.Result.Coverage),
		Reasons:      a.Result.Reasons,
		Events:       len(a.Result.Window.Ops),
		File:         "audit.jsonl",
		DigestSHA256: a.Digest,
		WindowStart:  a.StartNonce,
		WindowEnd:    a.EndNonce,
	}
}

// EvaluationSummary renders the qualification-manifest view.
func (a *AuditWindowInfo) EvaluationSummary() *evaluation.AuditSummary {
	if a == nil || a.Result == nil {
		return nil
	}
	cov := evaluation.CoverageAbsent
	switch a.Result.Coverage {
	case audit.CoverageComplete:
		cov = evaluation.CoverageComplete
	case audit.CoverageIncomplete:
		cov = evaluation.CoverageIncomplete
	}
	return &evaluation.AuditSummary{
		Observed:   true,
		Coverage:   cov,
		Reason:     joinReasons(a.Result.Reasons),
		Path:       "audit.jsonl",
		Digest:     a.Digest,
		EventCount: len(a.Result.Window.Ops),
	}
}

// auditWindowInfo assembles the sealed payload (nil-safe).
func auditWindowInfo(res *audit.Result, jsonl []byte, digest string, w *auditWindowCollector) *AuditWindowInfo {
	if res == nil {
		return nil
	}
	info := &AuditWindowInfo{Result: res, JSONL: jsonl, Digest: digest}
	if w != nil {
		info.StartNonce, info.EndNonce = w.startNonce, w.endNonce
	}
	return info
}
