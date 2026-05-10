package config

import (
	"os"
	"os/exec"
	"strings"
)

type RuntimeCapabilities struct {
	ConfigExists        bool
	SearchProfileReady  bool
	SearchSourcesReady  bool
	SearchPromptReady   bool
	LLMPreferred        bool
	LLMFeaturesSelected bool
	LLMDisabled         bool
	LLMConfigured       bool
	LLMAuthAvailableNow bool
	CanRunNonLLM        bool
	CanRunLLM           bool
	SetupIssues         []string
}

func evaluateRuntimeCapabilities() RuntimeCapabilities {
	cfg, err := loadAppConfig(runtimeConfigPath)
	if err != nil {
		return RuntimeCapabilities{
			SetupIssues: []string{"config.yaml is missing or invalid"},
		}
	}

	promptPresent := true
	if cfg.LLM.JobSearch {
		promptPresent = searchPromptPresent(runtimeSearchPromptPath)
	}

	return evaluateCapabilitiesForConfig(cfg, promptPresent)
}

func EvaluateRuntimeCapabilities() RuntimeCapabilities {
	return evaluateRuntimeCapabilities()
}

func evaluateCapabilitiesForConfig(cfg *AppConfig, promptPresent bool) RuntimeCapabilities {
	var caps RuntimeCapabilities
	if cfg == nil {
		caps.SetupIssues = append(caps.SetupIssues, "config.yaml is missing or invalid")
		return caps
	}

	caps.ConfigExists = true
	caps.SearchProfileReady = criteriaConfigReady(&cfg.Criteria)
	nonLLMSourcesReady := appConfigHasEnabledSources(cfg)
	llmWebSourcesReady := appConfigHasLLMWebSource(cfg)
	caps.SearchSourcesReady = nonLLMSourcesReady || llmWebSourcesReady
	caps.LLMFeaturesSelected = cfg.LLM.JobFiltering || cfg.LLM.JobSearch || cfg.LLM.CompanyHealth || cfg.Sources.LLMWeb.Enabled
	caps.LLMDisabled = !cfg.LLM.Enabled
	caps.LLMPreferred = cfg.LLM.Enabled && (cfg.LLM.JobFiltering || cfg.LLM.JobSearch || cfg.Sources.LLMWeb.Enabled)
	provider, providerCfg, providerOK := effectiveLLMProvider(cfg)
	caps.LLMConfigured = providerOK && strings.TrimSpace(provider) != "" && strings.TrimSpace(providerCfg.Model) != ""
	if caps.LLMConfigured {
		caps.LLMAuthAvailableNow = llmAuthAvailableNow(cfg)
	}
	caps.SearchPromptReady = !cfg.LLM.JobSearch || promptPresent

	caps.CanRunNonLLM = caps.ConfigExists && caps.SearchProfileReady && nonLLMSourcesReady
	caps.CanRunLLM = caps.LLMPreferred && caps.LLMConfigured && caps.LLMAuthAvailableNow && caps.SearchPromptReady && (cfg.LLM.JobSearch || cfg.Sources.LLMWeb.Enabled || caps.CanRunNonLLM)

	if !caps.SearchProfileReady {
		caps.SetupIssues = append(caps.SetupIssues, "search profile is incomplete in config.yaml")
	}
	if !caps.SearchSourcesReady {
		caps.SetupIssues = append(caps.SetupIssues, "no enabled search sources are configured")
	}
	if caps.LLMDisabled && caps.LLMFeaturesSelected {
		caps.SetupIssues = append(caps.SetupIssues, "LLM feature toggles are enabled but llm.enabled is false")
	}
	if caps.LLMPreferred && !caps.LLMConfigured {
		caps.SetupIssues = append(caps.SetupIssues, "LLM mode is enabled but provider/model are incomplete")
	}
	if cfg.LLM.JobSearch && !caps.SearchPromptReady {
		caps.SetupIssues = append(caps.SetupIssues, "SEARCH_PROMPT.md is required for LLM job search")
	}

	return caps
}

func EvaluateCapabilitiesForConfig(cfg *AppConfig, promptPresent bool) RuntimeCapabilities {
	return evaluateCapabilitiesForConfig(cfg, promptPresent)
}

func criteriaConfigReady(cfg *CriteriaConfig) bool {
	if cfg == nil {
		return false
	}
	return len(cfg.Filters.TitleRequires) > 0 || len(cfg.Filters.TitleIncludes) > 0 || len(cfg.RoleFamilies) > 0
}

func appConfigHasEnabledSources(cfg *AppConfig) bool {
	if cfg == nil || !cfg.Sources.Enabled {
		return false
	}
	if cfg.Sources.RSS.Enabled && (len(cfg.Sources.RSS.Feeds) > 0 || len(cfg.Criteria.RoleFamilies) > 0) {
		return true
	}
	for _, source := range cfg.Sources.APIs {
		if source.Enabled {
			return true
		}
	}
	return cfg.Sources.SiteSearch.Enabled && (len(cfg.Sources.SiteSearch.Sites) > 0 || len(cfg.Criteria.RoleFamilies) > 0)
}

func appConfigHasLLMWebSource(cfg *AppConfig) bool {
	return cfg != nil && cfg.Sources.LLMWeb.Enabled && len(cfg.Sources.LLMWeb.Targets) > 0
}

func effectiveLLMProvider(cfg *AppConfig) (string, LLMProviderConfig, bool) {
	if cfg == nil || !cfg.LLM.Enabled {
		return "", LLMProviderConfig{}, false
	}
	normalizeLLMConfig(cfg)
	provider := strings.TrimSpace(cfg.LLM.Provider)
	if provider != "" {
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
		if strings.TrimSpace(providerCfg.Model) != "" {
			return candidate, providerCfg, true
		}
	}
	return "", LLMProviderConfig{}, false
}

func llmAuthAvailableNow(cfg *AppConfig) bool {
	name, providerCfg, ok := effectiveLLMProvider(cfg)
	if !ok {
		return false
	}
	auth := normalizeLLMProviderConfig(name, providerCfg).Auth
	if auth.None {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(auth.Mode)) {
	case llmAuthModeEnv:
		return strings.TrimSpace(auth.EnvVar) != "" && strings.TrimSpace(os.Getenv(auth.EnvVar)) != ""
	case llmAuthModeLiteral:
		return strings.TrimSpace(auth.Value) != ""
	case llmAuthModeCommand:
		if strings.TrimSpace(auth.Command) == "" {
			return false
		}
		out, err := exec.Command("sh", "-c", auth.Command).Output()
		return err == nil && strings.TrimSpace(string(out)) != ""
	default:
		return false
	}
}

func LLMAuthAvailableNow(cfg *AppConfig) bool {
	return llmAuthAvailableNow(cfg)
}
