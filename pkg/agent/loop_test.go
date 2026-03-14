package agent

import (
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
