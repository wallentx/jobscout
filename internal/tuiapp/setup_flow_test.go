package tuiapp

import (
	"os"
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/config"
)

func TestNewSetupPlanForEditCriteriaAnchorsConfigMenu(t *testing.T) {
	plan := newSetupPlan(setupModeEdit, setupSectionCriteria, true, false)

	want := []setupStep{
		setupStepConfigMenu,
		setupStepCriteriaField,
		setupStepSummary,
	}
	if len(plan.steps) != len(want) {
		t.Fatalf("len(plan.steps) = %d; want %d", len(plan.steps), len(want))
	}
	for i, step := range want {
		if plan.steps[i] != step {
			t.Fatalf("plan.steps[%d] = %v; want %v", i, plan.steps[i], step)
		}
	}
}

func TestNewSetupPlanForRecoveryStartsAtRecoveryChoice(t *testing.T) {
	plan := newSetupPlan(setupModeRecovery, setupSectionLLM, true, false)

	want := []setupStep{
		setupStepRecoveryChoice,
	}
	if len(plan.steps) != len(want) {
		t.Fatalf("len(plan.steps) = %d; want %d", len(plan.steps), len(want))
	}
	for i, step := range want {
		if plan.steps[i] != step {
			t.Fatalf("plan.steps[%d] = %v; want %v", i, plan.steps[i], step)
		}
	}
}

func TestNewSetupPlanForRepairLLMIncludesAuthSteps(t *testing.T) {
	plan := newSetupPlan(setupModeRepair, setupSectionLLM, true, false)

	want := []setupStep{
		setupStepConfigMenu,
		setupStepLLMConfigMenu,
		setupStepProviderChoice,
		setupStepAuthModeChoice,
		setupStepAuthValueField,
		setupStepModelChoice,
		setupStepSummary,
	}
	if len(plan.steps) != len(want) {
		t.Fatalf("len(plan.steps) = %d, want %d", len(plan.steps), len(want))
	}
	for i, step := range want {
		if plan.steps[i] != step {
			t.Fatalf("plan.steps[%d] = %v, want %v", i, plan.steps[i], step)
		}
	}
}

func TestNewSetupPlanForBootstrapCanOfferResumePrefill(t *testing.T) {
	plan := newSetupPlan(setupModeBootstrap, setupSectionNone, true, true)

	want := []setupStep{
		setupStepLLMChoice,
		setupStepProviderChoice,
		setupStepAuthModeChoice,
		setupStepAuthValueField,
		setupStepModelChoice,
		setupStepResumeChoice,
		setupStepResumePathField,
		setupStepCriteriaField,
		setupStepSummary,
		setupStepPromptReview,
		setupStepPreviewConfirm,
	}
	if len(plan.steps) != len(want) {
		t.Fatalf("len(plan.steps) = %d, want %d", len(plan.steps), len(want))
	}
	for i, step := range want {
		if plan.steps[i] != step {
			t.Fatalf("plan.steps[%d] = %v, want %v", i, plan.steps[i], step)
		}
	}
}

func TestNewSetupPlanForRepairPromptStartsAtPrompt(t *testing.T) {
	plan := newSetupPlan(setupModeRepair, setupSectionPrompt, true, false)

	want := []setupStep{
		setupStepConfigMenu,
		setupStepPromptReview,
	}
	if len(plan.steps) != len(want) {
		t.Fatalf("len(plan.steps) = %d; want %d", len(plan.steps), len(want))
	}
	for i, step := range want {
		if plan.steps[i] != step {
			t.Fatalf("plan.steps[%d] = %v; want %v", i, plan.steps[i], step)
		}
	}
}

func TestSetupSectionForCapabilitiesPrefersPromptRepair(t *testing.T) {
	caps := config.RuntimeCapabilities{
		ConfigExists:       true,
		SearchProfileReady: true,
		SearchSourcesReady: true,
		LLMPreferred:       true,
		LLMConfigured:      true,
		SearchPromptReady:  false,
	}

	if got := setupSectionForCapabilities(caps); got != setupSectionPrompt {
		t.Fatalf("setupSectionForCapabilities() = %q; want prompt", got)
	}
}

func TestSetupSectionForCapabilitiesPrefersLLMWhenDisabledWithFeatureToggles(t *testing.T) {
	caps := config.RuntimeCapabilities{
		ConfigExists:        true,
		SearchProfileReady:  true,
		SearchSourcesReady:  true,
		LLMDisabled:         true,
		LLMFeaturesSelected: true,
	}

	if got := setupSectionForCapabilities(caps); got != setupSectionLLM {
		t.Fatalf("setupSectionForCapabilities() = %q; want llm", got)
	}
}

func TestProviderOptionsIncludeOllama(t *testing.T) {
	options := config.ProviderOptions()
	found := false
	for _, option := range options {
		if option == "ollama" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("config.ProviderOptions() = %#v, want ollama included", options)
	}
}

func TestInitialModelOpensRepairMenuWhenOnlyPromptIsMissing(t *testing.T) {
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

	appCfg := config.DefaultAppConfig()
	appCfg.Criteria = config.DefaultCriteriaConfig()
	appCfg.Criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}
	appCfg.LLM.JobSearch = true
	if err := config.SaveAppConfig(configFilePath, &appCfg); err != nil {
		t.Fatalf("config.SaveAppConfig(): %v", err)
	}

	m := initialModel()
	if !m.setupRequired {
		t.Fatal("setupRequired = false; want true when SEARCH_PROMPT.md is missing for LLM job search")
	}
	if m.overlay.kind != overlaySetup {
		t.Fatalf("overlay.kind = %v; want overlaySetup", m.overlay.kind)
	}
	if m.setup.mode != setupModeRepair {
		t.Fatalf("setup.mode = %v; want setupModeRepair", m.setup.mode)
	}
	if m.setup.section != setupSectionPrompt {
		t.Fatalf("setup.section = %q; want prompt", m.setup.section)
	}
	if m.setup.step != setupStepConfigMenu {
		t.Fatalf("setup.step = %v; want setupStepConfigMenu", m.setup.step)
	}
	if m.setup.choiceIdx != setupConfigMenuIndexByID(setupMenuPrompt) {
		t.Fatalf("setup.choiceIdx = %d; want prompt menu index", m.setup.choiceIdx)
	}
}

func TestInitialModelAcceptsSavedConfigWhenLLMDisabled(t *testing.T) {
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

	appCfg := config.DefaultAppConfig()
	appCfg.Criteria = config.DefaultCriteriaConfig()
	appCfg.Criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}
	appCfg.LLM.Enabled = false
	appCfg.LLM.JobSearch = true
	appCfg.LLM.JobFiltering = true
	appCfg.LLM.CompanyHealth = true
	if err := config.SaveAppConfig(configFilePath, &appCfg); err != nil {
		t.Fatalf("config.SaveAppConfig(): %v", err)
	}

	m := initialModel()
	if m.setupRequired {
		t.Fatal("setupRequired = true; want false when LLM is disabled and non-LLM search profile is ready")
	}
}

func TestSetupConfigMenuSpecsPromptRequiresLLM(t *testing.T) {
	m := model{
		setup: setupState{
			useLLM:    false,
			appConfig: config.DefaultAppConfig(),
			step:      setupStepConfigMenu,
		},
	}

	var promptItem *setupMenuItemSpec
	for i := range setupConfigMenuSpecs() {
		item := setupConfigMenuSpecs()[i]
		if item.ID == setupMenuPrompt {
			promptItem = &item
			break
		}
	}
	if promptItem == nil {
		t.Fatal("prompt menu item missing")
	}

	cmd := promptItem.Select(&m)
	if cmd != nil {
		t.Fatal("prompt menu select returned command; want nil for local state update")
	}
	if m.setup.message == "" {
		t.Fatal("setup.message empty; want prompt warning when LLM is disabled")
	}
	if m.setup.step != setupStepConfigMenu {
		t.Fatalf("setup.step = %v; want setupStepConfigMenu", m.setup.step)
	}
}

func TestSetupConfigMenuIndexForSection(t *testing.T) {
	if got := setupConfigMenuIndexForSection(setupSectionCriteria); got != setupConfigMenuIndexByID(setupMenuCriteria) {
		t.Fatalf("criteria menu index = %d; want criteria menu item index", got)
	}
	if got := setupConfigMenuIndexForSection(setupSectionLLM); got != setupConfigMenuIndexByID(setupMenuLLM) {
		t.Fatalf("llm menu index = %d; want llm menu item index", got)
	}
	if got := setupConfigMenuIndexForSection(setupSectionPrompt); got != setupConfigMenuIndexByID(setupMenuPrompt) {
		t.Fatalf("prompt menu index = %d; want prompt menu item index", got)
	}
}

func TestSearchProfileFieldsIncludeExamples(t *testing.T) {
	for _, field := range searchProfileGroupSpec().Fields {
		if !strings.Contains(strings.ToLower(field.Help), "e.g.") && !strings.Contains(strings.ToLower(field.Help), "example") {
			t.Fatalf("field %q help = %q, want example text", field.Key, field.Help)
		}
	}
}

func TestRefreshSetupGeneratedStatePreservesPromptWhenRequested(t *testing.T) {
	m := model{
		setup: setupState{
			fieldValues: map[string]string{
				"candidate.city":                "Example City",
				"candidate.state":               "EX",
				"candidate.country_code":        "US",
				"candidate.years_of_experience": "3",
				"role_families":                 "devops_sre_systems",
				"filters.title_requires":        "Engineer",
				"filters.title_includes":        "backend",
				"filters.title_excludes":        "manager",
				"filters.work_settings":         "remote",
				"filters.min_base_usd":          "100000",
				"priority_signals":              "reliability",
			},
			prompt: "custom prompt",
		},
	}

	if err := m.refreshSetupGeneratedState(false); err != nil {
		t.Fatalf("refreshSetupGeneratedState(false) error = %v", err)
	}
	if m.setup.prompt != "custom prompt" {
		t.Fatalf("setup.prompt = %q; want custom prompt preserved", m.setup.prompt)
	}

	if err := m.refreshSetupGeneratedState(true); err != nil {
		t.Fatalf("refreshSetupGeneratedState(true) error = %v", err)
	}
	if m.setup.prompt == "custom prompt" {
		t.Fatal("setup.prompt unchanged after overwrite; want regenerated prompt")
	}
}
