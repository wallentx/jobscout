package llm

import (
	"context"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

type maxTokensCappedLLM struct {
	inner       llms.Model
	provider    string
	providerCfg LLMProviderConfig
}

func capLLMMaxTokens(model llms.Model, provider string, providerCfg LLMProviderConfig) llms.Model {
	if model == nil {
		return nil
	}
	return &maxTokensCappedLLM{
		inner:       model,
		provider:    provider,
		providerCfg: providerCfg,
	}
}

func (m *maxTokensCappedLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	return m.inner.GenerateContent(ctx, messages, capLLMCallOptions(m.provider, m.providerCfg, options)...)
}

func (m *maxTokensCappedLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return llms.GenerateFromSinglePrompt(ctx, m, prompt, options...)
}

func (m *maxTokensCappedLLM) Close() error {
	if closeable, ok := m.inner.(closeableLLM); ok {
		return closeable.Close()
	}
	return nil
}

func (m *maxTokensCappedLLM) SupportsReasoning() bool {
	if reasoning, ok := m.inner.(llms.ReasoningModel); ok {
		return reasoning.SupportsReasoning()
	}
	return false
}

func capLLMCallOptions(provider string, providerCfg LLMProviderConfig, options []llms.CallOption) []llms.CallOption {
	cap := llmMaxTokenCap(provider, providerCfg)
	if cap <= 0 {
		return options
	}

	requested := llmRequestedMaxTokens(options)
	if requested > 0 && requested < cap {
		cap = requested
	}
	out := append([]llms.CallOption{}, options...)
	out = append(out, llms.WithMaxTokens(cap))
	return out
}

func llmRequestedMaxTokens(options []llms.CallOption) int {
	var opts llms.CallOptions
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	return opts.MaxTokens
}

func llmMaxTokenCap(provider string, providerCfg LLMProviderConfig) int {
	var cap int
	if providerCfg.MaxTokens != nil && *providerCfg.MaxTokens > 0 {
		cap = *providerCfg.MaxTokens
	}
	if modelCap := modelMaxTokenCap(provider, providerCfg.Model); modelCap > 0 && (cap == 0 || modelCap < cap) {
		cap = modelCap
	}
	return cap
}

func modelMaxTokenCap(provider string, model string) int {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.ToLower(strings.TrimSpace(model))
	if provider != "openai" || model == "" {
		return 0
	}
	if model == "gpt-4" || strings.HasPrefix(model, "gpt-4-0") {
		return 2048
	}
	return 0
}
