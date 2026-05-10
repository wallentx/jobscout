package llm

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

// ExtractTokenUsageFromContentResponse extracts token usage from the first choice that reports it.
func ExtractTokenUsageFromContentResponse(resp *llms.ContentResponse) LLMTokenUsage {
	if resp == nil {
		return LLMTokenUsage{}
	}

	for _, choice := range resp.Choices {
		if choice == nil {
			continue
		}
		usage := ExtractTokenUsage(choice.GenerationInfo)
		if usage.Available() {
			return usage
		}
	}

	return LLMTokenUsage{}
}

// ExtractTokenUsage extracts token usage from provider generation info or raw usage maps.
func ExtractTokenUsage(info map[string]any) LLMTokenUsage {
	usage := extractTokenUsage(info)
	if usage.TotalTokens == nil && usage.InputTokens != nil && usage.OutputTokens != nil {
		usage.TotalTokens = intPtr(*usage.InputTokens + *usage.OutputTokens)
	}
	return usage
}

func extractTokenUsage(info map[string]any) LLMTokenUsage {
	if info == nil {
		return LLMTokenUsage{}
	}

	usage := LLMTokenUsage{}

	usage.InputTokens = firstTokenInt(info,
		"PromptTokens",
		"InputTokens",
		"inputTokens",
		"input_tokens",
		"prompt_tokens",
		"promptTokenCount",
		"prompt_eval_count",
	)
	usage.OutputTokens = firstTokenInt(info,
		"CompletionTokens",
		"OutputTokens",
		"outputTokens",
		"output_tokens",
		"completion_tokens",
		"candidatesTokenCount",
		"eval_count",
	)
	usage.TotalTokens = firstTokenInt(info,
		"TotalTokens",
		"totalTokens",
		"total_tokens",
		"totalTokenCount",
	)
	usage.CachedTokens = firstTokenInt(info,
		"CachedTokens",
		"PromptCachedTokens",
		"CacheReadInputTokens",
		"cached_tokens",
		"cache_read_input_tokens",
		"cachedContentTokenCount",
	)
	if cacheCreation := firstTokenInt(info, "CacheCreationInputTokens", "cache_creation_input_tokens"); cacheCreation != nil {
		usage.CachedTokens = addOptionalInts(usage.CachedTokens, cacheCreation)
	}
	usage.ReasoningTokens = firstTokenInt(info,
		"ReasoningTokens",
		"CompletionReasoningTokens",
		"reasoning_tokens",
	)
	usage.ThinkingTokens = firstTokenInt(info,
		"ThinkingTokens",
		"ThinkingInputTokens",
		"ThinkingOutputTokens",
		"thinking_tokens",
		"thoughtsTokenCount",
	)

	for _, key := range []string{"usage", "Usage", "usage_metadata", "usageMetadata", "UsageMetadata"} {
		nested, ok := mapValue(info[key])
		if !ok {
			continue
		}
		mergeTokenUsage(&usage, extractTokenUsage(nested))
	}

	for _, key := range []string{"input_tokens_details", "prompt_tokens_details"} {
		nested, ok := mapValue(info[key])
		if !ok {
			continue
		}
		usage.CachedTokens = firstNonNilInt(usage.CachedTokens, firstTokenInt(nested, "cached_tokens", "CachedTokens"))
	}

	for _, key := range []string{"output_tokens_details", "completion_tokens_details"} {
		nested, ok := mapValue(info[key])
		if !ok {
			continue
		}
		usage.ReasoningTokens = firstNonNilInt(usage.ReasoningTokens, firstTokenInt(nested, "reasoning_tokens", "ReasoningTokens"))
	}

	return usage
}

func mergeTokenUsage(usage *LLMTokenUsage, other LLMTokenUsage) {
	if usage == nil {
		return
	}
	usage.InputTokens = firstNonNilInt(usage.InputTokens, other.InputTokens)
	usage.OutputTokens = firstNonNilInt(usage.OutputTokens, other.OutputTokens)
	usage.TotalTokens = firstNonNilInt(usage.TotalTokens, other.TotalTokens)
	usage.CachedTokens = firstNonNilInt(usage.CachedTokens, other.CachedTokens)
	usage.ReasoningTokens = firstNonNilInt(usage.ReasoningTokens, other.ReasoningTokens)
	usage.ThinkingTokens = firstNonNilInt(usage.ThinkingTokens, other.ThinkingTokens)
}

func firstTokenInt(info map[string]any, keys ...string) *int {
	for _, key := range keys {
		value, ok := tokenInt(info[key])
		if ok {
			return &value
		}
	}
	return nil
}

func tokenInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		if v > int64(math.MaxInt) || v < int64(math.MinInt) {
			return 0, false
		}
		return int(v), true
	case uint:
		if uint64(v) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		if uint64(v) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(v), true
	case uint64:
		if v > uint64(math.MaxInt) {
			return 0, false
		}
		return int(v), true
	case float32:
		return wholeFloatTokenInt(float64(v))
	case float64:
		return wholeFloatTokenInt(v)
	case json.Number:
		i, err := v.Int64()
		if err != nil || i > int64(math.MaxInt) || i < int64(math.MinInt) {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

func wholeFloatTokenInt(value float64) (int, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return 0, false
	}
	if value > float64(math.MaxInt) || value < float64(math.MinInt) {
		return 0, false
	}
	return int(value), true
}

func mapValue(value any) (map[string]any, bool) {
	switch v := value.(type) {
	case map[string]any:
		return v, true
	default:
		return nil, false
	}
}

func firstNonNilInt(first *int, rest ...*int) *int {
	if first != nil {
		return first
	}
	for _, value := range rest {
		if value != nil {
			return value
		}
	}
	return nil
}

func addOptionalInts(a, b *int) *int {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return intPtr(*a + *b)
}

func intPtr(value int) *int {
	return &value
}

func tokenUsagePtr(usage LLMTokenUsage) *LLMTokenUsage {
	if !usage.Available() {
		return nil
	}
	return &usage
}

func formatTokenUsageForLog(usage LLMTokenUsage) string {
	if !usage.Available() {
		return "unavailable"
	}
	parts := make([]string, 0, 6)
	appendTokenUsagePart := func(label string, value *int) {
		if value != nil {
			parts = append(parts, label+"="+formatInt(*value))
		}
	}
	appendTokenUsagePart("input", usage.InputTokens)
	appendTokenUsagePart("output", usage.OutputTokens)
	appendTokenUsagePart("total", usage.TotalTokens)
	appendTokenUsagePart("cached", usage.CachedTokens)
	appendTokenUsagePart("reasoning", usage.ReasoningTokens)
	appendTokenUsagePart("thinking", usage.ThinkingTokens)
	return strings.Join(parts, " ")
}

func logLLMTokenUsage(scope string, usage LLMTokenUsage) {
	logDebug("%s token_usage %s", scope, formatTokenUsageForLog(usage))
}

func formatInt(value int) string {
	return strconv.Itoa(value)
}
