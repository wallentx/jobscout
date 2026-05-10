package config

import (
	"os"
	"strings"
	"sync"

	"github.com/wallentx/jobscout/internal/domain"

	"gopkg.in/yaml.v3"
)

type inMemoryRuntimeConfig struct {
	enabled      bool
	appConfig    AppConfig
	searchPrompt string
}

var inMemoryRuntime = struct {
	sync.RWMutex
	state inMemoryRuntimeConfig
}{}

func DemoCriteriaConfig() CriteriaConfig {
	cfg := defaultCriteriaConfig()
	cfg.Candidate.City = "Seattle"
	cfg.Candidate.State = "WA"
	cfg.Candidate.CountryCode = "US"
	cfg.Candidate.YearsOfExperience = 2
	cfg.Filters.TitleRequires = []string{}
	cfg.Filters.TitleIncludes = []string{
		"Software Engineer",
		"Software Developer",
		"Application Developer",
		"Frontend Developer",
		"Backend Developer",
		"Full Stack Developer",
	}
	cfg.Filters.TitleExcludes = []string{
		"manager",
		"lead",
		"senior",
		"sr.",
		"staff",
		"principal",
		"director",
		"architect",
	}
	cfg.Filters.WorkSettings.Remote = true
	cfg.Filters.WorkSettings.Hybrid = true
	cfg.Filters.WorkSettings.Onsite = true
	cfg.Filters.MaxDistanceMiles = 35
	cfg.Filters.MinBaseUSD = 85000
	cfg.RoleFamilies = []domain.RoleFamilyID{
		domain.RoleFrontendEngineering,
		domain.RoleBackendEngineering,
		domain.RoleFullStackEngineering,
	}
	cfg.PrioritySignals = []string{
		"JavaScript",
		"TypeScript",
		"React",
		"Node.js",
		"Go",
		"Python",
		"SQL",
		"REST APIs",
		"Git",
		"Docker",
		"AWS basics",
		"unit tests",
		"CI/CD",
	}
	return cfg
}

func DemoAppConfig() AppConfig {
	cfg := defaultAppConfig()
	provider := demoProvider()
	cfg.Criteria = DemoCriteriaConfig()
	cfg.Sources.Enabled = true
	cfg.Sources.BuiltinsEnabled = true
	cfg.Sources.RSS.Enabled = true
	cfg.Sources.SiteSearch.Enabled = true
	cfg.LLM.Enabled = true
	cfg.LLM.Provider = provider
	cfg.LLM.Model = defaultModelForProvider(provider)
	cfg.LLM.JobSearch = true
	cfg.LLM.JobFiltering = true
	cfg.LLM.CompanyHealth = true
	cfg.LLM.FallbackToNonLLM = true
	cfg.LLM.Auth = LLMAuthConfig{
		Mode:   llmAuthModeEnv,
		EnvVar: envVarForProvider(provider),
	}
	cfg.LLM.Providers = defaultLLMProviders()
	normalizeLLMConfig(&cfg)
	return cfg
}

func demoProvider() string {
	for _, provider := range []string{"gemini", "openai", "anthropic"} {
		if envVar := envVarForProvider(provider); strings.TrimSpace(envVar) != "" && strings.TrimSpace(os.Getenv(envVar)) != "" {
			return provider
		}
	}
	return "gemini"
}

func ConfigureInMemoryRuntime(appCfg AppConfig, searchPrompt string) func() {
	normalizeLLMConfig(&appCfg)
	searchPrompt = strings.TrimSpace(searchPrompt)
	if searchPrompt == "" {
		searchPrompt = defaultSearchPrompt(&appCfg.Criteria)
	}

	inMemoryRuntime.Lock()
	previous := inMemoryRuntime.state
	inMemoryRuntime.state = inMemoryRuntimeConfig{
		enabled:      true,
		appConfig:    cloneAppConfig(appCfg),
		searchPrompt: searchPrompt,
	}
	inMemoryRuntime.Unlock()

	return func() {
		inMemoryRuntime.Lock()
		inMemoryRuntime.state = previous
		inMemoryRuntime.Unlock()
	}
}

func inMemoryAppConfig() (*AppConfig, bool) {
	inMemoryRuntime.RLock()
	defer inMemoryRuntime.RUnlock()
	if !inMemoryRuntime.state.enabled {
		return nil, false
	}
	cfg := cloneAppConfig(inMemoryRuntime.state.appConfig)
	return &cfg, true
}

func setInMemoryAppConfig(cfg *AppConfig) bool {
	if cfg == nil {
		return false
	}
	cfgCopy := cloneAppConfig(*cfg)
	normalizeLLMConfig(&cfgCopy)

	inMemoryRuntime.Lock()
	defer inMemoryRuntime.Unlock()
	if !inMemoryRuntime.state.enabled {
		return false
	}
	inMemoryRuntime.state.appConfig = cfgCopy
	return true
}

func inMemoryCriteriaConfig() (*CriteriaConfig, bool) {
	cfg, ok := inMemoryAppConfig()
	if !ok {
		return nil, false
	}
	criteria := cfg.Criteria
	return &criteria, true
}

func inMemorySearchPrompt() (string, bool) {
	inMemoryRuntime.RLock()
	defer inMemoryRuntime.RUnlock()
	if !inMemoryRuntime.state.enabled {
		return "", false
	}
	return inMemoryRuntime.state.searchPrompt, true
}

func setInMemorySearchPrompt(content string) bool {
	inMemoryRuntime.Lock()
	defer inMemoryRuntime.Unlock()
	if !inMemoryRuntime.state.enabled {
		return false
	}
	inMemoryRuntime.state.searchPrompt = content
	return true
}

func cloneAppConfig(cfg AppConfig) AppConfig {
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return cfg
	}
	var out AppConfig
	if err := yaml.Unmarshal(data, &out); err != nil {
		return cfg
	}
	normalizeLLMConfig(&out)
	return out
}
