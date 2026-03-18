package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// Executor runs tool calls. Implemented by ToolExecutor.
type Executor interface {
	Execute(ctx context.Context, tc ToolCall) string
	// EvidenceMode returns how evidence is recorded for this executor.
	EvidenceMode() EvidenceMode
}

// EvidenceMode describes how evidence is recorded during a run.
type EvidenceMode int

const (
	// EvidenceModeNone — no evidence recording. Baseline runs.
	EvidenceModeNone EvidenceMode = iota
	// EvidenceModeDirect — agent calls full prescribe/report with artifact.
	EvidenceModeDirect
	// EvidenceModeSmart — agent calls simplified prescribe (tool+operation only, no artifact).
	EvidenceModeSmart
	// EvidenceModeProxy — harness auto-records prescribe/report for mutations.
	EvidenceModeProxy
)

// LoopConfig configures the agent loop.
type LoopConfig struct {
	Provider     Provider
	Executor     Executor
	Model        string
	MaxTurns     int
	MemoryWindow int // -1 = full history (default), 0 = stateless, N = keep last N exchanges
	Temperature  float64
	MaxTokens    int
	SystemPrompt string
	TaskPrompt   string
}

// LoopResult is the outcome of the agent loop.
type LoopResult struct {
	Turns        int
	Messages     []Message
	TotalUsage   Usage
	FinalOutput  string
	Completed    bool
	Duration     time.Duration
	MemoryWindow int
}

// RunLoop executes the multi-turn tool-use agent loop.
func RunLoop(ctx context.Context, cfg LoopConfig) (*LoopResult, error) {
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 25
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
	start := time.Now()

	// Tool list depends on evidence mode:
	// - Direct: full evidra tools (prescribe with artifact, report)
	// - Smart: simplified evidra tools (prescribe with tool+operation only)
	// - Proxy/None: only run_command (no evidra tools)
	mode := cfg.Executor.EvidenceMode()
	var tools []ToolDef
	switch mode {
	case EvidenceModeSmart:
		tools = SmartPrescribeTools()
	case EvidenceModeDirect:
		tools = BenchTools()
	default:
		tools = BenchTools()
		filtered := make([]ToolDef, 0, len(tools))
		for _, t := range tools {
			if !strings.HasPrefix(t.Name, "evidra_") {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}

	// Full message history (kept for transcript)
	allMessages := []Message{
		{Role: "system", Content: cfg.SystemPrompt},
		{Role: "user", Content: cfg.TaskPrompt},
	}

	var totalUsage Usage
	completed := false
	finalOutput := ""

	for turn := 0; turn < cfg.MaxTurns; turn++ {
		windowLabel := "full"
		if cfg.MemoryWindow >= 0 {
			windowLabel = fmt.Sprintf("window=%d", cfg.MemoryWindow)
		}
		log.Printf("[agent-loop] turn %d/%d provider=%s model=%s memory=%s",
			turn+1, cfg.MaxTurns, cfg.Provider.Name(), cfg.Model, windowLabel)

		// Build the context window the model sees
		contextMessages := buildContextWindow(allMessages, cfg.MemoryWindow)

		resp, err := cfg.Provider.Chat(ctx, ChatRequest{
			Model:       cfg.Model,
			Messages:    contextMessages,
			Tools:       tools,
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("agent.RunLoop: turn %d: %w", turn+1, err)
		}

		totalUsage.PromptTokens += resp.Usage.PromptTokens
		totalUsage.CompletionTokens += resp.Usage.CompletionTokens

		// Append assistant message to full history
		assistantMsg := Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		allMessages = append(allMessages, assistantMsg)

		if resp.Done() {
			completed = true
			finalOutput = resp.Content
			break
		}

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			log.Printf("[agent-loop] tool_call: %s(%s)", tc.Name, truncate(tc.Arguments, 100))
			result := cfg.Executor.Execute(ctx, tc)
			log.Printf("[agent-loop] tool_result: %s", truncate(result, 200))
			allMessages = append(allMessages, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return &LoopResult{
		Turns:        len(allMessages),
		Messages:     allMessages,
		TotalUsage:   totalUsage,
		FinalOutput:  finalOutput,
		Completed:    completed,
		Duration:     time.Since(start),
		MemoryWindow: cfg.MemoryWindow,
	}, nil
}

// buildContextWindow returns the messages the model should see for this turn.
//
// memoryWindow semantics:
//
//	-1 = full history (all messages)
//	 0 = stateless (system + task + last tool result only)
//	 N = keep system + task + last N assistant/tool exchange pairs
func buildContextWindow(messages []Message, memoryWindow int) []Message {
	if memoryWindow < 0 || len(messages) <= 2 {
		return messages
	}

	// Always include system prompt (index 0) and task prompt (index 1).
	// Deep copy base to avoid aliasing the original slice.
	base := make([]Message, 2)
	copy(base, messages[:2])
	conversation := messages[2:]

	if memoryWindow == 0 {
		// Stateless: only the last tool result (if any)
		if len(conversation) == 0 {
			return base
		}
		// Find the last tool result message and the assistant message before it
		result := make([]Message, len(base))
		copy(result, base)
		lastIdx := len(conversation) - 1
		// Include from the last assistant message through end
		for i := lastIdx; i >= 0; i-- {
			if conversation[i].Role == "assistant" {
				result = append(result, conversation[i:]...)
				return result
			}
		}
		// Fallback: just the last message
		result = append(result, conversation[lastIdx])
		return result
	}

	// Window of N: count assistant messages from the end
	// Each "exchange" is an assistant message + its tool results
	assistantCount := 0
	cutoff := 0
	for i := len(conversation) - 1; i >= 0; i-- {
		if conversation[i].Role == "assistant" {
			assistantCount++
			if assistantCount >= memoryWindow {
				cutoff = i
				break
			}
		}
	}

	result := make([]Message, len(base))
	copy(result, base)
	result = append(result, conversation[cutoff:]...)
	return result
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
