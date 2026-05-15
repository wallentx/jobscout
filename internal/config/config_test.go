package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCriteriaSampleLoadsFromConfigYAML(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "criteria-sample.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(criteria-sample.yaml) error = %v", err)
	}
	var criteria CriteriaConfig
	if err := yaml.Unmarshal(data, &criteria); err != nil {
		t.Fatalf("Unmarshal(criteria-sample.yaml) error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := defaultAppConfig()
	cfg.Criteria = criteria
	if err := saveAppConfig(path, &cfg); err != nil {
		t.Fatalf("saveAppConfig(%q) error = %v", path, err)
	}

	loaded, err := loadAppConfig(path)
	if err != nil {
		t.Fatalf("loadAppConfig(%q) error = %v", path, err)
	}
	if !loaded.Criteria.Filters.WorkSettings.Remote {
		t.Fatal("criteria-sample.yaml remote work setting = false; want true")
	}
}

func TestSaveAppConfigUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("existing: true\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	cfg := defaultAppConfig()
	cfg.LLM.Provider = "openai"
	cfg.LLM.Providers["openai"] = LLMProviderConfig{
		Model: "gpt-5.3-chat",
		Auth: LLMAuthConfig{
			Mode:  llmAuthModeLiteral,
			Value: "secret-token",
		},
	}

	if err := saveAppConfig(path, &cfg); err != nil {
		t.Fatalf("saveAppConfig() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("config permissions = %v; want 0600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if strings.Contains(string(data), "secret-token") {
		t.Fatalf("saved config contains literal API token")
	}
	if strings.Contains(string(data), "mode: literal") {
		t.Fatalf("saved config contains literal auth mode")
	}
}

func TestModelOptionsForProviderIncludesCurrentOpenAITextModels(t *testing.T) {
	options := ModelOptionsForProvider("openai")
	for _, model := range []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.4-nano", "gpt-5-mini", "gpt-5-nano", "gpt-5.3-chat", "gpt-5.2-chat", "gpt-4o", "gpt-4o-mini", "o3"} {
		if !slices.Contains(options, model) {
			t.Fatalf("ModelOptionsForProvider(openai) missing %q in %#v", model, options)
		}
	}
	for _, model := range []string{"gpt-5.5-pro", "gpt-5.4-pro", "gpt-5.2-pro", "gpt-5-pro", "gpt-4o-2024-11-20", "gpt-4.1-nano-2025-04-14", "gpt-4.1-nano", "gpt-4.5-preview", "gpt-4-turbo", "gpt-3.5-turbo", "chat-latest", "gpt-5.1-chat", "gpt-5-chat-latest", "o1", "o3-mini", "o4-mini"} {
		if slices.Contains(options, model) {
			t.Fatalf("ModelOptionsForProvider(openai) included blocked model %q in %#v", model, options)
		}
	}

	options[0] = "mutated"
	if got := ModelOptionsForProvider("openai")[0]; got == "mutated" {
		t.Fatalf("ModelOptionsForProvider(openai) returned mutable shared backing array")
	}
}

func TestGeminiModelDefaultsFollowDeprecationGuidance(t *testing.T) {
	cfg := defaultAppConfig()
	gemini := cfg.LLM.Providers["gemini"]

	if gemini.Model != "gemini-3.1-flash-lite" {
		t.Fatalf("default Gemini model = %q, want gemini-3.1-flash-lite", gemini.Model)
	}
	if got := DefaultModelForProvider("gemini"); got != "gemini-3.1-flash-lite" {
		t.Fatalf("DefaultModelForProvider(gemini) = %q, want gemini-3.1-flash-lite", got)
	}

	for task, got := range gemini.Models {
		if got != "gemini-3.1-flash-lite" {
			t.Fatalf("Gemini Models[%q] = %q, want gemini-3.1-flash-lite", task, got)
		}
	}

	options := ModelOptionsForProvider("gemini")
	for _, model := range []string{"gemini-3.1-flash-lite", "gemini-3.1-pro-preview", "gemini-3-flash-preview"} {
		if !slices.Contains(options, model) {
			t.Fatalf("ModelOptionsForProvider(gemini) missing %q in %#v", model, options)
		}
	}
	for _, model := range []string{"gemini-2.5-flash-lite", "gemini-2.5-flash", "gemini-2.5-pro", "gemini-3.1-flash-lite-preview", "gemini-3-pro-preview", "gemini-flash-lite-latest", "gemini-pro-latest"} {
		if slices.Contains(options, model) {
			t.Fatalf("ModelOptionsForProvider(gemini) included deprecated model %q in %#v", model, options)
		}
	}
}

func TestOpenAIModelDefaultsUseBenchmarkRecommendations(t *testing.T) {
	cfg := defaultAppConfig()
	openAI := cfg.LLM.Providers["openai"]

	if openAI.Model != "gpt-4o-mini" {
		t.Fatalf("default OpenAI model = %q, want gpt-4o-mini", openAI.Model)
	}
	if got := DefaultModelForProvider("openai"); got != "gpt-4o-mini" {
		t.Fatalf("DefaultModelForProvider(openai) = %q, want gpt-4o-mini", got)
	}

	wantTaskModels := map[string]string{
		llmTaskJobSearch:      "gpt-4o-mini",
		llmTaskCompanyHealth:  "gpt-4o-mini",
		llmTaskFiltering:      "gpt-5.4-mini",
		llmTaskJobIdentity:    "gpt-4o-mini",
		llmTaskResumeCriteria: "gpt-4o-mini",
	}
	for task, want := range wantTaskModels {
		if got := openAI.Models[task]; got != want {
			t.Fatalf("OpenAI Models[%q] = %q, want %q", task, got, want)
		}
	}
}

func TestOpenAIModelOptionsPrioritizeBenchmarkRecommendations(t *testing.T) {
	options := ModelOptionsForProvider("openai")
	wantPrefix := []string{"gpt-4o-mini", "gpt-5.4-mini", "gpt-5.4-nano", "gpt-4.1-mini", "gpt-4o"}
	if len(options) < len(wantPrefix) {
		t.Fatalf("ModelOptionsForProvider(openai) len = %d, want at least %d", len(options), len(wantPrefix))
	}
	for i, want := range wantPrefix {
		if options[i] != want {
			t.Fatalf("ModelOptionsForProvider(openai)[%d] = %q in %#v, want %q", i, options[i], options, want)
		}
	}
}

func TestDefaultSiteSearchSitesAreSearchableTargets(t *testing.T) {
	cfg := defaultAppConfig()
	blocked := []string{
		"boards.greenhouse.io",
		"jobs.lever.co",
		"myworkdayjobs.com",
		"icims.com",
		"smartrecruiters.com",
	}
	for _, site := range cfg.Sources.SiteSearch.Sites {
		for _, bad := range blocked {
			if strings.EqualFold(strings.TrimSpace(site), bad) {
				t.Fatalf("defaultAppConfig().Sources.SiteSearch.Sites contains bare ATS host %q", site)
			}
		}
	}
	for _, want := range []string{
		"https://www.indeed.com/jobs",
		"https://www.linkedin.com/jobs/search",
		"https://www.ycombinator.com/jobs",
		"https://himalayas.app/jobs",
		"https://builtin.com/jobs/remote",
	} {
		if !containsString(cfg.Sources.SiteSearch.Sites, want) {
			t.Fatalf("defaultAppConfig().Sources.SiteSearch.Sites = %#v; want %q", cfg.Sources.SiteSearch.Sites, want)
		}
	}
	if cfg.Sources.LLMWeb.Enabled {
		t.Fatal("defaultAppConfig().Sources.LLMWeb.Enabled = true; want false")
	}
	for _, want := range []string{
		"site:job-boards.greenhouse.io",
		"site:jobs.lever.co",
		"site:myworkdayjobs.com",
		"site:jobs.ashbyhq.com",
		"site:careers.smartrecruiters.com",
		"site:api.smartrecruiters.com",
		"site:jobs.icims.com",
		"site:careers-*.icims.com",
		"site:*.bamboohr.com/jobs",
	} {
		if !containsString(cfg.Sources.LLMWeb.Targets, want) {
			t.Fatalf("defaultAppConfig().Sources.LLMWeb.Targets = %#v; want %q", cfg.Sources.LLMWeb.Targets, want)
		}
	}
}

func TestLoadAppConfigKeepsExperimentalSourcesInert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
sources:
  enabled: true
  apis:
    - name: Remotive
      type: remotive
      url: https://remotive.com/api/remote-jobs
      enabled: true
  llm_web:
    enabled: true
    targets:
      - site:jobs.lever.co
criteria:
  filters:
    title_includes:
      - engineer
llm:
  enabled: true
  provider: gemini
  llm_job_search: true
  llm_job_filtering: true
`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	cfg, err := loadAppConfig(path)
	if err != nil {
		t.Fatalf("loadAppConfig(%q) error = %v", path, err)
	}
	if cfg.Sources.LLMWeb.Enabled {
		t.Fatal("loadAppConfig(...).Sources.LLMWeb.Enabled = true; want false")
	}
	if len(cfg.Sources.LLMWeb.Targets) != 1 || cfg.Sources.LLMWeb.Targets[0] != "site:jobs.lever.co" {
		t.Fatalf("loadAppConfig(...).Sources.LLMWeb.Targets = %#v; want configured target retained", cfg.Sources.LLMWeb.Targets)
	}
	if len(cfg.Sources.APIs) != 1 {
		t.Fatalf("loadAppConfig(...).Sources.APIs len = %d; want 1", len(cfg.Sources.APIs))
	}
	if cfg.Sources.APIs[0].Enabled {
		t.Fatal("loadAppConfig(...).Sources.APIs[0].Enabled = true; want false")
	}

	ApplyFetchSourceSelection(cfg, []string{"api", "llm_web"})
	if !cfg.Sources.LLMWeb.Enabled {
		t.Fatal("ApplyFetchSourceSelection(loaded config).Sources.LLMWeb.Enabled = false; want true")
	}
	if !cfg.Sources.APIs[0].Enabled {
		t.Fatal("ApplyFetchSourceSelection(loaded config).Sources.APIs[0].Enabled = false; want true")
	}
}

func TestApplyFetchSourceSelectionLimitsFetchSources(t *testing.T) {
	cfg := defaultAppConfig()
	cfg.Sources.APIs = []APISource{
		{Name: "One", Type: "remotive", URL: "https://example.test/one", Enabled: false},
	}

	ApplyFetchSourceSelection(&cfg, []string{"rss", "site"})

	if !cfg.Sources.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.Enabled = false; want true")
	}
	if !cfg.Sources.RSS.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.RSS.Enabled = false; want true")
	}
	if !cfg.Sources.SiteSearch.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.SiteSearch.Enabled = false; want true")
	}
	if cfg.Sources.LLMWeb.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.LLMWeb.Enabled = true; want false")
	}
	if !cfg.Sources.BuiltinsEnabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.BuiltinsEnabled = false; want true")
	}
	if cfg.LLM.JobSearch {
		t.Fatal("ApplyFetchSourceSelection(...).LLM.JobSearch = true; want false")
	}
	if cfg.Sources.APIs[0].Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.APIs[0].Enabled = true; want false")
	}
}

func TestApplyFetchSourceSelectionCanSelectOnlyAPI(t *testing.T) {
	cfg := defaultAppConfig()
	cfg.Sources.APIs = []APISource{
		{Name: "One", Type: "remotive", URL: "https://example.test/one", Enabled: false},
	}

	ApplyFetchSourceSelection(&cfg, []string{"api"})

	if !cfg.Sources.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.Enabled = false; want true")
	}
	if cfg.Sources.RSS.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.RSS.Enabled = true; want false")
	}
	if cfg.Sources.SiteSearch.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.SiteSearch.Enabled = true; want false")
	}
	if cfg.Sources.LLMWeb.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.LLMWeb.Enabled = true; want false")
	}
	if cfg.Sources.BuiltinsEnabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.BuiltinsEnabled = true; want false")
	}
	if !cfg.Sources.APIs[0].Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.APIs[0].Enabled = false; want true")
	}
}

func TestApplyFetchSourceSelectionCanSelectOnlyLLMWeb(t *testing.T) {
	cfg := defaultAppConfig()
	cfg.LLM.Enabled = false

	ApplyFetchSourceSelection(&cfg, []string{"llm_web"})

	if !cfg.LLM.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).LLM.Enabled = false; want true")
	}
	if cfg.LLM.JobSearch {
		t.Fatal("ApplyFetchSourceSelection(...).LLM.JobSearch = true; want false")
	}
	if cfg.Sources.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.Enabled = true; want false")
	}
	if cfg.Sources.RSS.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.RSS.Enabled = true; want false")
	}
	if cfg.Sources.SiteSearch.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.SiteSearch.Enabled = true; want false")
	}
	if !cfg.Sources.LLMWeb.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.LLMWeb.Enabled = false; want true")
	}
	if cfg.Sources.BuiltinsEnabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.BuiltinsEnabled = true; want false")
	}
	if len(cfg.Sources.SiteSearch.Sites) != 0 {
		t.Fatalf("ApplyFetchSourceSelection(...).Sources.SiteSearch.Sites = %#v; want none", cfg.Sources.SiteSearch.Sites)
	}
}

func TestApplyFetchSourceSelectionCanSelectOnlyLLM(t *testing.T) {
	cfg := defaultAppConfig()
	cfg.LLM.Enabled = false

	ApplyFetchSourceSelection(&cfg, []string{"llm"})

	if !cfg.LLM.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).LLM.Enabled = false; want true")
	}
	if !cfg.LLM.JobSearch {
		t.Fatal("ApplyFetchSourceSelection(...).LLM.JobSearch = false; want true")
	}
	if cfg.Sources.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.Enabled = true; want false")
	}
	if cfg.Sources.RSS.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.RSS.Enabled = true; want false")
	}
	if cfg.Sources.SiteSearch.Enabled {
		t.Fatal("ApplyFetchSourceSelection(...).Sources.SiteSearch.Enabled = true; want false")
	}
}

func TestNormalizeLLMConfigPreservesExplicitDisabled(t *testing.T) {
	cfg := defaultAppConfig()
	cfg.LLM.Enabled = false
	cfg.LLM.JobSearch = true
	cfg.LLM.JobFiltering = true
	cfg.LLM.CompanyHealth = true

	normalizeLLMConfig(&cfg)

	if cfg.LLM.Enabled {
		t.Fatal("normalizeLLMConfig(...).LLM.Enabled = true; want false")
	}
}

func TestNormalizeLLMConfigDoesNotApplyOtherProviderTopLevelEnvAuth(t *testing.T) {
	cfg := defaultAppConfig()
	cfg.LLM.Provider = "openai"
	cfg.LLM.Auth = LLMAuthConfig{
		Mode:   llmAuthModeEnv,
		EnvVar: "GEMINI_API_KEY",
	}

	normalizeLLMConfig(&cfg)

	if got := cfg.LLM.Auth.EnvVar; got != "OPENAI_API_KEY" {
		t.Fatalf("normalizeLLMConfig(...).LLM.Auth.EnvVar = %q, want OPENAI_API_KEY", got)
	}
	if got := cfg.LLM.Providers["openai"].Auth.EnvVar; got != "OPENAI_API_KEY" {
		t.Fatalf("normalizeLLMConfig(...).Providers[openai].Auth.EnvVar = %q, want OPENAI_API_KEY", got)
	}
}

func TestEvaluateCapabilitiesFlagsDisabledLLMWithFeatureToggles(t *testing.T) {
	cfg := defaultAppConfig()
	cfg.LLM.Enabled = false
	cfg.LLM.JobSearch = true
	cfg.LLM.JobFiltering = true
	cfg.LLM.CompanyHealth = true
	cfg.Criteria.RoleFamilies = []RoleFamilyID{RoleBackendEngineering}

	caps := evaluateCapabilitiesForConfig(&cfg, true)

	if !caps.LLMDisabled {
		t.Fatal("evaluateCapabilitiesForConfig(...).LLMDisabled = false; want true")
	}
	if !caps.LLMFeaturesSelected {
		t.Fatal("evaluateCapabilitiesForConfig(...).LLMFeaturesSelected = false; want true")
	}
	want := "LLM feature toggles are enabled but llm.enabled is false"
	if !containsString(caps.SetupIssues, want) {
		t.Fatalf("evaluateCapabilitiesForConfig(...).SetupIssues = %#v; want %q", caps.SetupIssues, want)
	}
}

func TestDefaultLLMProvidersIncludesOpenRouter(t *testing.T) {
	providers := DefaultLLMProviders()
	cfg, ok := providers["openrouter"]
	if !ok {
		t.Fatal("DefaultLLMProviders() does not contain openrouter")
	}
	if cfg.Model != "deepseek/deepseek-v4-flash" {
		t.Fatalf("DefaultLLMProviders()[openrouter].Model = %q, want deepseek/deepseek-v4-flash", cfg.Model)
	}
	if cfg.Endpoint != "https://openrouter.ai/api/v1" {
		t.Fatalf("DefaultLLMProviders()[openrouter].Endpoint = %q, want https://openrouter.ai/api/v1", cfg.Endpoint)
	}
	if cfg.Auth.EnvVar != "OPENROUTER_API_KEY" {
		t.Fatalf("DefaultLLMProviders()[openrouter].Auth.EnvVar = %q, want OPENROUTER_API_KEY", cfg.Auth.EnvVar)
	}
}

func TestEnvVarForProviderReturnsOpenRouterKey(t *testing.T) {
	envVar := EnvVarForProvider("openrouter")
	if envVar != "OPENROUTER_API_KEY" {
		t.Fatalf("EnvVarForProvider(openrouter) = %q, want OPENROUTER_API_KEY", envVar)
	}
}

func TestDefaultModelForProviderReturnsFirstForOpenRouter(t *testing.T) {
	model := DefaultModelForProvider("openrouter")
	if model != "deepseek/deepseek-v4-flash" {
		t.Fatalf("DefaultModelForProvider(openrouter) = %q, want deepseek/deepseek-v4-flash", model)
	}
}

func TestEvaluateRuntimeCapabilitiesUsesConfiguredRuntimePath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	restore := ConfigureRuntime(configPath, filepath.Join(tmpDir, "SEARCH_PROMPT.md"))
	t.Cleanup(restore)

	cfg := defaultAppConfig()
	cfg.Criteria.RoleFamilies = []RoleFamilyID{RoleBackendEngineering}
	cfg.LLM.Enabled = false
	cfg.LLM.JobSearch = false
	cfg.LLM.JobFiltering = false
	cfg.LLM.CompanyHealth = false
	if err := saveAppConfig(configPath, &cfg); err != nil {
		t.Fatalf("saveAppConfig(%q) error = %v", configPath, err)
	}

	caps := EvaluateRuntimeCapabilities()
	if !caps.ConfigExists {
		t.Fatal("EvaluateRuntimeCapabilities().ConfigExists = false; want true")
	}
	if !caps.SearchProfileReady {
		t.Fatal("EvaluateRuntimeCapabilities().SearchProfileReady = false; want true")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
