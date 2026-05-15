package llm

import "strings"

type benchmarkModelPricing struct {
	InputPerMillion       float64
	CachedInputPerMillion float64
	OutputPerMillion      float64
}

// benchmarkOpenAIPricing estimates report costs from token counts when the
// provider response does not include a cost. Values are USD per 1M tokens from
// OpenAI API pricing, last reviewed 2026-05-15:
// https://platform.openai.com/docs/pricing
var benchmarkOpenAIPricing = map[string]benchmarkModelPricing{
	"chat-latest":   {InputPerMillion: 5.00, CachedInputPerMillion: 0.50, OutputPerMillion: 30.00},
	"gpt-3.5-turbo": {InputPerMillion: 0.50, CachedInputPerMillion: 0.50, OutputPerMillion: 1.50},
	"gpt-4":         {InputPerMillion: 30.00, CachedInputPerMillion: 30.00, OutputPerMillion: 60.00},
	"gpt-4.1":       {InputPerMillion: 2.00, CachedInputPerMillion: 0.50, OutputPerMillion: 8.00},
	"gpt-4.1-mini":  {InputPerMillion: 0.40, CachedInputPerMillion: 0.10, OutputPerMillion: 1.60},
	"gpt-4.1-nano":  {InputPerMillion: 0.10, CachedInputPerMillion: 0.025, OutputPerMillion: 0.40},
	"gpt-4o":        {InputPerMillion: 2.50, CachedInputPerMillion: 1.25, OutputPerMillion: 10.00},
	"gpt-4o-mini":   {InputPerMillion: 0.15, CachedInputPerMillion: 0.075, OutputPerMillion: 0.60},
	"gpt-5":         {InputPerMillion: 1.25, CachedInputPerMillion: 0.125, OutputPerMillion: 10.00},
	"gpt-5-mini":    {InputPerMillion: 0.25, CachedInputPerMillion: 0.025, OutputPerMillion: 2.00},
	"gpt-5-nano":    {InputPerMillion: 0.05, CachedInputPerMillion: 0.005, OutputPerMillion: 0.40},
	"gpt-5-pro":     {InputPerMillion: 15.00, CachedInputPerMillion: 15.00, OutputPerMillion: 120.00},
	"gpt-5.1":       {InputPerMillion: 1.25, CachedInputPerMillion: 0.125, OutputPerMillion: 10.00},
	"gpt-5.2":       {InputPerMillion: 1.75, CachedInputPerMillion: 0.175, OutputPerMillion: 14.00},
	"gpt-5.2-pro":   {InputPerMillion: 21.00, CachedInputPerMillion: 21.00, OutputPerMillion: 168.00},
	"gpt-5.4":       {InputPerMillion: 2.50, CachedInputPerMillion: 0.25, OutputPerMillion: 15.00},
	"gpt-5.4-mini":  {InputPerMillion: 0.75, CachedInputPerMillion: 0.075, OutputPerMillion: 4.50},
	"gpt-5.4-nano":  {InputPerMillion: 0.20, CachedInputPerMillion: 0.02, OutputPerMillion: 1.25},
	"gpt-5.4-pro":   {InputPerMillion: 30.00, CachedInputPerMillion: 30.00, OutputPerMillion: 180.00},
	"gpt-5.5":       {InputPerMillion: 5.00, CachedInputPerMillion: 0.50, OutputPerMillion: 30.00},
	"gpt-5.5-pro":   {InputPerMillion: 30.00, CachedInputPerMillion: 30.00, OutputPerMillion: 180.00},
	"o1":            {InputPerMillion: 15.00, CachedInputPerMillion: 7.50, OutputPerMillion: 60.00},
	"o1-mini":       {InputPerMillion: 1.10, CachedInputPerMillion: 0.55, OutputPerMillion: 4.40},
	"o3":            {InputPerMillion: 2.00, CachedInputPerMillion: 0.50, OutputPerMillion: 8.00},
	"o3-mini":       {InputPerMillion: 1.10, CachedInputPerMillion: 0.55, OutputPerMillion: 4.40},
	"o4-mini":       {InputPerMillion: 1.10, CachedInputPerMillion: 0.275, OutputPerMillion: 4.40},
}

func benchmarkRecordCostUSD(record llmBenchmarkRunRecord) (float64, bool) {
	if record.EstimatedCostUSD != nil && *record.EstimatedCostUSD > 0 {
		return *record.EstimatedCostUSD, true
	}
	return estimateBenchmarkRecordCostUSD(record)
}

func estimateBenchmarkRecordCostUSD(record llmBenchmarkRunRecord) (float64, bool) {
	pricing, ok := benchmarkPricingForModel(record.Provider, record.Model)
	if !ok {
		return 0, false
	}
	inputTokens := benchmarkPtrIntValue(record.InputTokens)
	outputTokens := benchmarkPtrIntValue(record.OutputTokens)
	if inputTokens <= 0 && outputTokens <= 0 {
		return 0, false
	}

	cachedTokens := 0
	if record.Details != nil {
		if parsed, ok := benchmarkIntFromAny(record.Details["cached_tokens"]); ok && parsed > 0 {
			cachedTokens = parsed
		}
	}
	if cachedTokens > inputTokens {
		cachedTokens = inputTokens
	}
	uncachedInputTokens := inputTokens - cachedTokens
	cachedInputPrice := pricing.CachedInputPerMillion
	if cachedInputPrice <= 0 {
		cachedInputPrice = pricing.InputPerMillion
	}
	cost := (float64(uncachedInputTokens)*pricing.InputPerMillion +
		float64(cachedTokens)*cachedInputPrice +
		float64(outputTokens)*pricing.OutputPerMillion) / 1_000_000
	return cost, cost > 0
}

func benchmarkPricingForModel(provider string, model string) (benchmarkModelPricing, bool) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		model = benchmarkCanonicalPricingModel(model, benchmarkOpenAIPricing)
		if model == "" {
			return benchmarkModelPricing{}, false
		}
		pricing, ok := benchmarkOpenAIPricing[model]
		return pricing, ok
	default:
		return benchmarkModelPricing{}, false
	}
}

func benchmarkCanonicalPricingModel(model string, prices map[string]benchmarkModelPricing) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return ""
	}
	if _, ok := prices[model]; ok {
		return model
	}
	best := ""
	for candidate := range prices {
		if strings.HasPrefix(model, candidate+"-") && len(candidate) > len(best) {
			best = candidate
		}
	}
	return best
}

func benchmarkPtrIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
