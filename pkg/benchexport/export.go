// Package benchexport converts Evidra Bench run artifacts into a
// samebits.com/evidra external evidence bundle (evidra-external-bundle/v1).
//
// The bundle reuses the core's append-only chain format verbatim (see
// pkg/evidrawire), so a consumer can open any exported run with
// `evidra validate --evidence-dir <bundle>` and `evidra scorecard`.
package benchexport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/evaluation"
	"github.com/vitas/evidra-bench/pkg/evidrawire"
)

// ProducerName is written into bundle.json and entry metadata.
const ProducerName = "evidra-bench"

// Request describes one export.
type Request struct {
	// RunDir is a benchmark artifact directory containing run.json
	// (the layout written by pkg/artifact.Writer).
	RunDir string
	// OutDir is the destination bundle directory. It must not exist yet.
	OutDir string
	// ProducerVersion is the bench version string for bundle.json.
	ProducerVersion string
}

// Result summarizes an export.
type Result struct {
	BundlePath   string
	Entries      int
	Prescription string
	Report       string
	ToolCalls    int
}

type toolCallLine struct {
	Tool string `json:"tool"`
}

type checksFile struct {
	Checks []struct {
		Verdict string `json:"verdict"`
	} `json:"checks"`
}

// Export reads one run bundle and writes a verified external evidence bundle.
//
// v1 mapping is run-level: session_start -> prescribe (declared intent: the
// benchmark task) -> report (exit code + verdict) -> annotation (run summary)
// -> session_end. Per-tool-call prescribe/report entries need exit-code
// fidelity in adapter.ToolCallRecord and are deferred to v2; the tool-call
// count and names are captured in the annotation meanwhile.
func Export(req Request) (*Result, error) {
	if req.RunDir == "" || req.OutDir == "" {
		return nil, fmt.Errorf("benchexport.Export: RunDir and OutDir are required")
	}
	run, err := readRunBundle(req.RunDir)
	if err != nil {
		return nil, err
	}
	if run.RunID == "" {
		return nil, fmt.Errorf("benchexport.Export: %s: run.json has no run_id", req.RunDir)
	}
	if _, err := os.Stat(filepath.Join(req.OutDir, evidrawire.BundleFileName)); err == nil {
		return nil, fmt.Errorf("benchexport.Export: %s already contains an exported bundle", req.OutDir)
	}

	signer, err := evidrawire.NewEphemeralSigner()
	if err != nil {
		return nil, fmt.Errorf("benchexport.Export: %w", err)
	}
	adapterName := firstNonEmpty(run.Adapter, "unknown")
	traceID := "bench-" + run.RunID
	// Cohort stamp (Phase 11): the exported bundle carries the source
	// run's semantics_version; documents predating the stamp export as
	// legacy — readable, never comparable (see evidrawire.Cohort).
	w, err := evidrawire.NewBundleWriter(
		req.OutDir, traceID, traceID,
		ProducerName+"/"+adapterName, req.ProducerVersion, signer,
		evidrawire.WithSemanticsVersion(run.Metadata[semanticsMetaKey]),
	)
	if err != nil {
		return nil, fmt.Errorf("benchexport.Export: %w", err)
	}

	actor := evidrawire.Actor{
		Type:       "agent",
		ID:         firstNonEmpty(run.Metadata["model"], adapterName),
		Provenance: ProducerName,
		Version:    req.ProducerVersion,
	}

	toolCalls, err := countToolCalls(req.RunDir)
	if err != nil {
		return nil, err
	}
	checksPassed, checksTotal, err := countChecks(req.RunDir)
	if err != nil {
		return nil, err
	}

	exitCode := run.ExitCode
	verdict, verdictSource := mapVerdict(run.Verdict, exitCode)

	// 1. session_start — run metadata as labels.
	startLabels := map[string]string{
		"run_id":           run.RunID,
		"scenario_id":      run.ScenarioID,
		"adapter":          run.Adapter,
		"passed":           strconv.FormatBool(run.Passed),
		"verdict":          firstNonEmpty(run.Verdict, string(evaluation.VerdictIncomplete)),
		"verdict_source":   verdictSource,
		"duration_seconds": strconv.FormatFloat(run.EndTime.Sub(run.StartTime).Seconds(), 'f', 3, 64),
		"exported_at":      time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range run.Metadata {
		startLabels["meta_"+k] = v
	}
	if _, err := w.Append(evidrawire.EntryBuildParams{
		Type:    evidrawire.EntryTypeSessionStart,
		Actor:   actor,
		Payload: must(evidrawire.SessionStartPayload{Labels: startLabels}),
	}); err != nil {
		return nil, fmt.Errorf("benchexport.Export: session_start: %w", err)
	}

	// 2. prescribe — the declared intent of the benchmark task.
	intent := evidrawire.DeclaredIntent{
		Tool:      adapterName,
		Operation: "bench-run:" + run.ScenarioID,
		Target:    run.ScenarioID,
		Command:   oneLine(run.Prompt, 512),
	}
	prescriptionID := "bench-prescription-" + run.RunID
	prescribePayload := evidrawire.PrescriptionPayload{
		PrescriptionID: prescriptionID,
		Intent:         &intent,
		TTLMs:          evidrawire.DefaultTTLMs,
		CanonSource:    ProducerName,
		Flavor:         evidrawire.FlavorImperative,
		Evidence:       &evidrawire.EvidenceMetadata{Kind: evidrawire.EvidenceKindDeclared},
		Source:         &evidrawire.SourceMetadata{System: ProducerName},
	}
	if _, err := w.Append(evidrawire.EntryBuildParams{
		Type:         evidrawire.EntryTypePrescribe,
		OperationID:  "bench-op-" + run.RunID,
		Actor:        actor,
		Payload:      must(prescribePayload),
		IntentDigest: evidrawire.ComputeDeclaredIntentDigest(intent),
	}); err != nil {
		return nil, fmt.Errorf("benchexport.Export: prescribe: %w", err)
	}

	// 3. report — the observed outcome of the run. The wire verdict comes
	// from the CANONICAL evaluation verdict (run.json "verdict"), never
	// from the agent process exit code: an unsafe or unevaluated run that
	// exited 0 must not export as success. Legacy run.json documents
	// without a verdict fall back to exit-code derivation and are labeled
	// so consumers can see the weaker provenance.
	reportID := "bench-report-" + run.RunID
	reportPayload := evidrawire.ReportPayload{
		ReportID:       reportID,
		PrescriptionID: prescriptionID,
		ExitCode:       &exitCode,
		Verdict:        verdict,
		Flavor:         evidrawire.FlavorImperative,
		Evidence:       &evidrawire.EvidenceMetadata{Kind: evidrawire.EvidenceKindObserved},
		Source:         &evidrawire.SourceMetadata{System: ProducerName},
	}
	if _, err := w.Append(evidrawire.EntryBuildParams{
		Type:           evidrawire.EntryTypeReport,
		OperationID:    "bench-op-" + run.RunID,
		ArtifactDigest: promptDigest(run.Prompt),
		Actor:          actor,
		Payload:        must(reportPayload),
	}); err != nil {
		return nil, fmt.Errorf("benchexport.Export: report: %w", err)
	}

	// 4. annotation — coarse run summary until per-tool-call mapping lands.
	// What the run-level record CAN honestly state is which evidence
	// contract produced it.
	cohort := evidrawire.Cohort(evidrawire.BundleManifest{SemanticsVersion: run.Metadata[semanticsMetaKey]})
	summary := fmt.Sprintf(
		`{"tool_calls":%d,"checks_passed":%d,"checks_total":%d,"chaos_enabled":%t,"canonical_verdict":%q,"verdict_source":%q,"semantics_version":%q,"safety_note":%q}`,
		toolCalls, checksPassed, checksTotal, run.ChaosEnabled,
		firstNonEmpty(run.Verdict, "unknown"), verdictSource,
		cohort, cohortSafetyNote(cohort),
	)
	if _, err := w.Append(evidrawire.EntryBuildParams{
		Type:  evidrawire.EntryTypeAnnotation,
		Actor: evidrawire.Actor{Type: "system", ID: ProducerName, Provenance: ProducerName},
		Payload: must(evidrawire.AnnotationPayload{
			Key:     "evidra-bench/run_summary",
			Value:   summary,
			Message: "Run-level export; per-tool-call evidence requires adapter exit-code fidelity (v2).",
		}),
	}); err != nil {
		return nil, fmt.Errorf("benchexport.Export: annotation: %w", err)
	}

	// 5. session_end.
	endStatus := "completed"
	if exitCode < 0 {
		endStatus = "error"
	}
	if _, err := w.Append(evidrawire.EntryBuildParams{
		Type:    evidrawire.EntryTypeSessionEnd,
		Actor:   actor,
		Payload: must(evidrawire.SessionEndPayload{Status: endStatus}),
	}); err != nil {
		return nil, fmt.Errorf("benchexport.Export: session_end: %w", err)
	}

	if err := w.Close(evidrawire.BundleProducer{
		Name:    ProducerName,
		Version: req.ProducerVersion,
		URL:     "https://github.com/vitas/evidra-bench",
	}, "Entry timestamps reflect export time; run window is in session_start labels."); err != nil {
		return nil, fmt.Errorf("benchexport.Export: close: %w", err)
	}

	if _, err := evidrawire.VerifyBundle(req.OutDir); err != nil {
		return nil, fmt.Errorf("benchexport.Export: produced bundle failed verification: %w", err)
	}

	return &Result{
		BundlePath:   req.OutDir,
		Entries:      5,
		Prescription: prescriptionID,
		Report:       reportID,
		ToolCalls:    toolCalls,
	}, nil
}

// mapVerdict translates the canonical evaluation verdict to the wire
// vocabulary. PASS->success; FAIL/UNSAFE->failure (an unsafe run is not a
// success regardless of exit code); INCOMPLETE->error (could not be
// evaluated). An empty/unknown canonical verdict degrades to exit-code
// derivation and reports the weaker source.
func mapVerdict(canonical string, exitCode int) (evidrawire.Verdict, string) {
	switch evaluation.Verdict(canonical) {
	case evaluation.VerdictPass:
		return evidrawire.VerdictSuccess, "evaluation-verdict"
	case evaluation.VerdictFail, evaluation.VerdictUnsafe:
		return evidrawire.VerdictFailure, "evaluation-verdict"
	case evaluation.VerdictIncomplete:
		return evidrawire.VerdictError, "evaluation-verdict"
	default:
		return evidrawire.VerdictFromExitCode(exitCode), "legacy-exit-code"
	}
}

func readRunBundle(runDir string) (*artifact.RunBundle, error) {
	raw, err := os.ReadFile(filepath.Join(runDir, artifact.RunJSON))
	if err != nil {
		return nil, fmt.Errorf("benchexport: read run.json: %w", err)
	}
	var run artifact.RunBundle
	if err := json.Unmarshal(raw, &run); err != nil {
		return nil, fmt.Errorf("benchexport: parse run.json: %w", err)
	}
	return &run, nil
}

func countToolCalls(runDir string) (int, error) {
	raw, err := os.ReadFile(filepath.Join(runDir, artifact.ToolCallsFile))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("benchexport: read tool-calls.json: %w", err)
	}
	var calls []toolCallLine
	if err := json.Unmarshal(raw, &calls); err != nil {
		return 0, fmt.Errorf("benchexport: parse tool-calls.json: %w", err)
	}
	return len(calls), nil
}

func countChecks(runDir string) (passed, total int, err error) {
	raw, err := os.ReadFile(filepath.Join(runDir, artifact.VerifierFile))
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("benchexport: read verifier.json: %w", err)
	}
	var cf checksFile
	if err := json.Unmarshal(raw, &cf); err != nil {
		return 0, 0, fmt.Errorf("benchexport: parse verifier.json: %w", err)
	}
	for _, c := range cf.Checks {
		total++
		if c.Verdict == "pass" {
			passed++
		}
	}
	return passed, total, nil
}

func must(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("benchexport: marshal " + err.Error())
	}
	return data
}

func promptDigest(prompt string) string {
	if strings.TrimSpace(prompt) == "" {
		return ""
	}
	return evidrawire.SHA256Hex([]byte(prompt))
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

const semanticsMetaKey = "semantics_version"

// cohortSafetyNote is the run-level honesty line for the annotation.
func cohortSafetyNote(cohort string) string {
	if evidrawire.IsLegacy(cohort) {
		return "preview-telemetry verdicts; predates authoritative evidence capture (docs/adr/0001-process-safety-matching.md)"
	}
	return "authoritative evidence capture (audit + snapshots + sandbox); per-case verdicts live in the evaluation-result document"
}
