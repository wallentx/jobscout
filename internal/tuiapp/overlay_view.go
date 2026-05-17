package tuiapp

import (
	"fmt"
	"strings"

	"github.com/wallentx/jobscout/internal/tui"
)

func (m model) buildNoticeOverlaySpec() popupSpec {
	dialogWidth := clampPopupWidth(m.termWidth, 40, 0)
	maxNoticeLines := popupMaxViewportLinesWithChrome(m.termHeight, 4, 11)
	bodyText := m.overlay.notice.message
	titleView := ""
	if m.overlay.notice.busy {
		titleView = renderLoadingTitle(m.overlay.notice.title, m.loading.frame)
	}
	viewport := renderScrollablePopupTextWithInheritedStyle(bodyText, dialogWidth-6, maxNoticeLines, m.overlay.notice.scrollOffset, noticePopupLineStylerForOrigin)
	footerText := "Enter/Esc: Close"
	if m.pendingFetch != nil {
		footerText = "Enter: Keep • Esc: Discard"
	}
	if viewport.maxOffset > 0 {
		footerText += " • ↑/↓/PgUp/PgDn: Scroll"
	}
	footer := popupHintStyle.Render(footerText)
	if m.overlay.notice.busy {
		footer = popupHintStyle.Render("Please wait.")
	}
	return popupSpec{
		width:     dialogWidth,
		title:     m.overlay.notice.title,
		titleView: titleView,
		body:      popupScrollableTextBody(viewport.content),
		footer:    footer,
	}
}

func (m model) buildDetailOverlaySpec() popupSpec {
	dialogWidth := int(float64(m.termWidth)*0.8) + 1
	if dialogWidth > 100 {
		dialogWidth = 100
	}
	if dialogWidth < 40 {
		dialogWidth = 40
	}
	innerW := dialogWidth - 6
	maxDetailLines := popupMaxViewportLinesWithChrome(m.termHeight, 6, 8)
	details := m.currentDetailText(innerW)
	viewport := renderScrollablePopupText(details, innerW, maxDetailLines, m.overlay.detail.scrollOffset, nil)
	footerText := "↑/↓: Prev/Next • u: Update • o: Open URL • Enter/Esc: Return"
	if viewport.maxOffset > 0 {
		footerText = "↑/↓: Prev/Next • u: Update • o: Open URL • j/k/PgUp/PgDn: Scroll • Enter/Esc: Return"
	}
	return popupSpec{
		width:  dialogWidth,
		body:   popupScrollableTextBody(viewport.content),
		footer: popupHintStyle.Render(footerText),
	}
}

func (m model) buildStatusOverlaySpec() popupSpec {
	items := make([]popupMenuItem, 0, len(statuses))
	for i, status := range statuses {
		items = append(items, popupMenuItem{
			Prefix:   statusEmoji(status),
			Label:    status,
			Selected: m.overlay.statusIdx == i,
		})
	}
	header := popupBodyStyle.Render("Select Status")
	footer := popupHintStyle.Render("Press Enter to confirm • Esc to cancel")
	return popupSpec{
		width:  fitPopupWidth(m.termWidth, 36, 72, "Select Status", renderPopupMenu(items), "Press Enter to confirm • Esc to cancel"),
		header: header,
		menu:   items,
		footer: footer,
	}
}

func (m model) buildFilterOverlaySpec() popupSpec {
	var header strings.Builder
	header.WriteString(popupTitleStyle.Render("Filter Jobs"))
	header.WriteString("\n\n")
	header.WriteString(popupHintStyle.Render("Toggle statuses with Space/Enter. Esc closes."))
	items := make([]popupMenuItem, 0, len(statuses)+1)
	for i, status := range statuses {
		marker := "( )"
		if m.overlay.filter.values[status] {
			marker = "(*)"
		}
		items = append(items, popupMenuItem{
			Prefix:   marker,
			Label:    status,
			Detail:   statusEmoji(status),
			Selected: m.overlay.filter.idx == i,
		})
	}

	saveLine := "Save current filter config as default"
	if m.overlay.filter.saved {
		saveLine += " [saved]"
	}
	items = append(items, popupMenuItem{
		Label:    saveLine,
		Selected: m.overlay.filter.idx == len(statuses),
	})

	return popupSpec{width: 52, header: header.String(), menu: items}
}

func (m model) buildBackgroundTaskOverlaySpec() popupSpec {
	targetWidth := clampPopupWidth(m.termWidth, 40, 0)
	minWidth := 10
	progress := m.backgroundTask.animProgress
	if !m.backgroundTask.animating && m.backgroundTask.expanded {
		progress = 1.0
	}

	width := int(float64(minWidth) + float64(targetWidth-minWidth)*progress)
	if width < minWidth {
		width = minWidth
	}

	// Calculate target position (centered)
	// We don't know the final height yet, but we can estimate or just interpolate x for now.
	// Actually, renderPopupWithTitle will center it if x/y are nil.
	// To animate, we MUST provide x/y.
	targetX := m.termWidth/2 - targetWidth/2
	targetY := m.termHeight/2 - 5 // Rough estimate of center Y

	startX := 0
	startY := 0

	posX := int(float64(startX) + float64(targetX-startX)*progress)
	posY := int(float64(startY) + float64(targetY-startY)*progress)

	maxNoticeLines := popupMaxViewportLinesWithChrome(m.termHeight, 4, 11)
	if progress < 1.0 {
		maxNoticeLines = int(float64(maxNoticeLines) * progress)
		if maxNoticeLines < 1 {
			maxNoticeLines = 1
		}
	}

	bodyText := m.backgroundTaskMessage()
	viewport := renderScrollablePopupText(bodyText, width-6, maxNoticeLines, 0, nil)
	footer := popupHintStyle.Render("t: Minimize")

	return popupSpec{
		width:     width,
		x:         &posX,
		y:         &posY,
		title:     m.backgroundTask.title,
		titleView: renderLoadingTitle(m.backgroundTask.title, m.loading.frame),
		body:      popupScrollableTextBody(viewport.content),
		footer:    footer,
	}
}

func (m model) buildSingleHealthTaskOverlaySpec() popupSpec {
	targetWidth := clampPopupWidth(m.termWidth, 40, 0)
	minWidth := 10
	progress := m.backgroundHealth.progress
	if !m.backgroundHealth.animating && m.backgroundHealth.expanded {
		progress = 1.0
	}

	width := int(float64(minWidth) + float64(targetWidth-minWidth)*progress)
	if width < minWidth {
		width = minWidth
	}

	targetX := m.termWidth/2 - targetWidth/2
	targetY := m.termHeight/2 - 5
	startX := 0
	startY := 0
	posX := int(float64(startX) + float64(targetX-startX)*progress)
	posY := int(float64(startY) + float64(targetY-startY)*progress)

	maxNoticeLines := popupMaxViewportLinesWithChrome(m.termHeight, 4, 11)
	if progress < 1.0 {
		maxNoticeLines = int(float64(maxNoticeLines) * progress)
		if maxNoticeLines < 1 {
			maxNoticeLines = 1
		}
	}

	title := m.singleHealthTaskTitle()
	viewport := renderScrollablePopupText(m.singleHealthTaskMessage(), width-6, maxNoticeLines, 0, nil)
	_, selectedIdx, totalTasks, _ := m.selectedSingleHealthTask()
	footer := "t: Minimize"
	if totalTasks > 1 {
		indicator := fmt.Sprintf("[%d/%d]", selectedIdx+1, totalTasks)
		footer = tui.CenterVisible(indicator, width-6) + "\n" + "←/→: Task • t: Minimize"
	}
	return popupSpec{
		width:     width,
		x:         &posX,
		y:         &posY,
		title:     title,
		titleView: renderLoadingTitle(title, m.loading.frame),
		body:      popupScrollableTextBody(viewport.content),
		footer:    popupHintStyle.Render(footer),
	}
}

func (m model) buildMainOverlaySpec() (popupSpec, bool) {
	switch m.overlay.kind {
	case overlaySetup:
		if m.blockingLoadingOverlayMinimized() {
			return popupSpec{}, false
		}
		return m.buildSetupOverlaySpec(), true
	case overlayNotice:
		if m.overlay.notice.visible && !m.blockingLoadingOverlayMinimized() {
			return m.buildNoticeOverlaySpec(), true
		}
	case overlayHealth:
		if m.blockingLoadingOverlayMinimized() {
			return popupSpec{}, false
		}
		return m.buildHealthOverlaySpec(), true
	case overlayDetail:
		if len(m.filteredJobs) > 0 {
			return m.buildDetailOverlaySpec(), true
		}
	case overlayStatus:
		return m.buildStatusOverlaySpec(), true
	case overlayFilter:
		return m.buildFilterOverlaySpec(), true
	}
	return popupSpec{}, false
}

func (m model) buildActiveFetchOverlaySpecIfActive() (popupSpec, bool) {
	if m.fetchingJobs && (m.activeFetch.expanded || m.activeFetch.animating) {
		targetWidth := clampPopupWidth(m.termWidth, 40, 0)
		minWidth := 10
		progress := m.activeFetch.animProgress
		if !m.activeFetch.animating && m.activeFetch.expanded {
			progress = 1.0
		}

		width := int(float64(minWidth) + float64(targetWidth-minWidth)*progress)
		if width < minWidth {
			width = minWidth
		}

		targetX := m.termWidth/2 - targetWidth/2
		targetY := m.termHeight/2 - 5

		startX := 0
		startY := 0

		posX := int(float64(startX) + float64(targetX-startX)*progress)
		posY := int(float64(startY) + float64(targetY-startY)*progress)

		maxNoticeLines := popupMaxViewportLinesWithChrome(m.termHeight, 4, 11)
		if progress < 1.0 {
			maxNoticeLines = int(float64(maxNoticeLines) * progress)
			if maxNoticeLines < 1 {
				maxNoticeLines = 1
			}
		}

		bodyText := m.activeFetch.progress
		if strings.TrimSpace(bodyText) == "" {
			bodyText = "Working..."
		}
		viewport := renderScrollablePopupText(bodyText, width-6, maxNoticeLines, 0, nil)
		footer := popupHintStyle.Render("t: Minimize")

		return popupSpec{
			width:     width,
			x:         &posX,
			y:         &posY,
			title:     m.activeFetch.title,
			titleView: renderLoadingTitle(m.activeFetch.title, m.loading.frame),
			body:      popupScrollableTextBody(viewport.content),
			footer:    footer,
		}, true
	}
	return popupSpec{}, false
}

func (m model) buildSingleHealthTaskOverlaySpecIfActive() (popupSpec, bool) {
	if !m.singleHealthTasksActive() || (!m.backgroundHealth.expanded && !m.backgroundHealth.animating) {
		return popupSpec{}, false
	}
	return m.buildSingleHealthTaskOverlaySpec(), true
}

func (m model) buildBackgroundTaskOverlaySpecIfActive() (popupSpec, bool) {
	if m.backgroundTask.active && (m.backgroundTask.expanded || m.backgroundTask.animating) {
		return m.buildBackgroundTaskOverlaySpec(), true
	}
	return popupSpec{}, false
}
