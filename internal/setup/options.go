package setup

import (
	"fmt"
	"strings"

	"github.com/wallentx/jobscout/internal/config"
)

type TaskModelSpec struct {
	Key   string
	Label string
}

func FieldUsesTextarea(key string) bool {
	switch key {
	case "filters.title_requires", "filters.title_includes", "filters.title_excludes", "priority_signals":
		return true
	default:
		return false
	}
}

func ProviderLabel(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		return "Anthropic"
	case "gemini":
		return "Google"
	case "openai":
		return "OpenAI"
	case "ollama":
		return "Local"
	default:
		return provider
	}
}

func ProviderOptionLabel(provider string) string {
	label := ProviderLabel(provider)
	if label == provider {
		return label
	}
	return fmt.Sprintf("%s (%s)", label, provider)
}

func ProviderOptionLabels(providers []string) []string {
	labels := make([]string, 0, len(providers))
	for _, provider := range providers {
		labels = append(labels, ProviderOptionLabel(provider))
	}
	return labels
}

func ProviderOptionIndex(provider string) int {
	options := config.ProviderOptions()
	for i, option := range options {
		if option == provider {
			return i
		}
	}
	return 0
}

func OptionsEnabledDisabled(enabled bool) string {
	if enabled {
		return "Enabled"
	}
	return "Disabled"
}

func TaskModelSpecs() []TaskModelSpec {
	return []TaskModelSpec{
		{Key: "llm_job_search", Label: "LLM job search"},
		{Key: "llm_job_filtering", Label: "LLM job filtering"},
		{Key: "job_identity", Label: "Job identity enrichment"},
		{Key: "llm_company_health", Label: "LLM company health"},
		{Key: "resume_to_criteria", Label: "Resume criteria prefill"},
		{Key: "benchmark", Label: "Benchmarks"},
	}
}

func TaskModelLabel(key string) string {
	key = config.NormalizeLLMTaskKey(key)
	for _, spec := range TaskModelSpecs() {
		if spec.Key == key {
			return spec.Label
		}
	}
	if key == "" {
		return "Task"
	}
	return strings.ReplaceAll(key, "_", " ")
}

func TaskModelSpecAt(idx int) (TaskModelSpec, bool) {
	specs := TaskModelSpecs()
	if idx < 0 || idx >= len(specs) {
		return TaskModelSpec{}, false
	}
	return specs[idx], true
}

func TaskModelFallbackOption(task string) string {
	if config.NormalizeLLMTaskKey(task) == "default" {
		return "Use provider default"
	}
	return "Use fallback/default model"
}

func AuthModeOptions() []string {
	return []string{"env", "command"}
}

func AuthModeLabel(mode string, provider string) string {
	switch mode {
	case "literal":
		return "Stored token (legacy)"
	case "command":
		return "Run command to load token"
	default:
		return fmt.Sprintf("Use environment variable (%s)", config.EnvVarForProvider(provider))
	}
}

func AuthModeIndex(mode string) int {
	options := AuthModeOptions()
	for i, option := range options {
		if option == mode {
			return i
		}
	}
	return 0
}
