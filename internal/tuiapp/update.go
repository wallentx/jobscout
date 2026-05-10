package tuiapp

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadingTickMsg:
		return m.handleLoadingTickMsg(msg)
	case setupPromptEditedMsg:
		return m.handleSetupPromptEditedMsg(msg)
	case setupResumeCriteriaMsg:
		return m.handleSetupResumeCriteriaMsg(msg)
	case setupPreviewMsg:
		return m.handleSetupPreviewMsg(msg)
	case setupModelsFetchedMsg:
		return m.handleSetupModelsFetchedMsg(msg)
	case healthLoadedMsg:
		return m.handleHealthLoadedMsg(msg)
	case bulkHealthStepMsg:
		return m.handleBulkHealthStepMsg(msg)
	case jobEditedMsg:
		return m.handleJobEditedMsg(msg)
	case fetchJobsMsg:
		return m.handleFetchJobsMsg(msg)
	case fetchJobsProgressMsg:
		return m.handleFetchJobsProgressMsg(msg)
	case acceptedFetchSavedMsg:
		return m.handleAcceptedFetchSavedMsg(msg)
	case acceptedFetchEnrichedMsg:
		return m.handleAcceptedFetchEnrichedMsg(msg)
	case backgroundTaskProgressMsg:
		return m.handleBackgroundTaskProgressMsg(msg)
	case backgroundJobEnrichedMsg:
		return m.handleBackgroundJobEnrichedMsg(msg)
	case backgroundTaskAnimMsg:
		return m.handleBackgroundTaskAnimMsg(msg)
	case updateCheckMsg:
		return m.handleUpdateCheckMsg(msg)
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.tableHeight = calculateTableHeight(m.termHeight)
		return m, nil
	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}
