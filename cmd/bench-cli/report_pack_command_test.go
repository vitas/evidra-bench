package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vitas/evidra-bench/pkg/config"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

func TestReportPackCommand_DryRunPrintsTwoPhasesAndReportLinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReportPackScenario(t, dir, "kubernetes/broken-deployment", "broken-deployment", "kubernetes")
	writeReportPackScenario(t, dir, "kubernetes/network-policy-fix", "network-policy-fix", "kubernetes")

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"report-pack",
		"--scenarios-dir", dir,
		"--runs-dir", filepath.Join(dir, "runs"),
		"--dry-run",
		"--model", "sonnet",
		"--provider", "claude",
		"--bench-url", "https://api.evidra.cc",
		"--bench-ui-url", "https://bench.evidra.cc",
		"--report-id", "kubernetes-mcp-readiness-2026-05",
		"--mcp-server", "npx -y @vendor/kubernetes-mcp --stdio",
		"--tool-server-id", "kubernetes-mcp",
		"--tool-server-version", "1.2.3",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("report-pack dry-run failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"PRIVATE REPORT PACK",
		"Phase:       both",
		"Report ID:   kubernetes-mcp-readiness-2026-05",
		"baseline: direct Bench tools",
		"candidate: tool_server=kubernetes-mcp version=1.2.3",
		"broken-deployment",
		"network-policy-fix",
		"https://bench.evidra.cc/bench/reports/tool-server?",
		"https://api.evidra.cc/v1/bench/reports/tool-server?",
		"format=markdown",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestReportPackCommand_BaselinePhaseDoesNotRequireMCPServer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReportPackScenario(t, dir, "kubernetes/broken-deployment", "broken-deployment", "kubernetes")

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"report-pack",
		"--scenarios-dir", dir,
		"--runs-dir", filepath.Join(dir, "runs"),
		"--dry-run",
		"--phase", "baseline",
		"--model", "sonnet",
		"--provider", "claude",
		"--bench-url", "https://api.evidra.cc",
		"--report-id", "kubernetes-mcp-readiness-2026-05",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("baseline phase dry-run failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"Phase:       baseline",
		"Report ID:   kubernetes-mcp-readiness-2026-05",
		"baseline: direct Bench tools",
		"Dry-run: no runs executed.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
	if strings.Contains(output, "candidate:") {
		t.Fatalf("baseline-only plan should not print candidate phase, got:\n%s", output)
	}
}

func TestReportPackCommand_BaselinePhasePrintsMatrixLinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReportPackScenario(t, dir, "kubernetes/broken-deployment", "broken-deployment", "kubernetes")
	writeReportPackScenario(t, dir, "kubernetes/network-policy-fix", "network-policy-fix", "kubernetes")

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"report-pack",
		"--scenarios-dir", dir,
		"--runs-dir", filepath.Join(dir, "runs"),
		"--dry-run",
		"--phase", "baseline",
		"--model", "sonnet",
		"--provider", "claude",
		"--bench-url", "https://api.evidra.cc",
		"--bench-ui-url", "https://bench.evidra.cc",
		"--report-id", "kubernetes-mcp-readiness-2026-05-pilot",
		"--matrix-tool-server-id", "flux159-mcp-server-kubernetes",
		"--matrix-tool-server-id", "containers-kubernetes-mcp-server",
		"--matrix-tool-server-version", "1.0.0",
		"--matrix-tool-server-version", "2.0.0",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("baseline phase dry-run failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"Phase:       baseline",
		"https://bench.evidra.cc/bench/reports/kubernetes-mcp-readiness-2026-05-pilot?",
		"https://api.evidra.cc/v1/bench/reports/tool-server-matrix?",
		"tool_servers=flux159-mcp-server-kubernetes%2Ccontainers-kubernetes-mcp-server",
		"tool_server_versions=1.0.0%2C2.0.0",
		"format=markdown",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestReportPackCommand_CandidatePhaseRequiresMCPIdentity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeReportPackScenario(t, dir, "kubernetes/broken-deployment", "broken-deployment", "kubernetes")

	var buf strings.Builder
	cmd := newRootCommand()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"report-pack",
		"--scenarios-dir", dir,
		"--runs-dir", filepath.Join(dir, "runs"),
		"--dry-run",
		"--phase", "candidate",
		"--model", "sonnet",
		"--provider", "claude",
		"--bench-url", "https://api.evidra.cc",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("candidate phase without MCP identity succeeded, want error")
	}
	if !strings.Contains(err.Error(), "--mcp-server") {
		t.Fatalf("error = %q, want --mcp-server requirement", err.Error())
	}
}

func TestReportPackRunConfigsKeepPromptButSeparateToolServerIdentity(t *testing.T) {
	t.Parallel()

	base := config.Config{
		Model:             "sonnet",
		Provider:          "claude",
		SystemPromptFile:  "prompts/agent.md",
		SkillFile:         "skills/platform-eng.md",
		SkillID:           "platform-eng",
		ContractVersion:   "contract-v1",
		MCPServer:         "npx -y @vendor/kubernetes-mcp --stdio",
		ToolServerID:      "kubernetes-mcp",
		ToolServerVersion: "1.2.3",
	}

	baseline, candidate := reportPackRunConfigs(base)

	if baseline.MCPServer != "" || baseline.ToolServerID != "" || baseline.ToolServerVersion != "" {
		t.Fatalf("baseline should clear MCP identity, got server=%q id=%q version=%q",
			baseline.MCPServer, baseline.ToolServerID, baseline.ToolServerVersion)
	}
	if baseline.SystemPromptFile != base.SystemPromptFile || baseline.SkillFile != base.SkillFile || baseline.SkillID != base.SkillID || baseline.ContractVersion != base.ContractVersion {
		t.Fatalf("baseline should keep prompt config, got prompt=%q skill_file=%q skill_id=%q contract=%q",
			baseline.SystemPromptFile, baseline.SkillFile, baseline.SkillID, baseline.ContractVersion)
	}

	if candidate.MCPServer != base.MCPServer || candidate.ToolServerID != base.ToolServerID || candidate.ToolServerVersion != base.ToolServerVersion {
		t.Fatalf("candidate should keep MCP identity, got server=%q id=%q version=%q",
			candidate.MCPServer, candidate.ToolServerID, candidate.ToolServerVersion)
	}
}

func TestBuildReportPackLinks_DerivesProductionUIURL(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		BenchURL:          "https://api.evidra.cc",
		Model:             "claude-sonnet-4-20250514",
		ToolServerID:      "kubernetes-mcp",
		ToolServerVersion: "1.2.3",
	}
	links, err := buildReportPackLinks(cfg, "", []string{"broken-deployment", "network-policy-fix"}, nil, nil)
	if err != nil {
		t.Fatalf("build links: %v", err)
	}

	ui, err := url.Parse(links.UI)
	if err != nil {
		t.Fatalf("parse UI URL: %v", err)
	}
	if got := ui.Scheme + "://" + ui.Host; got != "https://bench.evidra.cc" {
		t.Fatalf("UI origin = %q, want production Bench UI", got)
	}
	if got := ui.Path; got != "/bench/reports/tool-server" {
		t.Fatalf("UI path = %q, want /bench/reports/tool-server", got)
	}
	q := ui.Query()
	if q.Get("model") != cfg.Model || q.Get("tool_server") != cfg.ToolServerID || q.Get("tool_server_version") != cfg.ToolServerVersion {
		t.Fatalf("unexpected UI query: %s", ui.RawQuery)
	}
	if q.Get("scenarios") != "broken-deployment,network-policy-fix" {
		t.Fatalf("scenarios query = %q", q.Get("scenarios"))
	}

	api, err := url.Parse(links.Markdown)
	if err != nil {
		t.Fatalf("parse markdown URL: %v", err)
	}
	if got := api.Scheme + "://" + api.Host; got != "https://api.evidra.cc" {
		t.Fatalf("API origin = %q, want production API", got)
	}
	if api.Query().Get("format") != "markdown" {
		t.Fatalf("markdown URL should request markdown format: %s", links.Markdown)
	}
}

func TestSelectReportPackScenariosMatchesIDPathAndCategory(t *testing.T) {
	t.Parallel()

	all := []*scenario.Scenario{
		{ID: "broken-deployment", Path: "kubernetes/broken-deployment", Category: "kubernetes"},
		{ID: "helm-pending-release", Path: "helm/pending-release", Category: "helm"},
		{ID: "terraform-state-drift", Path: "terraform/state-drift", Category: "terraform"},
	}
	selected := selectReportPackScenarios(all, []string{"kubernetes/broken-deployment", "helm", "terraform-state-drift"})

	got := make([]string, 0, len(selected))
	for _, s := range selected {
		got = append(got, s.ID)
	}
	want := "broken-deployment,helm-pending-release,terraform-state-drift"
	if strings.Join(got, ",") != want {
		t.Fatalf("selected IDs = %q, want %q", strings.Join(got, ","), want)
	}
}

func writeReportPackScenario(t *testing.T, root, relPath, id, category string) {
	t.Helper()

	scenarioDir := filepath.Join(root, relPath)
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yamlContent := "id: " + id + `
title: Test scenario
category: ` + category + `
prompt: prompts/task.md
break:
  type: kubectl
  command: "get pods"
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}
}
