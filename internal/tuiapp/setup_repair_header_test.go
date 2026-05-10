package tuiapp

import (
	"os"
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestSetupHeaderExplainsFirstRunPurpose(t *testing.T) {
	setup := newSetupState(setupModeBootstrap, setupSectionNone)
	setup.step = setupStepLLMChoice
	m := model{
		termWidth:  100,
		termHeight: 30,
		setup:      setup,
		overlay:    overlayState{kind: overlaySetup},
	}

	spec := m.buildSetupOverlaySpec()
	header := ansi.Strip(spec.header)

	for _, want := range []string{
		"Create a search profile and app settings",
		"Optional LLM features",
		"company health summaries",
	} {
		if !strings.Contains(header, want) {
			t.Fatalf("setup header = %q; want %q", header, want)
		}
	}
}

func TestSetupHeaderExplainsRepairIssues(t *testing.T) {
	setup := newSetupState(setupModeRepair, setupSectionCriteria)
	setup.step = setupStepConfigMenu
	m := model{
		termWidth:  100,
		termHeight: 30,
		setup:      setup,
		setupIssues: []string{
			"Config: missing fields",
		},
		overlay: overlayState{kind: overlaySetup},
	}

	spec := m.buildSetupOverlaySpec()
	header := ansi.Strip(spec.header)

	for _, want := range []string{
		"Some setup data is missing or incomplete",
		"What needs attention",
		"Config: missing fields",
	} {
		if !strings.Contains(header, want) {
			t.Fatalf("setup header = %q; want %q", header, want)
		}
	}
}

func TestSetupSummaryRedirectsToLLMAuthWhenUnavailable(t *testing.T) {
	tmpDir := t.TempDir()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tmpDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	t.Setenv("GEMINI_API_KEY", "")

	appCfg := config.DefaultAppConfig()
	appCfg.LLM.JobSearch = true

	m := model{
		setup: setupState{
			mode:      setupModeRepair,
			step:      setupStepSummary,
			section:   setupSectionCriteria,
			useLLM:    true,
			appConfig: appCfg,
			fieldValues: map[string]string{
				"candidate.city":                "",
				"candidate.state":               "",
				"candidate.country_code":        "",
				"candidate.years_of_experience": "",
				"role_families":                 "devops_sre_systems",
				"filters.title_requires":        "",
				"filters.title_includes":        "platform",
				"filters.title_excludes":        "",
				"filters.work_settings":         "remote",
				"filters.min_base_usd":          "",
				"priority_signals":              "",
			},
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.rebuildPlan()
	if config.LLMAuthAvailableNow(&m.setup.appConfig) {
		t.Fatal("config.LLMAuthAvailableNow() = true, want false for empty GEMINI_API_KEY")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if got.setup.section != setupSectionLLM {
		t.Fatalf("setup.section = %q, want %q (message=%q)", got.setup.section, setupSectionLLM, got.setup.message)
	}
	if got.setup.step != setupStepAuthModeChoice {
		t.Fatalf("setup.step = %v, want %v", got.setup.step, setupStepAuthModeChoice)
	}
	if !strings.Contains(got.setup.message, "Saved your criteria") {
		t.Fatalf("setup.message = %q, want saved-criteria guidance", got.setup.message)
	}

	savedCfg, err := config.LoadAppConfig(configFilePath)
	if err != nil {
		t.Fatalf("config.LoadAppConfig(%q) error = %v", configFilePath, err)
	}
	if gotRoleFamilies := savedCfg.Criteria.RoleFamilies; len(gotRoleFamilies) != 1 || gotRoleFamilies[0] != RoleDevOpsSRESystems {
		t.Fatalf("savedCfg.Criteria.RoleFamilies = %#v, want [RoleDevOpsSRESystems]", gotRoleFamilies)
	}
}

func TestRepairPromptSaveClosesWhenNoIssuesRemain(t *testing.T) {
	tmpDir := t.TempDir()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tmpDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	appCfg := config.DefaultAppConfig()
	appCfg.Criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}
	appCfg.LLM.JobSearch = true
	appCfg.LLM.JobFiltering = true
	appCfg.LLM.Provider = "ollama"
	appCfg.LLM.Providers = config.DefaultLLMProviders()
	appCfg.LLM.Providers["ollama"] = LLMProviderConfig{
		Model:    "llama3",
		Endpoint: "http://localhost:11434",
		Auth: LLMAuthConfig{
			None: true,
		},
	}
	if err := config.SaveAppConfig(configFilePath, &appCfg); err != nil {
		t.Fatalf("config.SaveAppConfig(%q) error = %v", configFilePath, err)
	}

	m := model{
		setup: setupState{
			mode:        setupModeRepair,
			section:     setupSectionPrompt,
			step:        setupStepPromptReview,
			useLLM:      true,
			appConfig:   appCfg,
			prompt:      "existing prompt",
			fieldValues: setupFieldValues(appCfg.Criteria),
		},
		setupRequired: true,
		setupIssues:   []string{"SEARCH_PROMPT.md is required for LLM job search"},
		overlay:       overlayState{kind: overlaySetup},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if got.overlay.kind != overlayNotice {
		t.Fatalf("overlay.kind = %v, want overlayNotice (setupIssues=%#v message=%q)", got.overlay.kind, got.setupIssues, got.setup.message)
	}
	if !strings.Contains(got.overlay.notice.title, "Configuration Repaired") {
		t.Fatalf("overlay.notice.title = %q, want Configuration Repaired", got.overlay.notice.title)
	}
}
