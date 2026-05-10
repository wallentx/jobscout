package tuiapp

import (
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCurrentSetupConfigOptions(t *testing.T) {
	m := model{
		setup: setupState{
			appConfig: config.DefaultAppConfig(),
		},
	}

	options := m.currentSetupConfigOptions()
	if len(options) != 8 {
		t.Fatalf("currentSetupConfigOptions() len = %d; want 8", len(options))
	}

	for _, expected := range []string{
		"Populate criteria from resume",
		"Search sources:",
		"RSS feeds:",
		"Site search:",
		"Built In sources:",
	} {
		found := false
		for _, option := range options {
			if strings.Contains(option, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("currentSetupConfigOptions() missing %q in %#v", expected, options)
		}
	}
}

func TestCurrentSetupConfigOptionsShowRepairStatuses(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")

	m := model{
		setup: setupState{
			mode:      setupModeRepair,
			useLLM:    true,
			appConfig: config.DefaultAppConfig(),
			fieldValues: map[string]string{
				"candidate.city":                "",
				"candidate.state":               "",
				"candidate.country_code":        "",
				"candidate.years_of_experience": "",
				"role_families":                 "",
				"filters.title_requires":        "",
				"filters.title_includes":        "",
				"filters.title_excludes":        "",
				"filters.work_settings":         "",
				"filters.min_base_usd":          "",
				"priority_signals":              "",
			},
		},
	}

	options := m.currentSetupConfigOptions()

	var foundCriteria bool
	var foundLLM bool
	for _, option := range options {
		if strings.Contains(option, "Search criteria [needs attention]") {
			foundCriteria = true
		}
		if strings.Contains(option, "LLM settings [needs attention]") {
			foundLLM = true
		}
	}
	if !foundCriteria {
		t.Fatalf("currentSetupConfigOptions() missing criteria repair status in %#v", options)
	}
	if !foundLLM {
		t.Fatalf("currentSetupConfigOptions() missing LLM repair status in %#v", options)
	}
}

func TestSetupConfigMenuCanOpenResumeCriteriaPath(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "token")

	setup := newSetupState(setupModeEdit, setupSectionNone)
	setup.step = setupStepConfigMenu
	setup.appConfig = config.DefaultAppConfig()
	setup.useLLM = true
	setup.choiceIdx = setupConfigMenuIndexByID(setupMenuResumeCriteria)
	m := model{
		setup:   setup,
		overlay: overlayState{kind: overlaySetup},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if got.setup.step != setupStepResumePathField {
		t.Fatalf("setup.step = %v, want setupStepResumePathField", got.setup.step)
	}
	if !got.setup.input.Focused() {
		t.Fatal("setup.input.Focused() = false, want resume path input focused")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(*model)
	if got.setup.step != setupStepConfigMenu {
		t.Fatalf("setup.step after Esc = %v, want setupStepConfigMenu", got.setup.step)
	}
	if got.setup.choiceIdx != setupConfigMenuIndexByID(setupMenuResumeCriteria) {
		t.Fatalf("setup.choiceIdx after Esc = %d, want resume criteria menu index", got.setup.choiceIdx)
	}
}
