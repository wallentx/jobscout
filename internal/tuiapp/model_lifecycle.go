package tuiapp

import (
	"context"
	"fmt"
	"os"

	"github.com/wallentx/jobscout/internal/config"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func initialModel() model {
	taskCtx, cancelTasks := context.WithCancel(context.Background())

	jobs, err := loadRuntimeJobs()
	if err != nil {
		if os.IsNotExist(err) {
			jobs = []Job{}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Could not load jobs from runtime store, starting empty (%v)\n", err)
			jobs = []Job{}
		}
	}

	cache, err := runtimeHealthStore.LoadHealthCache()
	if err != nil {
		fmt.Printf("Warning: Could not load health cache: %v\n", err)
		cache = make(HealthCache)
	}

	// Detect actual terminal dimensions
	width, height, err := term.GetSize(int(uint(os.Stdout.Fd())))
	if err != nil {
		// Fallback to defaults if detection fails
		width = 100
		height = 30
	}

	// Initial setup
	ti := textinput.New()
	ti.Placeholder = "Search company or title..."
	ti.CharLimit = 50
	ti.Width = 30

	caps := config.EvaluateRuntimeCapabilities()

	m := model{
		taskCtx:       taskCtx,
		cancelTasks:   cancelTasks,
		allJobs:       jobs,
		healthCache:   cache,
		setupRequired: len(caps.SetupIssues) > 0,
		setupIssues:   caps.SetupIssues,
		termWidth:     width,
		termHeight:    height,
		tableHeight:   calculateTableHeight(height),
		sortBy:        0,    // Default to Health sort
		sortDesc:      true, // Default Health to descending (highest first)
		textInput:     ti,
	}
	if appCfg, err := config.LoadAppConfig(runtimeConfigPath); err == nil {
		m.activeFilters = filterValuesFromStatuses(appCfg.UI.DefaultFilterStatuses)
	} else {
		m.activeFilters = filterValuesFromStatuses(nil)
	}
	if m.setupRequired {
		setupMode := setupModeForCapabilities(caps)
		m.openSetupOverlay(setupMode, setupSectionForCapabilities(caps))
	} else if caps.LLMPreferred && caps.LLMConfigured && !caps.LLMAuthAvailableNow && caps.CanRunNonLLM {
		m.openSetupOverlay(setupModeRecovery, setupSectionLLM)
	}
	m.applyFilterAndSort()
	return m
}

func (m model) getMaxHealthScroll() int {
	if m.overlay.kind != overlayHealth || m.overlay.health.report == nil {
		return 0
	}
	width := clampPopupWidth(m.termWidth, 40, 0)
	targetLineWidth := width - 6
	fullReport := renderHealthReport(m.overlay.health.report, targetLineWidth)

	maxVisibleLines := popupMaxViewportLinesWithChrome(m.termHeight, 1, 8)
	lines := structuredPopupLines(fullReport, targetLineWidth)
	return renderPopupViewport(lines, targetLineWidth, maxVisibleLines, m.overlay.health.scrollOffset, nil).maxOffset
}

func (m model) getMaxNoticeScroll() int {
	if m.overlay.kind != overlayNotice || !m.overlay.notice.visible {
		return 0
	}
	width := clampPopupWidth(m.termWidth, 40, 0)
	maxVisibleLines := popupMaxViewportLinesWithChrome(m.termHeight, 4, 11)
	lines := structuredPopupLines(m.overlay.notice.message, width-6)
	return renderPopupViewport(lines, width-6, maxVisibleLines, m.overlay.notice.scrollOffset, noticePopupLineStyle).maxOffset
}

func (m model) getMaxDetailScroll() int {
	if m.overlay.kind != overlayDetail || len(m.filteredJobs) == 0 {
		return 0
	}
	dialogWidth := int(float64(m.termWidth)*0.8) + 1
	if dialogWidth > 100 {
		dialogWidth = 100
	}
	if dialogWidth < 40 {
		dialogWidth = 40
	}
	innerW := dialogWidth - 6
	maxVisibleLines := popupMaxViewportLinesWithChrome(m.termHeight, 6, 8)
	lines := structuredPopupLines(m.currentDetailText(innerW), innerW)
	return renderPopupViewport(lines, innerW, maxVisibleLines, m.overlay.detail.scrollOffset, nil).maxOffset
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.overlay.kind == overlaySetup && m.setup.step == setupStepCriteriaField {
		cmds = append(cmds, textinput.Blink)
		cmds = append(cmds, textarea.Blink)
	}
	if m.isLoadingActive() {
		cmds = append(cmds, nextLoadingTick(m.loading.generation))
	}
	if cmd := maybeCheckForUpdateCmd(m.taskCtx, runtimeBuildVersion); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
