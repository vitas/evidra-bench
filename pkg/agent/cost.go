package agent

import (
	"fmt"
	"strings"
)

// ModelPricing holds per-token pricing for a model (USD per 1M tokens).
type ModelPricing struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

// CostEstimate is the estimated cost for a run.
type CostEstimate struct {
	InputTokens  int
	OutputTokens int
	InputCost    float64
	OutputCost   float64
	TotalCost    float64
	Model        string
	Currency     string
}

// String formats the cost estimate for display.
func (c CostEstimate) String() string {
	if c.TotalCost == 0 {
		return ""
	}
	return fmt.Sprintf("$%.4f (in: $%.4f/%dT, out: $%.4f/%dT)",
		c.TotalCost, c.InputCost, c.InputTokens, c.OutputCost, c.OutputTokens)
}

// EstimateCost calculates the cost for a given token usage and model.
func EstimateCost(model string, usage Usage) CostEstimate {
	pricing := LookupPricing(model)
	inputCost := float64(usage.PromptTokens) / 1_000_000 * pricing.InputPerMillion
	outputCost := float64(usage.CompletionTokens) / 1_000_000 * pricing.OutputPerMillion
	return CostEstimate{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		InputCost:    inputCost,
		OutputCost:   outputCost,
		TotalCost:    inputCost + outputCost,
		Model:        model,
		Currency:     "USD",
	}
}

// LookupPricing returns pricing for a model. Returns zero pricing for unknown models.
// Uses longest-prefix-match to avoid ambiguity (e.g. "openai/gpt-4o-mini" matches
// "openai/gpt-4o-mini" not "openai/gpt-4o").
func LookupPricing(model string) ModelPricing {
	if p, ok := pricingTable[model]; ok {
		return p
	}
	bestLen := 0
	var bestPricing ModelPricing
	for prefix, p := range pricingTable {
		if len(prefix) > bestLen && strings.HasPrefix(model, prefix) {
			bestLen = len(prefix)
			bestPricing = p
		}
	}
	return bestPricing
}

// pricingTable contains known model pricing (USD per 1M tokens).
// Updated: March 2026. Source: provider pricing pages.
var pricingTable = map[string]ModelPricing{
	// Anthropic Claude
	"opus":                        {InputPerMillion: 15.0, OutputPerMillion: 75.0},
	"sonnet":                      {InputPerMillion: 3.0, OutputPerMillion: 15.0},
	"haiku":                       {InputPerMillion: 0.25, OutputPerMillion: 1.25},
	"anthropic/claude-3-5-sonnet": {InputPerMillion: 3.0, OutputPerMillion: 15.0},
	"anthropic/claude-3-5-haiku":  {InputPerMillion: 0.25, OutputPerMillion: 1.25},
	"anthropic/claude-3-opus":     {InputPerMillion: 15.0, OutputPerMillion: 75.0},
	"anthropic/claude-sonnet-4":   {InputPerMillion: 3.0, OutputPerMillion: 15.0},
	"anthropic/claude-opus-4":     {InputPerMillion: 15.0, OutputPerMillion: 75.0},

	// OpenAI
	"gpt-5.2":            {InputPerMillion: 2.0, OutputPerMillion: 8.0},
	"gpt-5.2-pro":        {InputPerMillion: 10.0, OutputPerMillion: 40.0},
	"gpt-5":              {InputPerMillion: 2.0, OutputPerMillion: 8.0},
	"gpt-5-mini":         {InputPerMillion: 0.30, OutputPerMillion: 1.20},
	"gpt-4.1":            {InputPerMillion: 2.0, OutputPerMillion: 8.0},
	"gpt-4.1-mini":       {InputPerMillion: 0.40, OutputPerMillion: 1.60},
	"gpt-4.1-nano":       {InputPerMillion: 0.10, OutputPerMillion: 0.40},
	"gpt-4o":             {InputPerMillion: 2.5, OutputPerMillion: 10.0},
	"gpt-4o-mini":        {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"openai/gpt-4o":      {InputPerMillion: 2.5, OutputPerMillion: 10.0},
	"openai/gpt-4o-mini": {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"openai/gpt-4-turbo": {InputPerMillion: 10.0, OutputPerMillion: 30.0},
	"openai/o1":          {InputPerMillion: 15.0, OutputPerMillion: 60.0},
	"openai/o1-mini":     {InputPerMillion: 3.0, OutputPerMillion: 12.0},

	// Anthropic (direct API)
	"claude-sonnet-4-20250514": {InputPerMillion: 3.0, OutputPerMillion: 15.0},
	"claude-opus-4-20250514":   {InputPerMillion: 15.0, OutputPerMillion: 75.0},
	"claude-haiku-4-20250514":  {InputPerMillion: 0.80, OutputPerMillion: 4.0},

	// Google Gemini
	"google/gemini-2.5-pro":   {InputPerMillion: 1.25, OutputPerMillion: 10.0},
	"google/gemini-2.5-flash": {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"gemini-2.5-pro":          {InputPerMillion: 1.25, OutputPerMillion: 10.0},
	"gemini-2.5-flash":        {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"gemini-2.0-flash":        {InputPerMillion: 0.10, OutputPerMillion: 0.40},

	// DeepSeek
	"deepseek-chat":     {InputPerMillion: 0.27, OutputPerMillion: 1.10},
	"deepseek-reasoner": {InputPerMillion: 0.55, OutputPerMillion: 2.19},

	// Alibaba Qwen (DashScope international pricing)
	"qwen-plus":        {InputPerMillion: 0.80, OutputPerMillion: 2.0},
	"qwen-max":         {InputPerMillion: 1.60, OutputPerMillion: 6.40},
	"qwen-turbo":       {InputPerMillion: 0.30, OutputPerMillion: 0.60},
	"qwen3.5-plus":     {InputPerMillion: 0.80, OutputPerMillion: 2.0},
	"qwen3-max":        {InputPerMillion: 1.60, OutputPerMillion: 6.40},
	"qwen3-coder-plus": {InputPerMillion: 2.0, OutputPerMillion: 8.0},
}
