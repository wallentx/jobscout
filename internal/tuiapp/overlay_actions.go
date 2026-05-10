package tuiapp

import (
	"os"
	"strings"

	"github.com/wallentx/jobscout/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) showNotice(title string, message string, busy bool) {
	m.overlay = overlayState{
		kind: overlayNotice,
		notice: noticeState{
			visible:      true,
			busy:         busy,
			title:        title,
			message:      message,
			scrollOffset: 0,
		},
	}
}

func (m *model) clearOverlay() {
	m.overlay = overlayState{}
}

func (m *model) quitCommand() tea.Cmd {
	m.quitting = true
	m.cancelRunningTasks()
	return tea.Quit
}

func (m *model) moveCursor(delta int) {
	if len(m.filteredJobs) == 0 {
		m.cursor = 0
		m.yOffset = 0
		return
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filteredJobs) {
		m.cursor = len(m.filteredJobs) - 1
	}

	if m.cursor < m.yOffset {
		m.yOffset = m.cursor
	}
	if m.cursor >= m.yOffset+m.tableHeight {
		m.yOffset = m.cursor - m.tableHeight + 1
	}
	if m.yOffset < 0 {
		m.yOffset = 0
	}
}

func (m *model) openFilterMenu() {
	m.overlay.kind = overlayFilter
	m.overlay.filter.idx = 0
	m.overlay.filter.saved = false
	m.overlay.filter.values = cloneFilterMap(m.activeFilters)
}

func (m *model) openStatusMenu() {
	m.overlay.kind = overlayStatus
	m.overlay.statusIdx = 0
	if len(m.filteredJobs) == 0 {
		return
	}
	currentStatus := m.filteredJobs[m.cursor].Status
	for i, s := range statuses {
		if s == currentStatus {
			m.overlay.statusIdx = i
			break
		}
	}
}

func (m *model) openDetailOverlay() {
	m.overlay.kind = overlayDetail
	m.overlay.detail.scrollOffset = 0
}

func (m *model) openHealthOverlay(loading bool, loadingText string, report *CompanyHealthResult, errText string) {
	m.overlay.kind = overlayHealth
	m.overlay.health.loading = loading
	m.overlay.health.minimized = false
	m.overlay.health.loadingText = loadingText
	m.overlay.health.report = report
	m.overlay.health.err = errText
	m.overlay.health.scrollOffset = 0
}

func (m *model) openSetupOverlay(mode setupMode, section setupSection) {
	m.setup = newSetupState(mode, section)
	m.overlay.kind = overlaySetup
}

func (m *model) saveDefaultFilters() error {
	appCfg, err := config.LoadAppConfig(runtimeConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := config.DefaultAppConfig()
			appCfg = &cfg
		} else {
			return err
		}
	}

	appCfg.UI.DefaultFilterStatuses = selectedStatuses(m.overlay.filter.values)
	if err := config.SaveAppConfig(runtimeConfigPath, appCfg); err != nil {
		return err
	}

	m.overlay.filter.saved = true
	return nil
}

func buildHelpText(width int, items []string) string {
	if width < 20 {
		return strings.Join(items, " ")
	}

	var lines []string
	var current strings.Builder

	for _, item := range items {
		if item == "" {
			continue
		}

		if current.Len() == 0 {
			current.WriteString(item)
			continue
		}

		candidate := current.String() + " • " + item
		if lipgloss.Width(candidate) <= width {
			current.Reset()
			current.WriteString(candidate)
			continue
		}

		lines = append(lines, current.String())
		current.Reset()
		current.WriteString(item)
	}

	if current.Len() > 0 {
		lines = append(lines, current.String())
	}

	return strings.Join(lines, "\n")
}

func formatHelpItem(key string, description string) string {
	return helpKeyStyle.Render(key) + helpValueStyle.Render(": "+description)
}
