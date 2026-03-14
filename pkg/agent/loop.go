package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// LoopConfig configures the agent loop.
type LoopConfig struct {
	Provider     Provider
	Executor     *ToolExecutor
	Model        string
	MaxTurns     int
	Temperature  float64
	MaxTokens    int
	SystemPrompt string
	TaskPrompt   string
}

// LoopResult is the outcome of the agent loop.
type LoopResult struct {
	Turns       int
	Messages    []Message
	TotalUsage  Usage
	FinalOutput string
	Completed   bool
	Duration    time.Duration
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
	tools := BenchTools()

	// Don't include evidra tools if no evidra binary configured
	if cfg.Executor.EvidraBin == "" {
		filtered := make([]ToolDef, 0, len(tools))
		for _, t := range tools {
			if !strings.HasPrefix(t.Name, "evidra_") {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}

	messages := []Message{
		{Role: "system", Content: cfg.SystemPrompt},
		{Role: "user", Content: cfg.TaskPrompt},
	}

	var totalUsage Usage
	completed := false
	finalOutput := ""

	for turn := 0; turn < cfg.MaxTurns; turn++ {
		log.Printf("[agent-loop] turn %d/%d provider=%s model=%s", turn+1, cfg.MaxTurns, cfg.Provider.Name(), cfg.Model)

		resp, err := cfg.Provider.Chat(ctx, ChatRequest{
			Model:       cfg.Model,
			Messages:    messages,
			Tools:       tools,
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("agent.RunLoop: turn %d: %w", turn+1, err)
		}

		totalUsage.PromptTokens += resp.Usage.PromptTokens
		totalUsage.CompletionTokens += resp.Usage.CompletionTokens

		// Append assistant message
		assistantMsg := Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		messages = append(messages, assistantMsg)

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
			messages = append(messages, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return &LoopResult{
		Turns:       len(messages),
		Messages:    messages,
		TotalUsage:  totalUsage,
		FinalOutput: finalOutput,
		Completed:   completed,
		Duration:    time.Since(start),
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
