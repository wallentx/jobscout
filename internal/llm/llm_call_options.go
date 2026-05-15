package llm

import "github.com/tmc/langchaingo/llms"

func llmJSONCallOptions(temperature float64, maxTokens int) []llms.CallOption {
	return []llms.CallOption{
		llms.WithTemperature(temperature),
		llms.WithMaxTokens(maxTokens),
		llms.WithJSONMode(),
	}
}
