package llm

import (
	"context"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

type optionCaptureLLM struct {
	content        string
	generationInfo map[string]any
	options        llms.CallOptions
}

func (m *optionCaptureLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	var opts llms.CallOptions
	for _, option := range options {
		option(&opts)
	}
	m.options = opts
	content := m.content
	if content == "" {
		content = `{"ok":true}`
	}
	return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: content, GenerationInfo: m.generationInfo}}}, nil
}

func (m *optionCaptureLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	var opts llms.CallOptions
	for _, option := range options {
		option(&opts)
	}
	m.options = opts
	content := m.content
	if content == "" {
		content = `{"ok":true}`
	}
	return content, nil
}

func TestConfiguredLLMCapsGPT4MaxTokens(t *testing.T) {
	inner := &optionCaptureLLM{}
	model := capLLMMaxTokens(inner, "openai", LLMProviderConfig{Model: "gpt-4"})

	_, err := model.GenerateContent(context.Background(), nil, llms.WithMaxTokens(8192))
	if err != nil {
		t.Fatalf("GenerateContent(...) error = %v", err)
	}
	if inner.options.MaxTokens != 2048 {
		t.Fatalf("MaxTokens = %d, want 2048 for gpt-4 cap", inner.options.MaxTokens)
	}
}

func TestConfiguredLLMUsesConfiguredMaxTokensAsUpperBound(t *testing.T) {
	inner := &optionCaptureLLM{}
	maxTokens := 512
	model := capLLMMaxTokens(inner, "openai", LLMProviderConfig{
		Model:     "gpt-4.1",
		MaxTokens: &maxTokens,
	})

	_, err := model.GenerateContent(context.Background(), nil, llms.WithMaxTokens(8192))
	if err != nil {
		t.Fatalf("GenerateContent(...) error = %v", err)
	}
	if inner.options.MaxTokens != 512 {
		t.Fatalf("MaxTokens = %d, want configured cap 512", inner.options.MaxTokens)
	}
}

func TestConfiguredLLMDoesNotRaiseLowerRequestedMaxTokens(t *testing.T) {
	inner := &optionCaptureLLM{}
	maxTokens := 4096
	model := capLLMMaxTokens(inner, "openai", LLMProviderConfig{
		Model:     "gpt-4.1",
		MaxTokens: &maxTokens,
	})

	_, err := model.GenerateContent(context.Background(), nil, llms.WithMaxTokens(1024))
	if err != nil {
		t.Fatalf("GenerateContent(...) error = %v", err)
	}
	if inner.options.MaxTokens != 1024 {
		t.Fatalf("MaxTokens = %d, want requested lower cap 1024", inner.options.MaxTokens)
	}
}
