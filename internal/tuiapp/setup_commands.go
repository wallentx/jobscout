package tuiapp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/fetcher"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
	setupcfg "github.com/wallentx/jobscout/internal/setup"

	tea "github.com/charmbracelet/bubbletea"
)

func setupFooter(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return "q: Quit"
	}
	return base + " • q: Quit"
}

func (m *model) setupCriteriaFromState() (*CriteriaConfig, error) {
	return setupcfg.CriteriaFromSearchProfileValues(m.setup.fieldValues)
}

func (m *model) saveSetupArtifacts() error {
	if m.setup.generated == nil {
		criteriaCfg, err := m.setupCriteriaFromState()
		if err != nil {
			return err
		}
		m.setup.generated = criteriaCfg
	}

	appCfg := m.setup.appConfig
	appCfg.Criteria.ResolvedSourceIDs = fetcher.ResolvedSourceIDs(m.setup.generated.RoleFamilies)
	appCfg.Criteria = *m.setup.generated
	appCfg.Criteria.ResolvedSourceIDs = fetcher.ResolvedSourceIDs(appCfg.Criteria.RoleFamilies)
	if err := refreshLinkedInCriteriaHints(context.Background(), &appCfg.Criteria); err != nil {
		m.setup.message = fmt.Sprintf("Saved setup, but could not resolve LinkedIn location filter: %v", err)
	}
	if !m.setup.useLLM {
		appCfg.LLM.JobFiltering = false
		appCfg.LLM.JobSearch = false
		appCfg.LLM.CompanyHealth = false
	}

	if m.setup.prompt == "" {
		m.setup.prompt = config.DefaultSearchPrompt(m.setup.generated)
	}

	if err := config.SaveAppConfig(runtimeConfigPath, &appCfg); err != nil {
		return err
	}
	if err := config.SaveSearchPrompt(runtimeSearchPromptPath, m.setup.prompt); err != nil {
		return err
	}

	caps := config.EvaluateRuntimeCapabilities()
	m.setupRequired = len(caps.SetupIssues) > 0
	m.setupIssues = caps.SetupIssues

	return nil
}

func refreshLinkedInCriteriaHints(ctx context.Context, criteria *CriteriaConfig) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	return fetcher.RefreshLinkedInCriteriaHints(ctx, criteria)
}

func (m *model) refreshSetupGeneratedState(overwritePrompt bool) error {
	criteriaCfg, err := m.setupCriteriaFromState()
	if err != nil {
		return err
	}

	m.setup.generated = criteriaCfg
	if overwritePrompt || strings.TrimSpace(m.setup.prompt) == "" {
		m.setup.prompt = config.DefaultSearchPrompt(criteriaCfg)
	}
	m.setup.previewErr = ""
	return nil
}

func (m *model) saveSetupArtifactsOrMessage() bool {
	if err := m.saveSetupArtifacts(); err != nil {
		m.setup.message = fmt.Sprintf("Could not save setup: %v", err)
		return false
	}
	return true
}

func (m *model) completeSetupSave(successMessage string) (tea.Model, tea.Cmd) {
	switch m.setup.mode {
	case setupModeRepair:
		caps := config.EvaluateRuntimeCapabilities()
		m.setupRequired = len(caps.SetupIssues) > 0
		m.setupIssues = caps.SetupIssues
		if caps.LLMPreferred && caps.LLMConfigured && caps.LLMAuthAvailableNow {
			m.sessionLLMDisabled = false
		}
		if m.setupRequired {
			nextSection := setupSectionForCapabilities(caps)
			m.setup.configureSection(nextSection)
			m.setup.setStep(setupStepConfigMenu)
			m.setup.choiceIdx = setupConfigMenuIndexForSection(nextSection)
			if nextSection != setupSectionNone {
				m.setup.message = fmt.Sprintf("Saved. Remaining repair area: %s.", setupSectionLabel(nextSection))
			} else {
				m.setup.message = "Saved. Review the remaining configuration sections."
			}
			return m, nil
		}

		m.clearOverlay()
		m.setup.mode = setupModeEdit
		m.setup.firstRun = false
		m.setup.message = ""
		m.showNotice("Configuration Repaired", successMessage, false)
		return m, nil
	case setupModeEdit:
		m.clearOverlay()
		return m, nil
	default:
		return m, nil
	}
}

func (m *model) beginSetupPreview() tea.Cmd {
	m.setup.setStep(setupStepPreviewConfirm)
	m.setup.previewBusy = true
	m.setup.loadingMinimized = false
	m.setup.previewJobs = nil
	m.setup.message = ""
	return tea.Batch(previewFetchCmd(m.setup.appConfig, m.setup.generated), m.restartLoadingIndicator())
}

func editPromptContent(content string) tea.Cmd {
	return func() tea.Msg {
		f, err := os.CreateTemp("", "search_prompt_*.md")
		if err != nil {
			return setupPromptEditedMsg{err: fmt.Errorf("failed to create temp file: %v", err)}
		}

		tmpPath := f.Name()
		if _, err := f.WriteString(content); err != nil {
			_ = f.Close()
			return setupPromptEditedMsg{err: fmt.Errorf("failed to write temp file: %v", err)}
		}
		if err := f.Close(); err != nil {
			return setupPromptEditedMsg{err: fmt.Errorf("failed to close temp file: %v", err)}
		}
		defer func() {
			_ = os.Remove(tmpPath)
		}()

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}

		cmd := exec.Command(editor, tmpPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return setupPromptEditedMsg{err: fmt.Errorf("editor failed: %v", err)}
		}

		edited, err := os.ReadFile(tmpPath)
		if err != nil {
			return setupPromptEditedMsg{err: fmt.Errorf("failed to read edited prompt: %v", err)}
		}

		return setupPromptEditedMsg{content: string(edited)}
	}
}

func fetchSetupModelsCmd(appCfg AppConfig) tea.Cmd {
	provider := strings.ToLower(strings.TrimSpace(appCfg.LLM.Provider))
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		models, err := llmpkg.FetchAvailableLLMModels(ctx, appCfg)
		return setupModelsFetchedMsg{
			provider: provider,
			models:   models,
			err:      err,
		}
	}
}

func generateResumeCriteriaCmd(appCfg AppConfig, resumePath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		criteria, err := llmpkg.GenerateCriteriaFromResume(ctx, appCfg, resumePath)
		return setupResumeCriteriaMsg{
			path:     resumePath,
			criteria: criteria,
			err:      err,
		}
	}
}
