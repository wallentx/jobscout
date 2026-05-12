package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"

	"gopkg.in/yaml.v3"
)

type RSSSource struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type RSSSourcesConfig struct {
	Enabled bool        `yaml:"enabled"`
	Feeds   []RSSSource `yaml:"feeds"`
}

type APISource struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	URL     string `yaml:"url"`
	Enabled bool   `yaml:"enabled"`
}

type SiteSearchConfig struct {
	Enabled bool     `yaml:"enabled"`
	Sites   []string `yaml:"sites"`
}

type LLMWebSearchConfig struct {
	Enabled bool     `yaml:"enabled"`
	Targets []string `yaml:"targets"`
}

type SourcesConfig struct {
	Enabled         bool               `yaml:"enabled"`
	BuiltinsEnabled bool               `yaml:"builtins_enabled,omitempty"`
	RSS             RSSSourcesConfig   `yaml:"rss"`
	APIs            []APISource        `yaml:"apis"`
	SiteSearch      SiteSearchConfig   `yaml:"site_search"`
	LLMWeb          LLMWebSearchConfig `yaml:"llm_web,omitempty"`
	Remotive        struct {
		Enabled bool   `yaml:"enabled"`
		URL     string `yaml:"url"`
	} `yaml:"remotive,omitempty"`
}

type LLMAuthConfig struct {
	Mode    string `yaml:"mode,omitempty"`
	EnvVar  string `yaml:"env_var,omitempty"`
	Value   string `yaml:"value,omitempty"`
	Command string `yaml:"command,omitempty"`
	None    bool   `yaml:"none,omitempty"`
}

type LLMProviderConfig struct {
	Model       string                 `yaml:"model,omitempty"`
	Models      map[string]string      `yaml:"models,omitempty"`
	Endpoint    string                 `yaml:"endpoint,omitempty"`
	Temperature *float32               `yaml:"temperature,omitempty"`
	MaxTokens   *int                   `yaml:"max_tokens,omitempty"`
	Timeout     string                 `yaml:"timeout,omitempty"`
	Auth        LLMAuthConfig          `yaml:"auth,omitempty"`
	Options     map[string]interface{} `yaml:"options,omitempty"`
}

type LLMConfig struct {
	Enabled          bool                         `yaml:"enabled,omitempty"`
	Provider         string                       `yaml:"provider"`
	Model            string                       `yaml:"model,omitempty"`
	Models           map[string]string            `yaml:"models,omitempty"`
	JobSearch        bool                         `yaml:"llm_job_search"`
	JobFiltering     bool                         `yaml:"llm_job_filtering"`
	CompanyHealth    bool                         `yaml:"llm_company_health,omitempty"`
	FallbackToNonLLM bool                         `yaml:"fallback_to_non_llm,omitempty"`
	PreferredOrder   []string                     `yaml:"preferred_order,omitempty"`
	Auth             LLMAuthConfig                `yaml:"auth,omitempty"`
	Providers        map[string]LLMProviderConfig `yaml:"providers,omitempty"`
}

type AppConfig struct {
	Sources  SourcesConfig  `yaml:"sources"`
	Criteria CriteriaConfig `yaml:"criteria,omitempty"`
	LLM      LLMConfig      `yaml:"llm"`
	UI       struct {
		DefaultFilterStatuses []string `yaml:"default_filter_statuses"`
	} `yaml:"ui"`
}

type WorkSettingsConfig = domain.WorkSettingsConfig
type CriteriaConfig = domain.CriteriaConfig
type RoleFamilyID = domain.RoleFamilyID

const (
	llmAuthModeEnv     = "env"
	llmAuthModeLiteral = "literal"
	llmAuthModeCommand = "command"

	llmTaskJobSearch        = "llm_job_search"
	llmTaskCompanyHealth    = "llm_company_health"
	llmTaskFiltering        = "llm_job_filtering"
	llmTaskJobIdentity      = "job_identity"
	llmTaskResumeCriteria   = "resume_to_criteria"
	llmTaskBenchmarkDefault = "benchmark"
)

const (
	RoleFrontendEngineering  = domain.RoleFrontendEngineering
	RoleBackendEngineering   = domain.RoleBackendEngineering
	RoleFullStackEngineering = domain.RoleFullStackEngineering
	RoleDevOpsSRESystems     = domain.RoleDevOpsSRESystems
	RoleAIMLEngineering      = domain.RoleAIMLEngineering
	RoleData                 = domain.RoleData
	RoleDesign               = domain.RoleDesign
	RoleProductManagement    = domain.RoleProductManagement
	RoleOtherSpecialized     = domain.RoleOtherSpecialized
)

func defaultAppConfig() AppConfig {
	var cfg AppConfig

	cfg.Sources.Enabled = true
	cfg.Sources.BuiltinsEnabled = true
	cfg.Sources.RSS.Enabled = true
	cfg.Sources.RSS.Feeds = []RSSSource{}
	cfg.Sources.APIs = []APISource{}
	cfg.Sources.SiteSearch.Enabled = true
	cfg.Sources.SiteSearch.Sites = []string{
		"https://www.indeed.com/jobs",
		"https://www.linkedin.com/jobs/search",
		"https://www.ycombinator.com/jobs",
		"https://himalayas.app/jobs",
		"https://builtin.com/jobs/remote",
	}
	cfg.Sources.LLMWeb.Enabled = false
	cfg.Sources.LLMWeb.Targets = []string{
		"site:job-boards.greenhouse.io",
		"site:jobs.lever.co",
		"site:myworkdayjobs.com",
		"site:jobs.ashbyhq.com",
		"site:careers.smartrecruiters.com",
		"site:api.smartrecruiters.com",
		"site:jobs.icims.com",
		"site:careers-*.icims.com",
		"site:*.bamboohr.com/jobs",
	}

	cfg.LLM.Enabled = true
	cfg.LLM.Provider = "gemini"
	cfg.LLM.Model = "gemini-2.5-flash-lite"
	cfg.LLM.JobSearch = true
	cfg.LLM.JobFiltering = true
	cfg.LLM.CompanyHealth = true
	cfg.LLM.FallbackToNonLLM = true
	cfg.LLM.PreferredOrder = []string{"gemini", "openai", "openrouter", "anthropic", "ollama"}
	cfg.LLM.Auth.Mode = llmAuthModeEnv
	cfg.LLM.Auth.EnvVar = envVarForProvider(cfg.LLM.Provider)
	cfg.LLM.Providers = defaultLLMProviders()
	cfg.UI.DefaultFilterStatuses = []string{}

	return cfg
}

func DefaultAppConfig() AppConfig {
	return defaultAppConfig()
}

func defaultCriteriaConfig() CriteriaConfig {
	var cfg CriteriaConfig

	return cfg
}

func DefaultCriteriaConfig() CriteriaConfig {
	return defaultCriteriaConfig()
}

func defaultLLMProviders() map[string]LLMProviderConfig {
	return map[string]LLMProviderConfig{
		"gemini": {
			Model: "gemini-2.5-flash-lite",
			Models: map[string]string{
				llmTaskJobSearch:        "gemini-2.5-flash-lite",
				llmTaskCompanyHealth:    "gemini-flash-lite-latest",
				llmTaskFiltering:        "gemini-2.5-flash-lite",
				llmTaskJobIdentity:      "gemini-2.5-flash-lite",
				llmTaskResumeCriteria:   "gemini-2.5-flash-lite",
				llmTaskBenchmarkDefault: "gemini-2.5-flash-lite",
			},
			Auth: LLMAuthConfig{
				Mode:   llmAuthModeEnv,
				EnvVar: "GEMINI_API_KEY",
			},
		},
		"openai": {
			Model: "gpt-4.1",
			Models: map[string]string{
				llmTaskJobSearch:        "gpt-4.1",
				llmTaskCompanyHealth:    "gpt-4o-2024-11-20",
				llmTaskFiltering:        "gpt-4.1",
				llmTaskJobIdentity:      "gpt-4.1",
				llmTaskResumeCriteria:   "gpt-5.3-chat-latest",
				llmTaskBenchmarkDefault: "gpt-4.1",
			},
			Auth: LLMAuthConfig{
				Mode:   llmAuthModeEnv,
				EnvVar: "OPENAI_API_KEY",
			},
		},
		"anthropic": {
			Model: "claude-3-5-sonnet-latest",
			Auth: LLMAuthConfig{
				Mode:   llmAuthModeEnv,
				EnvVar: "ANTHROPIC_API_KEY",
			},
		},
		"openrouter": {
			Model:    "openai/gpt-4o",
			Endpoint: "https://openrouter.ai/api/v1",
			Models: map[string]string{
				llmTaskJobSearch:        "openai/gpt-4o",
				llmTaskCompanyHealth:    "openai/gpt-4o",
				llmTaskFiltering:        "openai/gpt-4o",
				llmTaskJobIdentity:      "openai/gpt-4o",
				llmTaskResumeCriteria:   "openai/gpt-4o",
				llmTaskBenchmarkDefault: "openai/gpt-4o",
			},
			Auth: LLMAuthConfig{
				Mode:   llmAuthModeEnv,
				EnvVar: "OPENROUTER_API_KEY",
			},
		},
		"ollama": {
			Model:    "llama3",
			Endpoint: "http://localhost:11434",
			Auth: LLMAuthConfig{
				None: true,
			},
		},
	}
}

func DefaultLLMProviders() map[string]LLMProviderConfig {
	return defaultLLMProviders()
}

func saveAppConfig(path string, cfg *AppConfig) error {
	if setInMemoryAppConfig(cfg) {
		return nil
	}
	normalizeLLMConfig(cfg)
	cfgToSave := *cfg
	cfgToSave.LLM.Model = ""
	cfgToSave.LLM.Auth = LLMAuthConfig{}
	if len(cfgToSave.LLM.Providers) > 0 {
		providers := make(map[string]LLMProviderConfig, len(cfgToSave.LLM.Providers))
		for provider, providerCfg := range cfgToSave.LLM.Providers {
			providerCfg.Auth = sanitizeLLMAuthForSave(provider, providerCfg.Auth)
			providers[provider] = providerCfg
		}
		cfgToSave.LLM.Providers = providers
	}
	data, err := yaml.Marshal(&cfgToSave)
	if err != nil {
		return err
	}

	if err := ensureParentDir(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func SaveAppConfig(path string, cfg *AppConfig) error {
	return saveAppConfig(path, cfg)
}

func sanitizeLLMAuthForSave(provider string, auth LLMAuthConfig) LLMAuthConfig {
	// If the user chooses to input a literal API key, we should NOT save the
	// plaintext value to disk for security reasons. Instead, we switch the config
	// back to 'env' mode and use the default env var for that provider, so the
	// user will be prompted to set the environment variable on next launch.
	if strings.EqualFold(strings.TrimSpace(auth.Mode), llmAuthModeLiteral) {
		return LLMAuthConfig{
			Mode:   llmAuthModeEnv,
			EnvVar: envVarForProvider(provider),
		}
	}

	// For 'env' and 'command' modes, clear the literal value field just in case
	auth.Value = ""
	return auth
}

func configHasSection(data []byte, key string) (bool, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, err
	}
	_, ok := raw[key]
	return ok, nil
}

func loadCriteriaSectionFromConfigData(data []byte) (*CriteriaConfig, bool, error) {
	var raw struct {
		Criteria CriteriaConfig `yaml:"criteria"`
		Search   CriteriaConfig `yaml:"search"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, false, err
	}

	hasCriteria, err := configHasSection(data, "criteria")
	if err != nil {
		return nil, false, err
	}
	if hasCriteria {
		return &raw.Criteria, true, nil
	}

	hasSearch, err := configHasSection(data, "search")
	if err != nil {
		return nil, false, err
	}
	if hasSearch {
		return &raw.Search, true, nil
	}

	return nil, false, nil
}

func loadAppConfig(path string) (*AppConfig, error) {
	if cfg, ok := inMemoryAppConfig(); ok {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := defaultAppConfig()
	cfg.Sources.Enabled = true
	cfg.Sources.BuiltinsEnabled = true
	cfg.Sources.RSS.Enabled = true
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	if len(cfg.Sources.APIs) == 0 && cfg.Sources.Remotive.Enabled {
		cfg.Sources.APIs = append(cfg.Sources.APIs, APISource{
			Name:    "Remotive",
			Type:    "remotive",
			URL:     cfg.Sources.Remotive.URL,
			Enabled: true,
		})
	}
	disableExperimentalFetchSources(&cfg)
	if criteria, ok, err := loadCriteriaSectionFromConfigData(data); err != nil {
		return nil, err
	} else if ok {
		cfg.Criteria = *criteria
	}
	normalizeLLMConfig(&cfg)
	return &cfg, nil
}

func LoadAppConfig(path string) (*AppConfig, error) {
	return loadAppConfig(path)
}

// Experimental fetch sources are opt-in through --sources; keep their definitions
// available for explicit test runs without activating them during normal refreshes.
func disableExperimentalFetchSources(cfg *AppConfig) {
	if cfg == nil {
		return
	}
	cfg.Sources.LLMWeb.Enabled = false
	for i := range cfg.Sources.APIs {
		cfg.Sources.APIs[i].Enabled = false
	}
}

func loadCriteriaConfig(path string) (*CriteriaConfig, error) {
	if cfg, ok := inMemoryCriteriaConfig(); ok {
		return cfg, nil
	}

	configPath := strings.TrimSpace(path)
	if configPath == "" {
		configPath = runtimeConfigPath
	}
	cfg, err := loadAppConfig(configPath)
	if err != nil {
		return nil, err
	}
	if hasCriteria, err := func() (bool, error) {
		data, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return false, readErr
		}
		hasCriteria, hasCriteriaErr := configHasSection(data, "criteria")
		if hasCriteriaErr != nil {
			return false, hasCriteriaErr
		}
		if hasCriteria {
			return true, nil
		}
		return configHasSection(data, "search")
	}(); err != nil {
		return nil, err
	} else if hasCriteria {
		return &cfg.Criteria, nil
	}

	return nil, os.ErrNotExist
}

func LoadCriteriaConfig(path string) (*CriteriaConfig, error) {
	return loadCriteriaConfig(path)
}

func selectedWorkSettings(settings WorkSettingsConfig) []string {
	values := make([]string, 0, 3)
	if settings.Remote {
		values = append(values, "remote")
	}
	if settings.Hybrid {
		values = append(values, "hybrid")
	}
	if settings.Onsite {
		values = append(values, "onsite")
	}
	return values
}

func defaultSearchPrompt(criteria *CriteriaConfig) string {
	var titleRequires []string
	var titleIncludes []string
	var workSettings []string
	var roleFamilies []string
	minBase := 0
	prioritySignals := []string{}

	if criteria != nil {
		titleRequires = criteria.Filters.TitleRequires
		titleIncludes = criteria.Filters.TitleIncludes
		workSettings = selectedWorkSettings(criteria.Filters.WorkSettings)
		roleFamilies = make([]string, 0, len(criteria.RoleFamilies))
		for _, roleFamily := range domain.NormalizeRoleFamilies(criteria.RoleFamilies) {
			roleFamilies = append(roleFamilies, domain.RoleFamilyLabel(roleFamily))
		}
		minBase = criteria.Filters.MinBaseUSD
		prioritySignals = criteria.PrioritySignals
	}

	var builder strings.Builder
	builder.WriteString("# Search Prompt\n\n")
	builder.WriteString("Find current job openings that match this candidate profile.\n\n")
	builder.WriteString("## Hard Requirements\n")
	fmt.Fprintf(&builder, "- Required title prefixes/levels: %s\n", joinOrFallback(titleRequires, "none specified"))
	fmt.Fprintf(&builder, "- Target title names: %s\n", joinOrFallback(titleIncludes, "none specified"))
	fmt.Fprintf(&builder, "- Work settings: %s\n", joinOrFallback(workSettings, "none specified"))
	fmt.Fprintf(&builder, "- Role families: %s\n", joinOrFallback(roleFamilies, "none specified"))
	if minBase > 0 {
		fmt.Fprintf(&builder, "- Minimum base compensation: $%d USD\n", minBase)
	} else {
		builder.WriteString("- Minimum base compensation: not specified\n")
	}
	builder.WriteString("\n## Priority Signals\n")
	fmt.Fprintf(&builder, "- Prioritize roles mentioning: %s\n", joinOrFallback(prioritySignals, "none specified"))
	builder.WriteString("\n## Output Instructions\n")
	builder.WriteString("- Return current opportunities only.\n")
	builder.WriteString("- Prefer direct application links.\n")
	builder.WriteString("- If available, include the actual company's website, not the job board or application host.\n")
	builder.WriteString("- If available, include a brief factual summary of what the company does.\n")
	builder.WriteString("- If available, include the company's industry.\n")
	builder.WriteString("- Include enough detail to explain why each role matches.\n")
	builder.WriteString("- Return each result with company, title, remote, compensation, apply_url, description, company_website, company_summary, company_industry, and why_matches fields.\n")

	return builder.String()
}

func DefaultSearchPrompt(criteria *CriteriaConfig) string {
	return defaultSearchPrompt(criteria)
}

func saveSearchPrompt(path string, content string) error {
	if setInMemorySearchPrompt(content) {
		return nil
	}
	if err := ensureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0600)
}

func SaveSearchPrompt(path string, content string) error {
	return saveSearchPrompt(path, content)
}

func loadSearchPrompt(path string) (string, error) {
	if prompt, ok := inMemorySearchPrompt(); ok {
		return prompt, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func LoadSearchPrompt(path string) (string, error) {
	return loadSearchPrompt(path)
}

func searchPromptPresent(path string) bool {
	if prompt, ok := inMemorySearchPrompt(); ok {
		return strings.TrimSpace(prompt) != ""
	}
	_, err := os.Stat(path)
	return err == nil
}

func SearchPromptPresent(path string) bool {
	return searchPromptPresent(path)
}
