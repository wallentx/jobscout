package llm

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
)

const (
	appConfigDirName = "jobscout"
	configFilePath   = "config.yaml"
)

var runtimeConfigPath = configFilePath

func ConfigureRuntime(configPath string) func() {
	previousConfig := runtimeConfigPath
	if strings.TrimSpace(configPath) != "" {
		runtimeConfigPath = configPath
	}
	return func() {
		runtimeConfigPath = previousConfig
	}
}

func defaultAppConfig() AppConfig {
	return config.DefaultAppConfig()
}

func loadAppConfig(path string) (*AppConfig, error) {
	return config.LoadAppConfig(path)
}

func defaultRuntimeDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		homeDir, homeErr := os.UserHomeDir()
		if homeErr == nil && strings.TrimSpace(homeDir) != "" {
			configDir = filepath.Join(homeDir, ".config")
		} else {
			configDir = "."
		}
	}
	return filepath.Join(configDir, appConfigDirName)
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || strings.TrimSpace(dir) == "" {
		return nil
	}
	return os.MkdirAll(dir, 0700)
}

func modelOptionsForProvider(provider string) []string {
	switch strings.ToLower(provider) {
	case "openai":
		return []string{"gpt-4.1", "gpt-4o-2024-11-20", "gpt-5.3-chat-latest", "gpt-5-chat-latest", "gpt-4.1-nano-2025-04-14", "gpt-4.1-mini", "gpt-5.4-mini"}
	case "anthropic":
		return []string{"claude-opus-4-1-20250805", "claude-sonnet-4-20250514", "claude-3-7-sonnet-latest", "claude-3-5-haiku-latest"}
	case "ollama":
		return []string{"llama3", "mistral", "qwen2.5"}
	default:
		return []string{"gemini-2.5-flash-lite", "gemini-flash-lite-latest", "gemini-3.1-flash-lite-preview", "gemini-2.5-flash", "gemini-pro-latest", "gemini-3-pro-preview", "gemini-3-flash-preview"}
	}
}

func defaultModelForProvider(provider string) string {
	options := modelOptionsForProvider(provider)
	if len(options) == 0 {
		return ""
	}
	return options[0]
}

func envVarForProvider(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "OPENAI_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "ollama":
		return ""
	default:
		return "GEMINI_API_KEY"
	}
}

func selectedWorkSettings(settings domain.WorkSettingsConfig) []string {
	return domain.SelectedWorkSettings(settings)
}

func defaultLLMProviders() map[string]LLMProviderConfig {
	return config.DefaultAppConfig().LLM.Providers
}
