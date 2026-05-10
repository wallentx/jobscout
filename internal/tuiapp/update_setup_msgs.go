package tuiapp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wallentx/jobscout/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleSetupPromptEditedMsg(msg setupPromptEditedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setup.message = msg.err.Error()
	} else {
		m.setup.prompt = msg.content
		m.setup.message = "Search prompt updated."
	}
	return m, nil
}

func (m model) handleSetupResumeCriteriaMsg(msg setupResumeCriteriaMsg) (tea.Model, tea.Cmd) {
	m.setup.resumeGenerating = false
	m.setup.loadingMinimized = false
	if msg.err != nil {
		m.setup.message = fmt.Sprintf("Could not prefill criteria from resume: %v", msg.err)
		m.setup.setStep(setupStepResumePathField)
		m.setup.syncInput()
		return m, nil
	}
	if msg.criteria == nil {
		m.setup.message = "Could not read resume: LLM returned no criteria."
		m.setup.setStep(setupStepResumePathField)
		m.setup.syncInput()
		return m, nil
	}
	m.setup.resumePath = msg.path
	m.setup.fieldValues = setupFieldValues(*msg.criteria)
	m.setup.generated = msg.criteria
	m.setup.prompt = config.DefaultSearchPrompt(msg.criteria)
	m.setup.message = "Criteria fields were prefilled from your resume. Review and edit anything that looks off."
	m.setup.setStep(setupStepCriteriaField)
	m.setup.fieldIdx = 0
	m.setup.syncInput()
	return m, nil
}

func (m model) handleSetupPreviewMsg(msg setupPreviewMsg) (tea.Model, tea.Cmd) {
	m.setup.previewBusy = false
	m.setup.loadingMinimized = false
	if msg.err != nil {
		m.setup.previewErr = msg.err.Error()
		m.setup.previewJobs = nil
		m.setup.message = ""
		return m, nil
	}
	m.setup.message = formatFetchSummary(len(msg.jobs), len(msg.jobs), summarizeJobs(msg.jobs), msg.notices, msg.rejected)
	m.setup.previewErr = ""
	m.setup.previewJobs = slices.Clone(msg.jobs)
	m.setup.step = setupStepPreviewConfirm
	return m, nil
}

func (m model) handleSetupModelsFetchedMsg(msg setupModelsFetchedMsg) (tea.Model, tea.Cmd) {
	if m.overlay.kind != overlaySetup || strings.ToLower(strings.TrimSpace(m.setup.appConfig.LLM.Provider)) != msg.provider {
		return m, nil
	}
	m.setup.modelsLoading = false
	if msg.err != nil {
		m.setup.message = fmt.Sprintf("Could not fetch model list; using fallback options. %v", msg.err)
		return m, nil
	}
	if len(msg.models) == 0 {
		m.setup.message = "Model list was empty; using fallback options."
		return m, nil
	}
	if m.setup.modelsByProvider == nil {
		m.setup.modelsByProvider = make(map[string][]string)
	}
	m.setup.modelsByProvider[msg.provider] = slices.Clone(msg.models)
	if m.setup.firstRun && m.setup.useLLM {
		m.setup.resumePrefillAvailable = true
	}
	options := m.currentSetupModelOptions()
	if m.setup.step == setupStepTaskModelChoice {
		options = m.currentSetupTaskModelOptions()
	}
	if m.setup.step == setupStepProviderConfigMenu && m.setup.providerModelDropdownOpen {
		if m.setup.providerModelDropdownIdx >= len(options) {
			m.setup.providerModelDropdownIdx = len(options) - 1
		}
		if m.setup.providerModelDropdownIdx < 0 {
			m.setup.providerModelDropdownIdx = 0
		}
		m.setup.message = fmt.Sprintf("Fetched %d models for %s.", len(msg.models), msg.provider)
		return m, nil
	}
	if m.setup.step == setupStepTaskModelMenu && m.setup.taskModelDropdownOpen {
		options = m.currentSetupTaskModelOptions()
		if m.setup.taskModelDropdownIdx >= len(options) {
			m.setup.taskModelDropdownIdx = len(options) - 1
		}
		if m.setup.taskModelDropdownIdx < 0 {
			m.setup.taskModelDropdownIdx = 0
		}
		m.setup.message = fmt.Sprintf("Fetched %d models for %s.", len(msg.models), msg.provider)
		return m, nil
	}
	if m.setup.choiceIdx >= len(options) {
		m.setup.choiceIdx = len(options) - 1
	}
	m.setup.message = fmt.Sprintf("Fetched %d models for %s.", len(msg.models), msg.provider)
	return m, nil
}
