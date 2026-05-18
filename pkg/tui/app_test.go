package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vitas/evidra-bench/pkg/adapter"
	"github.com/vitas/evidra-bench/pkg/artifact"
	"github.com/vitas/evidra-bench/pkg/harness"
	"github.com/vitas/evidra-bench/pkg/scenario"
)

func TestAgentCommandRequired(t *testing.T) {
	t.Parallel()

	if !agentCommandRequired(LabConfig{}) {
		t.Fatal("expected empty config to require agent command")
	}
	if agentCommandRequired(LabConfig{Provider: "bifrost"}) {
		t.Fatal("provider mode should not require agent command")
	}
	if agentCommandRequired(LabConfig{Adapter: "a2a", A2AAgentURL: "https://agent.example/rpc"}) {
		t.Fatal("a2a mode should not require agent command")
	}
	if agentCommandRequired(LabConfig{AgentCommand: "/usr/bin/agent"}) {
		t.Fatal("explicit agent command should satisfy requirement")
	}
}

func TestNewApp_UsesConfiguredRunsDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	scenariosDir := filepath.Join(root, "scenarios")
	scenarioDir := filepath.Join(scenariosDir, "kubernetes", "broken-deployment")
	if err := os.MkdirAll(filepath.Join(scenarioDir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "prompts", "task.md"), []byte("fix it"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "scenario.yaml"), []byte(`id: broken-deployment
title: Fix broken deployment
category: kubernetes
prompt: prompts/task.md
break:
  type: kubectl
  command: "patch deployment web -n bench -p '{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"nginx\",\"image\":\"nginx:99.99\"}]}}}}'"
checks:
  - type: deployment-ready
    namespace: bench
    name: web
`), 0644); err != nil {
		t.Fatal(err)
	}

	runsDir := filepath.Join(root, "custom-runs")
	app, err := NewApp(scenariosDir, filepath.Join(root, ".lab-config.yaml"), LabConfig{
		RunsDir: runsDir,
	}, harness.Deps{})
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}
	if app.runsDir != runsDir {
		t.Fatalf("runsDir = %q, want %q", app.runsDir, runsDir)
	}
}

func TestBuildRunConfigIncludesRunCommandOptions(t *testing.T) {
	t.Parallel()

	s := &scenario.Scenario{ID: "broken-deployment"}
	labCfg := LabConfig{
		Adapter:             "a2a",
		A2AAgentURL:         "https://agent.example/rpc",
		EnvironmentProvider: "k3d",
		Provider:            "bifrost",
		AgentCommand:        "./agent --flag",
		Model:               "gpt-5.2",
		RunsDir:             "custom-runs",
		Timeout:             "7m",
		DryRun:              false,
		EvidenceDir:         "evidence-in",
		BenchURL:            "https://bench.example",
		BenchAPIKey:         "secret",
		MemoryWindow:        4,
		ReuseCluster:        true,
		ClusterName:         "custom-cluster",
		SystemPromptFile:    "prompts/system.md",
		SkillFile:           "skills/k8s.md",
		SkillID:             "k8s-admin",
		SkillVersion:        "2026.05",
		SkillSource:         "local-file",
		SkillSHA256:         "abc123",
		MCPServer:           "npx -y kubernetes-mcp-server",
		ToolServerID:        "kubernetes-mcp",
		ToolServerVersion:   "1.2.3",
		ReportID:            "public-report",
		ContractVersion:     "v1.2.0",
		Parallel:            3,
		DatabaseURL:         "postgres://bench",
	}

	got := buildRunConfig(s, "/repo/scenarios", labCfg)

	if got.Scenario != "broken-deployment" {
		t.Fatalf("Scenario = %q, want broken-deployment", got.Scenario)
	}
	if got.ScenariosDir != "/repo/scenarios" {
		t.Fatalf("ScenariosDir = %q, want /repo/scenarios", got.ScenariosDir)
	}
	if got.Adapter != labCfg.Adapter || got.A2AAgentURL != labCfg.A2AAgentURL {
		t.Fatalf("adapter/a2a = %q/%q, want %q/%q", got.Adapter, got.A2AAgentURL, labCfg.Adapter, labCfg.A2AAgentURL)
	}
	if got.EnvironmentProvider != labCfg.EnvironmentProvider || got.ClusterName != labCfg.ClusterName {
		t.Fatalf("environment/cluster = %q/%q, want %q/%q", got.EnvironmentProvider, got.ClusterName, labCfg.EnvironmentProvider, labCfg.ClusterName)
	}
	if got.Provider != labCfg.Provider || got.AgentCommand != labCfg.AgentCommand || got.Model != labCfg.Model {
		t.Fatalf("agent config = %q/%q/%q", got.Provider, got.AgentCommand, got.Model)
	}
	if got.RunsDir != labCfg.RunsDir || got.EvidenceDir != labCfg.EvidenceDir {
		t.Fatalf("paths = %q/%q", got.RunsDir, got.EvidenceDir)
	}
	if got.Timeout != 7*time.Minute {
		t.Fatalf("Timeout = %s, want 7m", got.Timeout)
	}
	if got.DryRun != labCfg.DryRun || got.ReuseCluster != labCfg.ReuseCluster {
		t.Fatalf("booleans = dry_run:%v reuse:%v", got.DryRun, got.ReuseCluster)
	}
	if got.BenchURL != labCfg.BenchURL || got.BenchAPIKey != labCfg.BenchAPIKey {
		t.Fatalf("bench reporting = %q/%q", got.BenchURL, got.BenchAPIKey)
	}
	if got.MemoryWindow != labCfg.MemoryWindow {
		t.Fatalf("MemoryWindow = %d, want %d", got.MemoryWindow, labCfg.MemoryWindow)
	}
	if got.SystemPromptFile != labCfg.SystemPromptFile || got.SkillFile != labCfg.SkillFile {
		t.Fatalf("prompt files = %q/%q", got.SystemPromptFile, got.SkillFile)
	}
	if got.SkillID != labCfg.SkillID || got.SkillVersion != labCfg.SkillVersion || got.SkillSource != labCfg.SkillSource || got.SkillSHA256 != labCfg.SkillSHA256 {
		t.Fatalf("skill identity = %q/%q/%q/%q", got.SkillID, got.SkillVersion, got.SkillSource, got.SkillSHA256)
	}
	if got.MCPServer != labCfg.MCPServer || got.ToolServerID != labCfg.ToolServerID || got.ToolServerVersion != labCfg.ToolServerVersion {
		t.Fatalf("tool server = %q/%q/%q", got.MCPServer, got.ToolServerID, got.ToolServerVersion)
	}
	if got.ReportID != labCfg.ReportID || got.ContractVersion != labCfg.ContractVersion {
		t.Fatalf("report/contract = %q/%q", got.ReportID, got.ContractVersion)
	}
	if got.Parallel != labCfg.Parallel || got.DatabaseURL != labCfg.DatabaseURL {
		t.Fatalf("parallel/database = %d/%q", got.Parallel, got.DatabaseURL)
	}
}

func TestBuildRunConfigKeepsBenchReportingEnvDefaults(t *testing.T) {
	t.Setenv("BENCH_API_URL", "https://env-bench.example")
	t.Setenv("BENCH_API_KEY", "env-secret")

	got := buildRunConfig(&scenario.Scenario{ID: "broken-deployment"}, "/repo/scenarios", LabConfig{
		Adapter:      "cli",
		RunsDir:      "runs",
		Timeout:      "5m",
		MemoryWindow: -1,
	})

	if got.BenchURL != "https://env-bench.example" {
		t.Fatalf("BenchURL = %q, want env default", got.BenchURL)
	}
	if got.BenchAPIKey != "env-secret" {
		t.Fatalf("BenchAPIKey = %q, want env default", got.BenchAPIKey)
	}
}

func TestBuildRunDepsProvidesLocalAdapters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		adapter string
		want    any
	}{
		{name: "cli", adapter: "cli", want: &adapter.CLIAdapter{}},
		{name: "mcp", adapter: "mcp", want: &adapter.MCPAdapter{}},
		{name: "a2a", adapter: "a2a", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, err := buildRunDeps(harness.Deps{}, tt.adapter, nil)
			if err != nil {
				t.Fatalf("buildRunDeps() error = %v", err)
			}
			if tt.want == nil {
				if deps.Adapter != nil {
					t.Fatalf("Adapter = %T, want nil", deps.Adapter)
				}
				return
			}
			if deps.Adapter == nil {
				t.Fatalf("Adapter is nil, want %T", tt.want)
			}
			if gotType, wantType := reflect.TypeOf(deps.Adapter), reflect.TypeOf(tt.want); gotType != wantType {
				t.Fatalf("Adapter type = %v, want %v", gotType, wantType)
			}
		})
	}
}

func TestCycleAdapterIncludesA2A(t *testing.T) {
	t.Parallel()

	app := &App{cfg: LabConfig{Adapter: "cli"}}
	app.cycleAdapter()
	if app.cfg.Adapter != "mcp" {
		t.Fatalf("after first cycle adapter = %q, want mcp", app.cfg.Adapter)
	}
	app.cycleAdapter()
	if app.cfg.Adapter != "a2a" {
		t.Fatalf("after second cycle adapter = %q, want a2a", app.cfg.Adapter)
	}
	app.cycleAdapter()
	if app.cfg.Adapter != "cli" {
		t.Fatalf("after third cycle adapter = %q, want cli", app.cfg.Adapter)
	}
}

func TestRenderConfigShowsRunParityFields(t *testing.T) {
	t.Parallel()

	app := &App{cfg: LabConfig{
		Adapter:           "a2a",
		A2AAgentURL:       "https://agent.example/rpc",
		MCPServer:         "npx -y kubernetes-mcp-server",
		ToolServerID:      "kubernetes-mcp",
		ToolServerVersion: "1.2.3",
		SkillID:           "k8s-admin",
		SkillVersion:      "2026.05",
		ReportID:          "report-a",
		ContractVersion:   "v1.2.0",
		ClusterName:       "bench-cli",
		MemoryWindow:      -1,
		ReuseCluster:      true,
	}}

	view := app.renderConfig()
	for _, want := range []string{
		"A2A URL:       https://agent.example/rpc",
		"MCP server:    npx -y kubernetes-mcp-server",
		"Tool server:   kubernetes-mcp @ 1.2.3",
		"Skill:         k8s-admin @ 2026.05",
		"Report ID:     report-a",
		"Contract:      v1.2.0",
		"Cluster:       bench-cli",
		"Memory window: -1",
		"Reuse cluster: true",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("config view missing %q:\n%s", want, view)
		}
	}
}

func TestOpenLatestArtifactsForSelectedScenario(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeArtifactFile(t, dir, "transcript.txt", "latest transcript")
	app := &App{
		filtered: []CatalogItem{{Scenario: &scenario.Scenario{ID: "broken-deployment"}}},
		cursor:   0,
		history: []RunRecord{
			{
				RunBundle: artifact.RunBundle{ScenarioID: "other"},
				Dir:       filepath.Join(t.TempDir(), "other"),
			},
			{
				RunBundle: artifact.RunBundle{ScenarioID: "broken-deployment"},
				Dir:       dir,
			},
		},
	}

	if !app.openLatestArtifactsForSelectedScenario() {
		t.Fatal("expected artifacts to open")
	}
	if app.view != viewArtifact {
		t.Fatalf("view = %d, want viewArtifact", app.view)
	}
	if app.artifacts == nil || app.artifacts.Dir != dir {
		t.Fatalf("artifacts = %#v, want dir %s", app.artifacts, dir)
	}
	if app.artifacts.Transcript != "latest transcript" {
		t.Fatalf("transcript = %q", app.artifacts.Transcript)
	}
}
