package jobqueue

import (
	"encoding/json"
	"testing"
)

func TestBenchJobArgs_Kind(t *testing.T) {
	t.Parallel()
	args := BenchJobArgs{ScenarioID: "kubernetes/broken-deployment"}
	if args.Kind() != "bench_scenario" {
		t.Errorf("expected bench_scenario, got %s", args.Kind())
	}
}

func TestBenchJobArgs_InsertOpts(t *testing.T) {
	t.Parallel()
	args := BenchJobArgs{}
	opts := args.InsertOpts()
	if opts.MaxAttempts != MaxJobAttempts {
		t.Errorf("expected MaxAttempts=%d, got %d", MaxJobAttempts, opts.MaxAttempts)
	}
}

func TestBenchJobArgs_PreservesMCPToolServerIdentity(t *testing.T) {
	t.Parallel()

	args := BenchJobArgs{
		ScenarioID:        "s1",
		Model:             "sonnet",
		MCPServer:         "npx -y @vendor/kubernetes-mcp --stdio",
		ToolServer:        "kubernetes-mcp",
		ToolServerVersion: "1.2.3",
	}
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded BenchJobArgs
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.MCPServer != args.MCPServer {
		t.Fatalf("MCPServer = %q, want %q", decoded.MCPServer, args.MCPServer)
	}
	if decoded.ToolServer != args.ToolServer {
		t.Fatalf("ToolServer = %q, want %q", decoded.ToolServer, args.ToolServer)
	}
	if decoded.ToolServerVersion != args.ToolServerVersion {
		t.Fatalf("ToolServerVersion = %q, want %q", decoded.ToolServerVersion, args.ToolServerVersion)
	}
}

func TestBenchJobArgs_PreservesSkillIdentity(t *testing.T) {
	t.Parallel()

	args := BenchJobArgs{
		ScenarioID:   "s1",
		Model:        "sonnet",
		SkillFile:    "/tmp/skill.md",
		SkillID:      "k8s-admin",
		SkillVersion: "2026-05-13",
		SkillSource:  "local-temp",
		SkillSHA256:  "abc123",
	}
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded BenchJobArgs
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SkillFile != args.SkillFile {
		t.Fatalf("SkillFile = %q, want %q", decoded.SkillFile, args.SkillFile)
	}
	if decoded.SkillID != args.SkillID {
		t.Fatalf("SkillID = %q, want %q", decoded.SkillID, args.SkillID)
	}
	if decoded.SkillVersion != args.SkillVersion {
		t.Fatalf("SkillVersion = %q, want %q", decoded.SkillVersion, args.SkillVersion)
	}
	if decoded.SkillSource != args.SkillSource {
		t.Fatalf("SkillSource = %q, want %q", decoded.SkillSource, args.SkillSource)
	}
	if decoded.SkillSHA256 != args.SkillSHA256 {
		t.Fatalf("SkillSHA256 = %q, want %q", decoded.SkillSHA256, args.SkillSHA256)
	}
}
