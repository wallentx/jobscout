package tuiapp

import tea "github.com/charmbracelet/bubbletea"

func (m model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.overlay.kind == overlaySetup || m.overlay.kind == overlayStatus || m.overlay.kind == overlayFilter {
		return m, nil
	}
	if m.overlay.kind == overlayNotice {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.overlay.notice.scrollOffset = clampPopupScroll(m.overlay.notice.scrollOffset-1, m.getMaxNoticeScroll())
		case tea.MouseButtonWheelDown:
			m.overlay.notice.scrollOffset = clampPopupScroll(m.overlay.notice.scrollOffset+1, m.getMaxNoticeScroll())
		}
		return m, nil
	}
	if m.overlay.kind == overlayHealth {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.overlay.health.scrollOffset = clampPopupScroll(m.overlay.health.scrollOffset-1, m.getMaxHealthScroll())
			return m, nil
		case tea.MouseButtonWheelDown:
			m.overlay.health.scrollOffset = clampPopupScroll(m.overlay.health.scrollOffset+1, m.getMaxHealthScroll())
			return m, nil
		}
	}
	if m.overlay.kind == overlayDetail {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.overlay.detail.scrollOffset = clampPopupScroll(m.overlay.detail.scrollOffset-1, m.getMaxDetailScroll())
		case tea.MouseButtonWheelDown:
			m.overlay.detail.scrollOffset = clampPopupScroll(m.overlay.detail.scrollOffset+1, m.getMaxDetailScroll())
		}
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.moveCursor(-1)
	case tea.MouseButtonWheelDown:
		m.moveCursor(1)
	}
	return m, nil

}
