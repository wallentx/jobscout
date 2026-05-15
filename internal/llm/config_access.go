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
	return config.ModelOptionsForProvider(provider)
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
	case "openrouter":
		return "OPENROUTER_API_KEY"
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
