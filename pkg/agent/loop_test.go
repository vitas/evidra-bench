package agent

import (
	"context"
	"testing"
)

func TestBuildContextWindow_FullHistory(t *testing.T) {
	t.Parallel()
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
		{Role: "assistant", Content: "a1"},
		{Role: "tool", Content: "r1"},
		{Role: "assistant", Content: "a2"},
		{Role: "tool", Content: "r2"},
	}
	result := buildContextWindow(msgs, -1)
	if len(result) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(result))
	}
}

func TestBuildContextWindow_Stateless(t *testing.T) {
	t.Parallel()
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
		{Role: "assistant", Content: "a1"},
		{Role: "tool", Content: "r1"},
		{Role: "assistant", Content: "a2"},
		{Role: "tool", Content: "r2"},
	}
	result := buildContextWindow(msgs, 0)
	// system + task + last assistant + last tool = 4
	if len(result) != 4 {
		t.Fatalf("expected 4 messages (sys+task+last exchange), got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Fatalf("first should be system, got %s", result[0].Role)
	}
	if result[1].Role != "user" {
		t.Fatalf("second should be user, got %s", result[1].Role)
	}
	if result[2].Content != "a2" {
		t.Fatalf("third should be a2, got %s", result[2].Content)
	}
}

func TestBuildContextWindow_Window1(t *testing.T) {
	t.Parallel()
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
		{Role: "assistant", Content: "a1"},
		{Role: "tool", Content: "r1"},
		{Role: "assistant", Content: "a2"},
		{Role: "tool", Content: "r2"},
		{Role: "assistant", Content: "a3"},
		{Role: "tool", Content: "r3"},
	}
	result := buildContextWindow(msgs, 1)
	// system + task + last 1 assistant exchange (a3 + r3) = 4
	if len(result) != 4 {
		t.Fatalf("expected 4, got %d", len(result))
	}
	if result[2].Content != "a3" {
		t.Fatalf("expected a3, got %s", result[2].Content)
	}
}

func TestBuildContextWindow_Window2(t *testing.T) {
	t.Parallel()
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
		{Role: "assistant", Content: "a1"},
		{Role: "tool", Content: "r1"},
		{Role: "assistant", Content: "a2"},
		{Role: "tool", Content: "r2"},
		{Role: "assistant", Content: "a3"},
		{Role: "tool", Content: "r3"},
	}
	result := buildContextWindow(msgs, 2)
	// system + task + last 2 exchanges (a2+r2+a3+r3) = 6
	if len(result) != 6 {
		t.Fatalf("expected 6, got %d", len(result))
	}
	if result[2].Content != "a2" {
		t.Fatalf("expected a2, got %s", result[2].Content)
	}
}

func TestBuildContextWindow_EmptyConversation(t *testing.T) {
	t.Parallel()
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
	}
	result := buildContextWindow(msgs, 0)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestBuildContextWindow_WindowLargerThanHistory(t *testing.T) {
	t.Parallel()
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
		{Role: "assistant", Content: "a1"},
		{Role: "tool", Content: "r1"},
	}
	result := buildContextWindow(msgs, 10)
	// Window larger than history — returns everything
	if len(result) != 4 {
		t.Fatalf("expected 4, got %d", len(result))
	}
}

// mockProvider records messages it receives and returns canned responses.
type mockProvider struct {
	responses []ChatResponse
	turn      int
	received  [][]Message // messages received per turn
}

func (p *mockProvider) Name() string { return "mock" }
func (p *mockProvider) Chat(_ context.Context, req ChatRequest) (*ChatResponse, error) {
	p.received = append(p.received, req.Messages)
	if p.turn >= len(p.responses) {
		return &ChatResponse{}, nil // done (no tool calls)
	}
	resp := p.responses[p.turn]
	p.turn++
	return &resp, nil
}

// mockExecutor returns a fixed result for any tool call.
type mockExecutor struct{}

func (e *mockExecutor) Execute(_ context.Context, tc ToolCall) string {
	return "ok"
}
func (e *mockExecutor) EvidenceMode() EvidenceMode { return EvidenceModeNone }

func TestRunLoop_InjectMessage(t *testing.T) {
	t.Parallel()

	injectChan := make(chan Message, 1)

	provider := &mockProvider{
		responses: []ChatResponse{
			// Turn 1: return a tool call
			{ToolCalls: []ToolCall{{ID: "1", Name: "run_command", Arguments: `{"command":"echo hi"}`}}},
			// Turn 2: after injection, should see injected message. Return done.
			{Content: "done"},
		},
	}

	// Pre-load the buffered channel — will be drained after turn 1's tool execution.
	injectChan <- Message{Role: "user", Content: "New issue: secret is missing"}

	result, err := RunLoop(context.Background(), LoopConfig{
		Provider:     provider,
		Executor:     &mockExecutor{},
		Model:        "test",
		MaxTurns:     10,
		MemoryWindow: -1,
		SystemPrompt: "sys",
		TaskPrompt:   "task",
		InjectChan:   injectChan,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}

	// The injected message should be in the full conversation.
	found := false
	for _, m := range result.Messages {
		if m.Role == "user" && m.Content == "New issue: secret is missing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("injected message not found in conversation")
	}

	// Turn 2 should have received the injected message.
	if len(provider.received) < 2 {
		t.Fatalf("expected at least 2 turns, got %d", len(provider.received))
	}
	turn2Msgs := provider.received[1]
	injectedFound := false
	for _, m := range turn2Msgs {
		if m.Role == "user" && m.Content == "New issue: secret is missing" {
			injectedFound = true
		}
	}
	if !injectedFound {
		t.Fatal("injected message not sent to provider on turn 2")
	}
}

func TestRunLoop_MemoryReset(t *testing.T) {
	t.Parallel()

	memChan := make(chan int, 1)

	provider := &mockProvider{
		responses: []ChatResponse{
			// Turn 1: tool call
			{ToolCalls: []ToolCall{{ID: "1", Name: "run_command", Arguments: `{"command":"echo 1"}`}}},
			// Turn 2: tool call (after memory reset sent between turns)
			{ToolCalls: []ToolCall{{ID: "2", Name: "run_command", Arguments: `{"command":"echo 2"}`}}},
			// Turn 3: done — check what messages were sent
			{Content: "done"},
		},
	}

	// Pre-load the buffered channel — will be picked up after turn 1's tool execution.
	memChan <- 0 // reset = window 0 (stateless)

	result, err := RunLoop(context.Background(), LoopConfig{
		Provider:        provider,
		Executor:        &mockExecutor{},
		Model:           "test",
		MaxTurns:        10,
		MemoryWindow:    -1, // starts as full
		SystemPrompt:    "sys",
		TaskPrompt:      "task",
		MemoryResetChan: memChan,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if !result.Completed {
		t.Fatal("expected completed")
	}

	// After memory reset to 0, provider should receive fewer messages.
	// Turn 3 should only have system + task + last exchange (not full history).
	if len(provider.received) < 3 {
		t.Fatalf("expected 3 turns, got %d", len(provider.received))
	}
	turn3Msgs := provider.received[2]
	// Stateless (window=0): system + task + last assistant + last tool = 4
	if len(turn3Msgs) != 4 {
		t.Errorf("after reset, expected 4 messages (sys+task+last exchange), got %d", len(turn3Msgs))
	}
}
