package tuiapp

import (
	"fmt"

	"github.com/wallentx/jobscout/internal/config"
	setupcfg "github.com/wallentx/jobscout/internal/setup"

	tea "github.com/charmbracelet/bubbletea"
)

type FieldSpec = setupcfg.FieldSpec
type GroupSpec = setupcfg.GroupSpec

type setupMenuItemSpec struct {
	ID     setupMenuID
	Label  func(setupState) string
	Select func(*model) tea.Cmd
}

type setupMenuID string

const (
	setupMenuCriteria          setupMenuID = "criteria"
	setupMenuResumeCriteria    setupMenuID = "resume_criteria"
	setupMenuLLM               setupMenuID = "llm"
	setupMenuSourcesEnabled    setupMenuID = "sources_enabled"
	setupMenuRSSEnabled        setupMenuID = "rss_enabled"
	setupMenuSiteSearchEnabled setupMenuID = "site_search_enabled"
	setupMenuBuiltinsEnabled   setupMenuID = "builtins_enabled"
	setupMenuPrompt            setupMenuID = "prompt"
)

type setupMode int

const (
	setupModeBootstrap setupMode = iota
	setupModeRepair
	setupModeEdit
	setupModeRecovery
)

type setupSection string

const (
	setupSectionNone     setupSection = ""
	setupSectionCriteria setupSection = "criteria"
	setupSectionLLM      setupSection = "llm"
	setupSectionPrompt   setupSection = "prompt"
)

type setupPlan struct {
	steps []setupStep
}

func newSetupPlan(mode setupMode, section setupSection, useLLM bool, resumePrefill bool) setupPlan {
	steps := make([]setupStep, 0, 10)

	appendUnique := func(step setupStep) {
		for _, existing := range steps {
			if existing == step {
				return
			}
		}
		steps = append(steps, step)
	}

	switch mode {
	case setupModeRecovery:
		appendUnique(setupStepRecoveryChoice)
	case setupModeEdit:
		appendUnique(setupStepConfigMenu)
		switch section {
		case setupSectionCriteria:
			appendUnique(setupStepCriteriaField)
			appendUnique(setupStepSummary)
		case setupSectionLLM:
			appendUnique(setupStepLLMConfigMenu)
			appendUnique(setupStepProviderChoice)
			appendUnique(setupStepAuthModeChoice)
			appendUnique(setupStepAuthValueField)
			appendUnique(setupStepModelChoice)
			appendUnique(setupStepSummary)
		case setupSectionPrompt:
			appendUnique(setupStepPromptReview)
		}
	case setupModeRepair:
		switch section {
		case setupSectionCriteria:
			appendUnique(setupStepConfigMenu)
			appendUnique(setupStepCriteriaField)
			appendUnique(setupStepSummary)
		case setupSectionLLM:
			appendUnique(setupStepConfigMenu)
			appendUnique(setupStepLLMConfigMenu)
			appendUnique(setupStepProviderChoice)
			appendUnique(setupStepAuthModeChoice)
			appendUnique(setupStepAuthValueField)
			appendUnique(setupStepModelChoice)
			appendUnique(setupStepSummary)
		case setupSectionPrompt:
			appendUnique(setupStepConfigMenu)
			appendUnique(setupStepPromptReview)
		default:
			appendUnique(setupStepConfigMenu)
		}
	default:
		appendUnique(setupStepLLMChoice)
		if useLLM {
			appendUnique(setupStepProviderChoice)
			appendUnique(setupStepAuthModeChoice)
			appendUnique(setupStepAuthValueField)
			appendUnique(setupStepModelChoice)
			if resumePrefill {
				appendUnique(setupStepResumeChoice)
				appendUnique(setupStepResumePathField)
			}
		}
		appendUnique(setupStepCriteriaField)
		appendUnique(setupStepSummary)
		if useLLM {
			appendUnique(setupStepPromptReview)
		}
		appendUnique(setupStepPreviewConfirm)
	}

	if len(steps) == 0 {
		appendUnique(setupStepConfigMenu)
	}

	return setupPlan{steps: steps}
}

func (p setupPlan) contains(step setupStep) bool {
	for _, candidate := range p.steps {
		if candidate == step {
			return true
		}
	}
	return false
}

func (p setupPlan) first() (setupStep, bool) {
	if len(p.steps) == 0 {
		return 0, false
	}
	return p.steps[0], true
}

func (p setupPlan) next(step setupStep) (setupStep, bool) {
	for i, candidate := range p.steps {
		if candidate == step && i+1 < len(p.steps) {
			return p.steps[i+1], true
		}
	}
	return 0, false
}

func (p setupPlan) prev(step setupStep) (setupStep, bool) {
	for i, candidate := range p.steps {
		if candidate == step && i > 0 {
			return p.steps[i-1], true
		}
	}
	return 0, false
}

func setupModeForCapabilities(caps config.RuntimeCapabilities) setupMode {
	if caps.ConfigExists {
		return setupModeRepair
	}
	return setupModeBootstrap
}

func setupSectionForCapabilities(caps config.RuntimeCapabilities) setupSection {
	switch {
	case !caps.SearchProfileReady || !caps.SearchSourcesReady:
		return setupSectionCriteria
	case caps.LLMDisabled && caps.LLMFeaturesSelected:
		return setupSectionLLM
	case caps.LLMPreferred && !caps.LLMConfigured:
		return setupSectionLLM
	case caps.LLMPreferred && !caps.SearchPromptReady:
		return setupSectionPrompt
	default:
		return setupSectionNone
	}
}

func setupTitleForMode(mode setupMode) string {
	switch mode {
	case setupModeRecovery:
		return "LLM recovery"
	case setupModeRepair:
		return "Configuration repair"
	case setupModeEdit:
		return "Configuration"
	default:
		return "Setup required"
	}
}

func setupSectionLabel(section setupSection) string {
	switch section {
	case setupSectionCriteria:
		return "Search criteria"
	case setupSectionLLM:
		return "LLM settings"
	case setupSectionPrompt:
		return "Search prompt"
	default:
		return "Configuration"
	}
}

func (s *setupState) configureSection(section setupSection) {
	s.section = section
	s.rebuildPlan()
}

func (s *setupState) rebuildPlan() {
	s.plan = newSetupPlan(s.mode, s.section, s.useLLM, s.resumePrefillAvailable)
}

func (s *setupState) setStep(step setupStep) {
	s.rebuildPlan()
	if s.plan.contains(step) {
		s.step = step
		return
	}
	if first, ok := s.plan.first(); ok {
		s.step = first
	}
}

func (s *setupState) advanceStep() bool {
	s.rebuildPlan()
	next, ok := s.plan.next(s.step)
	if !ok {
		return false
	}
	s.step = next
	return true
}

func (s *setupState) retreatStep() bool {
	s.rebuildPlan()
	prev, ok := s.plan.prev(s.step)
	if !ok {
		return false
	}
	s.step = prev
	return true
}

func setupConfigMenuSpecs() []setupMenuItemSpec {
	return []setupMenuItemSpec{
		{
			ID: setupMenuCriteria,
			Label: func(setup setupState) string {
				return "Search criteria"
			},
			Select: func(m *model) tea.Cmd {
				m.setup.configureSection(setupSectionCriteria)
				m.setup.setStep(setupStepCriteriaField)
				m.setup.fieldIdx = 0
				m.setup.message = ""
				m.setup.syncInput()
				return nil
			},
		},
		{
			ID: setupMenuResumeCriteria,
			Label: func(setup setupState) string {
				return "Populate criteria from resume"
			},
			Select: func(m *model) tea.Cmd {
				if !m.setup.useLLM {
					m.setup.message = "Enable LLM settings before generating criteria from a resume."
					return nil
				}
				if !config.LLMAuthAvailableNow(&m.setup.appConfig) {
					m.setup.configureSection(setupSectionLLM)
					m.setup.message = "Finish LLM provider auth before generating criteria from a resume."
					m.setup.setStep(setupStepLLMConfigMenu)
					m.setup.choiceIdx = 2
					return nil
				}
				m.setup.configureSection(setupSectionCriteria)
				m.setup.step = setupStepResumePathField
				m.setup.resumePath = ""
				m.setup.message = ""
				m.setup.syncInput()
				return nil
			},
		},
		{
			ID: setupMenuLLM,
			Label: func(setup setupState) string {
				return "LLM settings"
			},
			Select: func(m *model) tea.Cmd {
				m.setup.configureSection(setupSectionLLM)
				m.setup.setStep(setupStepLLMConfigMenu)
				m.setup.choiceIdx = 0
				m.setup.message = ""
				return nil
			},
		},
		{
			ID: setupMenuSourcesEnabled,
			Label: func(setup setupState) string {
				return fmt.Sprintf("Search sources: %t", setup.appConfig.Sources.Enabled)
			},
			Select: func(m *model) tea.Cmd {
				m.setup.appConfig.Sources.Enabled = !m.setup.appConfig.Sources.Enabled
				if err := config.SaveAppConfig(runtimeConfigPath, &m.setup.appConfig); err != nil {
					m.setup.message = fmt.Sprintf("Could not save source setting: %v", err)
				} else {
					m.setup.message = fmt.Sprintf("Search sources set to %t.", m.setup.appConfig.Sources.Enabled)
				}
				return nil
			},
		},
		{
			ID: setupMenuRSSEnabled,
			Label: func(setup setupState) string {
				return fmt.Sprintf("RSS feeds: %t", setup.appConfig.Sources.RSS.Enabled)
			},
			Select: func(m *model) tea.Cmd {
				m.setup.appConfig.Sources.RSS.Enabled = !m.setup.appConfig.Sources.RSS.Enabled
				if err := config.SaveAppConfig(runtimeConfigPath, &m.setup.appConfig); err != nil {
					m.setup.message = fmt.Sprintf("Could not save RSS setting: %v", err)
				} else {
					m.setup.message = fmt.Sprintf("RSS feeds set to %t.", m.setup.appConfig.Sources.RSS.Enabled)
				}
				return nil
			},
		},
		{
			ID: setupMenuSiteSearchEnabled,
			Label: func(setup setupState) string {
				return fmt.Sprintf("Site search: %t", setup.appConfig.Sources.SiteSearch.Enabled)
			},
			Select: func(m *model) tea.Cmd {
				m.setup.appConfig.Sources.SiteSearch.Enabled = !m.setup.appConfig.Sources.SiteSearch.Enabled
				if err := config.SaveAppConfig(runtimeConfigPath, &m.setup.appConfig); err != nil {
					m.setup.message = fmt.Sprintf("Could not save site search setting: %v", err)
				} else {
					m.setup.message = fmt.Sprintf("Site search set to %t.", m.setup.appConfig.Sources.SiteSearch.Enabled)
				}
				return nil
			},
		},
		{
			ID: setupMenuBuiltinsEnabled,
			Label: func(setup setupState) string {
				return fmt.Sprintf("Built In sources: %t", setup.appConfig.Sources.BuiltinsEnabled)
			},
			Select: func(m *model) tea.Cmd {
				m.setup.appConfig.Sources.BuiltinsEnabled = !m.setup.appConfig.Sources.BuiltinsEnabled
				if err := config.SaveAppConfig(runtimeConfigPath, &m.setup.appConfig); err != nil {
					m.setup.message = fmt.Sprintf("Could not save Built In setting: %v", err)
				} else {
					m.setup.message = fmt.Sprintf("Built In sources set to %t.", m.setup.appConfig.Sources.BuiltinsEnabled)
				}
				return nil
			},
		},
		{
			ID: setupMenuPrompt,
			Label: func(setup setupState) string {
				return "Search prompt"
			},
			Select: func(m *model) tea.Cmd {
				m.setup.configureSection(setupSectionPrompt)
				if err := m.refreshSetupGeneratedState(false); err != nil {
					m.setup.generated = nil
					m.setup.prompt = config.DefaultSearchPrompt(nil)
				}
				if !m.setup.useLLM {
					m.setup.message = "Enable LLM settings first to manage the search prompt."
					return nil
				}
				m.setup.setStep(setupStepPromptReview)
				m.setup.message = ""
				return nil
			},
		},
	}
}

func setupConfigMenuIndexByID(target setupMenuID) int {
	items := setupConfigMenuSpecs()
	for i, item := range items {
		if item.ID == target {
			return i
		}
	}
	return 0
}

func setupConfigMenuIndexForSection(section setupSection) int {
	switch section {
	case setupSectionLLM:
		return setupConfigMenuIndexByID(setupMenuLLM)
	case setupSectionPrompt:
		return setupConfigMenuIndexByID(setupMenuPrompt)
	default:
		return setupConfigMenuIndexByID(setupMenuCriteria)
	}
}

func searchProfileGroupSpec() GroupSpec {
	return setupcfg.SearchProfileGroupSpec()
}

func setupFieldSpecAt(idx int) (FieldSpec, bool) {
	return setupcfg.FieldSpecAt(idx)
}
