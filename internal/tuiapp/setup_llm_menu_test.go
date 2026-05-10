package tuiapp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestCurrentSetupLLMConfigOptions(t *testing.T) {
	m := model{
		setup: setupState{
			useLLM: true,
			appConfig: AppConfig{
				LLM: LLMConfig{
					Provider: "gemini",
				},
			},
		},
	}

	options := m.currentSetupLLMConfigOptions()
	for _, expected := range []string{
		"LLM Features: Enabled",
		"Provider: Google",
		"Provider Config",
	} {
		found := false
		for _, option := range options {
			if option == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("currentSetupLLMConfigOptions() missing %q in %#v", expected, options)
		}
	}
}

func TestSetupLLMConfigProviderUsesDropdown(t *testing.T) {
	restoreRuntimePathsAfterTest(t)
	runtimeConfigPath = filepath.Join(t.TempDir(), configFilePath)

	setup := newSetupState(setupModeEdit, setupSectionLLM)
	setup.step = setupStepLLMConfigMenu
	setup.useLLM = true
	setup.choiceIdx = 1
	setup.appConfig = config.DefaultAppConfig()
	setup.appConfig.LLM.Provider = "gemini"
	setup.appConfig.LLM.Providers = config.DefaultLLMProviders()
	m := model{
		setup:   setup,
		overlay: overlayState{kind: overlaySetup},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if got.setup.step != setupStepLLMConfigMenu {
		t.Fatalf("setup.step = %v, want setupStepLLMConfigMenu", got.setup.step)
	}
	if !got.setup.providerDropdownOpen {
		t.Fatalf("providerDropdownOpen = false, want true")
	}

	got.setup.providerDropdownIdx = providerOptionIndex("openai")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(*model)

	if got.setup.appConfig.LLM.Provider != "openai" {
		t.Fatalf("LLM.Provider = %q, want openai", got.setup.appConfig.LLM.Provider)
	}
	if got.setup.providerDropdownOpen {
		t.Fatalf("providerDropdownOpen = true, want false")
	}
	if got.setup.step != setupStepLLMConfigMenu {
		t.Fatalf("setup.step = %v, want setupStepLLMConfigMenu", got.setup.step)
	}
}

func TestCurrentSetupTaskModelMenuOptions(t *testing.T) {
	m := model{
		setup: setupState{
			appConfig: AppConfig{
				LLM: LLMConfig{
					Provider: "gemini",
					Model:    "gemini-flash-lite-latest",
					Providers: map[string]LLMProviderConfig{
						"gemini": {
							Model: "gemini-flash-lite-latest",
							Models: map[string]string{
								llmTaskFiltering: "gemini-2.5-flash-lite",
							},
						},
					},
				},
			},
		},
	}

	options := m.currentSetupTaskModelMenuOptions()
	for _, expected := range []string{
		"LLM job filtering: gemini-2.5-flash-lite",
		"LLM company health: provider default",
	} {
		found := false
		for _, option := range options {
			if option == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("currentSetupTaskModelMenuOptions() missing %q in %#v", expected, options)
		}
	}
}

func TestSetupProviderSwitchKeepsProviderAuthIsolated(t *testing.T) {
	restoreRuntimePathsAfterTest(t)
	runtimeConfigPath = filepath.Join(t.TempDir(), configFilePath)

	setup := newSetupState(setupModeEdit, setupSectionLLM)
	setup.step = setupStepProviderChoice
	setup.useLLM = true
	setup.appConfig = config.DefaultAppConfig()
	setup.appConfig.LLM.Provider = "gemini"
	setup.appConfig.LLM.Providers = config.DefaultLLMProviders()
	setup.appConfig.LLM.Providers["gemini"] = LLMProviderConfig{
		Model: "gemini-2.5-flash",
		Auth: LLMAuthConfig{
			Mode:   llmAuthModeEnv,
			EnvVar: "GEMINI_API_KEY",
		},
	}
	setup.appConfig.LLM.Providers["openai"] = LLMProviderConfig{
		Model: "gpt-5.4",
		Auth: LLMAuthConfig{
			Mode:   llmAuthModeEnv,
			EnvVar: "OPENAI_API_KEY",
		},
	}
	setup.choiceIdx = providerOptionIndex("openai")
	m := model{
		setup:   setup,
		overlay: overlayState{kind: overlaySetup},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if got.setup.appConfig.LLM.Provider != "openai" {
		t.Fatalf("LLM.Provider = %q, want openai", got.setup.appConfig.LLM.Provider)
	}
	if got.setup.appConfig.LLM.Auth.EnvVar != "OPENAI_API_KEY" {
		t.Fatalf("selected auth env = %q, want OPENAI_API_KEY", got.setup.appConfig.LLM.Auth.EnvVar)
	}
	if got.setup.appConfig.LLM.Providers["gemini"].Auth.EnvVar != "GEMINI_API_KEY" {
		t.Fatalf("gemini auth env = %q, want GEMINI_API_KEY", got.setup.appConfig.LLM.Providers["gemini"].Auth.EnvVar)
	}
	if got.setup.appConfig.LLM.Providers["openai"].Auth.EnvVar != "OPENAI_API_KEY" {
		t.Fatalf("openai auth env = %q, want OPENAI_API_KEY", got.setup.appConfig.LLM.Providers["openai"].Auth.EnvVar)
	}
	if got.setup.step != setupStepLLMConfigMenu {
		t.Fatalf("setup.step = %v, want setupStepLLMConfigMenu", got.setup.step)
	}
}

func TestSetupModelChoiceSkipsAuthStepsForOllama(t *testing.T) {
	m := model{
		setup: setupState{
			mode:    setupModeRepair,
			section: setupSectionLLM,
			step:    setupStepModelChoice,
			useLLM:  true,
			input:   textinput.New(),
			appConfig: AppConfig{
				LLM: LLMConfig{
					Provider: "ollama",
					Model:    "llama3",
					Providers: map[string]LLMProviderConfig{
						"ollama": {
							Model:    "llama3",
							Endpoint: "http://localhost:11434",
							Auth: LLMAuthConfig{
								None: true,
							},
						},
					},
				},
			},
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.rebuildPlan()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if got.setup.step != setupStepSummary {
		t.Fatalf("setup.step = %v, want setupStepSummary", got.setup.step)
	}
}

func TestSetupModelChoiceAllowsManualModelEntry(t *testing.T) {
	setup := newSetupState(setupModeRepair, setupSectionLLM)
	setup.step = setupStepModelChoice
	setup.useLLM = true
	setup.appConfig = AppConfig{
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-5.4",
			Providers: map[string]LLMProviderConfig{
				"openai": {
					Model: "gpt-5.4",
					Auth: LLMAuthConfig{
						Mode:   llmAuthModeEnv,
						EnvVar: "OPENAI_API_KEY",
					},
				},
			},
		},
	}
	setup.modelsByProvider = map[string][]string{
		"openai": {"gpt-5.4"},
	}
	m := model{
		setup:   setup,
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.rebuildPlan()
	options := m.currentSetupModelOptions()
	m.setup.choiceIdx = len(options) - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)
	if got.setup.step != setupStepModelValueField {
		t.Fatalf("setup.step = %v, want setupStepModelValueField", got.setup.step)
	}

	got.setup.input.SetValue("gpt-5.4-preview-custom")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(*model)

	if got.setup.appConfig.LLM.Model != "gpt-5.4-preview-custom" {
		t.Fatalf("LLM.Model = %q, want custom model", got.setup.appConfig.LLM.Model)
	}
	if got.setup.appConfig.LLM.Providers["openai"].Model != "gpt-5.4-preview-custom" {
		t.Fatalf("provider model = %q, want custom model", got.setup.appConfig.LLM.Providers["openai"].Model)
	}
	if got.setup.step != setupStepSummary {
		t.Fatalf("setup.step = %v, want setupStepSummary", got.setup.step)
	}
}

func TestSetupProviderConfigDefaultModelUsesDropdown(t *testing.T) {
	restoreRuntimePathsAfterTest(t)
	runtimeConfigPath = filepath.Join(t.TempDir(), configFilePath)

	setup := newSetupState(setupModeEdit, setupSectionLLM)
	setup.step = setupStepProviderConfigMenu
	setup.choiceIdx = 1
	setup.appConfig = AppConfig{
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4.1",
			Providers: map[string]LLMProviderConfig{
				"openai": {
					Model: "gpt-4.1",
					Auth: LLMAuthConfig{
						Mode:   llmAuthModeEnv,
						EnvVar: "OPENAI_API_KEY",
					},
				},
			},
		},
	}
	m := model{
		setup:   setup,
		overlay: overlayState{kind: overlaySetup},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if got.setup.step != setupStepProviderConfigMenu {
		t.Fatalf("setup.step = %v, want setupStepProviderConfigMenu", got.setup.step)
	}
	if !got.setup.providerModelDropdownOpen {
		t.Fatalf("providerModelDropdownOpen = false, want true")
	}

	updated, _ = got.Update(setupModelsFetchedMsg{
		provider: "openai",
		models:   []string{"gpt-5.3-chat-latest", "gpt-4.1"},
	})
	gotValue := updated.(model)
	gotValue.setup.providerModelDropdownIdx = 0

	updated, _ = gotValue.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(*model)

	if got.setup.appConfig.LLM.Model != "gpt-5.3-chat-latest" {
		t.Fatalf("LLM.Model = %q, want gpt-5.3-chat-latest", got.setup.appConfig.LLM.Model)
	}
	if got.setup.appConfig.LLM.Providers["openai"].Model != "gpt-5.3-chat-latest" {
		t.Fatalf("provider model = %q, want gpt-5.3-chat-latest", got.setup.appConfig.LLM.Providers["openai"].Model)
	}
	if got.setup.providerModelDropdownOpen {
		t.Fatalf("providerModelDropdownOpen = true, want false")
	}
	if got.setup.step != setupStepProviderConfigMenu {
		t.Fatalf("setup.step = %v, want setupStepProviderConfigMenu", got.setup.step)
	}
}

func TestSetupTaskModelChoiceSetsAndClearsOverride(t *testing.T) {
	restoreRuntimePathsAfterTest(t)
	runtimeConfigPath = filepath.Join(t.TempDir(), configFilePath)

	setup := newSetupState(setupModeEdit, setupSectionLLM)
	setup.step = setupStepTaskModelChoice
	setup.taskModelKey = llmTaskFiltering
	setup.choiceIdx = 1
	setup.appConfig = AppConfig{
		LLM: LLMConfig{
			Provider: "gemini",
			Model:    "gemini-flash-lite-latest",
			Providers: map[string]LLMProviderConfig{
				"gemini": {
					Model: "gemini-flash-lite-latest",
					Auth: LLMAuthConfig{
						Mode:   llmAuthModeEnv,
						EnvVar: "GEMINI_API_KEY",
					},
				},
			},
		},
	}
	setup.modelsByProvider = map[string][]string{
		"gemini": {"gemini-2.5-flash-lite"},
	}
	m := model{
		setup:   setup,
		overlay: overlayState{kind: overlaySetup},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if got.setup.appConfig.LLM.Providers["gemini"].Models[llmTaskFiltering] != "gemini-2.5-flash-lite" {
		t.Fatalf("provider Models[%q] = %q, want gemini-2.5-flash-lite", llmTaskFiltering, got.setup.appConfig.LLM.Providers["gemini"].Models[llmTaskFiltering])
	}
	if got.setup.step != setupStepTaskModelMenu {
		t.Fatalf("setup.step = %v, want setupStepTaskModelMenu", got.setup.step)
	}

	got.setup.step = setupStepTaskModelChoice
	got.setup.taskModelKey = llmTaskFiltering
	got.setup.choiceIdx = 0
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(*model)

	if _, ok := got.setup.appConfig.LLM.Providers["gemini"].Models[llmTaskFiltering]; ok {
		t.Fatalf("provider Models[%q] still set after fallback clear", llmTaskFiltering)
	}
}

func TestSetupTaskModelValueFieldAllowsManualModelEntry(t *testing.T) {
	restoreRuntimePathsAfterTest(t)
	runtimeConfigPath = filepath.Join(t.TempDir(), configFilePath)

	setup := newSetupState(setupModeEdit, setupSectionLLM)
	setup.step = setupStepTaskModelChoice
	setup.taskModelKey = llmTaskCompanyHealth
	setup.appConfig = AppConfig{
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o-2024-11-20",
			Providers: map[string]LLMProviderConfig{
				"openai": {
					Model: "gpt-4o-2024-11-20",
					Auth: LLMAuthConfig{
						Mode:   llmAuthModeEnv,
						EnvVar: "OPENAI_API_KEY",
					},
				},
			},
		},
	}
	setup.modelsByProvider = map[string][]string{
		"openai": {"gpt-4o-2024-11-20"},
	}
	m := model{
		setup:   setup,
		overlay: overlayState{kind: overlaySetup},
	}
	options := m.currentSetupTaskModelOptions()
	m.setup.choiceIdx = len(options) - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)
	if got.setup.step != setupStepTaskModelValueField {
		t.Fatalf("setup.step = %v, want setupStepTaskModelValueField", got.setup.step)
	}

	got.setup.input.SetValue("gpt-4o-custom-health")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(*model)

	if got.setup.appConfig.LLM.Providers["openai"].Models[llmTaskCompanyHealth] != "gpt-4o-custom-health" {
		t.Fatalf("provider Models[%q] = %q, want gpt-4o-custom-health", llmTaskCompanyHealth, got.setup.appConfig.LLM.Providers["openai"].Models[llmTaskCompanyHealth])
	}
	if got.setup.step != setupStepTaskModelMenu {
		t.Fatalf("setup.step = %v, want setupStepTaskModelMenu", got.setup.step)
	}
}

func TestSetupModelChoiceUsesScrollableMenu(t *testing.T) {
	setup := newSetupState(setupModeRepair, setupSectionLLM)
	setup.step = setupStepModelChoice
	setup.useLLM = true
	setup.appConfig = AppConfig{
		LLM: LLMConfig{
			Provider: "openai",
			Model:    "model-19",
			Providers: map[string]LLMProviderConfig{
				"openai": {
					Model: "model-19",
					Auth: LLMAuthConfig{
						Mode:   llmAuthModeEnv,
						EnvVar: "OPENAI_API_KEY",
					},
				},
			},
		},
	}
	models := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		models = append(models, "model-"+string(rune('a'+i)))
	}
	setup.modelsByProvider = map[string][]string{"openai": models}
	setup.choiceIdx = 19
	m := model{
		termWidth:  100,
		termHeight: 20,
		setup:      setup,
		overlay:    overlayState{kind: overlaySetup},
	}

	spec := m.buildSetupOverlaySpec()
	body := ansi.Strip(spec.body.content)

	if len(spec.menu) != 0 {
		t.Fatalf("spec.menu len = %d; want scrollable menu rendered in body", len(spec.menu))
	}
	if !strings.Contains(spec.body.content, "█") {
		t.Fatalf("spec.body.content = %q; want scrollbar thumb", spec.body.content)
	}
	if !strings.Contains(body, "model-t") {
		t.Fatalf("body = %q; want selected model-t visible", body)
	}
	if strings.Contains(body, "model-a") {
		t.Fatalf("body = %q; did not want first model visible when selection is near end", body)
	}
	if !strings.Contains(ansi.Strip(spec.footer), "Scroll") {
		t.Fatalf("footer = %q; want scroll hint", ansi.Strip(spec.footer))
	}
}
