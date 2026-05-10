package tuiapp

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/wallentx/jobscout/internal/config"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
	"github.com/wallentx/jobscout/internal/storage"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) handleSetupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.setup.step {
	case setupStepRecoveryChoice:
		options := []string{
			"Continue without LLM for this session",
			"Fix LLM settings now",
			"Quit",
		}
		switch msg.String() {
		case "up", "k":
			if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.choiceIdx < len(options)-1 {
				m.setup.choiceIdx++
			}
		case "enter":
			switch m.setup.choiceIdx {
			case 0:
				m.sessionLLMDisabled = true
				m.clearOverlay()
				m.showNotice("LLM Recovery", "LLM is unavailable in this session. Continuing with non-LLM sources until you restart or re-enable LLM in configuration.", false)
				return m, nil
			case 1:
				m.openSetupOverlay(setupModeRepair, setupSectionLLM)
				return m, nil
			default:
				return m, m.quitCommand()
			}
		}
	case setupStepLLMChoice:
		switch msg.String() {
		case "up", "k":
			if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.choiceIdx < 1 {
				m.setup.choiceIdx++
			}
		case "enter":
			m.setup.useLLM = m.setup.choiceIdx == 0
			if m.setup.useLLM {
				if m.setup.appConfig.LLM.Provider == "" {
					m.setup.appConfig.LLM.Provider = "gemini"
				}
				m.setup.setCurrentSetupProvider(m.setup.appConfig.LLM.Provider)
				m.setup.appConfig.LLM.Enabled = true
				m.setup.appConfig.LLM.JobFiltering = true
				m.setup.appConfig.LLM.JobSearch = true
				m.setup.appConfig.LLM.CompanyHealth = true
				m.setup.choiceIdx = 0
				m.setup.setStep(setupStepProviderChoice)
			} else {
				m.setup.appConfig.LLM.Enabled = false
				m.setup.appConfig.LLM.JobFiltering = false
				m.setup.appConfig.LLM.JobSearch = false
				m.setup.appConfig.LLM.CompanyHealth = false
				if m.setup.section == setupSectionNone && m.setup.mode != setupModeEdit {
					m.setup.configureSection(setupSectionCriteria)
				}
				if m.setup.advanceStep() && m.setup.step == setupStepCriteriaField {
					m.setup.fieldIdx = 0
					m.setup.syncInput()
				}
			}
		case "esc":
			if m.setup.mode == setupModeEdit {
				m.clearOverlay()
			}
		}
	case setupStepConfigMenu:
		items := setupConfigMenuSpecs()
		switch msg.String() {
		case "up", "k":
			if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.choiceIdx < len(items)-1 {
				m.setup.choiceIdx++
			}
		case "enter":
			if m.setup.choiceIdx >= 0 && m.setup.choiceIdx < len(items) {
				return m, items[m.setup.choiceIdx].Select(m)
			}
		case "esc":
			m.clearOverlay()
		}
	case setupStepLLMConfigMenu:
		options := m.currentSetupLLMConfigOptions()
		switch msg.String() {
		case "up", "k":
			if m.setup.providerDropdownOpen {
				if m.setup.providerDropdownIdx > 0 {
					m.setup.providerDropdownIdx--
				}
			} else if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.providerDropdownOpen {
				providerOptions := m.currentSetupProviderOptions()
				if m.setup.providerDropdownIdx < len(providerOptions)-1 {
					m.setup.providerDropdownIdx++
				}
			} else if m.setup.choiceIdx < len(options)-1 {
				m.setup.choiceIdx++
			}
		case "enter":
			switch m.setup.choiceIdx {
			case 0:
				m.setup.providerDropdownOpen = false
				m.setup.useLLM = !m.setup.useLLM
				m.setup.appConfig.LLM.Enabled = m.setup.useLLM
				m.setup.appConfig.LLM.JobFiltering = m.setup.useLLM
				m.setup.appConfig.LLM.JobSearch = m.setup.useLLM
				m.setup.appConfig.LLM.CompanyHealth = m.setup.useLLM
				if m.setup.useLLM {
					m.setup.setCurrentSetupProvider(m.setup.appConfig.LLM.Provider)
				}
				m.saveSetupConfigMessage(fmt.Sprintf("LLM features set to %s.", optionsEnabledDisabled(m.setup.useLLM)))
			case 1:
				if !m.setup.providerDropdownOpen {
					m.setup.providerDropdownIdx = providerOptionIndex(m.setup.appConfig.LLM.Provider)
					m.setup.providerDropdownOpen = true
					m.setup.message = ""
					return m, nil
				}
				providerOptions := m.currentSetupProviderOptions()
				if len(providerOptions) == 0 {
					return m, nil
				}
				m.setup.setCurrentSetupProvider(providerOptions[normalizeDropdownSelectedIdx(m.setup.providerDropdownIdx, len(providerOptions))])
				m.setup.appConfig.LLM.Enabled = true
				m.setup.appConfig.LLM.JobFiltering = true
				m.setup.appConfig.LLM.JobSearch = true
				m.setup.appConfig.LLM.CompanyHealth = true
				m.setup.useLLM = true
				m.saveSetupConfigMessage(fmt.Sprintf("Provider set to %s.", providerLabel(m.setup.appConfig.LLM.Provider)))
				m.setup.providerDropdownOpen = false
			case 2:
				m.setup.providerDropdownOpen = false
				m.setup.useLLM = true
				m.setup.appConfig.LLM.Enabled = true
				m.setup.appConfig.LLM.JobFiltering = true
				m.setup.appConfig.LLM.JobSearch = true
				m.setup.appConfig.LLM.CompanyHealth = true
				m.setup.setCurrentSetupProvider(m.setup.appConfig.LLM.Provider)
				m.setup.step = setupStepProviderConfigMenu
				m.setup.choiceIdx = 0
				m.setup.providerConfigReturn = false
				m.setup.message = ""
			}
		case "esc":
			if m.setup.providerDropdownOpen {
				m.setup.providerDropdownOpen = false
				m.setup.message = ""
				return m, nil
			}
			if m.setup.retreatStep() {
				m.setup.choiceIdx = setupConfigMenuIndexByID(setupMenuLLM)
			}
		}
	case setupStepProviderConfigMenu:
		options := m.currentSetupProviderConfigOptions()
		switch msg.String() {
		case "up", "k":
			if m.setup.providerModelDropdownOpen {
				if m.setup.providerModelDropdownIdx > 0 {
					m.setup.providerModelDropdownIdx--
				}
			} else if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.providerModelDropdownOpen {
				modelOptions := m.currentSetupModelOptions()
				if m.setup.providerModelDropdownIdx < len(modelOptions)-1 {
					m.setup.providerModelDropdownIdx++
				}
			} else if m.setup.choiceIdx < len(options)-1 {
				m.setup.choiceIdx++
			}
		case "enter":
			m.setup.providerConfigReturn = true
			switch m.setup.choiceIdx {
			case 0:
				m.setup.providerModelDropdownOpen = false
				if m.setup.currentSetupProviderNeedsAuth() {
					m.setup.step = setupStepAuthModeChoice
					m.setup.choiceIdx = setupAuthModeIndex(m.setup.currentSetupAuthMode())
					m.setup.message = ""
					return m, nil
				}
				m.setup.message = "This provider does not require credentials."
			case 1:
				if !m.setup.providerModelDropdownOpen {
					modelOptions := m.currentSetupModelOptions()
					m.setup.providerModelDropdownIdx = setupTaskModelOptionIndex(modelOptions, m.setup.appConfig.LLM.Model)
					m.setup.providerModelDropdownOpen = true
					m.setup.message = "Fetching available models..."
					m.setup.modelsLoading = true
					return m, fetchSetupModelsCmd(m.setup.appConfig)
				}
				modelOptions := m.currentSetupModelOptions()
				if len(modelOptions) == 0 {
					return m, nil
				}
				selected := modelOptions[normalizeDropdownSelectedIdx(m.setup.providerModelDropdownIdx, len(modelOptions))]
				if selected == llmpkg.ManualModelOption {
					m.setup.step = setupStepModelValueField
					m.setup.providerModelDropdownOpen = false
					m.setup.syncInput()
					return m, nil
				}
				m.setup.setCurrentSetupModel(selected)
				m.saveSetupConfigMessage("Default model updated.")
				m.setup.providerModelDropdownOpen = false
			case 2:
				m.setup.providerModelDropdownOpen = false
				m.setup.step = setupStepTaskModelMenu
				m.setup.choiceIdx = 0
				m.setup.taskModelKey = ""
				m.setup.taskModelDropdownOpen = false
				m.setup.taskModelDropdownIdx = 0
				m.setup.message = ""
			}
		case "esc":
			if m.setup.providerModelDropdownOpen {
				m.setup.providerModelDropdownOpen = false
				m.setup.message = ""
				return m, nil
			}
			m.setup.step = setupStepLLMConfigMenu
			m.setup.choiceIdx = 2
			m.setup.providerConfigReturn = false
			m.setup.providerModelDropdownIdx = 0
			m.setup.message = ""
		}
	case setupStepProviderChoice:
		options := m.currentSetupProviderOptions()
		switch msg.String() {
		case "up", "k":
			if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.choiceIdx < len(options)-1 {
				m.setup.choiceIdx++
			}
		case "enter":
			m.setup.setCurrentSetupProvider(options[m.setup.choiceIdx])
			m.setup.appConfig.LLM.Enabled = true
			m.setup.appConfig.LLM.JobFiltering = true
			m.setup.appConfig.LLM.JobSearch = true
			m.setup.appConfig.LLM.CompanyHealth = true
			m.setup.choiceIdx = 0
			m.setup.message = ""
			if m.setup.plan.contains(setupStepLLMConfigMenu) {
				m.saveSetupConfigMessage(fmt.Sprintf("Provider set to %s.", providerLabel(m.setup.appConfig.LLM.Provider)))
				m.setup.setStep(setupStepLLMConfigMenu)
				m.setup.choiceIdx = 1
				return m, nil
			}
			if m.setup.currentSetupProviderNeedsAuth() {
				m.setup.setStep(setupStepAuthModeChoice)
				m.setup.choiceIdx = setupAuthModeIndex(m.setup.currentSetupAuthMode())
				return m, nil
			}
			m.setup.message = "Fetching available models..."
			m.setup.modelsLoading = true
			m.setup.setStep(setupStepModelChoice)
			return m, fetchSetupModelsCmd(m.setup.appConfig)
		case "esc":
			if m.setup.retreatStep() {
				switch m.setup.step {
				case setupStepConfigMenu, setupStepLLMConfigMenu:
					m.setup.choiceIdx = 1
				default:
					m.setup.choiceIdx = 0
				}
			}
		}
	case setupStepModelChoice:
		options := m.currentSetupModelOptions()
		switch msg.String() {
		case "up", "k":
			if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.choiceIdx < len(options)-1 {
				m.setup.choiceIdx++
			}
		case "enter":
			if len(options) > 0 {
				if options[m.setup.choiceIdx] == llmpkg.ManualModelOption {
					m.setup.step = setupStepModelValueField
					m.setup.syncInput()
					return m, nil
				}
				m.setup.setCurrentSetupModel(options[m.setup.choiceIdx])
			}
			if m.setup.providerConfigReturn {
				m.saveSetupConfigMessage("Default model updated.")
				m.setup.step = setupStepProviderConfigMenu
				m.setup.choiceIdx = 1
				return m, nil
			}
			if m.setup.advanceStep() {
				switch m.setup.step {
				case setupStepLLMConfigMenu:
					m.setup.choiceIdx = 2
				case setupStepResumeChoice:
					m.setup.choiceIdx = 0
					m.setup.message = ""
				case setupStepCriteriaField:
					m.setup.fieldIdx = 0
					m.setup.syncInput()
				case setupStepSummary:
					if m.setup.mode == setupModeEdit && m.setup.section == setupSectionLLM {
						return m.completeSetupSave("Saved LLM provider configuration.")
					}
				}
			}
		case "esc":
			if m.setup.providerConfigReturn {
				m.setup.step = setupStepProviderConfigMenu
				m.setup.choiceIdx = 1
				m.setup.message = ""
				return m, nil
			}
			if !m.setup.currentSetupProviderNeedsAuth() {
				m.setup.setStep(setupStepProviderChoice)
				m.setup.choiceIdx = 0
			} else if m.setup.retreatStep() {
				if m.setup.step == setupStepAuthModeChoice {
					m.setup.choiceIdx = setupAuthModeIndex(m.setup.currentSetupAuthMode())
				} else {
					m.setup.choiceIdx = 0
				}
			}
		}
	case setupStepModelValueField:
		switch msg.String() {
		case "esc":
			if m.setup.providerConfigReturn {
				m.setup.step = setupStepProviderConfigMenu
				m.setup.input.Blur()
				m.setup.choiceIdx = 1
				return m, nil
			}
			m.setup.step = setupStepModelChoice
			m.setup.input.Blur()
			m.setup.choiceIdx = 0
		case "enter":
			value := strings.TrimSpace(m.setup.input.Value())
			if value == "" {
				m.setup.message = "Model ID cannot be empty."
				return m, nil
			}
			m.setup.setCurrentSetupModel(value)
			m.setup.input.Blur()
			if m.setup.providerConfigReturn {
				m.saveSetupConfigMessage("Default model updated.")
				m.setup.step = setupStepProviderConfigMenu
				m.setup.choiceIdx = 1
				return m, nil
			}
			m.setup.step = setupStepModelChoice
			if m.setup.advanceStep() {
				switch m.setup.step {
				case setupStepLLMConfigMenu:
					m.setup.choiceIdx = 2
				case setupStepResumeChoice:
					m.setup.choiceIdx = 0
					m.setup.message = ""
				case setupStepCriteriaField:
					m.setup.fieldIdx = 0
					m.setup.syncInput()
				case setupStepSummary:
					if m.setup.mode == setupModeEdit && m.setup.section == setupSectionLLM {
						return m.completeSetupSave("Saved LLM provider configuration.")
					}
				}
			}
		default:
			var cmd tea.Cmd
			m.setup.input, cmd = m.setup.input.Update(msg)
			return m, cmd
		}
	case setupStepTaskModelMenu:
		specs := setupTaskModelSpecs()
		switch msg.String() {
		case "up", "k":
			if m.setup.taskModelDropdownOpen {
				if m.setup.taskModelDropdownIdx > 0 {
					m.setup.taskModelDropdownIdx--
				}
			} else if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.taskModelDropdownOpen {
				options := m.currentSetupTaskModelOptions()
				if m.setup.taskModelDropdownIdx < len(options)-1 {
					m.setup.taskModelDropdownIdx++
				}
			} else if m.setup.choiceIdx < len(specs)-1 {
				m.setup.choiceIdx++
			}
		case "enter":
			spec, ok := setupTaskModelSpecAt(m.setup.choiceIdx)
			if !ok {
				return m, nil
			}
			m.setup.taskModelKey = spec.Key
			if !m.setup.taskModelDropdownOpen {
				options := m.currentSetupTaskModelOptions()
				m.setup.taskModelDropdownIdx = setupTaskModelOptionIndex(options, m.setup.currentSetupTaskModel())
				m.setup.taskModelDropdownOpen = true
				m.setup.message = "Fetching available models..."
				m.setup.modelsLoading = true
				return m, fetchSetupModelsCmd(m.setup.appConfig)
			}
			options := m.currentSetupTaskModelOptions()
			if len(options) == 0 {
				return m, nil
			}
			selected := options[normalizeDropdownSelectedIdx(m.setup.taskModelDropdownIdx, len(options))]
			switch selected {
			case setupTaskModelFallbackOption(m.setup.taskModelKey):
				m.setup.clearCurrentSetupTaskModel()
			case llmpkg.ManualModelOption:
				m.setup.step = setupStepTaskModelValueField
				m.setup.taskModelDropdownOpen = false
				m.setup.syncInput()
				return m, nil
			default:
				m.setup.setCurrentSetupTaskModel(selected)
			}
			m.saveSetupConfigMessage(fmt.Sprintf("%s model updated.", setupTaskModelLabel(m.setup.taskModelKey)))
			m.setup.taskModelDropdownOpen = false
		case "esc":
			if m.setup.taskModelDropdownOpen {
				m.setup.taskModelDropdownOpen = false
				m.setup.message = ""
				return m, nil
			}
			m.setup.step = setupStepProviderConfigMenu
			m.setup.choiceIdx = 2
			m.setup.taskModelKey = ""
			m.setup.taskModelDropdownIdx = 0
			m.setup.message = ""
		}
	case setupStepTaskModelChoice:
		options := m.currentSetupTaskModelOptions()
		switch msg.String() {
		case "up", "k":
			if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.choiceIdx < len(options)-1 {
				m.setup.choiceIdx++
			}
		case "enter":
			if len(options) == 0 {
				return m, nil
			}
			selected := options[m.setup.choiceIdx]
			switch selected {
			case setupTaskModelFallbackOption(m.setup.taskModelKey):
				m.setup.clearCurrentSetupTaskModel()
			case llmpkg.ManualModelOption:
				m.setup.step = setupStepTaskModelValueField
				m.setup.syncInput()
				return m, nil
			default:
				m.setup.setCurrentSetupTaskModel(selected)
			}
			m.saveSetupConfigMessage(fmt.Sprintf("%s model updated.", setupTaskModelLabel(m.setup.taskModelKey)))
			m.setup.step = setupStepTaskModelMenu
			m.setup.choiceIdx = 0
		case "esc":
			m.setup.step = setupStepTaskModelMenu
			m.setup.choiceIdx = 0
			m.setup.message = ""
		}
	case setupStepTaskModelValueField:
		switch msg.String() {
		case "esc":
			m.setup.step = setupStepTaskModelChoice
			m.setup.input.Blur()
			m.setup.choiceIdx = 0
		case "enter":
			value := strings.TrimSpace(m.setup.input.Value())
			m.setup.setCurrentSetupTaskModel(value)
			if value == "" {
				m.saveSetupConfigMessage(fmt.Sprintf("%s model cleared.", setupTaskModelLabel(m.setup.taskModelKey)))
			} else {
				m.saveSetupConfigMessage(fmt.Sprintf("%s model updated.", setupTaskModelLabel(m.setup.taskModelKey)))
			}
			m.setup.input.Blur()
			m.setup.step = setupStepTaskModelMenu
			m.setup.choiceIdx = 0
		default:
			var cmd tea.Cmd
			m.setup.input, cmd = m.setup.input.Update(msg)
			return m, cmd
		}
	case setupStepResumeChoice:
		switch msg.String() {
		case "up", "k":
			if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.choiceIdx < 1 {
				m.setup.choiceIdx++
			}
		case "enter":
			if m.setup.choiceIdx == 0 {
				m.setup.setStep(setupStepResumePathField)
				m.setup.message = ""
				m.setup.syncInput()
				return m, nil
			}
			m.setup.resumePrefillAvailable = false
			m.setup.setStep(setupStepCriteriaField)
			m.setup.fieldIdx = 0
			m.setup.message = ""
			m.setup.syncInput()
		case "esc":
			if m.setup.retreatStep() {
				m.setup.choiceIdx = 0
			}
		}
	case setupStepResumePathField:
		if m.setup.resumeGenerating {
			return m, nil
		}
		switch msg.String() {
		case "esc":
			if m.setup.mode == setupModeEdit || m.setup.mode == setupModeRepair {
				m.setup.setStep(setupStepConfigMenu)
				m.setup.choiceIdx = setupConfigMenuIndexByID(setupMenuResumeCriteria)
				m.setup.syncInput()
				return m, nil
			}
			m.setup.setStep(setupStepResumeChoice)
			m.setup.choiceIdx = 0
			m.setup.syncInput()
		case "enter":
			resumePath := strings.TrimSpace(m.setup.input.Value())
			if resumePath == "" {
				m.setup.message = "Enter a resume path, or press Esc and choose Skip."
				return m, nil
			}
			m.setup.resumePath = resumePath
			m.setup.message = "Reading resume and generating criteria..."
			m.setup.resumeGenerating = true
			m.setup.loadingMinimized = false
			return m, tea.Batch(generateResumeCriteriaCmd(m.setup.appConfig, resumePath), m.restartLoadingIndicator())
		default:
			var cmd tea.Cmd
			m.setup.input, cmd = m.setup.input.Update(msg)
			return m, cmd
		}
	case setupStepAuthModeChoice:
		options := currentSetupAuthModeOptions()
		switch msg.String() {
		case "up", "k":
			if m.setup.choiceIdx > 0 {
				m.setup.choiceIdx--
			}
		case "down", "j":
			if m.setup.choiceIdx < len(options)-1 {
				m.setup.choiceIdx++
			}
		case "enter":
			provider, providerCfg := m.setup.currentSetupProviderConfig()
			providerCfg.Auth.None = false
			providerCfg.Auth.Mode = options[m.setup.choiceIdx]
			if providerCfg.Auth.Mode == llmAuthModeEnv && strings.TrimSpace(providerCfg.Auth.EnvVar) == "" {
				providerCfg.Auth.EnvVar = config.EnvVarForProvider(provider)
			}
			m.setup.setCurrentSetupProviderConfig(provider, providerCfg)
			if m.setup.providerConfigReturn {
				m.setup.step = setupStepAuthValueField
				m.setup.syncInput()
				return m, nil
			}
			if m.setup.advanceStep() {
				m.setup.syncInput()
			}
		case "esc":
			if m.setup.providerConfigReturn {
				m.setup.step = setupStepProviderConfigMenu
				m.setup.choiceIdx = 0
				m.setup.message = ""
				return m, nil
			}
			if m.setup.retreatStep() {
				m.setup.choiceIdx = 0
			}
		}
	case setupStepAuthValueField:
		switch msg.String() {
		case "esc":
			if m.setup.providerConfigReturn {
				m.setup.step = setupStepAuthModeChoice
				m.setup.choiceIdx = setupAuthModeIndex(m.setup.currentSetupAuthMode())
				m.setup.syncInput()
				return m, nil
			}
			if m.setup.retreatStep() {
				m.setup.choiceIdx = setupAuthModeIndex(m.setup.currentSetupAuthMode())
				m.setup.syncInput()
			}
		case "enter":
			value := strings.TrimSpace(m.setup.input.Value())
			switch m.setup.currentSetupAuthMode() {
			case llmAuthModeLiteral:
				if value == "" {
					m.setup.message = "Paste a token, or press Esc and choose environment variable auth."
					return m, nil
				}
			case llmAuthModeCommand:
				if value == "" {
					m.setup.message = "Enter a shell command that prints the token."
					return m, nil
				}
			}
			m.setup.setCurrentSetupAuthValue(value)
			if m.setup.providerConfigReturn {
				m.saveSetupConfigMessage("Provider credentials updated.")
				m.setup.step = setupStepProviderConfigMenu
				m.setup.choiceIdx = 0
				return m, nil
			}
			m.setup.message = "Fetching available models..."
			if m.setup.advanceStep() {
				m.setup.choiceIdx = 0
				m.setup.modelsLoading = true
				return m, fetchSetupModelsCmd(m.setup.appConfig)
			}
		default:
			var cmd tea.Cmd
			m.setup.input, cmd = m.setup.input.Update(msg)
			return m, cmd
		}
	case setupStepCriteriaField:
		switch msg.String() {
		case "esc":
			if m.setup.fieldIdx == 0 {
				if m.setup.retreatStep() {
					if m.setup.step == setupStepConfigMenu {
						m.setup.choiceIdx = 0
					} else {
						m.setup.choiceIdx = 0
					}
				} else if m.setup.mode == setupModeEdit {
					m.setup.setStep(setupStepConfigMenu)
					m.setup.choiceIdx = 0
				}
				return m, nil
			}
			field, ok := setupFieldSpecAt(m.setup.fieldIdx)
			if !ok {
				return m, nil
			}
			options := m.currentSetupCriteriaChoiceOptions(field.Key)
			if len(options) > 0 {
				m.setup.fieldIdx--
				m.setup.syncInput()
				return m, nil
			}
			if setupFieldUsesTextarea(field.Key) {
				m.setup.fieldValues[field.Key] = m.setup.textarea.Value()
			} else {
				m.setup.fieldValues[field.Key] = m.setup.input.Value()
			}
			m.setup.fieldIdx--
			m.setup.syncInput()
		case "enter":
			field, ok := setupFieldSpecAt(m.setup.fieldIdx)
			if !ok {
				return m, nil
			}
			options := m.currentSetupCriteriaChoiceOptions(field.Key)
			if len(options) == 0 && setupFieldUsesTextarea(field.Key) {
				m.setup.fieldValues[field.Key] = m.setup.textarea.Value()
			} else if len(options) == 0 {
				m.setup.fieldValues[field.Key] = m.setup.input.Value()
			}
			if m.setup.fieldIdx == len(searchProfileGroupSpec().Fields)-1 {
				m.setup.advanceStep()
			} else {
				m.setup.fieldIdx++
				m.setup.syncInput()
			}
		case "ctrl+s":
			field, ok := setupFieldSpecAt(m.setup.fieldIdx)
			if !ok || !setupFieldUsesTextarea(field.Key) {
				return m, nil
			}
			m.setup.fieldValues[field.Key] = m.setup.textarea.Value()
			if m.setup.fieldIdx == len(searchProfileGroupSpec().Fields)-1 {
				m.setup.advanceStep()
			} else {
				m.setup.fieldIdx++
				m.setup.syncInput()
			}
		case "ctrl+u":
			field, ok := setupFieldSpecAt(m.setup.fieldIdx)
			if !ok || !setupFieldUsesTextarea(field.Key) {
				var cmd tea.Cmd
				m.setup.input, cmd = m.setup.input.Update(msg)
				return m, cmd
			}
			m.setup.textarea.SetValue("")
			m.setup.textarea.Focus()
			return m, nil
		case "up", "k":
			field, ok := setupFieldSpecAt(m.setup.fieldIdx)
			options := m.currentSetupCriteriaChoiceOptions(field.Key)
			if ok && len(options) > 0 {
				if m.setup.choiceIdx > 0 {
					m.setup.choiceIdx--
				}
				return m, nil
			}
			if ok && setupFieldUsesTextarea(field.Key) {
				var cmd tea.Cmd
				m.setup.textarea, cmd = m.setup.textarea.Update(msg)
				return m, cmd
			}
			var cmd tea.Cmd
			m.setup.input, cmd = m.setup.input.Update(msg)
			return m, cmd
		case "down", "j":
			field, ok := setupFieldSpecAt(m.setup.fieldIdx)
			options := m.currentSetupCriteriaChoiceOptions(field.Key)
			if ok && len(options) > 0 {
				if m.setup.choiceIdx < len(options)-1 {
					m.setup.choiceIdx++
				}
				return m, nil
			}
			if ok && setupFieldUsesTextarea(field.Key) {
				var cmd tea.Cmd
				m.setup.textarea, cmd = m.setup.textarea.Update(msg)
				return m, cmd
			}
			var cmd tea.Cmd
			m.setup.input, cmd = m.setup.input.Update(msg)
			return m, cmd
		case " ", "x":
			field, ok := setupFieldSpecAt(m.setup.fieldIdx)
			options := m.currentSetupCriteriaChoiceOptions(field.Key)
			if ok && len(options) > 0 {
				if m.setup.choiceIdx >= 0 && m.setup.choiceIdx < len(options) {
					m.toggleSetupCriteriaChoice(field.Key, options[m.setup.choiceIdx].Value)
				}
				return m, nil
			}
			if ok && setupFieldUsesTextarea(field.Key) {
				var cmd tea.Cmd
				m.setup.textarea, cmd = m.setup.textarea.Update(msg)
				return m, cmd
			}
			var cmd tea.Cmd
			m.setup.input, cmd = m.setup.input.Update(msg)
			return m, cmd
		default:
			field, ok := setupFieldSpecAt(m.setup.fieldIdx)
			if ok && setupFieldUsesTextarea(field.Key) {
				var cmd tea.Cmd
				m.setup.textarea, cmd = m.setup.textarea.Update(msg)
				return m, cmd
			}
			var cmd tea.Cmd
			m.setup.input, cmd = m.setup.input.Update(msg)
			return m, cmd
		}
	case setupStepSummary:
		switch msg.String() {
		case "esc":
			if m.setup.retreatStep() {
				switch m.setup.step {
				case setupStepCriteriaField:
					m.setup.fieldIdx = len(searchProfileGroupSpec().Fields) - 1
					m.setup.syncInput()
				case setupStepConfigMenu:
					m.setup.choiceIdx = setupConfigMenuIndexForSection(m.setup.section)
				case setupStepModelChoice:
					m.setup.choiceIdx = 0
				}
			}
		case "enter":
			if err := m.refreshSetupGeneratedState(true); err != nil {
				m.setup.message = fmt.Sprintf("Could not validate setup: %v", err)
				return m, nil
			}
			if m.setup.useLLM && !config.LLMAuthAvailableNow(&m.setup.appConfig) {
				return m.redirectSetupToLLMAuth("Saved your criteria. Finish LLM auth here, or choose 'No' in LLM settings to continue without LLM.")
			}
			if m.setup.mode == setupModeEdit && (m.setup.section == setupSectionCriteria || m.setup.section == setupSectionLLM) {
				if !m.saveSetupArtifactsOrMessage() {
					return m, nil
				}
				return m.completeSetupSave("Saved configuration changes.")
			}
			if m.setup.mode == setupModeRepair {
				if !m.saveSetupArtifactsOrMessage() {
					return m, nil
				}
				return m.completeSetupSave("Saved configuration changes.")
			}
			if m.setup.useLLM {
				m.setup.advanceStep()
			} else {
				if !m.saveSetupArtifactsOrMessage() {
					return m, nil
				}
				return m, m.beginSetupPreview()
			}
		}
	case setupStepPromptReview:
		switch msg.String() {
		case "esc":
			if m.setup.retreatStep() && m.setup.step == setupStepConfigMenu {
				m.setup.choiceIdx = setupConfigMenuIndexByID(setupMenuPrompt)
			}
		case "e", "E":
			return m, tea.ExecProcess(exec.Command("true"), func(err error) tea.Msg {
				return editPromptContent(m.setup.prompt)()
			})
		case "enter":
			if m.setup.useLLM && !config.LLMAuthAvailableNow(&m.setup.appConfig) {
				return m.redirectSetupToLLMAuth("Saved your prompt. Finish LLM auth here, or choose 'No' in LLM settings to continue without LLM.")
			}
			if !m.saveSetupArtifactsOrMessage() {
				return m, nil
			}
			if m.setup.mode == setupModeEdit && m.setup.section == setupSectionPrompt {
				return m.completeSetupSave("Saved configuration changes.")
			}
			if m.setup.mode == setupModeRepair {
				return m.completeSetupSave("Saved configuration changes.")
			}
			return m, m.beginSetupPreview()
		}
	case setupStepPreviewConfirm:
		if m.setup.previewBusy {
			return m, nil
		}
		switch msg.String() {
		case "esc", "r":
			m.setup.retreatStep()
			m.setup.previewErr = ""
			m.setup.previewJobs = nil
		case "enter":
			existingCount := len(m.allJobs)
			previewCount := len(m.setup.previewJobs)
			added, merged := storage.MergeJobs(m.allJobs, m.setup.previewJobs)
			if added > 0 {
				if err := saveRuntimeJobs(merged); err != nil {
					m.setup.previewErr = fmt.Sprintf("Could not save jobs: %v", err)
					return m, nil
				}
				m.allJobs = merged
				m.applyFilterAndSort()
			}
			m.clearOverlay()
			m.setup.mode = setupModeEdit
			m.setup.firstRun = false
			m.setup.message = ""
			m.setup.previewErr = ""
			m.setup.previewBusy = false
			m.setup.previewJobs = nil
			m.setupRequired = false
			m.showNotice("Setup Saved", setupPreviewNotice(existingCount, previewCount, added), false)
		}
	}

	return m, nil
}
