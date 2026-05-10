package tuiapp

import (
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	loadingTickInterval = 80 * time.Millisecond
)

type loadingIndicatorState struct {
	frame int
}

type loadingTickMsg struct{}

func nextLoadingTick() tea.Cmd {
	return tea.Tick(loadingTickInterval, func(time.Time) tea.Msg {
		return loadingTickMsg{}
	})
}

func nextBackgroundTaskAnimTick(target float64) tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg {
		return backgroundTaskAnimMsg{target: target}
	})
}

func (m *model) restartLoadingIndicator() tea.Cmd {
	m.loading.frame = 0
	return nextLoadingTick()
}

func (m model) isLoadingActive() bool {
	return strings.TrimSpace(m.activeLoadingLabel()) != ""
}

func (m model) canMinimizeBlockingLoadingOverlay() bool {
	switch {
	case m.overlay.kind == overlayNotice && m.overlay.notice.visible && m.overlay.notice.busy:
		return true
	case m.overlay.kind == overlayHealth && m.overlay.health.loading:
		return true
	case m.overlay.kind == overlaySetup && m.setup.resumeGenerating:
		return true
	case m.overlay.kind == overlaySetup && m.setup.previewBusy:
		return true
	default:
		return false
	}
}

func (m model) blockingLoadingOverlayMinimized() bool {
	switch m.overlay.kind {
	case overlayNotice:
		return m.overlay.notice.visible && m.overlay.notice.busy && m.overlay.notice.minimized
	case overlayHealth:
		return m.overlay.health.loading && m.overlay.health.minimized
	case overlaySetup:
		return (m.setup.resumeGenerating || m.setup.previewBusy) && m.setup.loadingMinimized
	default:
		return false
	}
}

func (m model) blockingLoadingOverlayTitle() string {
	switch {
	case m.overlay.kind == overlayNotice && m.overlay.notice.visible && m.overlay.notice.busy:
		if title := strings.TrimSpace(m.overlay.notice.title); title != "" {
			return title
		}
		return "Loading"
	case m.overlay.kind == overlayHealth && m.overlay.health.loading:
		if title := strings.TrimSpace(m.overlay.health.loadingText); title != "" {
			return title
		}
		return "Company Health"
	case m.overlay.kind == overlaySetup && m.setup.resumeGenerating:
		return "Resume Criteria"
	case m.overlay.kind == overlaySetup && m.setup.previewBusy:
		return "Preview Fetch"
	default:
		return ""
	}
}

func (m *model) setBlockingLoadingOverlayMinimized(minimized bool) bool {
	if !m.canMinimizeBlockingLoadingOverlay() {
		return false
	}

	switch m.overlay.kind {
	case overlayNotice:
		m.overlay.notice.minimized = minimized
	case overlayHealth:
		m.overlay.health.minimized = minimized
	case overlaySetup:
		m.setup.loadingMinimized = minimized
	default:
		return false
	}
	return true
}

func (m model) activeLoadingLabel() string {
	switch {
	case m.fetchingJobs:
		if strings.TrimSpace(m.activeFetch.title) != "" {
			return m.activeFetch.title
		}
		return "Fetching Jobs"
	case m.backgroundTask.active:
		if strings.TrimSpace(m.backgroundTask.title) != "" {
			return m.backgroundTask.title
		}
		return "Background task"
	case m.singleHealthTasksActive():
		return m.singleHealthTaskTitle()
	case m.overlay.kind == overlayHealth && m.overlay.health.loading:
		if strings.TrimSpace(m.overlay.health.loadingText) != "" {
			return m.overlay.health.loadingText
		}
		return "Company Health"
	case m.overlay.kind == overlayNotice && m.overlay.notice.visible && m.overlay.notice.busy:
		if strings.TrimSpace(m.overlay.notice.title) != "" {
			return m.overlay.notice.title
		}
		return "Loading"
	case m.overlay.kind == overlaySetup && m.setup.resumeGenerating:
		return "Resume Criteria"
	case m.overlay.kind == overlaySetup && m.setup.previewBusy:
		return "Preview Fetch"
	default:
		return ""
	}
}

func renderLoadingTitle(label string, frame int) string {
	return tui.RenderLoadingTitle(label, frame)
}

func renderLoadingBody(label string, message string, frame int, width int) string {
	return tui.RenderLoadingBody(label, message, frame, width)
}
