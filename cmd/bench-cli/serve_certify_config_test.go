package main

import (
	"testing"
	"time"

	"samebits.com/evidra-infra-bench/pkg/config"
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

func TestBuildCertifyRunConfig_UnknownProviderFallsBackToBase(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "bifrost"

	req := CertifyRequest{Model: "deepseek-chat", Provider: "deepseek"}

	got := buildCertifyRunConfig(base, req)

	if got.Provider != "bifrost" {
		t.Fatalf("Provider = %q, want bifrost (unknown provider should fall back to base)", got.Provider)
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

func TestBuildCertifyRunConfig_A2ARequestURLOverridesBase(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Adapter = "cli"
	base.A2AAgentURL = "https://base-agent.example"

	req := CertifyRequest{Model: "sonnet"}
	req.Config.Adapter = "a2a"
	req.Config.A2AAgentURL = "https://request-agent.example"

	got := buildCertifyRunConfig(base, req)
	if got.A2AAgentURL != "https://request-agent.example" {
		t.Fatalf("A2AAgentURL = %q, want request override", got.A2AAgentURL)
	}
}

func TestBuildCertifyRunConfig_EvidenceModeNoneClearsConflicts(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"
	base.MCPServer = "sample-mcp --stdio"
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

func TestBuildCertifyRunConfig_EvidenceModeMCPPreservesGenericMCPServer(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"
	base.MCPServer = "sample-mcp --stdio"
	base.SystemPromptFile = "/tmp/system-prompt.md"
	base.Role = "platform-eng"
	base.ContractVersion = "v9.9.9"

	req := CertifyRequest{Model: "sonnet"}
	req.Config.EvidenceMode = "mcp"

	got := buildCertifyRunConfig(base, req)

	if got.EvidenceMode != "mcp" {
		t.Fatalf("EvidenceMode = %q, want mcp", got.EvidenceMode)
	}
	if got.MCPServer != base.MCPServer {
		t.Fatalf("MCPServer = %q, want %q", got.MCPServer, base.MCPServer)
	}
	if got.SystemPromptFile != base.SystemPromptFile {
		t.Fatalf("SystemPromptFile = %q, want %q", got.SystemPromptFile, base.SystemPromptFile)
	}
	if got.Role != base.Role {
		t.Fatalf("Role = %q, want %q", got.Role, base.Role)
	}
	if got.ContractVersion != base.ContractVersion {
		t.Fatalf("ContractVersion = %q, want %q", got.ContractVersion, base.ContractVersion)
	}
}

func TestBuildCertifyRunConfig_UsesRequestMCPToolServerIdentity(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"

	req := CertifyRequest{Model: "sonnet"}
	req.Config.EvidenceMode = "mcp"
	req.Config.MCPServer = "npx -y @vendor/kubernetes-mcp --stdio"
	req.Config.ToolServer = "kubernetes-mcp"
	req.Config.ToolServerVersion = "1.2.3"

	got := buildCertifyRunConfig(base, req)

	if got.MCPServer != "npx -y @vendor/kubernetes-mcp --stdio" {
		t.Fatalf("MCPServer = %q, want request command", got.MCPServer)
	}
	if got.ToolServerID != "kubernetes-mcp" {
		t.Fatalf("ToolServerID = %q, want kubernetes-mcp", got.ToolServerID)
	}
	if got.ToolServerVersion != "1.2.3" {
		t.Fatalf("ToolServerVersion = %q, want 1.2.3", got.ToolServerVersion)
	}
}

func TestBuildCertifyRunConfig_EmptyEvidenceModePreservesDefaultInference(t *testing.T) {
	t.Parallel()

	base := config.Default()
	base.Provider = "claude"
	base.MCPServer = "sample-mcp --stdio"
	base.SystemPromptFile = "/tmp/system-prompt.md"

	got := buildCertifyRunConfig(base, CertifyRequest{Model: "sonnet"})

	if got.EvidenceMode != "" {
		t.Fatalf("EvidenceMode = %q, want empty", got.EvidenceMode)
	}
	if got.MCPServer != base.MCPServer {
		t.Fatalf("MCPServer = %q, want %q", got.MCPServer, base.MCPServer)
	}
	if got.SystemPromptFile != base.SystemPromptFile {
		t.Fatalf("SystemPromptFile = %q, want %q", got.SystemPromptFile, base.SystemPromptFile)
	}
}
