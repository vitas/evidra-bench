package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"samebits.com/evidra-infra-bench/pkg/a2a"
	"samebits.com/evidra-infra-bench/pkg/adapter"
	"samebits.com/evidra-infra-bench/pkg/agent"
	"samebits.com/evidra-infra-bench/pkg/config"
	"samebits.com/evidra-infra-bench/pkg/scenario"
)

func (h *Harness) executeSingleAgent(ctx context.Context, req RunRequest, s *scenario.Scenario, kubeconfigPath, promptContent string, timeout time.Duration, evidenceDir string) (*adapter.RunResult, error) {
	if req.Config.Adapter == "a2a" {
		return h.runWithA2A(ctx, req, s, promptContent, timeout)
	}
	if req.Config.Provider != "" {
		return h.runWithProvider(ctx, req, s, kubeconfigPath, promptContent, timeout, evidenceDir, nil, nil)
	}
	if h.deps.Adapter == nil {
		return nil, fmt.Errorf("harness: local adapter dependency is nil for adapter=%s", req.Config.Adapter)
	}

	return h.deps.Adapter.Run(ctx, adapter.RunInput{
		ScenarioID:     s.ID,
		PromptPath:     s.Prompt,
		WorkspaceDir:   req.Config.RunsDir,
		KubeconfigPath: kubeconfigPath,
		Timeout:        timeout,
		AgentCommand:   req.Config.AgentCommand,
		Model:          req.Config.Model,
	})
}

func shouldUseProviderEvidenceDir(cfg config.Config) bool {
	return cfg.Provider != "" && cfg.Adapter != "a2a"
}

func (h *Harness) runWithA2A(ctx context.Context, req RunRequest, s *scenario.Scenario, taskPrompt string, timeout time.Duration) (*adapter.RunResult, error) {
	client := a2a.NewClient(req.Config.ResolveA2AAgentURL(), nil)

	agentCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := client.RunTextTask(agentCtx, fmt.Sprintf("bench-%s-%d", s.ID, time.Now().UnixMilli()), taskPrompt)
	if err != nil {
		return nil, &InfraError{Err: fmt.Errorf("harness.Run: a2a: %w", err)}
	}

	exitCode := 0
	if !result.Completed {
		exitCode = 1
	}

	return &adapter.RunResult{
		ExitCode:   exitCode,
		Stdout:     result.Output,
		Transcript: result.Output,
		Metadata: map[string]string{
			"adapter":        "a2a",
			"a2a_agent_name": result.AgentName,
			"a2a_agent_url":  req.Config.ResolveA2AAgentURL(),
			"a2a_rpc_url":    result.RPCURL,
			"a2a_task_id":    result.TaskID,
			"a2a_context_id": result.ContextID,
			"a2a_state":      result.State,
		},
	}, nil
}

func (h *Harness) runWithProvider(ctx context.Context, req RunRequest, s *scenario.Scenario, kubeconfigPath, promptContent string, timeout time.Duration, evidenceDir string, injectChan <-chan agent.Message, memoryResetChan <-chan int) (*adapter.RunResult, error) {
	cfg := req.Config
	if config.IsSupportedEvidenceMode(cfg.EvidenceMode) {
		cfg = config.ApplyEvidenceMode(cfg, cfg.EvidenceMode)
	}

	provider, err := agent.ResolveProvider(cfg.Provider)
	if err != nil {
		return nil, err
	}

	if evidenceDir == "" {
		evidenceDir = providerEvidenceDir(cfg.EvidenceDir, cfg.RunsDir, s.ID, time.Now())
	}
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		return nil, fmt.Errorf("harness: create evidence dir: %w", err)
	}

	systemPrompt, err := buildSystemPrompt(cfg, s)
	if err != nil {
		return nil, err
	}

	agentCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Choose executor: MCP server or direct.
	var loopExecutor agent.Executor
	var mcpTools []agent.ToolDef
	var mcpExec *agent.MCPExecutor

	if cfg.MCPServer != "" {
		// MCP mode: route tool calls through MCP server.
		mcpEnv := append(req.ExtraEnv, "KUBECONFIG="+kubeconfigPath)
		var mcpErr error
		mcpExec, mcpErr = agent.NewMCPExecutor(agentCtx, cfg.MCPServer, mcpEnv)
		if mcpErr != nil {
			return nil, fmt.Errorf("harness: mcp executor: %w", mcpErr)
		}
		defer func() { _ = mcpExec.Close() }()
		loopExecutor = mcpExec

		// Get tools from MCP server.
		var toolErr error
		mcpTools, toolErr = mcpExec.Tools(agentCtx)
		if toolErr != nil {
			return nil, fmt.Errorf("harness: mcp tools: %w", toolErr)
		}
		log.Printf("[harness] using MCP server: %s (%d tools)", cfg.MCPServer, len(mcpTools))
	} else {
		// Direct mode: harness executes commands.
		executor := &agent.ToolExecutor{
			KubeconfigPath: kubeconfigPath,
			ExtraEnv:       req.ExtraEnv,
		}
		loopExecutor = executor
	}

	loopResult, err := agent.RunLoop(agentCtx, agent.LoopConfig{
		Provider:        provider,
		Executor:        loopExecutor,
		Model:           cfg.Model,
		MaxTurns:        25,
		MemoryWindow:    cfg.MemoryWindow,
		SystemPrompt:    systemPrompt,
		TaskPrompt:      promptContent,
		Tools:           mcpTools,
		InjectChan:      injectChan,
		MemoryResetChan: memoryResetChan,
	})
	if err != nil {
		return nil, fmt.Errorf("harness: agent loop: %w", err)
	}

	// Build transcript from messages
	var transcript strings.Builder
	for _, m := range loopResult.Messages {
		fmt.Fprintf(&transcript, "[%s] %s\n", m.Role, truncateForLog(m.Content, 500))
		for _, tc := range m.ToolCalls {
			fmt.Fprintf(&transcript, "  -> %s(%s)\n", tc.Name, truncateForLog(tc.Arguments, 200))
		}
	}

	exitCode := 0
	if !loopResult.Completed {
		exitCode = 1
	}

	return &adapter.RunResult{
		ExitCode:   exitCode,
		Transcript: transcript.String(),
		Stdout:     loopResult.FinalOutput,
		ToolCalls:  providerToolCalls(loopResult.Messages),
		Metadata:   buildRunMetadata(cfg, loopResult, evidenceDir),
	}, nil
}

// buildRunMetadata creates the metadata map for a provider-path run,
// including all version information for reproducibility.
func buildRunMetadata(cfg config.Config, loopResult *agent.LoopResult, evidenceDir string) map[string]string {
	if config.IsSupportedEvidenceMode(cfg.EvidenceMode) {
		cfg = config.ApplyEvidenceMode(cfg, cfg.EvidenceMode)
	}

	meta := map[string]string{
		"provider":          cfg.Provider,
		"model":             cfg.Model,
		"evidence_mode":     config.EffectiveEvidenceMode(cfg),
		"turns":             fmt.Sprintf("%d", loopResult.Turns),
		"memory_window":     fmt.Sprintf("%d", loopResult.MemoryWindow),
		"prompt_tokens":     fmt.Sprintf("%d", loopResult.TotalUsage.PromptTokens),
		"completion_tokens": fmt.Sprintf("%d", loopResult.TotalUsage.CompletionTokens),
		"estimated_cost":    agent.EstimateCost(cfg.Model, loopResult.TotalUsage).String(),
		"evidence_dir":      evidenceDir,
	}
	// Merge version info
	vi := config.CollectVersions(version, commit, cfg)
	for k, v := range vi.ToMetadata() {
		meta[k] = v
	}
	if toolServer, toolServerVersion := resolveToolServerIdentity(cfg); toolServer != "" || toolServerVersion != "" || cfg.MCPServer != "" {
		if toolServer != "" {
			meta["tool_server"] = toolServer
		}
		if toolServerVersion != "" {
			meta["tool_server_version"] = toolServerVersion
		}
		if cfg.MCPServer != "" {
			meta["tool_server_cmd"] = cfg.MCPServer
		}
	}
	if reportID := strings.TrimSpace(cfg.ReportID); reportID != "" {
		meta["report_id"] = reportID
	}
	return meta
}

// buildSystemPrompt loads the system prompt from file, role skill, or returns the default.
func buildSystemPrompt(cfg config.Config, s *scenario.Scenario) (string, error) {
	// 1. Explicit system prompt file takes precedence over everything.
	promptFile := cfg.ResolveSystemPromptFile()
	if promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			return "", fmt.Errorf("harness: read system prompt file: %w", err)
		}
		prompt := string(data)
		prompt += fmt.Sprintf("\n\nTarget namespace: %s\n", strings.Join(s.Scope.Namespaces, ", "))
		return prompt, nil
	}

	// 2. Role-based skill: load from skills/<role>.md
	if cfg.Role != "" {
		skillPath := filepath.Join(cfg.ScenariosDir, "..", "skills", cfg.Role+".md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			return "", fmt.Errorf("harness: role skill %q not found at %s: %w", cfg.Role, skillPath, err)
		}
		prompt := string(data)
		prompt += fmt.Sprintf("\n\nTarget namespace: %s\n", strings.Join(s.Scope.Namespaces, ", "))
		log.Printf("[harness] loaded role skill: %s (%d bytes)", cfg.Role, len(data))
		return prompt, nil
	}

	if strings.TrimSpace(cfg.MCPServer) != "" {
		return fmt.Sprintf(
			"You are an infrastructure agent. Fix the problem described in the task.\n"+
				"KUBECONFIG is already set for the MCP tool server. Use the available MCP tools to inspect, diagnose, and safely repair the target environment.\n"+
				"Prefer read-only inspection before changing resources, and keep fixes scoped to the task.\n"+
				"Namespace: %s",
			strings.Join(s.Scope.Namespaces, ", "),
		), nil
	}

	// Default prompt — no protocol skill
	return fmt.Sprintf(
		"You are an infrastructure agent. Fix the problem described in the task.\n"+
			"KUBECONFIG is already set. Use kubectl, helm, or other tools via the run_command tool.\n"+
			"For read-only commands (get, describe, logs): just use run_command directly.\n"+
			"Namespace: %s",
		strings.Join(s.Scope.Namespaces, ", "),
	), nil
}

func providerEvidenceDir(configuredRoot, runsDir, scenarioID string, started time.Time) string {
	root := configuredRoot
	if root == "" {
		root = filepath.Join(runsDir, "evidence")
	}
	safeScenarioID := strings.ReplaceAll(scenarioID, "/", "-")
	return filepath.Join(root, fmt.Sprintf("%s-%d", safeScenarioID, started.UnixMilli()))
}

func providerToolCalls(messages []agent.Message) []adapter.ToolCallRecord {
	var calls []adapter.ToolCallRecord
	callIndexByID := map[string]int{}

	for _, msg := range messages {
		switch msg.Role {
		case "assistant":
			for _, tc := range msg.ToolCalls {
				args := map[string]any{}
				if strings.TrimSpace(tc.Arguments) != "" {
					_ = json.Unmarshal([]byte(tc.Arguments), &args)
				}
				calls = append(calls, adapter.ToolCallRecord{
					Tool: tc.Name,
					Args: args,
				})
				callIndexByID[tc.ID] = len(calls) - 1
			}
		case "tool":
			if idx, ok := callIndexByID[msg.ToolCallID]; ok {
				calls[idx].Result = msg.Content
			}
		}
	}

	return calls
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
