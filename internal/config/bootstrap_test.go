package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingRequiredFiles(t *testing.T) {
	tmpDir := t.TempDir()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q): %v", tmpDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	caps := evaluateRuntimeCapabilities()
	if caps.ConfigExists {
		t.Fatal("caps.ConfigExists = true; want false before config is written")
	}

	appCfg := defaultAppConfig()
	appCfg.Criteria = defaultCriteriaConfig()
	appCfg.Criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}
	if err := saveAppConfig(configFilePath, &appCfg); err != nil {
		t.Fatalf("saveAppConfig(): %v", err)
	}
	if err := saveSearchPrompt(searchPromptFilePath, "prompt"); err != nil {
		t.Fatalf("saveSearchPrompt(): %v", err)
	}

	caps = evaluateRuntimeCapabilities()
	if !caps.ConfigExists {
		t.Fatal("caps.ConfigExists = false; want true after config is written")
	}
	if !caps.SearchProfileReady {
		t.Fatal("caps.SearchProfileReady = false; want true with default search config")
	}
	if !caps.SearchSourcesReady {
		t.Fatal("caps.SearchSourcesReady = false; want true with default sources config")
	}
	if !caps.SearchPromptReady {
		t.Fatal("caps.SearchPromptReady = false; want true after SEARCH_PROMPT.md is written")
	}
}

func TestDefaultArtifacts(t *testing.T) {
	cfg := defaultAppConfig()
	if len(cfg.Sources.APIs) != 0 {
		t.Fatal("defaultAppConfig() should not include default API sources")
	}
	if len(cfg.Sources.RSS.Feeds) != 0 {
		t.Fatal("defaultAppConfig() should not include default custom RSS feeds")
	}
	if !cfg.Sources.BuiltinsEnabled {
		t.Fatal("defaultAppConfig() should enable Built In sources by default")
	}
	if cfg.LLM.Provider == "" {
		t.Fatal("defaultAppConfig() should set a default LLM provider")
	}

	criteria := &CriteriaConfig{}
	criteria.Filters.TitleRequires = []string{"Engineer", "Developer"}
	criteria.Filters.TitleIncludes = []string{"backend", "systems"}
	criteria.Filters.WorkSettings.Remote = true
	criteria.Filters.MinBaseUSD = 100000
	criteria.PrioritySignals = []string{"reliability", "automation"}

	prompt := defaultSearchPrompt(criteria)
	for _, expected := range []string{
		"Engineer, Developer",
		"backend, systems",
		"remote",
		"$100000",
		"reliability, automation",
		"actual company's website",
		"brief factual summary",
		"company_website",
		"company_summary",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("defaultSearchPrompt() missing %q", expected)
		}
	}
}

func TestLoadCriteriaConfigReadsConfigCriteriaSection(t *testing.T) {
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "config.yaml")
	restore := ConfigureRuntime(configPath, filepath.Join(tmpDir, "SEARCH_PROMPT.md"))
	t.Cleanup(restore)

	appCfg := defaultAppConfig()
	appCfg.Criteria.Filters.TitleIncludes = []string{"platform", "sre"}
	if err := saveAppConfig(configPath, &appCfg); err != nil {
		t.Fatalf("saveAppConfig(): %v", err)
	}

	loadedCfg, err := loadCriteriaConfig(configPath)
	if err != nil {
		t.Fatalf("loadCriteriaConfig(): %v", err)
	}
	if got := loadedCfg.Filters.TitleIncludes; len(got) != 2 || got[0] != "platform" || got[1] != "sre" {
		t.Fatalf("loaded search filters = %#v; want config.yaml criteria", got)
	}
}
