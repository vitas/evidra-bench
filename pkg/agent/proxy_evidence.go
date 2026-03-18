package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SimpleProxyEvidence implements ProxyEvidenceWriter by writing JSONL
// entries to a file in the evidence directory. Used by the bench harness
// when --proxy-mode is enabled.
type SimpleProxyEvidence struct {
	mu   sync.Mutex
	dir  string
	file *os.File
}

// NewSimpleProxyEvidence creates a proxy evidence writer in the given directory.
func NewSimpleProxyEvidence(evidenceDir string) (*SimpleProxyEvidence, error) {
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		return nil, fmt.Errorf("proxy evidence: mkdir: %w", err)
	}

	path := filepath.Join(evidenceDir, "proxy-evidence.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("proxy evidence: open: %w", err)
	}

	log.Printf("[proxy] evidence recording to %s", path)
	return &SimpleProxyEvidence{dir: evidenceDir, file: f}, nil
}

// Close closes the evidence file.
func (p *SimpleProxyEvidence) Close() error {
	if p.file != nil {
		return p.file.Close()
	}
	return nil
}

type proxyEntry struct {
	Type           string    `json:"type"`
	PrescriptionID string    `json:"prescription_id"`
	Tool           string    `json:"tool,omitempty"`
	Operation      string    `json:"operation,omitempty"`
	Command        string    `json:"command,omitempty"`
	ExitCode       *int      `json:"exit_code,omitempty"`
	Verdict        string    `json:"verdict,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// Prescribe records a pre-execution entry and returns the prescription ID.
func (p *SimpleProxyEvidence) Prescribe(command string) string {
	words := strings.Fields(strings.TrimSpace(command))
	tool, operation := "", ""
	if len(words) >= 1 {
		tool = words[0]
	}
	if len(words) >= 2 {
		operation = words[1]
	}

	id := fmt.Sprintf("proxy-%d", time.Now().UnixNano())

	p.write(proxyEntry{
		Type:           "prescribe",
		PrescriptionID: id,
		Tool:           tool,
		Operation:      operation,
		Command:        command,
		Timestamp:      time.Now().UTC(),
	})

	log.Printf("[proxy] prescribe: %s %s (id=%s)", tool, operation, id)
	return id
}

// Report records a post-execution entry.
func (p *SimpleProxyEvidence) Report(prescriptionID string, exitCode int) {
	verdict := "success"
	if exitCode != 0 {
		verdict = "failure"
	}

	p.write(proxyEntry{
		Type:           "report",
		PrescriptionID: prescriptionID,
		ExitCode:       &exitCode,
		Verdict:        verdict,
		Timestamp:      time.Now().UTC(),
	})

	log.Printf("[proxy] report: %s exit=%d verdict=%s", prescriptionID, exitCode, verdict)
}

func (p *SimpleProxyEvidence) write(entry proxyEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.file.Write(append(data, '\n'))
}
