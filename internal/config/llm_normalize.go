package config

import (
	"reflect"
	"strings"
)

func defaultLLMPreferredOrder() []string {
	return []string{"gemini", "openai", "openrouter", "anthropic", "ollama"}
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

func NormalizeLLMProviderConfig(name string, providerCfg LLMProviderConfig) LLMProviderConfig {
	return normalizeLLMProviderConfig(name, providerCfg)
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
	if legacyTopLevelAuthBelongsToDifferentProvider(provider, legacyAuth) {
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

func legacyTopLevelAuthBelongsToDifferentProvider(provider string, legacyAuth LLMAuthConfig) bool {
	mode := strings.ToLower(strings.TrimSpace(legacyAuth.Mode))
	if mode != "" && mode != llmAuthModeEnv {
		return false
	}
	envVar := strings.TrimSpace(legacyAuth.EnvVar)
	if envVar == "" {
		return false
	}
	for name := range defaultLLMProviders() {
		if strings.EqualFold(name, provider) {
			continue
		}
		if envVar == envVarForProvider(name) {
			return true
		}
	}
	return false
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

func NormalizeLLMTaskKey(task string) string {
	return normalizeLLMTaskKey(task)
}

func NormalizeLLMConfig(cfg *AppConfig) {
	normalizeLLMConfig(cfg)
}
