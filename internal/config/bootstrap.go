package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	configFilePath       = "config.yaml"
	searchPromptFilePath = "SEARCH_PROMPT.md"
)

var runtimeConfigPath = configFilePath
var runtimeSearchPromptPath = searchPromptFilePath

func ConfigureRuntime(configPath string, searchPromptPath string) func() {
	previousConfig := runtimeConfigPath
	previousSearchPrompt := runtimeSearchPromptPath
	if strings.TrimSpace(configPath) != "" {
		runtimeConfigPath = configPath
	}
	if strings.TrimSpace(searchPromptPath) != "" {
		runtimeSearchPromptPath = searchPromptPath
	}
	return func() {
		runtimeConfigPath = previousConfig
		runtimeSearchPromptPath = previousSearchPrompt
	}
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || strings.TrimSpace(dir) == "" {
		return nil
	}
	return os.MkdirAll(dir, 0700)
}

func joinOrFallback(items []string, fallback string) string {
	if len(items) == 0 {
		return fallback
	}

	return strings.Join(items, ", ")
}

func providerOptions() []string {
	return []string{"gemini", "openai", "anthropic", "openrouter", "ollama"}
}

func ProviderOptions() []string {
	return providerOptions()
}

func modelOptionsForProvider(provider string) []string {
	switch strings.ToLower(provider) {
	case "openai":
		return []string{"gpt-4o-mini", "gpt-5.4-mini", "gpt-5.4-nano", "gpt-4.1-mini", "gpt-4o", "gpt-4.1", "gpt-5.3-chat", "gpt-5.2-chat", "gpt-5.2", "gpt-5.1", "o3", "gpt-5.4", "gpt-5.5", "gpt-5-mini", "gpt-5", "gpt-5-nano"}
	case "anthropic":
		return []string{"claude-opus-4-1-20250805", "claude-sonnet-4-20250514", "claude-3-7-sonnet-latest", "claude-3-5-haiku-latest"}
	case "openrouter":
		return []string{"deepseek/deepseek-v4-flash"}
	case "ollama":
		return []string{"llama3", "mistral", "qwen2.5"}
	default:
		return []string{"gemini-3.1-flash-lite", "gemini-3.1-pro-preview", "gemini-3.1-pro-preview-customtools", "gemini-3-flash-preview"}
	}
}

func ModelOptionsForProvider(provider string) []string {
	return append([]string(nil), modelOptionsForProvider(provider)...)
}

func defaultModelForProvider(provider string) string {
	options := modelOptionsForProvider(provider)
	if len(options) == 0 {
		return ""
	}
	return options[0]
}

func DefaultModelForProvider(provider string) string {
	return defaultModelForProvider(provider)
}

func envVarForProvider(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "OPENAI_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "ollama":
		return ""
	default:
		return "GEMINI_API_KEY"
	}
}

func EnvVarForProvider(provider string) string {
	return envVarForProvider(provider)
}
