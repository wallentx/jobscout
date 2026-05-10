package tuiapp

import tea "github.com/charmbracelet/bubbletea"

func (m model) handleLoadingTickMsg(msg loadingTickMsg) (tea.Model, tea.Cmd) {
	if msg.generation != m.loading.generation {
		return m, nil
	}
	if !m.isLoadingActive() {
		return m, nil
	}
	m.loading.frame++
	return m, nextLoadingTick(m.loading.generation)
}

func (m model) handleBackgroundTaskAnimMsg(msg backgroundTaskAnimMsg) (tea.Model, tea.Cmd) {
	animating := false
	const animSpeed = 0.15

	if m.backgroundTask.animating {
		if m.backgroundTask.animProgress < msg.target {
			m.backgroundTask.animProgress += animSpeed
			if m.backgroundTask.animProgress > msg.target {
				m.backgroundTask.animProgress = msg.target
			}
		} else if m.backgroundTask.animProgress > msg.target {
			m.backgroundTask.animProgress -= animSpeed
			if m.backgroundTask.animProgress < msg.target {
				m.backgroundTask.animProgress = msg.target
			}
		}

		if m.backgroundTask.animProgress == msg.target {
			m.backgroundTask.animating = false
			m.backgroundTask.expanded = (msg.target == 1.0)
		} else {
			animating = true
		}
	}

	if m.fetchingJobs && m.activeFetch.animating {
		if m.activeFetch.animProgress < msg.target {
			m.activeFetch.animProgress += animSpeed
			if m.activeFetch.animProgress > msg.target {
				m.activeFetch.animProgress = msg.target
			}
		} else if m.activeFetch.animProgress > msg.target {
			m.activeFetch.animProgress -= animSpeed
			if m.activeFetch.animProgress < msg.target {
				m.activeFetch.animProgress = msg.target
			}
		}

		if m.activeFetch.animProgress == msg.target {
			m.activeFetch.animating = false
			m.activeFetch.expanded = (msg.target == 1.0)
		} else {
			animating = true
		}
	}

	if m.singleHealthTasksActive() && m.backgroundHealth.animating {
		if m.backgroundHealth.progress < msg.target {
			m.backgroundHealth.progress += animSpeed
			if m.backgroundHealth.progress > msg.target {
				m.backgroundHealth.progress = msg.target
			}
		} else if m.backgroundHealth.progress > msg.target {
			m.backgroundHealth.progress -= animSpeed
			if m.backgroundHealth.progress < msg.target {
				m.backgroundHealth.progress = msg.target
			}
		}

		if m.backgroundHealth.progress == msg.target {
			m.backgroundHealth.animating = false
			m.backgroundHealth.expanded = (msg.target == 1.0)
		} else {
			animating = true
		}
	}

	if animating {
		return m, nextBackgroundTaskAnimTick(msg.target)
	}

	return m, nil
}
