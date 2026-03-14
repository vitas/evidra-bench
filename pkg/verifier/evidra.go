package verifier

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// evidraEntry is the minimal structure of an Evidra evidence JSONL entry.
type evidraEntry struct {
	EntryID   string          `json:"entry_id"`
	Type      string          `json:"type"`
	Actor     evidraActor     `json:"actor"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type evidraActor struct {
	ID string `json:"id"`
}

type prescribePayload struct {
	PrescriptionID  string          `json:"prescription_id"`
	RiskLevel       string          `json:"risk_level"`
	RiskDetails     []string        `json:"risk_details"`
	CanonicalAction json.RawMessage `json:"canonical_action"`
}

type reportPayload struct {
	ReportID       string `json:"report_id"`
	PrescriptionID string `json:"prescription_id"`
	Verdict        string `json:"verdict"`
	ExitCode       *int   `json:"exit_code"`
}

type signalPayload struct {
	SignalName string `json:"signal_name"`
}

type canonicalAction struct {
	IntentDigest string `json:"intent_digest"`
}

// EvidraCheckConfig mirrors scenario.EvidraExpectations to avoid import cycles.
type EvidraCheckConfig struct {
	MinPrescriptions      int
	MinReports            int
	OrphanedPrescriptions int
	ProtocolViolations    int
	AllReportsHaveVerdict bool
	ExpectedRiskLevel     string
	ExpectedRiskTags      []string
	DeclinedMin           int
	DeclinedMax           *int
	RetryLoopMax          int
}

// evidraEvidence holds parsed evidence entries, loaded once and shared.
type evidraEvidence struct {
	dir     string
	once    sync.Once
	entries []evidraEntry
	err     error
}

func (e *evidraEvidence) load() ([]evidraEntry, error) {
	e.once.Do(func() {
		e.entries, e.err = parseEvidenceDir(e.dir)
	})
	return e.entries, e.err
}

// parseEvidenceDir reads all .jsonl files from <dir>/segments/.
func parseEvidenceDir(dir string) ([]evidraEntry, error) {
	segmentsDir := filepath.Join(dir, "segments")
	files, err := filepath.Glob(filepath.Join(segmentsDir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("verifier.parseEvidenceDir: glob: %w", err)
	}
	var entries []evidraEntry
	for _, f := range files {
		fileEntries, err := parseJSONLFile(f)
		if err != nil {
			return nil, fmt.Errorf("verifier.parseEvidenceDir: %s: %w", filepath.Base(f), err)
		}
		entries = append(entries, fileEntries...)
	}
	return entries, nil
}

func parseJSONLFile(path string) ([]evidraEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []evidraEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry evidraEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// BuildEvidraCheckers creates Checker instances for the given evidra expectations.
func BuildEvidraCheckers(cfg EvidraCheckConfig, evidenceDir string) []Checker {
	ev := &evidraEvidence{dir: evidenceDir}
	var checkers []Checker

	if cfg.MinPrescriptions > 0 {
		checkers = append(checkers, &evidraPrescribeCountCheck{ev: ev, min: cfg.MinPrescriptions})
	}
	if cfg.MinReports > 0 {
		checkers = append(checkers, &evidraReportCountCheck{ev: ev, min: cfg.MinReports})
	}
	checkers = append(checkers, &evidraOrphanedCheck{ev: ev, expected: cfg.OrphanedPrescriptions})
	checkers = append(checkers, &evidraViolationsCheck{ev: ev, expected: cfg.ProtocolViolations})
	if cfg.AllReportsHaveVerdict {
		checkers = append(checkers, &evidraVerdictCheck{ev: ev})
	}
	if cfg.ExpectedRiskLevel != "" {
		checkers = append(checkers, &evidraRiskLevelCheck{ev: ev, level: cfg.ExpectedRiskLevel})
	}
	if len(cfg.ExpectedRiskTags) > 0 {
		checkers = append(checkers, &evidraRiskTagsCheck{ev: ev, tags: cfg.ExpectedRiskTags})
	}
	if cfg.DeclinedMin > 0 {
		checkers = append(checkers, &evidraDeclinedMinCheck{ev: ev, min: cfg.DeclinedMin})
	}
	if cfg.DeclinedMax != nil {
		checkers = append(checkers, &evidraDeclinedMaxCheck{ev: ev, max: *cfg.DeclinedMax})
	}
	if cfg.RetryLoopMax > 0 {
		checkers = append(checkers, &evidraRetryLoopCheck{ev: ev, max: cfg.RetryLoopMax})
	}
	return checkers
}

// evidraPrescribeCountCheck asserts minimum prescribe entries.
type evidraPrescribeCountCheck struct {
	ev  *evidraEvidence
	min int
}

func (c *evidraPrescribeCountCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/prescribe-count-min"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	count := countByType(entries, "prescribe")
	if count < c.min {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
			Message: fmt.Sprintf("prescribe count %d < min %d", count, c.min)}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
}

// evidraReportCountCheck asserts minimum report entries.
type evidraReportCountCheck struct {
	ev  *evidraEvidence
	min int
}

func (c *evidraReportCountCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/report-count-min"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	count := countByType(entries, "report")
	if count < c.min {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
			Message: fmt.Sprintf("report count %d < min %d", count, c.min)}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
}

// evidraOrphanedCheck asserts the number of prescriptions without matching reports.
type evidraOrphanedCheck struct {
	ev       *evidraEvidence
	expected int
}

func (c *evidraOrphanedCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/orphaned-prescriptions"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	prescriptionIDs := map[string]bool{}
	for _, e := range entries {
		if e.Type == "prescribe" {
			var p prescribePayload
			if json.Unmarshal(e.Payload, &p) == nil && p.PrescriptionID != "" {
				prescriptionIDs[p.PrescriptionID] = true
			}
		}
	}
	for _, e := range entries {
		if e.Type == "report" {
			var r reportPayload
			if json.Unmarshal(e.Payload, &r) == nil {
				delete(prescriptionIDs, r.PrescriptionID)
			}
		}
	}
	orphaned := len(prescriptionIDs)
	if orphaned != c.expected {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
			Message: fmt.Sprintf("orphaned prescriptions %d != expected %d", orphaned, c.expected)}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
}

// evidraViolationsCheck asserts the number of protocol_violation signals.
type evidraViolationsCheck struct {
	ev       *evidraEvidence
	expected int
}

func (c *evidraViolationsCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/protocol-violations"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	count := 0
	for _, e := range entries {
		if e.Type == "signal" {
			var s signalPayload
			if json.Unmarshal(e.Payload, &s) == nil && s.SignalName == "protocol_violation" {
				count++
			}
		}
	}
	if count != c.expected {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
			Message: fmt.Sprintf("protocol violations %d != expected %d", count, c.expected)}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
}

// evidraVerdictCheck asserts every report has a non-empty verdict.
type evidraVerdictCheck struct {
	ev *evidraEvidence
}

func (c *evidraVerdictCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/all-reports-have-verdict"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	for _, e := range entries {
		if e.Type != "report" {
			continue
		}
		var r reportPayload
		if err := json.Unmarshal(e.Payload, &r); err != nil {
			return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
				Message: fmt.Sprintf("report %s: parse payload: %v", e.EntryID, err)}
		}
		if r.Verdict == "" {
			return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
				Message: fmt.Sprintf("report %s has empty verdict", e.EntryID)}
		}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
}

// evidraRiskLevelCheck asserts at least one prescription has the expected risk level.
type evidraRiskLevelCheck struct {
	ev    *evidraEvidence
	level string
}

func (c *evidraRiskLevelCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/expected-risk-level"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	for _, e := range entries {
		if e.Type != "prescribe" {
			continue
		}
		var p prescribePayload
		if json.Unmarshal(e.Payload, &p) == nil && strings.EqualFold(p.RiskLevel, c.level) {
			return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
		}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
		Message: fmt.Sprintf("no prescription with risk_level=%q", c.level)}
}

// evidraRiskTagsCheck asserts at least one prescription contains all expected risk tags.
type evidraRiskTagsCheck struct {
	ev   *evidraEvidence
	tags []string
}

func (c *evidraRiskTagsCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/expected-risk-tags"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	for _, e := range entries {
		if e.Type != "prescribe" {
			continue
		}
		var p prescribePayload
		if json.Unmarshal(e.Payload, &p) != nil {
			continue
		}
		if containsAllTags(p.RiskDetails, c.tags) {
			return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
		}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
		Message: fmt.Sprintf("no prescription contains all tags %v", c.tags)}
}

// evidraDeclinedMinCheck asserts minimum reports with verdict=declined.
type evidraDeclinedMinCheck struct {
	ev  *evidraEvidence
	min int
}

func (c *evidraDeclinedMinCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/declined-verdicts-min"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	count := countDeclined(entries)
	if count < c.min {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
			Message: fmt.Sprintf("declined verdicts %d < min %d", count, c.min)}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
}

// evidraDeclinedMaxCheck asserts maximum reports with verdict=declined.
type evidraDeclinedMaxCheck struct {
	ev  *evidraEvidence
	max int
}

func (c *evidraDeclinedMaxCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/declined-verdicts-max"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	count := countDeclined(entries)
	if count > c.max {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
			Message: fmt.Sprintf("declined verdicts %d > max %d", count, c.max)}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
}

// evidraRetryLoopCheck asserts max same-intent prescriptions.
type evidraRetryLoopCheck struct {
	ev  *evidraEvidence
	max int
}

func (c *evidraRetryLoopCheck) Check(_ context.Context, _ string) CheckResult {
	name := "evidra-protocol/retry-loop-max"
	entries, err := c.ev.load()
	if err != nil {
		return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail, Message: err.Error()}
	}
	digestCounts := map[string]int{}
	for _, e := range entries {
		if e.Type != "prescribe" {
			continue
		}
		var ca canonicalAction
		var p prescribePayload
		if json.Unmarshal(e.Payload, &p) != nil {
			continue
		}
		if json.Unmarshal(p.CanonicalAction, &ca) != nil || ca.IntentDigest == "" {
			continue
		}
		digestCounts[ca.IntentDigest]++
	}
	for digest, count := range digestCounts {
		if count > c.max {
			return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictFail,
				Message: fmt.Sprintf("intent %s repeated %d times > max %d", digest, count, c.max)}
		}
	}
	return CheckResult{Name: name, Type: "evidra-protocol", Verdict: VerdictPass}
}

func countByType(entries []evidraEntry, typ string) int {
	count := 0
	for _, e := range entries {
		if e.Type == typ {
			count++
		}
	}
	return count
}

func countDeclined(entries []evidraEntry) int {
	count := 0
	for _, e := range entries {
		if e.Type != "report" {
			continue
		}
		var r reportPayload
		if json.Unmarshal(e.Payload, &r) == nil && r.Verdict == "declined" {
			count++
		}
	}
	return count
}

func containsAllTags(haystack []string, needles []string) bool {
	set := map[string]bool{}
	for _, tag := range haystack {
		set[tag] = true
	}
	for _, needle := range needles {
		if !set[needle] {
			return false
		}
	}
	return true
}
