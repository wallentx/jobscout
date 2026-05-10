package llm

import (
	"reflect"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestExtractTokenUsageFromContentResponse(t *testing.T) {
	tests := []struct {
		name string
		resp *llms.ContentResponse
		want LLMTokenUsage
	}{
		{
			name: "OpenAI style generation info",
			resp: tokenUsageResponse(map[string]any{
				"PromptTokens":       120,
				"CompletionTokens":   34,
				"TotalTokens":        154,
				"PromptCachedTokens": 12,
				"ReasoningTokens":    7,
			}),
			want: tokenUsage(120, 34, 154, 12, 7, nil),
		},
		{
			name: "Anthropic style generation info",
			resp: tokenUsageResponse(map[string]any{
				"InputTokens":              200,
				"OutputTokens":             75,
				"CacheCreationInputTokens": 10,
				"CacheReadInputTokens":     90,
				"ThinkingTokens":           14,
			}),
			want: tokenUsage(200, 75, 275, 100, nil, 14),
		},
		{
			name: "Gemini style generation info",
			resp: tokenUsageResponse(map[string]any{
				"promptTokenCount":        int32(88),
				"candidatesTokenCount":    int32(22),
				"totalTokenCount":         int32(110),
				"cachedContentTokenCount": int32(8),
				"thoughtsTokenCount":      int32(6),
			}),
			want: tokenUsage(88, 22, 110, 8, nil, 6),
		},
		{
			name: "Ollama style generation info",
			resp: tokenUsageResponse(map[string]any{
				"prompt_eval_count": 412,
				"eval_count":        91,
			}),
			want: tokenUsage(412, 91, 503, nil, nil, nil),
		},
		{
			name: "nested raw OpenAI Responses style usage",
			resp: tokenUsageResponse(map[string]any{
				"usage": map[string]any{
					"input_tokens":  float64(321),
					"output_tokens": float64(45),
					"total_tokens":  float64(366),
					"input_tokens_details": map[string]any{
						"cached_tokens": float64(111),
					},
					"output_tokens_details": map[string]any{
						"reasoning_tokens": float64(12),
					},
				},
			}),
			want: tokenUsage(321, 45, 366, 111, 12, nil),
		},
		{
			name: "nested usage metadata",
			resp: tokenUsageResponse(map[string]any{
				"usage_metadata": map[string]any{
					"input_tokens":  19,
					"output_tokens": 5,
					"total_tokens":  24,
				},
			}),
			want: tokenUsage(19, 5, 24, nil, nil, nil),
		},
		{
			name: "unavailable when choices are empty",
			resp: &llms.ContentResponse{},
			want: LLMTokenUsage{},
		},
		{
			name: "unavailable when generation info has no token fields",
			resp: tokenUsageResponse(map[string]any{"model": "example"}),
			want: LLMTokenUsage{},
		},
		{
			name: "unavailable when response is nil",
			resp: nil,
			want: LLMTokenUsage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTokenUsageFromContentResponse(tt.resp)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractTokenUsageFromContentResponse() = %s, want %s", formatTokenUsage(got), formatTokenUsage(tt.want))
			}
			if got.Available() != tt.want.Available() {
				t.Fatalf("Available() = %v, want %v for usage %s", got.Available(), tt.want.Available(), formatTokenUsage(got))
			}
		})
	}
}

func TestExtractTokenUsageFromMap(t *testing.T) {
	got := ExtractTokenUsage(map[string]any{
		"input_tokens":  12,
		"output_tokens": 8,
	})
	want := tokenUsage(12, 8, 20, nil, nil, nil)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractTokenUsage() = %s, want %s", formatTokenUsage(got), formatTokenUsage(want))
	}
}

func tokenUsageResponse(info map[string]any) *llms.ContentResponse {
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			GenerationInfo: info,
		}},
	}
}

func tokenUsage(input, output, total, cached, reasoning, thinking any) LLMTokenUsage {
	return LLMTokenUsage{
		InputTokens:     testIntPtr(input),
		OutputTokens:    testIntPtr(output),
		TotalTokens:     testIntPtr(total),
		CachedTokens:    testIntPtr(cached),
		ReasoningTokens: testIntPtr(reasoning),
		ThinkingTokens:  testIntPtr(thinking),
	}
}

func testIntPtr(value any) *int {
	if value == nil {
		return nil
	}
	v := value.(int)
	return &v
}

func formatTokenUsage(usage LLMTokenUsage) map[string]any {
	return map[string]any{
		"input":     intPtrValue(usage.InputTokens),
		"output":    intPtrValue(usage.OutputTokens),
		"total":     intPtrValue(usage.TotalTokens),
		"cached":    intPtrValue(usage.CachedTokens),
		"reasoning": intPtrValue(usage.ReasoningTokens),
		"thinking":  intPtrValue(usage.ThinkingTokens),
		"available": usage.Available(),
	}
}

func intPtrValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
