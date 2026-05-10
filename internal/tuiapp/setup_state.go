package tuiapp

import (
	"fmt"
	"strings"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
	setupcfg "github.com/wallentx/jobscout/internal/setup"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type setupStep int

const (
	setupStepRecoveryChoice setupStep = iota
	setupStepLLMChoice
	setupStepConfigMenu
	setupStepLLMConfigMenu
	setupStepProviderConfigMenu
	setupStepProviderChoice
	setupStepModelChoice
	setupStepModelValueField
	setupStepTaskModelMenu
	setupStepTaskModelChoice
	setupStepTaskModelValueField
	setupStepAuthModeChoice
	setupStepAuthValueField
	setupStepResumeChoice
	setupStepResumePathField
	setupStepCriteriaField
	setupStepSummary
	setupStepPromptReview
	setupStepPreviewConfirm
)

type setupState struct {
	active                    bool
	firstRun                  bool
	mode                      setupMode
	step                      setupStep
	plan                      setupPlan
	choiceIdx                 int
	fieldIdx                  int
	input                     textinput.Model
	textarea                  textarea.Model
	useLLM                    bool
	appConfig                 AppConfig
	generated                 *CriteriaConfig
	prompt                    string
	previewErr                string
	previewBusy               bool
	loadingMinimized          bool
	previewJobs               []Job
	modelsByProvider          map[string][]string
	modelsLoading             bool
	taskModelKey              string
	taskModelDropdownOpen     bool
	taskModelDropdownIdx      int
	providerDropdownOpen      bool
	providerDropdownIdx       int
	providerModelDropdownOpen bool
	providerModelDropdownIdx  int
	providerConfigReturn      bool
	resumePrefillAvailable    bool
	resumeGenerating          bool
	resumePath                string
	fieldValues               map[string]string
	section                   setupSection
	message                   string
}

func newSetupState(mode setupMode, section setupSection) setupState {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 50
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetWidth(50)
	ta.SetHeight(4)

	appCfg := config.DefaultAppConfig()
	criteriaCfg := config.DefaultCriteriaConfig()

	if cfg, err := config.LoadAppConfig(runtimeConfigPath); err == nil {
		appCfg = *cfg
	}
	config.NormalizeLLMConfig(&appCfg)
	if cfg, err := config.LoadCriteriaConfig(runtimeConfigPath); err == nil {
		criteriaCfg = *cfg
	}

	state := setupState{
		active:           true,
		firstRun:         mode != setupModeEdit,
		mode:             mode,
		useLLM:           appCfg.LLM.Enabled,
		appConfig:        appCfg,
		input:            ti,
		textarea:         ta,
		section:          section,
		modelsByProvider: make(map[string][]string),
	}

	state.fieldValues = setupFieldValues(criteriaCfg)
	if !state.useLLM {
		state.appConfig.LLM.JobFiltering = false
		state.appConfig.LLM.JobSearch = false
		state.appConfig.LLM.CompanyHealth = false
	} else if state.appConfig.LLM.Model == "" {
		state.appConfig.LLM.Model = config.DefaultModelForProvider(state.appConfig.LLM.Provider)
	}
	if prompt, err := config.LoadSearchPrompt(runtimeSearchPromptPath); err == nil {
		state.prompt = prompt
	}
	state.rebuildPlan()
	state.setStep(setupStepLLMChoice)
	if state.step == setupStepConfigMenu {
		state.choiceIdx = setupConfigMenuIndexForSection(section)
	}
	state.syncInput()

	return state
}

func setupFieldValues(cfg CriteriaConfig) map[string]string {
	return setupcfg.SearchProfileValues(cfg)
}

func setupFieldUsesTextarea(key string) bool {
	return setupcfg.FieldUsesTextarea(key)
}

func (s *setupState) syncInput() {
	switch s.step {
	case setupStepCriteriaField:
		field, ok := setupFieldSpecAt(s.fieldIdx)
		if !ok {
			s.input.SetValue("")
			s.input.Blur()
			s.textarea.Blur()
			return
		}
		if field.Key == "role_families" || field.Key == "filters.work_settings" {
			s.choiceIdx = 0
			s.input.SetValue("")
			s.input.Blur()
			s.textarea.Blur()
			return
		}

		if setupFieldUsesTextarea(field.Key) {
			s.input.Blur()
			s.textarea.Placeholder = field.Help
			s.textarea.SetValue(s.fieldValues[field.Key])
			s.textarea.CursorStart()
			s.textarea.Focus()
			return
		}

		s.textarea.Blur()
		s.input.Placeholder = field.Help
		s.input.SetValue(s.fieldValues[field.Key])
		s.input.Focus()
		s.input.CursorEnd()
	case setupStepAuthValueField:
		s.textarea.Blur()
		label, help := s.currentSetupAuthField()
		s.input.Placeholder = help
		s.input.SetValue(s.currentSetupAuthValue())
		s.input.Focus()
		s.input.CursorEnd()
		_ = label
	case setupStepModelValueField:
		s.textarea.Blur()
		s.input.Placeholder = "Model ID, e.g. gpt-5.4"
		s.input.SetValue(s.appConfig.LLM.Model)
		s.input.Focus()
		s.input.CursorEnd()
	case setupStepTaskModelValueField:
		s.textarea.Blur()
		s.input.Placeholder = "Task model ID, or leave empty to use fallback"
		s.input.SetValue(s.currentSetupTaskModel())
		s.input.Focus()
		s.input.CursorEnd()
	case setupStepResumePathField:
		s.textarea.Blur()
		s.input.Placeholder = "~/Documents/resume.pdf"
		s.input.SetValue(s.resumePath)
		s.input.Focus()
		s.input.CursorEnd()
	default:
		s.input.Blur()
		s.textarea.Blur()
		return
	}
}

func (m *model) currentSetupProviderOptions() []string {
	return config.ProviderOptions()
}

func providerLabel(provider string) string {
	return setupcfg.ProviderLabel(provider)
}

func providerOptionLabel(provider string) string {
	return setupcfg.ProviderOptionLabel(provider)
}

func providerOptionLabels(providers []string) []string {
	return setupcfg.ProviderOptionLabels(providers)
}

func providerOptionIndex(provider string) int {
	return setupcfg.ProviderOptionIndex(provider)
}

func (m *model) currentSetupLLMConfigOptions() []string {
	enabled := "Disabled"
	if m.setup.useLLM {
		enabled = "Enabled"
	}
	return []string{
		"LLM Features: " + enabled,
		"Provider: " + providerLabel(m.setup.appConfig.LLM.Provider),
		"Provider Config",
	}
}

func (m *model) currentSetupProviderConfigOptions() []string {
	return []string{
		"Credentials",
		"Default Model",
		"Task Models",
	}
}

func optionsEnabledDisabled(enabled bool) string {
	return setupcfg.OptionsEnabledDisabled(enabled)
}

func (m *model) currentSetupModelOptions() []string {
	return llmpkg.SetupModelOptions(m.setup.appConfig.LLM.Provider, &m.setup.appConfig, m.setup.modelsByProvider)
}

func (s *setupState) setCurrentSetupModel(modelName string) {
	provider, providerCfg := s.currentSetupProviderConfig()
	providerCfg.Model = strings.TrimSpace(modelName)
	s.setCurrentSetupProviderConfig(provider, providerCfg)
}

type setupTaskModelSpec struct {
	Key   string
	Label string
}

func setupTaskModelSpecs() []setupTaskModelSpec {
	specs := setupcfg.TaskModelSpecs()
	out := make([]setupTaskModelSpec, 0, len(specs))
	for _, spec := range specs {
		out = append(out, setupTaskModelSpec{Key: spec.Key, Label: spec.Label})
	}
	return out
}

func setupTaskModelLabel(key string) string {
	return setupcfg.TaskModelLabel(key)
}

func setupTaskModelSpecAt(idx int) (setupTaskModelSpec, bool) {
	spec, ok := setupcfg.TaskModelSpecAt(idx)
	if !ok {
		return setupTaskModelSpec{}, false
	}
	return setupTaskModelSpec{Key: spec.Key, Label: spec.Label}, true
}

func setupTaskModelLabelWidth() int {
	width := 0
	for _, spec := range setupTaskModelSpecs() {
		label := spec.Label + ":"
		if labelWidth := lipgloss.Width(label); labelWidth > width {
			width = labelWidth
		}
	}
	return width
}

func (m *model) currentSetupTaskModelMenuOptions() []string {
	_, providerCfg := m.setup.currentSetupProviderConfig()
	options := make([]string, 0, len(setupTaskModelSpecs()))
	for _, spec := range setupTaskModelSpecs() {
		modelName := strings.TrimSpace(providerCfg.Models[spec.Key])
		if modelName == "" {
			modelName = "provider default"
		}
		options = append(options, fmt.Sprintf("%s: %s", spec.Label, modelName))
	}
	return options
}

func (m *model) setupTaskModelDisplayValue(key string) string {
	_, providerCfg := m.setup.currentSetupProviderConfig()
	modelName := strings.TrimSpace(providerCfg.Models[config.NormalizeLLMTaskKey(key)])
	if modelName == "" {
		return "provider default"
	}
	return modelName
}

func (m *model) currentSetupTaskModelOptions() []string {
	options := []string{setupTaskModelFallbackOption(m.setup.taskModelKey)}
	models := llmpkg.SetupModelOptions(m.setup.appConfig.LLM.Provider, &m.setup.appConfig, m.setup.modelsByProvider)
	for _, modelName := range models {
		options = appendUniqueString(options, modelName)
	}
	options = appendUniqueString(options, m.setup.currentSetupTaskModel())
	return options
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func setupTaskModelOptionIndex(options []string, currentModel string) int {
	currentModel = strings.TrimSpace(currentModel)
	if currentModel == "" {
		return 0
	}
	for idx, option := range options {
		if option == currentModel {
			return idx
		}
	}
	return 0
}

func setupTaskModelFallbackOption(task string) string {
	return setupcfg.TaskModelFallbackOption(task)
}

func (s *setupState) currentSetupTaskModel() string {
	key := config.NormalizeLLMTaskKey(s.taskModelKey)
	if key == "" {
		return ""
	}
	_, providerCfg := s.currentSetupProviderConfig()
	return strings.TrimSpace(providerCfg.Models[key])
}

func (s *setupState) setCurrentSetupTaskModel(modelName string) {
	key := config.NormalizeLLMTaskKey(s.taskModelKey)
	if key == "" {
		return
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		s.clearCurrentSetupTaskModel()
		return
	}
	provider, providerCfg := s.currentSetupProviderConfig()
	if providerCfg.Models == nil {
		providerCfg.Models = make(map[string]string)
	}
	providerCfg.Models[key] = modelName
	s.setCurrentSetupProviderConfig(provider, providerCfg)
}

func (s *setupState) clearCurrentSetupTaskModel() {
	key := config.NormalizeLLMTaskKey(s.taskModelKey)
	if key == "" {
		return
	}
	provider, providerCfg := s.currentSetupProviderConfig()
	if len(providerCfg.Models) == 0 {
		return
	}
	delete(providerCfg.Models, key)
	if len(providerCfg.Models) == 0 {
		providerCfg.Models = nil
	}
	s.setCurrentSetupProviderConfig(provider, providerCfg)
}

func (s *setupState) setCurrentSetupProvider(provider string) {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "gemini"
	}
	if s.appConfig.LLM.Providers == nil {
		s.appConfig.LLM.Providers = config.DefaultLLMProviders()
	}
	providerCfg, ok := s.appConfig.LLM.Providers[provider]
	if !ok {
		providerCfg = config.DefaultLLMProviders()[provider]
	}
	providerCfg = config.NormalizeLLMProviderConfig(provider, providerCfg)
	s.appConfig.LLM.Provider = provider
	s.appConfig.LLM.Providers[provider] = providerCfg
	s.appConfig.LLM.Model = providerCfg.Model
	s.appConfig.LLM.Auth = providerCfg.Auth
}

func currentSetupAuthModeOptions() []string {
	return setupcfg.AuthModeOptions()
}

func setupAuthModeLabel(mode string, provider string) string {
	return setupcfg.AuthModeLabel(mode, provider)
}

func setupAuthModeIndex(mode string) int {
	return setupcfg.AuthModeIndex(mode)
}

func (s *setupState) currentSetupProviderConfig() (string, LLMProviderConfig) {
	config.NormalizeLLMConfig(&s.appConfig)
	provider := strings.TrimSpace(s.appConfig.LLM.Provider)
	if provider == "" {
		provider = "gemini"
	}
	return provider, config.NormalizeLLMProviderConfig(provider, s.appConfig.LLM.Providers[provider])
}

func (s *setupState) setCurrentSetupProviderConfig(provider string, providerCfg LLMProviderConfig) {
	config.NormalizeLLMConfig(&s.appConfig)
	s.appConfig.LLM.Provider = provider
	s.appConfig.LLM.Providers[provider] = config.NormalizeLLMProviderConfig(provider, providerCfg)
	s.appConfig.LLM.Model = s.appConfig.LLM.Providers[provider].Model
	s.appConfig.LLM.Auth = s.appConfig.LLM.Providers[provider].Auth
}

func (m *model) saveSetupConfigMessage(success string) {
	if err := config.SaveAppConfig(runtimeConfigPath, &m.setup.appConfig); err != nil {
		m.setup.message = fmt.Sprintf("Could not save configuration: %v", err)
		return
	}
	m.setup.message = success
}

func (s *setupState) currentSetupAuthMode() string {
	provider, providerCfg := s.currentSetupProviderConfig()
	mode := strings.ToLower(strings.TrimSpace(providerCfg.Auth.Mode))
	if providerCfg.Auth.None {
		return ""
	}
	if mode == "" {
		mode = config.NormalizeLLMProviderConfig(provider, providerCfg).Auth.Mode
	}
	if mode == "" {
		return llmAuthModeEnv
	}
	return mode
}

func (s *setupState) currentSetupProviderNeedsAuth() bool {
	_, providerCfg := s.currentSetupProviderConfig()
	return !providerCfg.Auth.None
}

func (s *setupState) currentSetupAuthField() (string, string) {
	provider, _ := s.currentSetupProviderConfig()
	switch s.currentSetupAuthMode() {
	case llmAuthModeLiteral:
		return "API Token", "Token loaded from an existing legacy config"
	case llmAuthModeCommand:
		return "Auth Command", "Shell command that prints the token"
	default:
		return "Environment Variable", fmt.Sprintf("Environment variable name, e.g. %s", config.EnvVarForProvider(provider))
	}
}

func (s *setupState) currentSetupAuthValue() string {
	provider, providerCfg := s.currentSetupProviderConfig()
	switch s.currentSetupAuthMode() {
	case llmAuthModeLiteral:
		return providerCfg.Auth.Value
	case llmAuthModeCommand:
		return providerCfg.Auth.Command
	default:
		envVar := strings.TrimSpace(providerCfg.Auth.EnvVar)
		if envVar == "" {
			envVar = config.EnvVarForProvider(provider)
		}
		return envVar
	}
}

func (s *setupState) setCurrentSetupAuthValue(value string) {
	provider, providerCfg := s.currentSetupProviderConfig()
	switch s.currentSetupAuthMode() {
	case llmAuthModeLiteral:
		providerCfg.Auth.Value = strings.TrimSpace(value)
	case llmAuthModeCommand:
		providerCfg.Auth.Command = strings.TrimSpace(value)
	default:
		envVar := strings.TrimSpace(value)
		if envVar == "" {
			envVar = config.EnvVarForProvider(provider)
		}
		providerCfg.Auth.EnvVar = envVar
	}
	s.setCurrentSetupProviderConfig(provider, providerCfg)
}

func (m *model) redirectSetupToLLMAuth(message string) (tea.Model, tea.Cmd) {
	if !m.saveSetupArtifactsOrMessage() {
		return m, nil
	}

	m.setup.configureSection(setupSectionLLM)
	m.setup.message = message

	switch {
	case !m.setup.useLLM:
		m.setup.setStep(setupStepLLMConfigMenu)
		m.setup.choiceIdx = 0
	case strings.TrimSpace(m.setup.appConfig.LLM.Provider) == "":
		m.setup.setStep(setupStepProviderChoice)
		m.setup.choiceIdx = 0
	case strings.TrimSpace(m.setup.appConfig.LLM.Model) == "":
		m.setup.setStep(setupStepModelChoice)
		m.setup.choiceIdx = 0
	default:
		m.setup.setStep(setupStepAuthModeChoice)
		m.setup.choiceIdx = setupAuthModeIndex(m.setup.currentSetupAuthMode())
	}
	m.setup.syncInput()
	return m, nil
}

func (m *model) currentSetupConfigOptions() []string {
	caps := m.currentSetupCapabilities()
	items := setupConfigMenuSpecs()
	options := make([]string, 0, len(items))
	for _, item := range items {
		label := item.Label(m.setup)
		if status := m.setupConfigMenuStatus(item.ID, caps); status != "" {
			label += " [" + status + "]"
		}
		options = append(options, label)
	}
	return options
}

func (m *model) currentSetupCapabilities() config.RuntimeCapabilities {
	cfg := m.setup.appConfig
	if criteriaCfg, err := m.setupCriteriaFromState(); err == nil {
		cfg.Criteria = *criteriaCfg
	}

	promptPresent := true
	if cfg.LLM.JobSearch {
		promptPresent = strings.TrimSpace(m.setup.prompt) != ""
		if !promptPresent {
			promptPresent = config.SearchPromptPresent(runtimeSearchPromptPath)
		}
	}

	return config.EvaluateCapabilitiesForConfig(&cfg, promptPresent)
}

func (m *model) currentSetupRoleFamilies() []RoleFamilyID {
	values, err := domain.ParseRoleFamilyCSV(m.setup.fieldValues["role_families"])
	if err != nil {
		return nil
	}
	return values
}

func (m *model) setupRoleFamilySelected(id RoleFamilyID) bool {
	for _, role := range m.currentSetupRoleFamilies() {
		if role == id {
			return true
		}
	}
	return false
}

func (m *model) toggleSetupRoleFamily(id RoleFamilyID) {
	current := m.currentSetupRoleFamilies()
	selected := make([]RoleFamilyID, 0, len(current))
	removed := false
	for _, role := range current {
		if role == id {
			removed = true
			continue
		}
		selected = append(selected, role)
	}
	if !removed {
		selected = append(selected, id)
	}
	m.setup.fieldValues["role_families"] = domain.FormatRoleFamilyIDs(selected)
}

func setupWorkSettingOptions() []setupCriteriaChoiceOption {
	return []setupCriteriaChoiceOption{
		{Value: "remote", Label: "Remote"},
		{Value: "hybrid", Label: "Hybrid"},
		{Value: "onsite", Label: "Onsite"},
	}
}

func (m *model) currentSetupCriteriaChoiceOptions(fieldKey string) []setupCriteriaChoiceOption {
	switch fieldKey {
	case "role_families":
		specs := domain.RoleFamilySpecs()
		options := make([]setupCriteriaChoiceOption, 0, len(specs))
		for _, spec := range specs {
			options = append(options, setupCriteriaChoiceOption{
				Value: string(spec.ID),
				Label: spec.Label,
			})
		}
		return options
	case "filters.work_settings":
		return setupWorkSettingOptions()
	default:
		return nil
	}
}

func (m *model) currentSetupCriteriaChoiceSummary(fieldKey string) string {
	switch fieldKey {
	case "role_families":
		return domain.FormatRoleFamilyLabels(m.currentSetupRoleFamilies())
	case "filters.work_settings":
		return joinOrFallback(domain.SelectedWorkSettings(domain.ParseWorkSettings(m.setup.fieldValues[fieldKey])), "none")
	default:
		return "none"
	}
}

func (m *model) setupCriteriaChoiceSelected(fieldKey string, value string) bool {
	switch fieldKey {
	case "role_families":
		return m.setupRoleFamilySelected(RoleFamilyID(value))
	case "filters.work_settings":
		for _, item := range domain.SelectedWorkSettings(domain.ParseWorkSettings(m.setup.fieldValues[fieldKey])) {
			if item == value {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (m *model) toggleSetupCriteriaChoice(fieldKey string, value string) {
	switch fieldKey {
	case "role_families":
		m.toggleSetupRoleFamily(RoleFamilyID(value))
	case "filters.work_settings":
		settings := domain.ParseWorkSettings(m.setup.fieldValues[fieldKey])
		switch value {
		case "remote":
			settings.Remote = !settings.Remote
		case "hybrid":
			settings.Hybrid = !settings.Hybrid
		case "onsite":
			settings.Onsite = !settings.Onsite
		}
		m.setup.fieldValues[fieldKey] = strings.Join(domain.SelectedWorkSettings(settings), ", ")
	}
}

func (m *model) setupConfigMenuStatus(id setupMenuID, caps config.RuntimeCapabilities) string {
	if m.setup.mode != setupModeRepair {
		return ""
	}

	switch id {
	case setupMenuCriteria:
		if !caps.SearchProfileReady || !caps.SearchSourcesReady {
			return "needs attention"
		}
		return "ready"
	case setupMenuLLM:
		if !m.setup.useLLM {
			return "optional"
		}
		if !caps.LLMConfigured || !caps.LLMAuthAvailableNow {
			return "needs attention"
		}
		return "ready"
	case setupMenuPrompt:
		if !m.setup.useLLM {
			return "unused"
		}
		if !caps.SearchPromptReady {
			return "needs attention"
		}
		return "ready"
	default:
		return ""
	}
}
