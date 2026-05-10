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

	tableView := baseStyle.Copy().
		Width(cW + tW + sW + pW).
		Height(m.tableHeight + 1).
		Render(body.String())

	filterText := enabledFilterEmojiSummary(m.activeFilters)

	var helpView string
	if m.isFiltering {
		searchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
		helpText := searchStyle.Render("Search: ") + m.textInput.View() + "  (Enter: Apply • Esc: Clear)"
		helpView = helpStyle.Copy().
			Width(m.termWidth - 4).
			Render(helpText)
	} else {
		searchPrefix := ""
		if m.searchQuery != "" {
			searchPrefix = helpValueStyle.Render(fmt.Sprintf("[Search: %s]", m.searchQuery))
		}

		items := []string{}
		if searchPrefix != "" {
			items = append(items, searchPrefix)
		}
		items = append(items,
			formatHelpItem("↑/↓", "Nav"),
			formatHelpItem("Enter", "Details"),
			formatHelpItem("h", "Health"),
			formatHelpItem("s", "Status"),
			formatHelpItem("m", "Mark Viewed"),
			formatHelpItem("r", "Fetch"),
			formatHelpItem("c", "Config"),
			formatHelpItem("D", "Del"),
			formatHelpItem("E", "Edit"),
			formatHelpItem("/", "Search"),
			formatHelpItem("1-5", "Sort"),
			formatHelpItem("f", fmt.Sprintf("Filter %s", filterText)),
		)
		if m.backgroundTask.active || m.fetchingJobs || m.singleHealthTasksActive() {
			items = append(items, formatHelpItem("t", "Task"))
		}
		items = append(items, formatHelpItem("q", "Quit"))

		helpText := buildHelpText(m.termWidth-6, items)
		helpView = helpStyle.Copy().Width(m.termWidth - 4).Render(helpText)
	}

	baseView := tableView + "\n" + helpView + "\n"
	if activity := m.backgroundTaskActivityView(); activity != "" {
		baseView = lipgloss.PlaceOverlay(0, 0, activity, baseView)
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
