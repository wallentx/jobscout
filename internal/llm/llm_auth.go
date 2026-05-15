package llm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

const (
	llmAuthModeEnv     = "env"
	llmAuthModeLiteral = "literal"
	llmAuthModeCommand = "command"

	llmTaskJobSearch      = "llm_job_search"
	llmTaskCompanyHealth  = "llm_company_health"
	llmTaskFiltering      = "llm_job_filtering"
	llmTaskJobIdentity    = "job_identity"
	llmTaskResumeCriteria = "resume_to_criteria"
)

func defaultLLMPreferredOrder() []string {
	return []string{"gemini", "openai", "anthropic", "ollama"}
}

func llmAuthConfigDefined(auth LLMAuthConfig) bool {
	return auth.None || strings.TrimSpace(auth.Mode) != "" || strings.TrimSpace(auth.EnvVar) != "" || strings.TrimSpace(auth.Value) != "" || strings.TrimSpace(auth.Command) != ""
}

func normalizeLLMProviderConfig(name string, providerCfg LLMProviderConfig) LLMProviderConfig {
	if strings.TrimSpace(providerCfg.Model) == "" {
		providerCfg.Model = defaultModelForProvider(name)
	}
	if len(providerCfg.Models) > 0 {
		models := make(map[string]string, len(providerCfg.Models))
		for task, model := range providerCfg.Models {
			task = normalizeLLMTaskKey(task)
			model = strings.TrimSpace(model)
			if task == "" || model == "" {
				continue
			}
			models[task] = model
		}
		providerCfg.Models = models
	}
	if providerCfg.Auth.None {
		providerCfg.Auth.Mode = ""
		providerCfg.Auth.EnvVar = ""
		return providerCfg
	}
	if strings.TrimSpace(providerCfg.Auth.Mode) == "" {
		providerCfg.Auth.Mode = llmAuthModeEnv
	}
	if providerCfg.Auth.Mode == llmAuthModeEnv && strings.TrimSpace(providerCfg.Auth.EnvVar) == "" {
		providerCfg.Auth.EnvVar = envVarForProvider(name)
	}
	return providerCfg
}

func normalizeLLMConfig(cfg *AppConfig) {
	if cfg == nil {
		return
	}

	if len(cfg.LLM.PreferredOrder) == 0 {
		cfg.LLM.PreferredOrder = defaultLLMPreferredOrder()
	}
	if strings.TrimSpace(cfg.LLM.Provider) == "" {
		cfg.LLM.Provider = "gemini"
	}
	if !cfg.LLM.Enabled {
		cfg.LLM.JobSearch = false
		cfg.LLM.JobFiltering = false
		cfg.LLM.CompanyHealth = false
	}
	if cfg.LLM.Providers == nil {
		cfg.LLM.Providers = make(map[string]LLMProviderConfig)
	}
	if len(cfg.LLM.Models) > 0 {
		models := make(map[string]string, len(cfg.LLM.Models))
		for task, model := range cfg.LLM.Models {
			task = normalizeLLMTaskKey(task)
			model = strings.TrimSpace(model)
			if task == "" || model == "" {
				continue
			}
			models[task] = model
		}
		cfg.LLM.Models = models
	}

	for name, providerCfg := range defaultLLMProviders() {
		existing, ok := cfg.LLM.Providers[name]
		if !ok {
			cfg.LLM.Providers[name] = providerCfg
			continue
		}
		if strings.TrimSpace(existing.Model) == "" {
			existing.Model = providerCfg.Model
		}
		if strings.TrimSpace(existing.Endpoint) == "" {
			existing.Endpoint = providerCfg.Endpoint
		}
		if !existing.Auth.None && strings.TrimSpace(existing.Auth.Mode) == "" && strings.TrimSpace(existing.Auth.EnvVar) == "" {
			existing.Auth = providerCfg.Auth
		}
		cfg.LLM.Providers[name] = existing
	}

	selected := strings.TrimSpace(cfg.LLM.Provider)
	selectedCfg := cfg.LLM.Providers[selected]
	if strings.TrimSpace(selectedCfg.Model) == "" && strings.TrimSpace(cfg.LLM.Model) != "" {
		selectedCfg.Model = cfg.LLM.Model
	}
	if shouldApplyLegacyTopLevelAuth(selected, selectedCfg, cfg.LLM.Auth) {
		selectedCfg.Auth = cfg.LLM.Auth
	}
	if len(cfg.LLM.Models) > 0 {
		if selectedCfg.Models == nil {
			selectedCfg.Models = make(map[string]string, len(cfg.LLM.Models))
		}
		for task, model := range cfg.LLM.Models {
			selectedCfg.Models[task] = model
		}
		cfg.LLM.Models = nil
	}
	selectedCfg = normalizeLLMProviderConfig(selected, selectedCfg)
	cfg.LLM.Providers[selected] = selectedCfg
	cfg.LLM.Model = selectedCfg.Model
	cfg.LLM.Auth = selectedCfg.Auth
}

func shouldApplyLegacyTopLevelAuth(provider string, providerCfg LLMProviderConfig, legacyAuth LLMAuthConfig) bool {
	if !llmAuthConfigDefined(legacyAuth) {
		return false
	}

	normalizedProviderCfg := normalizeLLMProviderConfig(provider, providerCfg)
	if normalizedProviderCfg.Auth.None {
		return false
	}
	if !llmAuthConfigDefined(providerCfg.Auth) {
		return true
	}

	defaultProviderCfg, ok := defaultLLMProviders()[provider]
	if !ok {
		return false
	}
	defaultAuth := normalizeLLMProviderConfig(provider, defaultProviderCfg).Auth
	return reflect.DeepEqual(normalizedProviderCfg.Auth, defaultAuth)
}

func llmProviderConfigured(name string, providerCfg LLMProviderConfig) bool {
	providerCfg = normalizeLLMProviderConfig(name, providerCfg)
	if strings.TrimSpace(providerCfg.Model) == "" {
		return false
	}
	if providerCfg.Auth.None {
		return true
	}

	switch strings.ToLower(strings.TrimSpace(providerCfg.Auth.Mode)) {
	case llmAuthModeLiteral:
		return strings.TrimSpace(providerCfg.Auth.Value) != ""
	case llmAuthModeCommand:
		return strings.TrimSpace(providerCfg.Auth.Command) != ""
	default:
		return strings.TrimSpace(providerCfg.Auth.EnvVar) != ""
	}
}

func effectiveLLMProvider(cfg *AppConfig) (string, LLMProviderConfig, bool) {
	if cfg == nil || !cfg.LLM.Enabled {
		return "", LLMProviderConfig{}, false
	}
	normalizeLLMConfig(cfg)

	provider := strings.TrimSpace(cfg.LLM.Provider)
	if provider != "" && !strings.EqualFold(provider, "auto") {
		providerCfg, ok := cfg.LLM.Providers[provider]
		if !ok {
			return "", LLMProviderConfig{}, false
		}
		return provider, normalizeLLMProviderConfig(provider, providerCfg), true
	}

	for _, candidate := range cfg.LLM.PreferredOrder {
		providerCfg, ok := cfg.LLM.Providers[candidate]
		if !ok {
			continue
		}
		providerCfg = normalizeLLMProviderConfig(candidate, providerCfg)
		if strings.TrimSpace(providerCfg.Model) == "" {
			continue
		}
		return candidate, providerCfg, true
	}

	return "", LLMProviderConfig{}, false
}

func effectiveLLMProviderForTask(cfg *AppConfig, task string) (string, LLMProviderConfig, bool) {
	name, providerCfg, ok := effectiveLLMProvider(cfg)
	if !ok {
		return "", LLMProviderConfig{}, false
	}
	if model := llmModelForTask(providerCfg.Models, task); model != "" {
		providerCfg.Model = model
	}
	return name, providerCfg, true
}

func llmModelForTask(models map[string]string, task string) string {
	task = normalizeLLMTaskKey(task)
	if task == "" || len(models) == 0 {
		return ""
	}
	if model := strings.TrimSpace(models[task]); model != "" {
		return model
	}
	return ""
}

func normalizeLLMTaskKey(task string) string {
	task = strings.ToLower(strings.TrimSpace(task))
	task = strings.ReplaceAll(task, "-", "_")
	task = strings.ReplaceAll(task, " ", "_")
	switch task {
	case "default":
		return "default"
	case "filter", "filters", "filtering", "job_filter", "job_filter_batch", "job_filtering", "auto_filter", "llm_job_filtering":
		return llmTaskFiltering
	case "identity", "job_identity", "job_enrichment", "identity_enrichment", "company_identity":
		return llmTaskJobIdentity
	case "resume", "resume_criteria", "criteria", "criteria_generation":
		return llmTaskResumeCriteria
	case "health", "health_check", "health_checks", "company_health", "llm_company_health", "company_health_summary":
		return llmTaskCompanyHealth
	case "search", "job_search", "llm_search", "auto_search", "autonomous_search", "autonomous_job_search", "llm_job_search":
		return llmTaskJobSearch
	default:
		return task
	}
}

func llmAuthConfigured(cfg *AppConfig) bool {
	name, providerCfg, ok := effectiveLLMProvider(cfg)
	if !ok {
		return false
	}
	return llmProviderConfigured(name, providerCfg)
}

func LLMAuthConfigured(cfg *AppConfig) bool {
	return llmAuthConfigured(cfg)
}

func resolveLLMAuth(cfg *AppConfig) (string, string, error) {
	if cfg == nil {
		return "", "", fmt.Errorf("app config is nil")
	}

	name, providerCfg, ok := effectiveLLMProvider(cfg)
	if !ok {
		return "", "", fmt.Errorf("no effective llm provider is configured")
	}
	targetEnv := envVarForProvider(name)
	if providerCfg.Auth.None {
		return "", targetEnv, nil
	}
	switch strings.ToLower(strings.TrimSpace(providerCfg.Auth.Mode)) {
	case llmAuthModeLiteral:
		value := strings.TrimSpace(providerCfg.Auth.Value)
		if value == "" {
			return "", targetEnv, fmt.Errorf("llm auth literal value is not set")
		}
		return value, targetEnv, nil
	case llmAuthModeCommand:
		command := strings.TrimSpace(providerCfg.Auth.Command)
		if command == "" {
			return "", targetEnv, fmt.Errorf("llm auth command is not set")
		}
		cmd := exec.Command("bash", "-lc", command)
		output, err := cmd.Output()
		if err != nil {
			return "", targetEnv, fmt.Errorf("llm auth command failed: %w", err)
		}
		value := strings.TrimSpace(string(output))
		if value == "" {
			return "", targetEnv, fmt.Errorf("llm auth command returned an empty value")
		}
		return value, targetEnv, nil
	default:
		envVar := strings.TrimSpace(providerCfg.Auth.EnvVar)
		if envVar == "" {
			return "", targetEnv, fmt.Errorf("llm auth env var is not set")
		}
		value := strings.TrimSpace(os.Getenv(envVar))
		if value == "" {
			return "", targetEnv, fmt.Errorf("%s environment variable is not set", envVar)
		}
		return value, targetEnv, nil
	}
}

func applyResolvedLLMAuth(cfg *AppConfig) (func(), error) {
	value, targetEnv, err := resolveLLMAuth(cfg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(value) == "" {
		return func() {}, nil
	}

	previousValue, hadPrevious := os.LookupEnv(targetEnv)
	if err := os.Setenv(targetEnv, value); err != nil {
		return nil, err
	}

	return func() {
		if hadPrevious {
			_ = os.Setenv(targetEnv, previousValue)
			return
		}
		_ = os.Unsetenv(targetEnv)
	}, nil
}

func initConfiguredLLM(ctx context.Context, appCfg *AppConfig) (llms.Model, func(), error) {
	return initConfiguredLLMForTask(ctx, appCfg, "")
}

func initConfiguredLLMForTask(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error) {
	if appCfg == nil {
		return nil, nil, fmt.Errorf("app config is nil")
	}
	name, providerCfg, ok := effectiveLLMProviderForTask(appCfg, task)
	if !ok {
		return nil, nil, fmt.Errorf("no effective llm provider is configured")
	}
	restore, err := applyResolvedLLMAuth(appCfg)
	if err != nil {
		if providerCfg.Auth.None {
			restore = func() {}
		} else {
			return nil, nil, err
		}
	}
	if restore == nil {
		restore = func() {}
	}

	model, err := initLLM(ctx, name, providerCfg)
	if err != nil {
		restore()
		return nil, nil, err
	}
	return model, cleanupConfiguredLLM(model, restore), nil
}

type closeableLLM interface {
	Close() error
}

func cleanupConfiguredLLM(model llms.Model, restoreAuth func()) func() {
	return func() {
		if closeable, ok := model.(closeableLLM); ok {
			if err := closeable.Close(); err != nil {
				logDebug("llm cleanup: close failed: %v", err)
			}
		}
		if restoreAuth != nil {
			restoreAuth()
		}
	}
}

func InitConfiguredLLM(ctx context.Context, appCfg *AppConfig) (llms.Model, func(), error) {
	return initConfiguredLLM(ctx, appCfg)
}

func InitConfiguredLLMForTask(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error) {
	return initConfiguredLLMForTask(ctx, appCfg, task)
}
