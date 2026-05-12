package agent

import "testing"

func TestEstimateCost_KnownModel(t *testing.T) {
	t.Parallel()
	cost := EstimateCost("sonnet", Usage{PromptTokens: 1000, CompletionTokens: 500})
	if cost.TotalCost == 0 {
		t.Fatal("expected non-zero cost for sonnet")
	}
	// sonnet: $3/1M input, $15/1M output
	// 1000 input = $0.003, 500 output = $0.0075
	expectedTotal := 0.003 + 0.0075
	if diff := cost.TotalCost - expectedTotal; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("expected ~$%.4f, got $%.4f", expectedTotal, cost.TotalCost)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	t.Parallel()
	cost := EstimateCost("unknown/model", Usage{PromptTokens: 1000, CompletionTokens: 500})
	if cost.TotalCost != 0 {
		t.Fatalf("expected zero cost for unknown model, got $%.4f", cost.TotalCost)
	}
}

func TestEstimateCost_BifrostModel(t *testing.T) {
	t.Parallel()
	cost := EstimateCost("anthropic/claude-3-5-sonnet", Usage{PromptTokens: 10000, CompletionTokens: 5000})
	if cost.TotalCost == 0 {
		t.Fatal("expected non-zero cost")
	}
	if cost.Currency != "USD" {
		t.Fatalf("expected USD, got %s", cost.Currency)
	}
}

func TestEstimateCost_PublicReportClaudeAlias(t *testing.T) {
	t.Parallel()
	cost := EstimateCost("claude-sonnet-4-6", Usage{PromptTokens: 10000, CompletionTokens: 5000})
	if cost.TotalCost == 0 {
		t.Fatal("expected non-zero cost for public report Claude alias")
	}
}

func TestEstimateCost_DeepSeekV4FlashPromptCache(t *testing.T) {
	t.Parallel()
	cost := EstimateCost("deepseek-v4-flash", Usage{
		PromptTokens:          3000,
		PromptCacheHitTokens:  2000,
		PromptCacheMissTokens: 1000,
		CompletionTokens:      500,
	})
	expectedTotal := (2000.0/1_000_000.0)*0.0028 + (1000.0/1_000_000.0)*0.14 + (500.0/1_000_000.0)*0.28
	if diff := cost.TotalCost - expectedTotal; diff > 0.0000001 || diff < -0.0000001 {
		t.Fatalf("expected ~$%.7f, got $%.7f", expectedTotal, cost.TotalCost)
	}
	if cost.InputTokens != 3000 {
		t.Fatalf("input tokens = %d, want 3000", cost.InputTokens)
	}
}

func TestCostEstimate_String(t *testing.T) {
	t.Parallel()
	cost := EstimateCost("sonnet", Usage{PromptTokens: 1000, CompletionTokens: 500})
	s := cost.String()
	if s == "" {
		t.Fatal("expected non-empty string")
	}
}

func TestCostEstimate_String_Zero(t *testing.T) {
	t.Parallel()
	cost := CostEstimate{}
	if cost.String() != "" {
		t.Fatalf("expected empty string for zero cost, got %q", cost.String())
	}
}
