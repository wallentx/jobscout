package tuiapp

import (
	"fmt"
	"strings"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/fetcher"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
	appruntime "github.com/wallentx/jobscout/internal/runtime"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
)

func setupCriteriaTextareaHeight(termHeight int) int {
	if termHeight <= 0 {
		return 4
	}
	height := popupMaxViewportLinesWithChrome(termHeight, 4, 15)
	if height < 4 {
		return 4
	}
	if height > 8 {
		return 8
	}
	return height
}

func renderSetupTextareaWithScrollbar(textarea textarea.Model, width int, height int) string {
	view := strings.TrimRight(textarea.View(), "\n")
	lines := strings.Split(view, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	totalLines := setupTextareaVisualLineCount(textarea.Value(), textarea.Width())
	if totalLines <= height {
		return strings.Join(lines, "\n")
	}

	offset := popupMenuScrollOffset(textarea.LineInfo().RowOffset, totalLines, height)
	end := offset + height
	if end > totalLines {
		end = totalLines
	}
	barHeight := len(lines)
	percentStart := float64(offset) / float64(totalLines)
	percentEnd := float64(end) / float64(totalLines)
	startRow := int(percentStart * float64(barHeight))
	endRow := int(percentEnd * float64(barHeight))
	if endRow <= startRow {
		endRow = startRow + 1
	}
	if endRow > barHeight {
		endRow = barHeight
	}

	sbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	for i, line := range lines {
		scrollChar := "│"
		if i >= startRow && i < endRow {
			scrollChar = "█"
		}
		lines[i] = padPopupViewportLine(line, width) + " " + sbStyle.Render(scrollChar)
	}
	return strings.Join(lines, "\n")
}

func setupTextareaVisualLineCount(value string, width int) int {
	if width < 1 {
		width = 1
	}
	if value == "" {
		return 1
	}
	total := 0
	for _, line := range strings.Split(value, "\n") {
		lineWidth := lipgloss.Width(line)
		if lineWidth == 0 {
			total++
			continue
		}
		total += (lineWidth + width - 1) / width
	}
	return total
}

func tempSetupTableMessage(savedJobs int) string {
	message := "Setup jobs view. Preview jobs are not written to database until you confirm setup.\n"
	if savedJobs > 0 {
		message += fmt.Sprintf("%d saved jobs in the database and hidden during setup.", savedJobs)
	}
	return message
}

func (m model) buildSetupOverlaySpec() popupSpec {
	dialogWidth := clampPopupWidth(m.termWidth, 42, 76)

	var header strings.Builder
	var content strings.Builder
	var footer string
	header.WriteString(m.setupHeaderText())

	switch m.setup.step {
	case setupStepRecoveryChoice:
		options := []string{
			"Continue without LLM for this session",
			"Fix LLM settings now",
			"Quit",
		}
		items := make([]popupMenuItem, 0, len(options))
		for i, option := range options {
			items = append(items, popupMenuItem{
				Label:    option,
				Selected: m.setup.choiceIdx == i,
			})
		}
		content.WriteString("Your saved LLM settings are unchanged. Choose how to continue.")
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			menu:   items,
			footer: popupHintStyle.Render(setupFooter("Enter: Select")),
		}
	case setupStepLLMChoice:
		options := []string{"Yes, use LLM features", "No, use non-LLM sources only"}
		items := make([]popupMenuItem, 0, len(options))
		for i, option := range options {
			items = append(items, popupMenuItem{
				Label:    option,
				Selected: m.setup.choiceIdx == i,
			})
		}
		if m.setup.firstRun {
			content.WriteString("Privacy note: jobscout does not collect your information, run analytics, or send your data to the app author.\n\n")
			content.WriteString("If you enable LLM features, only the content needed for that LLM feature is sent to the provider you configure.")
			content.WriteString("\n\n")
		}
		content.WriteString("Do you want to enable LLM features?")
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			menu:   items,
			footer: popupHintStyle.Render(setupFooter(func() string {
				text := "Enter: Select"
				if !m.setup.firstRun {
					text += " • Esc: Close"
				}
				return text
			}())),
		}
	case setupStepConfigMenu:
		switch m.setup.mode {
		case setupModeRepair:
			content.WriteString("Configuration issues were detected. Select an area to repair.")
			if m.setup.section != setupSectionNone {
				content.WriteString("\n\n")
				content.WriteString(helpStyle.Render(fmt.Sprintf("Recommended: %s", setupSectionLabel(m.setup.section))))
			}
		default:
			content.WriteString("What do you want to configure?")
		}
		options := m.currentSetupConfigOptions()
		items := make([]popupMenuItem, 0, len(options))
		for i, option := range options {
			items = append(items, popupMenuItem{
				Label:    option,
				Selected: m.setup.choiceIdx == i,
			})
		}
		footer = "Enter: Select • Esc: Close"
		if m.setup.message != "" {
			footer = m.setup.message + "\n\n" + footer
		}
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			menu:   items,
			footer: popupHintStyle.Render(setupFooter(footer)),
		}
	case setupStepLLMConfigMenu:
		content.WriteString("LLM settings\n\n")
		labelWidth := lipgloss.Width("LLM Features:")
		dropdownWidth := dialogWidth - 10 - labelWidth
		if dropdownWidth < 24 {
			dropdownWidth = 24
		}
		if dropdownWidth > 46 {
			dropdownWidth = 46
		}
		maxDropdownRows := popupMaxViewportLinesWithChrome(m.termHeight, 4, 12) - len(m.currentSetupLLMConfigOptions())
		if maxDropdownRows < 3 {
			maxDropdownRows = 3
		}
		if maxDropdownRows > 8 {
			maxDropdownRows = 8
		}
		rowStartLine := strings.Count(content.String(), "\n")
		var dropdownOverlay popupDropdownOverlay
		dropdownOverlayX := 0
		dropdownOverlayY := 0
		llmRows := []struct {
			label string
			value string
		}{
			{label: "LLM Features:", value: optionsEnabledDisabled(m.setup.useLLM)},
			{label: "Provider:", value: providerLabel(m.setup.appConfig.LLM.Provider)},
			{label: "Provider Config:", value: "Credentials and models"},
		}
		for i, row := range llmRows {
			if i > 0 {
				content.WriteString("\n")
			}
			if i == 1 {
				providerOptions := []string{providerOptionLabel(m.setup.appConfig.LLM.Provider)}
				selectedIdx := 0
				open := m.setup.providerDropdownOpen && m.setup.choiceIdx == i
				if open {
					providerOptions = providerOptionLabels(m.currentSetupProviderOptions())
					selectedIdx = m.setup.providerDropdownIdx
					dropdownSpec := popupDropdownSpec{
						Label:       row.label,
						Value:       row.value,
						Items:       providerOptions,
						LabelWidth:  labelWidth,
						Width:       dropdownWidth,
						MaxOpenRows: maxDropdownRows,
						SelectedIdx: selectedIdx,
						Open:        true,
						Focused:     true,
					}
					dropdownOverlay = renderPopupDropdownOverlay(dropdownSpec)
					dropdownOverlayX = labelWidth + 1
					dropdownOverlayY = rowStartLine + i + 1
				}
				content.WriteString(renderPopupDropdown(popupDropdownSpec{
					Label:       row.label,
					Value:       row.value,
					Items:       providerOptions,
					LabelWidth:  labelWidth,
					Width:       dropdownWidth,
					MaxOpenRows: maxDropdownRows,
					SelectedIdx: selectedIdx,
					Open:        open,
					Focused:     m.setup.choiceIdx == i,
				}))
				continue
			}
			labelStyle := popupSelectInactiveStyle
			if m.setup.choiceIdx == i {
				labelStyle = popupSelectActiveStyle
			}
			content.WriteString(labelStyle.Render(padVisibleRight(row.label, labelWidth)))
			content.WriteString(" ")
			content.WriteString(popupBodyStyle.Render(truncate(row.value, dropdownWidth)))
		}
		reservedLines := maxDropdownRows + 3
		for i := 0; i < reservedLines; i++ {
			content.WriteString("\n")
		}
		statusLine := " "
		if strings.TrimSpace(m.setup.message) != "" {
			statusLine = m.setup.message
		}
		content.WriteString(popupHintStyle.Render(statusLine))
		if dropdownOverlay.content != "" {
			contentString := content.String()
			contentString = lipgloss.PlaceOverlay(
				dropdownOverlayX,
				dropdownOverlayY,
				dropdownOverlay.content,
				contentString,
			)
			content.Reset()
			content.WriteString(contentString)
		}
		footer = "Enter: Select • Esc: Back"
		if m.setup.providerDropdownOpen {
			footer = "Enter: Choose • Esc: Close dropdown"
		}
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupScrollableTextBody(popupBodyStyle.Render(content.String())),
			menu:   nil,
			footer: popupHintStyle.Render(setupFooter(footer)),
		}
	case setupStepProviderConfigMenu:
		fmt.Fprintf(&content, "Provider config: %s\n\n", providerLabel(m.setup.appConfig.LLM.Provider))
		content.WriteString(helpStyle.Render("Credentials and model choices are stored for this provider only."))
		content.WriteString("\n\n")
		labelWidth := lipgloss.Width("Default Model:")
		dropdownWidth := dialogWidth - 10 - labelWidth
		if dropdownWidth < 24 {
			dropdownWidth = 24
		}
		if dropdownWidth > 46 {
			dropdownWidth = 46
		}
		maxDropdownRows := popupMaxViewportLinesWithChrome(m.termHeight, 4, 13) - len(m.currentSetupProviderConfigOptions())
		if maxDropdownRows < 3 {
			maxDropdownRows = 3
		}
		if maxDropdownRows > 8 {
			maxDropdownRows = 8
		}
		rowStartLine := strings.Count(content.String(), "\n")
		var dropdownOverlay popupDropdownOverlay
		dropdownOverlayX := 0
		dropdownOverlayY := 0
		credentialValue := "Not required"
		if m.setup.currentSetupProviderNeedsAuth() {
			credentialValue = setupAuthModeLabel(m.setup.currentSetupAuthMode(), m.setup.appConfig.LLM.Provider)
		}
		providerRows := []struct {
			label string
			value string
		}{
			{label: "Credentials:", value: credentialValue},
			{label: "Default Model:", value: m.setup.appConfig.LLM.Model},
			{label: "Task Models:", value: "Configure overrides"},
		}
		for i, row := range providerRows {
			if i > 0 {
				content.WriteString("\n")
			}
			if i == 1 {
				modelOptions := []string{m.setup.appConfig.LLM.Model}
				selectedIdx := 0
				open := m.setup.providerModelDropdownOpen && m.setup.choiceIdx == i
				if open {
					modelOptions = m.currentSetupModelOptions()
					selectedIdx = m.setup.providerModelDropdownIdx
					dropdownSpec := popupDropdownSpec{
						Label:       row.label,
						Value:       row.value,
						Items:       modelOptions,
						LabelWidth:  labelWidth,
						Width:       dropdownWidth,
						MaxOpenRows: maxDropdownRows,
						SelectedIdx: selectedIdx,
						Open:        true,
						Focused:     true,
					}
					dropdownOverlay = renderPopupDropdownOverlay(dropdownSpec)
					dropdownOverlayX = labelWidth + 1
					dropdownOverlayY = rowStartLine + i + 1
				}
				content.WriteString(renderPopupDropdown(popupDropdownSpec{
					Label:       row.label,
					Value:       row.value,
					Items:       modelOptions,
					LabelWidth:  labelWidth,
					Width:       dropdownWidth,
					MaxOpenRows: maxDropdownRows,
					SelectedIdx: selectedIdx,
					Open:        open,
					Focused:     m.setup.choiceIdx == i,
				}))
				continue
			}
			labelStyle := popupSelectInactiveStyle
			if m.setup.choiceIdx == i {
				labelStyle = popupSelectActiveStyle
			}
			content.WriteString(labelStyle.Render(padVisibleRight(row.label, labelWidth)))
			content.WriteString(" ")
			content.WriteString(popupBodyStyle.Render(truncate(row.value, dropdownWidth)))
		}
		reservedLines := maxDropdownRows + 3
		for i := 0; i < reservedLines; i++ {
			content.WriteString("\n")
		}
		statusLine := " "
		if strings.TrimSpace(m.setup.message) != "" {
			statusLine = m.setup.message
		}
		content.WriteString(popupHintStyle.Render(statusLine))
		if dropdownOverlay.content != "" {
			contentString := content.String()
			contentString = lipgloss.PlaceOverlay(
				dropdownOverlayX,
				dropdownOverlayY,
				dropdownOverlay.content,
				contentString,
			)
			content.Reset()
			content.WriteString(contentString)
		}
		footer = "Enter: Select • Esc: Back"
		if m.setup.providerModelDropdownOpen {
			footer = "Enter: Choose • Esc: Close dropdown"
		}
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupScrollableTextBody(popupBodyStyle.Render(content.String())),
			menu:   nil,
			footer: popupHintStyle.Render(setupFooter(footer)),
		}
	case setupStepProviderChoice:
		content.WriteString("Choose an LLM provider:")
		options := m.currentSetupProviderOptions()
		items := make([]popupMenuItem, 0, len(options))
		for i, option := range options {
			items = append(items, popupMenuItem{
				Label:    providerOptionLabel(option),
				Selected: m.setup.choiceIdx == i,
			})
		}
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			menu:   items,
			footer: popupHintStyle.Render(setupFooter("Enter: Select • Esc: Back")),
		}
	case setupStepModelChoice:
		fmt.Fprintf(&content, "Choose a model for %s:", providerLabel(m.setup.appConfig.LLM.Provider))
		options := m.currentSetupModelOptions()
		items := make([]popupMenuItem, 0, len(options))
		for i, option := range options {
			items = append(items, popupMenuItem{
				Label:    option,
				Selected: m.setup.choiceIdx == i,
			})
		}
		content.WriteString("\n\n")
		if m.setup.modelsLoading {
			content.WriteString(helpStyle.Render("Fetching provider model list..."))
			content.WriteString("\n")
		}
		if m.setup.currentSetupProviderNeedsAuth() {
			content.WriteString(helpStyle.Render(fmt.Sprintf("Required env var: %s", config.EnvVarForProvider(m.setup.appConfig.LLM.Provider))))
		} else {
			_, providerCfg := m.setup.currentSetupProviderConfig()
			endpoint := strings.TrimSpace(providerCfg.Endpoint)
			if endpoint == "" {
				content.WriteString(helpStyle.Render("This provider does not require API auth."))
			} else {
				content.WriteString(helpStyle.Render(fmt.Sprintf("No API auth required. Endpoint: %s", endpoint)))
			}
		}
		maxModelLines := popupMaxViewportLines(m.termHeight, 4) - 4
		if maxModelLines < 4 {
			maxModelLines = 4
		}
		if maxModelLines > 14 {
			maxModelLines = 14
		}
		viewport := renderPopupMenuViewport(items, dialogWidth-6, maxModelLines, m.setup.choiceIdx)
		content.WriteString("\n\n")
		content.WriteString(viewport.content)
		footerText := "Enter: Select • Esc: Back"
		if viewport.maxOffset > 0 {
			footerText = "↑/↓: Scroll • " + footerText
		}
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			menu:   nil,
			footer: popupHintStyle.Render(setupFooter(footerText)),
		}
	case setupStepModelValueField:
		fmt.Fprintf(&content, "Enter a model ID for %s:", providerLabel(m.setup.appConfig.LLM.Provider))
		content.WriteString("\n\n")
		content.WriteString(m.setup.input.View())
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			footer: popupHintStyle.Render(setupFooter("Enter: Save • Esc: Back")),
		}
	case setupStepTaskModelMenu:
		content.WriteString("Task model overrides\n\n")
		content.WriteString(helpStyle.Render("Set a different model for one LLM task, or leave it on the provider default."))
		content.WriteString("\n\n")
		labelWidth := setupTaskModelLabelWidth()
		dropdownWidth := dialogWidth - 10 - labelWidth
		if dropdownWidth < 24 {
			dropdownWidth = 24
		}
		if dropdownWidth > 46 {
			dropdownWidth = 46
		}
		maxDropdownRows := popupMaxViewportLinesWithChrome(m.termHeight, 4, 16) - len(setupTaskModelSpecs())
		if maxDropdownRows < 3 {
			maxDropdownRows = 3
		}
		if maxDropdownRows > 8 {
			maxDropdownRows = 8
		}
		rowStartLine := strings.Count(content.String(), "\n")
		var dropdownOverlay popupDropdownOverlay
		dropdownOverlayX := 0
		dropdownOverlayY := 0
		for i, spec := range setupTaskModelSpecs() {
			if i > 0 {
				content.WriteString("\n")
			}
			options := []string{m.setupTaskModelDisplayValue(spec.Key)}
			selectedIdx := 0
			open := m.setup.taskModelDropdownOpen && m.setup.choiceIdx == i
			if open {
				options = m.currentSetupTaskModelOptions()
				selectedIdx = m.setup.taskModelDropdownIdx
				dropdownSpec := popupDropdownSpec{
					Label:       spec.Label + ":",
					Value:       m.setupTaskModelDisplayValue(spec.Key),
					Items:       options,
					LabelWidth:  labelWidth,
					Width:       dropdownWidth,
					MaxOpenRows: maxDropdownRows,
					SelectedIdx: selectedIdx,
					Open:        true,
					Focused:     true,
				}
				dropdownOverlay = renderPopupDropdownOverlay(dropdownSpec)
				dropdownOverlayX = labelWidth + 1
				dropdownOverlayY = rowStartLine + i + 1
			}
			content.WriteString(renderPopupDropdown(popupDropdownSpec{
				Label:       spec.Label + ":",
				Value:       m.setupTaskModelDisplayValue(spec.Key),
				Items:       options,
				LabelWidth:  labelWidth,
				Width:       dropdownWidth,
				MaxOpenRows: maxDropdownRows,
				SelectedIdx: selectedIdx,
				Open:        open,
				Focused:     m.setup.choiceIdx == i,
			}))
		}
		reservedLines := maxDropdownRows + 3
		for i := 0; i < reservedLines; i++ {
			content.WriteString("\n")
		}
		statusLine := " "
		if strings.TrimSpace(m.setup.message) != "" {
			statusLine = m.setup.message
		}
		content.WriteString(popupHintStyle.Render(statusLine))
		if dropdownOverlay.content != "" {
			contentString := content.String()
			contentString = lipgloss.PlaceOverlay(
				dropdownOverlayX,
				dropdownOverlayY,
				dropdownOverlay.content,
				contentString,
			)
			content.Reset()
			content.WriteString(contentString)
		}
		footer = "Enter: Select • Esc: Back"
		if m.setup.taskModelDropdownOpen {
			footer = "Enter: Choose • Esc: Close dropdown"
		}
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupScrollableTextBody(popupBodyStyle.Render(content.String())),
			footer: popupHintStyle.Render(setupFooter(footer)),
		}
	case setupStepTaskModelChoice:
		fmt.Fprintf(&content, "Choose a model for %s:", setupTaskModelLabel(m.setup.taskModelKey))
		options := m.currentSetupTaskModelOptions()
		items := make([]popupMenuItem, 0, len(options))
		for i, option := range options {
			items = append(items, popupMenuItem{
				Label:    option,
				Selected: m.setup.choiceIdx == i,
			})
		}
		content.WriteString("\n\n")
		if m.setup.modelsLoading {
			content.WriteString(helpStyle.Render("Fetching provider model list..."))
			content.WriteString("\n")
		}
		content.WriteString(helpStyle.Render("Fallback uses the provider model unless a default task override is configured."))
		maxModelLines := popupMaxViewportLines(m.termHeight, 4) - 5
		if maxModelLines < 4 {
			maxModelLines = 4
		}
		if maxModelLines > 14 {
			maxModelLines = 14
		}
		viewport := renderPopupMenuViewport(items, dialogWidth-6, maxModelLines, m.setup.choiceIdx)
		content.WriteString("\n\n")
		content.WriteString(viewport.content)
		footerText := "Enter: Select • Esc: Back"
		if viewport.maxOffset > 0 {
			footerText = "↑/↓: Scroll • " + footerText
		}
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			menu:   nil,
			footer: popupHintStyle.Render(setupFooter(footerText)),
		}
	case setupStepTaskModelValueField:
		fmt.Fprintf(&content, "Enter a model ID for %s:", setupTaskModelLabel(m.setup.taskModelKey))
		content.WriteString("\n\n")
		content.WriteString(helpStyle.Render("Leave blank to clear this override and use fallback."))
		content.WriteString("\n\n")
		content.WriteString(m.setup.input.View())
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			footer: popupHintStyle.Render(setupFooter("Enter: Save • Esc: Back")),
		}
	case setupStepResumeChoice:
		content.WriteString("Use your resume to prefill search criteria?\n\n")
		content.WriteString(helpStyle.Render("Sends parsed resume text to your configured LLM provider. The file is not stored."))
		options := []string{"Yes, enter a resume path", "Skip and enter criteria manually"}
		items := make([]popupMenuItem, 0, len(options))
		for i, option := range options {
			items = append(items, popupMenuItem{
				Label:    option,
				Selected: m.setup.choiceIdx == i,
			})
		}
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			menu:   items,
			footer: popupHintStyle.Render(setupFooter("Enter: Select • Esc: Back")),
		}
	case setupStepResumePathField:
		content.WriteString("Resume path\n")
		content.WriteString(helpStyle.Render("Supported: text/Markdown/CSV/YAML/JSON, DOCX, ODT, RTF, and PDF when pdftotext or mutool is installed."))
		content.WriteString("\n\n")
		if m.setup.resumeGenerating {
			content.WriteString(renderLoadingBody(
				"Resume Criteria",
				"Reading resume and asking the configured LLM for starter criteria...\n\nPlease wait.",
				m.loading.frame,
				dialogWidth-6,
			))
		} else {
			content.WriteString(m.setup.input.View())
		}
		footer = "Enter: Generate criteria • Esc: Back"
		if m.setup.message != "" {
			footer = m.setup.message + "\n\n" + footer
		}
	case setupStepAuthModeChoice:
		provider, _ := m.setup.currentSetupProviderConfig()
		content.WriteString("How should this provider load auth?\n\n")
		content.WriteString(helpStyle.Render(fmt.Sprintf("Provider: %s", providerLabel(provider))))
		options := currentSetupAuthModeOptions()
		items := make([]popupMenuItem, 0, len(options))
		for i, option := range options {
			items = append(items, popupMenuItem{
				Label:    setupAuthModeLabel(option, provider),
				Selected: m.setup.choiceIdx == i,
			})
		}
		return popupSpec{
			width:  dialogWidth,
			header: popupBodyStyle.Render(header.String()),
			body:   popupTextBody(popupBodyStyle.Render(content.String())),
			menu:   items,
			footer: popupHintStyle.Render(setupFooter("Enter: Select • Esc: Back")),
		}
	case setupStepAuthValueField:
		label, help := m.setup.currentSetupAuthField()
		provider, _ := m.setup.currentSetupProviderConfig()
		fmt.Fprintf(&content, "%s\n", label)
		content.WriteString(helpStyle.Render(help))
		content.WriteString("\n\n")
		content.WriteString(helpStyle.Render(fmt.Sprintf("Provider: %s", providerLabel(provider))))
		content.WriteString("\n\n")
		content.WriteString(m.setup.input.View())
		footer = "Enter: Next • Esc: Back"
		if m.setup.message != "" {
			footer = m.setup.message + "\n\n" + footer
		}
	case setupStepCriteriaField:
		field, ok := setupFieldSpecAt(m.setup.fieldIdx)
		if ok {
			fmt.Fprintf(&content, "Criteria %d of %d\n\n", m.setup.fieldIdx+1, len(searchProfileGroupSpec().Fields))
			fmt.Fprintf(&content, "%s\n", field.Label)
			content.WriteString(helpStyle.Render(field.Help))
			content.WriteString("\n\n")
			options := m.currentSetupCriteriaChoiceOptions(field.Key)
			if len(options) > 0 {
				content.WriteString(helpStyle.Render("Selected: " + m.currentSetupCriteriaChoiceSummary(field.Key)))
				content.WriteString("\n\n")
				for i, option := range options {
					cursor := " "
					if m.setup.choiceIdx == i {
						cursor = ">"
					}
					checked := " "
					if m.setupCriteriaChoiceSelected(field.Key, option.Value) {
						checked = "x"
					}
					fmt.Fprintf(&content, "%s [%s] %s\n", cursor, checked, option.Label)
				}
				footer = "Space: Toggle • Enter: Next • Esc: Back"
			} else {
				if setupFieldUsesTextarea(field.Key) {
					textareaWidth := dialogWidth - 9
					textareaHeight := setupCriteriaTextareaHeight(m.termHeight)
					m.setup.textarea.SetWidth(textareaWidth)
					m.setup.textarea.SetHeight(textareaHeight)
					content.WriteString(renderSetupTextareaWithScrollbar(m.setup.textarea, textareaWidth, textareaHeight))
					footer = "Enter: Next • Ctrl+U: Clear • Esc: Back"
				} else {
					content.WriteString(m.setup.input.View())
					footer = "Enter: Next • Esc: Back"
				}
			}
			if m.setup.message != "" {
				footer = m.setup.message + "\n\n" + footer
			}
		}
	case setupStepSummary:
		criteriaCfg, err := m.setupCriteriaFromState()
		content.WriteString("Review setup\n\n")
		if err != nil {
			fmt.Fprintf(&content, "Validation error: %v\n\n", err)
		} else {
			fmt.Fprintf(&content, "LLM features: %t\n", m.setup.useLLM)
			if m.setup.useLLM {
				available := config.LLMAuthAvailableNow(&m.setup.appConfig)
				fmt.Fprintf(&content, "Provider: %s\n", providerLabel(m.setup.appConfig.LLM.Provider))
				fmt.Fprintf(&content, "Model: %s\n", m.setup.appConfig.LLM.Model)
				fmt.Fprintf(&content, "Auth mode: %s\n", m.setup.appConfig.LLM.Auth.Mode)
				fmt.Fprintf(&content, "Auth configured: %t\n", llmpkg.LLMAuthConfigured(&m.setup.appConfig))
				fmt.Fprintf(&content, "Auth available now: %t\n", available)
				if !available {
					content.WriteString("\n")
					content.WriteString(helpStyle.Render("This provider cannot run yet. Press Enter to finish auth setup, or go back and choose No in LLM settings."))
					content.WriteString("\n")
				}
			}
			fmt.Fprintf(&content, "Location: %s, %s %s\n", criteriaCfg.Candidate.City, criteriaCfg.Candidate.State, criteriaCfg.Candidate.CountryCode)
			fmt.Fprintf(&content, "Minimum base: $%d\n", criteriaCfg.Filters.MinBaseUSD)
			fmt.Fprintf(&content, "Role families: %s\n", domain.FormatRoleFamilyLabels(criteriaCfg.RoleFamilies))
			fmt.Fprintf(&content, "Title prefixes: %s\n", joinOrFallback(criteriaCfg.Filters.TitleRequires, "none"))
			fmt.Fprintf(&content, "Target titles: %s\n", joinOrFallback(criteriaCfg.Filters.TitleIncludes, "none"))
			fmt.Fprintf(&content, "Work settings: %s\n", joinOrFallback(domain.SelectedWorkSettings(criteriaCfg.Filters.WorkSettings), "none"))
			resolved := fetcher.ResolveEffectiveSources(&m.setup.appConfig, criteriaCfg)
			fmt.Fprintf(&content, "Resolved sources: %d RSS, %d site targets, %d LLM web targets\n", len(resolved.RSSFeeds), len(resolved.SiteTargets), len(resolved.LLMWebTargets))
		}
		if m.setup.useLLM && !config.LLMAuthAvailableNow(&m.setup.appConfig) {
			footer = "Enter: Configure LLM auth • Esc: Back"
		} else {
			footer = "Enter: Save files • Esc: Back"
		}
		if m.setup.message != "" {
			footer = m.setup.message + "\n\n" + footer
		}
	case setupStepPromptReview:
		content.WriteString("Review generated SEARCH_PROMPT.md\n\n")
		content.WriteString(lipgloss.NewStyle().Width(dialogWidth - 6).Render(m.setup.prompt))
		if m.setup.useLLM && !config.LLMAuthAvailableNow(&m.setup.appConfig) {
			footer = "Enter: Configure LLM auth • e: Edit in $EDITOR • Esc: Back"
		} else {
			footer = "Enter: Save and preview • e: Edit in $EDITOR • Esc: Back"
		}
		if m.setup.message != "" {
			footer = m.setup.message + "\n\n" + footer
		}
	case setupStepPreviewConfirm:
		content.WriteString("Preview fetch results\n\n")
		if m.setup.previewBusy {
			content.WriteString(renderLoadingBody(
				"Preview Fetch",
				"Fetching jobs using the current settings...\n\nPlease wait.",
				m.loading.frame,
				dialogWidth-6,
			))
		} else if m.setup.previewErr != "" {
			fmt.Fprintf(&content, "Preview fetch failed:\n%s\n\n", m.setup.previewErr)
			footer = "Esc: Back • r: Revise setup"
		} else {
			previewCount := len(m.setup.previewJobs)
			fmt.Fprintf(&content, "Fetched %d temporary preview jobs with the current settings.\n", previewCount)
			if len(m.allJobs) > 0 {
				fmt.Fprintf(&content, "Saved jobs currently in the database: %d\n", len(m.allJobs))
				content.WriteString("Confirming setup will merge and deduplicate the preview list into the saved database.\n\n")
				footer = "Enter: Merge preview jobs • Esc: Back • r: Revise setup"
			} else {
				content.WriteString("Confirming setup will save the deduplicated preview list into the database.\n\n")
				footer = "Enter: Save preview jobs • Esc: Back • r: Revise setup"
			}
		}
	}

	if footer != "" {
		footer = popupHintStyle.Render(setupFooter(footer))
	}

	return popupSpec{
		width:  dialogWidth,
		header: popupBodyStyle.Render(header.String()),
		body:   popupFormBody(popupBodyStyle.Render(content.String())),
		footer: footer,
	}
}

func (m model) setupHeaderText() string {
	var header strings.Builder
	header.WriteString(setupTitleForMode(m.setup.mode))
	header.WriteString("\n\n")
	header.WriteString(m.setupModePurpose())

	if stepPurpose := setupStepPurpose(m.setup.step); stepPurpose != "" {
		header.WriteString("\n\n")
		header.WriteString(stepPurpose)
	}

	if len(m.setupIssues) > 0 && m.setup.mode != setupModeEdit {
		header.WriteString("\n\nWhat needs attention:\n")
		for _, issue := range m.setupIssues {
			fmt.Fprintf(&header, " - %s\n", issue)
		}
	}

	return strings.TrimSpace(header.String())
}

func (m model) setupModePurpose() string {
	switch m.setup.mode {
	case setupModeRecovery:
		return "Provider auth is not available in this shell."
	case setupModeRepair:
		return "Some setup data is missing or incomplete."
	case setupModeEdit:
		return "Change how jobscout searches, filters, and optionally uses your LLM provider."
	default:
		return fmt.Sprintf("Create a search profile and app settings under %s.", appruntime.DefaultDir())
	}
}

func setupStepPurpose(step setupStep) string {
	switch step {
	case setupStepRecoveryChoice:
		return ""
	case setupStepLLMChoice:
		return "Optional LLM features can assist search, filtering, resume setup, and company health summaries."
	case setupStepConfigMenu:
		return ""
	case setupStepLLMConfigMenu:
		return "Enable features, choose a provider, or edit provider config."
	case setupStepProviderConfigMenu:
		return "Configure credentials and models for the selected provider."
	case setupStepProviderChoice:
		return "Select the provider to use when LLM features are enabled."
	case setupStepAuthModeChoice:
		return "Environment variables or token commands avoid storing secrets in config files."
	case setupStepAuthValueField:
		return "Enter the auth source for this provider."
	case setupStepModelChoice:
		return "Choose the default model for LLM-assisted tasks."
	case setupStepModelValueField:
		return "Use a model ID that is not in the provider list."
	case setupStepTaskModelMenu:
		return "Optional per-task model overrides."
	case setupStepTaskModelChoice:
		return "Choose a model for this task, or keep it on the default fallback."
	case setupStepTaskModelValueField:
		return "Leave blank to clear this override."
	case setupStepResumeChoice:
		return "Optionally seed search criteria from a resume."
	case setupStepResumePathField:
		return "Provide the local resume path."
	case setupStepCriteriaField:
		return "Define the roles, locations, work settings, and compensation that should count as a useful match."
	case setupStepSummary:
		return "Review settings before saving."
	case setupStepPromptReview:
		return "Review the prompt used for LLM job search."
	case setupStepPreviewConfirm:
		return "Review temporary results before saving jobs."
	default:
		return ""
	}
}
