package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/orchestrator"
)

func TestBuildCertifyRunConfig_UsesRequestOverrides(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"
	base.Adapter = "cli"
	base.Timeout = 5 * time.Minute

	req := CertifyRequest{
		Model:    "sonnet",
		Provider: "bifrost",
	}
	req.Config.Adapter = "mcp"
	req.Config.TimeoutPerScenario = 120

	got := buildCertifyRunConfig(base, req)

	if got.Provider != "bifrost" {
		t.Fatalf("Provider = %q, want bifrost", got.Provider)
	}
	// When provider is set, adapter override is skipped (provider mode
	// uses its own agent loop, not CLI/MCP adapters).
	if got.Adapter != "cli" {
		t.Fatalf("Adapter = %q, want cli (provider mode ignores adapter override)", got.Adapter)
	}
	if got.Timeout != 120*time.Second {
		t.Fatalf("Timeout = %s, want 2m0s", got.Timeout)
	}
}

func TestBuildCertifyRunConfig_UsesFallbacksWhenRequestOmitted(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "anthropic"
	base.Adapter = "cli"
	base.Timeout = 3 * time.Minute

	got := buildCertifyRunConfig(base, CertifyRequest{Model: "haiku"})

	if got.Provider != "anthropic" {
		t.Fatalf("Provider = %q, want anthropic", got.Provider)
	}
	if got.Adapter != "cli" {
		t.Fatalf("Adapter = %q, want cli", got.Adapter)
	}
	if got.Timeout != 3*time.Minute {
		t.Fatalf("Timeout = %s, want 3m0s", got.Timeout)
	}
}

func TestBuildCertifyRunConfig_A2APreservesAdapter(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"
	base.Adapter = "cli"

	req := CertifyRequest{Model: "sonnet"}
	req.Config.Adapter = "a2a"

	got := buildCertifyRunConfig(base, req)
	if got.Adapter != "a2a" {
		t.Fatalf("Adapter = %q, want a2a", got.Adapter)
	}
}

func TestBuildCertifyRunConfig_A2ADoesNotDefaultProvider(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = ""
	base.Adapter = "cli"

	req := CertifyRequest{Model: "sonnet"}
	req.Config.Adapter = "a2a"

	got := buildCertifyRunConfig(base, req)
	if got.Provider != "" {
		t.Fatalf("Provider = %q, want empty", got.Provider)
	}
	if got.Adapter != "a2a" {
		t.Fatalf("Adapter = %q, want a2a", got.Adapter)
	}
}

func TestBuildCertifyRunConfig_EvidenceModeNoneClearsConflicts(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"
	base.MCPServer = "evidra-mcp --signing-mode optional"
	base.ProxyMode = true
	base.SmartPrescribe = true
	base.EvidraBin = "/usr/local/bin/evidra"
	base.SystemPromptFile = "/tmp/system-prompt.md"
	base.Role = "platform-eng"
	base.ContractVersion = "v9.9.9"

	req := CertifyRequest{Model: "sonnet"}
	req.Config.EvidenceMode = "none"

	got := buildCertifyRunConfig(base, req)

	if got.EvidenceMode != "none" {
		t.Fatalf("EvidenceMode = %q, want none", got.EvidenceMode)
	}
	if got.MCPServer != "" {
		t.Fatalf("MCPServer = %q, want empty", got.MCPServer)
	}
	if got.ProxyMode {
		t.Fatal("ProxyMode = true, want false")
	}
	if got.SmartPrescribe {
		t.Fatal("SmartPrescribe = true, want false")
	}
	if got.EvidraBin != "" {
		t.Fatalf("EvidraBin = %q, want empty", got.EvidraBin)
	}
	if got.SystemPromptFile != "" {
		t.Fatalf("SystemPromptFile = %q, want empty", got.SystemPromptFile)
	}
	if got.Role != "" {
		t.Fatalf("Role = %q, want empty", got.Role)
	}
	if got.ContractVersion != "" {
		t.Fatalf("ContractVersion = %q, want empty", got.ContractVersion)
	}
}

func TestBuildCertifyRunConfig_EvidenceModeSmartOverridesDefaults(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"
	base.MCPServer = "evidra-mcp --signing-mode optional"
	base.ProxyMode = true
	base.SmartPrescribe = false
	base.EvidraBin = "/usr/local/bin/evidra"
	base.SystemPromptFile = "/tmp/system-prompt.md"
	base.Role = "platform-eng"
	base.ContractVersion = "v9.9.9"

	req := CertifyRequest{Model: "sonnet"}
	req.Config.EvidenceMode = "smart"

	got := buildCertifyRunConfig(base, req)

	if got.EvidenceMode != "smart" {
		t.Fatalf("EvidenceMode = %q, want smart", got.EvidenceMode)
	}
	if got.MCPServer != "" {
		t.Fatalf("MCPServer = %q, want empty", got.MCPServer)
	}
	if got.ProxyMode {
		t.Fatal("ProxyMode = true, want false")
	}
	if !got.SmartPrescribe {
		t.Fatal("SmartPrescribe = false, want true")
	}
	if got.EvidraBin != "" {
		t.Fatalf("EvidraBin = %q, want empty", got.EvidraBin)
	}
	if got.SystemPromptFile != "" {
		t.Fatalf("SystemPromptFile = %q, want empty", got.SystemPromptFile)
	}
	if got.Role != "" {
		t.Fatalf("Role = %q, want empty", got.Role)
	}
	if got.ContractVersion != "" {
		t.Fatalf("ContractVersion = %q, want empty", got.ContractVersion)
	}
}

func TestHandleCertifyAPI_RejectsUnsupportedEvidenceMode(t *testing.T) {
	t.Parallel()

	handler := handleCertifyAPI(config.Default(), &noopParallelRunner{}, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/v1/certify", strings.NewReader(`{"config":{"evidence_mode":"proxy"}}`))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unsupported evidence_mode") {
		t.Fatalf("body = %q, want unsupported evidence_mode", rec.Body.String())
	}
}

func TestBuildCertifyRunConfig_EmptyEvidenceModePreservesLegacyBehavior(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"
	base.MCPServer = "evidra-mcp --signing-mode optional"
	base.ProxyMode = true
	base.SmartPrescribe = false
	base.EvidraBin = "/usr/local/bin/evidra"
	base.SystemPromptFile = "/tmp/system-prompt.md"

	got := buildCertifyRunConfig(base, CertifyRequest{Model: "sonnet"})

	if got.EvidenceMode != "" {
		t.Fatalf("EvidenceMode = %q, want empty", got.EvidenceMode)
	}
	if got.MCPServer != base.MCPServer {
		t.Fatalf("MCPServer = %q, want %q", got.MCPServer, base.MCPServer)
	}
	if !got.ProxyMode {
		t.Fatal("ProxyMode = false, want true")
	}
	if got.SmartPrescribe {
		t.Fatal("SmartPrescribe = true, want false")
	}
	if got.EvidraBin != base.EvidraBin {
		t.Fatalf("EvidraBin = %q, want %q", got.EvidraBin, base.EvidraBin)
	}
	if got.SystemPromptFile != base.SystemPromptFile {
		t.Fatalf("SystemPromptFile = %q, want %q", got.SystemPromptFile, base.SystemPromptFile)
	}
}

func TestEvidraReporter_SubmitBenchRunUsesExplicitEvidenceMode(t *testing.T) {
	t.Parallel()

	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/bench/runs" {
			t.Fatalf("path = %q, want /v1/bench/runs", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	reporter := &evidraReporter{
		evidraURL:    server.URL,
		evidenceMode: "none",
	}
	reporter.submitBenchRun(orchestratorScenarioEventForTest())

	if got["evidence_mode"] != "none" {
		t.Fatalf("evidence_mode = %v, want none", got["evidence_mode"])
	}
}

func TestEvidraReporter_SubmitBenchRunUsesA2AAdapter(t *testing.T) {
	t.Parallel()

	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/bench/runs" {
			t.Fatalf("path = %q, want /v1/bench/runs", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	reporter := &evidraReporter{
		evidraURL:    server.URL,
		evidenceMode: "none",
		adapter:      "a2a",
	}
	reporter.submitBenchRun(orchestratorScenarioEventForTest())

	if got["adapter"] != "a2a" {
		t.Fatalf("adapter = %v, want a2a", got["adapter"])
	}
}

func orchestratorScenarioEventForTest() orchestrator.ScenarioEvent {
	return orchestrator.ScenarioEvent{
		JobID:      "job-1",
		ScenarioID: "scenario-1",
		Model:      "sonnet",
		Provider:   "claude",
		RunID:      "run-1",
		Duration:   5 * time.Second,
		Passed:     true,
	}
}

type noopParallelRunner struct {
	called bool
}

func (r *noopParallelRunner) RunParallel(_ context.Context, _ config.Config, _ orchestrator.ProgressReporter, _ []string, _ []string, _ int, _ int, _ string) (*orchestrator.RunResult, error) {
	r.called = true
	return &orchestrator.RunResult{}, nil
}
