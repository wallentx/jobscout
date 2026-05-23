package tuiapp

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	// 1. Render Main Table (Background)
	// Calculate dimensions
	cW, tW, sW, pW := calculateColumnWidths(m.termWidth)
	widths := []int{cW, tW, sW, pW}

	var body strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).BorderBottom(true).BorderForeground(lipgloss.Color("240"))
	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(cW).Render("  Company"),
		lipgloss.NewStyle().Width(tW).Render(" Title"),
		lipgloss.NewStyle().Width(sW).Render("  Status"), // Shifted right 2
		lipgloss.NewStyle().Width(pW).Render(" Date"),    // Shifted right 1 (was 2)
	)
	body.WriteString(headerStyle.Render(headerRow) + "\n")

	displayJobs := m.filteredJobs
	showSetupTable := m.overlay.kind == overlaySetup
	rowsAvailable := m.tableHeight
	if showSetupTable {
		displayJobs = m.setup.previewJobs
	}
	if rowsAvailable < 0 {
		rowsAvailable = 0
	}

	// Viewport logic
	start := m.yOffset
	if showSetupTable {
		start = 0
	}
	contentRowsAvailable := rowsAvailable
	setupMessageLines := []string(nil)
	if showSetupTable {
		setupMessageLines = setupTableMessageLines(cW+tW+sW+pW, len(m.allJobs))
		if len(setupMessageLines) > contentRowsAvailable {
			setupMessageLines = setupMessageLines[:contentRowsAvailable]
		}
		contentRowsAvailable -= len(setupMessageLines)
		if contentRowsAvailable < 0 {
			contentRowsAvailable = 0
		}
	}

	end := start + contentRowsAvailable
	if end > len(displayJobs) {
		end = len(displayJobs)
	}

	for i := start; i < end; i++ {
		row := m.renderRow(displayJobs[i], !showSetupTable && i == m.cursor, widths)
		body.WriteString(row + "\n")
	}

	renderedRows := end - start
	if showSetupTable && len(displayJobs) == 0 && rowsAvailable > 0 {
		body.WriteString(renderSetupEmptyTable(cW+tW+sW+pW, rowsAvailable, len(m.allJobs), runtimeBuildVersion))
		body.WriteString("\n")
		renderedRows = rowsAvailable
	} else if !showSetupTable && len(displayJobs) == 0 && rowsAvailable > 0 {
		body.WriteString(renderEmptyTableLogo(cW+tW+sW+pW, rowsAvailable, runtimeBuildVersion))
		body.WriteString("\n")
		renderedRows = rowsAvailable
	}

	for i := renderedRows; i < contentRowsAvailable; i++ {
		blankRow := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(cW).Render(""),
			lipgloss.NewStyle().Width(tW).Render(""),
			lipgloss.NewStyle().Width(sW).Render(""),
			lipgloss.NewStyle().Width(pW).Render(""),
		)
		body.WriteString(blankRow + "\n")
	}
	if showSetupTable && len(displayJobs) > 0 {
		for _, line := range setupMessageLines {
			body.WriteString(line + "\n")
		}
	}

	tableView := baseStyle.
		Width(cW + tW + sW + pW).
		Height(m.tableHeight + 1).
		Render(body.String())

	var helpView string
	if m.isCommanding {
		commandStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
		commandHintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		commandResultTitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
		commandResultBodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		if m.commandResultError {
			commandResultTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		}
		lines := []string{}
		if m.commandResultMessage != "" {
			resultBody := strings.ReplaceAll(m.commandResultMessage, "\n", " • ")
			lines = append(lines, commandResultTitleStyle.Render(m.commandResultTitle)+helpValueStyle.Render(" ")+commandResultBodyStyle.Render(resultBody))
		} else {
			lines = append(lines, " ")
		}
		commandLine := commandStyle.Render(":") + " " + m.commandInputInlineView(m.operatorCommandGhostHint(), commandHintStyle)
		lines = append(lines, commandLine)
		lines = append(lines, commandHintStyle.Render("Tab Select/Cycle • Space Confirm • Enter Run • Esc Cancel"))
		if pool := m.operatorCommandPool(); len(pool) > 0 {
			commandPoolSelectedStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("60")).
				Bold(true)
			parts := make([]string, 0, len(pool))
			for i, completion := range pool {
				label := completion.Label
				if m.commandCompletionActive && i == m.commandCompletionIdx {
					label = commandPoolSelectedStyle.Render(label)
				} else {
					label = commandHintStyle.Render(label)
				}
				parts = append(parts, label)
			}
			lines = append(lines, strings.Join(parts, "  "))
		}
		helpText := strings.Join(lines, "\n")
		helpView = helpStyle.
			Width(m.termWidth - 4).
			Render(helpText)
	} else if m.isFiltering {
		searchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
		helpText := searchStyle.Render("Search: ") + m.textInput.View() + "  (Enter: Apply • Esc: Clear)"
		helpView = helpStyle.
			Width(m.termWidth - 4).
			Render(helpText)
	} else {
		searchPrefix := ""
		if m.searchQuery != "" {
			searchPrefix = helpValueStyle.Render(fmt.Sprintf("[Search: %s]", m.searchQuery))
		}
		filterLabel := "Filter"
		if filterText := enabledFilterEmojiSummary(m.activeFilters); filterText != "" {
			filterLabel = fmt.Sprintf("Filter %s", filterText)
		}

		items := []string{}
		if searchPrefix != "" {
			items = append(items, searchPrefix)
		}
		items = append(items,
			formatHelpItem("↑/↓", "Nav"),
			formatHelpItem("Enter", "Details"),
			formatHelpItem("h", "Health"),
			formatHelpItem("H", "All Health"),
			formatHelpItem("l", "Legend"),
			formatHelpItem("?", "Keys"),
			formatHelpItem("s", "Status"),
			formatHelpItem("m", "Viewed"),
			formatHelpItem("r", "Fetch"),
			formatHelpItem("U", "Update"),
			formatHelpItem("V", "Active"),
			formatHelpItem("c", "Config"),
			formatHelpItem("D", "Deleted"),
			formatHelpItem("E", "Edit"),
			formatHelpItem("/", "Search"),
			formatHelpItem(":", "Cmd"),
			formatHelpItem("1-5", "Sort"),
			formatHelpItem("f", filterLabel),
		)
		if m.backgroundTask.active || m.fetchingJobs || m.singleHealthTasksActive() {
			items = append(items, formatHelpItem("t", "Task"))
		}
		items = append(items, formatHelpItem("q", "Quit"))

		helpText := buildHelpText(m.termWidth-6, items)
		helpView = helpStyle.Width(m.termWidth - 4).Render(helpText)
	}

	baseView := tableView + "\n" + helpView + "\n"
	if activity := m.backgroundTaskActivityView(); activity != "" {
		baseView = placeOverlay(0, 0, activity, baseView)
	}

	// 2. Render Overlays (Health or Details)
	if m.setupRequired && m.overlay.kind == overlayNone {
		m.overlay.kind = overlaySetup
	}

	mainSpec, showMainOverlay := m.buildMainOverlaySpec()
	backgroundTaskSpec, showBackgroundTaskOverlay := m.buildBackgroundTaskOverlaySpecIfActive()
	singleHealthTaskSpec, showSingleHealthTaskOverlay := m.buildSingleHealthTaskOverlaySpecIfActive()
	activeFetchSpec, showActiveFetchOverlay := m.buildActiveFetchOverlaySpecIfActive()
	if showMainOverlay || showBackgroundTaskOverlay || showSingleHealthTaskOverlay || showActiveFetchOverlay {
		baseView = dimPopupBackground(baseView)
	}

	viewWithOverlays := baseView
	if showMainOverlay {
		spec := mainSpec
		viewWithOverlays = renderPopupSpecWithBackgroundDimming(viewWithOverlays, m.termWidth, m.termHeight, spec, true)
	}

	if showBackgroundTaskOverlay {
		spec := backgroundTaskSpec
		viewWithOverlays = renderPopupSpecWithBackgroundDimming(viewWithOverlays, m.termWidth, m.termHeight, spec, true)
	}

	if showSingleHealthTaskOverlay {
		spec := singleHealthTaskSpec
		viewWithOverlays = renderPopupSpecWithBackgroundDimming(viewWithOverlays, m.termWidth, m.termHeight, spec, true)
	}

	if showActiveFetchOverlay {
		spec := activeFetchSpec
		viewWithOverlays = renderPopupSpecWithBackgroundDimming(viewWithOverlays, m.termWidth, m.termHeight, spec, true)
	}

	return viewWithOverlays
}

func (m model) commandInputInlineView(ghost string, ghostStyle lipgloss.Style) string {
	input := m.commandInput
	if valueWidth := lipgloss.Width(input.Value()); valueWidth > 0 {
		input.Width = valueWidth
	}
	view := input.View()
	if ghost != "" && input.Position() >= len([]rune(input.Value())) {
		view += ghostStyle.Render(ghost)
	}
	return view
}
