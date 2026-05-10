package tuiapp

import (
	"fmt"
	"os/exec"

	"github.com/wallentx/jobscout/internal/domain"
	healthpkg "github.com/wallentx/jobscout/internal/health"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "t" && m.overlay.kind == overlayHealth && m.overlay.health.minimized && m.singleHealthTasksActive() && !m.backgroundHealth.animating {
		m.backgroundHealth.animating = true
		val := 0.0
		if !m.backgroundHealth.expanded {
			val = 1.0
		}
		return m, nextBackgroundTaskAnimTick(val)
	}
	if msg.String() == "t" && m.canMinimizeBlockingLoadingOverlay() {
		m.setBlockingLoadingOverlayMinimized(!m.blockingLoadingOverlayMinimized())
		return m, nil
	}
	if m.overlay.kind != overlaySetup && msg.String() == "t" {
		var target *float64
		handled := false
		if m.backgroundTask.active && !m.backgroundTask.animating {
			m.backgroundTask.animating = true
			val := 0.0
			if !m.backgroundTask.expanded {
				val = 1.0
			}
			target = &val
			handled = true
		} else if m.fetchingJobs && !m.activeFetch.animating {
			m.activeFetch.animating = true
			val := 0.0
			if !m.activeFetch.expanded {
				val = 1.0
			}
			target = &val
			handled = true
		} else if m.singleHealthTasksActive() && !m.backgroundHealth.animating {
			m.backgroundHealth.animating = true
			val := 0.0
			if !m.backgroundHealth.expanded {
				val = 1.0
			}
			target = &val
			handled = true
		}
		if handled && target != nil {
			return m, nextBackgroundTaskAnimTick(*target)
		}
	}
	if m.overlay.kind != overlaySetup && m.singleHealthTasksActive() && m.backgroundHealth.expanded && !m.backgroundHealth.animating {
		switch msg.String() {
		case "left":
			if m.cycleSingleHealthTask(-1) {
				return m, nil
			}
		case "right":
			if m.cycleSingleHealthTask(1) {
				return m, nil
			}
		}
	}
	if m.overlay.kind == overlaySetup {
		switch msg.String() {
		case "ctrl+c":
			return m, m.quitCommand()
		case "q":
			if m.setupAcceptsTextInput() {
				break
			}
			return m, m.quitCommand()
		}
		return m.handleSetupKey(msg)
	}
	if m.overlay.kind == overlayNotice {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, m.quitCommand()
		case "up", "k":
			m.overlay.notice.scrollOffset = clampPopupScroll(m.overlay.notice.scrollOffset-1, m.getMaxNoticeScroll())
		case "down", "j":
			m.overlay.notice.scrollOffset = clampPopupScroll(m.overlay.notice.scrollOffset+1, m.getMaxNoticeScroll())
		case "pgup":
			m.overlay.notice.scrollOffset = clampPopupScroll(m.overlay.notice.scrollOffset-10, m.getMaxNoticeScroll())
		case "pgdown":
			m.overlay.notice.scrollOffset = clampPopupScroll(m.overlay.notice.scrollOffset+10, m.getMaxNoticeScroll())
		case "home":
			m.overlay.notice.scrollOffset = 0
		case "end":
			m.overlay.notice.scrollOffset = m.getMaxNoticeScroll()
		case "enter", "esc":
			if m.pendingFetch != nil {
				if msg.String() == "enter" {
					pendingFetch := m.pendingFetch
					m.pendingFetch = nil
					addedJobs := pendingFetch.AddedJobs()
					if len(addedJobs) == 0 {
						m.clearOverlay()
						return m, nil
					}
					saved, merged := domain.MergeJobs(m.allJobs, addedJobs)
					if err := saveRuntimeJobs(merged); err != nil {
						m.showNotice("Fetch Save Failed", err.Error(), false)
						return m, nil
					}
					m.allJobs = append([]Job(nil), merged...)
					m.applyFilterAndSort()
					m.clearOverlay()

					m.nextBackgroundTask++
					taskID := m.nextBackgroundTask
					progressCh := make(chan string, 8)
					jobCh := make(chan Job, 16)
					m.backgroundTask = backgroundTaskState{
						active:        true,
						expanded:      true,
						animProgress:  1,
						id:            taskID,
						title:         "Post-acceptance enrichment",
						progress:      fmt.Sprintf("Saved %d jobs. Enriching accepted jobs in the background...", saved),
						pendingFields: pendingFieldsForJobs(addedJobs),
					}
					logDebug("Accepted fetch saved %d jobs; background enrichment task %d started for %d accepted jobs", saved, taskID, len(addedJobs))
					return m, tea.Batch(
						enrichAcceptedFetchCmd(taskID, m.sessionLLMDisabled, addedJobs, progressCh, jobCh),
						waitForBackgroundTaskProgress(taskID, progressCh, jobCh),
						m.restartLoadingIndicator(),
					)
				}
				m.pendingFetch = nil
				m.clearOverlay()
				return m, nil
			}
			if !m.overlay.notice.busy {
				m.clearOverlay()
			}
		}
		return m, nil
	}

	if m.isFiltering {
		switch msg.String() {
		case "esc":
			m.isFiltering = false
			m.searchQuery = ""
			m.textInput.SetValue("")
			m.textInput.Blur()
			m.applyFilterAndSort()
			return m, nil
		case "enter":
			m.isFiltering = false
			m.searchQuery = m.textInput.Value()
			m.textInput.Blur()
			m.applyFilterAndSort()
			return m, nil
		default:
			var cmd2 tea.Cmd
			m.textInput, cmd2 = m.textInput.Update(msg)
			m.searchQuery = m.textInput.Value()
			m.applyFilterAndSort()
			return m, cmd2
		}
	}

	if m.overlay.kind == overlayStatus {
		switch msg.String() {
		case "esc":
			m.clearOverlay()
			return m, nil
		case "up", "k":
			if m.overlay.statusIdx > 0 {
				m.overlay.statusIdx--
			}
		case "down", "j":
			if m.overlay.statusIdx < len(statuses)-1 {
				m.overlay.statusIdx++
			}
		case "enter":
			// Update Status
			if len(m.filteredJobs) > 0 {
				jobID := -1
				// Find original job ID
				current := m.filteredJobs[m.cursor]
				for i, job := range m.allJobs {
					if job.Company == current.Company && job.Title == current.Title {
						jobID = i
						break
					}
				}
				if jobID != -1 {
					m.allJobs[jobID].Status = statuses[m.overlay.statusIdx]
					_ = saveRuntimeJobs(m.allJobs)
					m.applyFilterAndSort()
				}
			}
			m.clearOverlay()
			return m, nil
		}
		return m, nil
	}

	if m.overlay.kind == overlayFilter {
		switch msg.String() {
		case "esc":
			m.clearOverlay()
			return m, nil
		case "up", "k":
			if m.overlay.filter.idx > 0 {
				m.overlay.filter.idx--
			}
		case "down", "j":
			if m.overlay.filter.idx < len(statuses) {
				m.overlay.filter.idx++
			}
		case " ", "x":
			if m.overlay.filter.idx < len(statuses) {
				status := statuses[m.overlay.filter.idx]
				m.overlay.filter.values[status] = !m.overlay.filter.values[status]
				m.overlay.filter.saved = false
				m.activeFilters = cloneFilterMap(m.overlay.filter.values)
				m.applyFilterAndSort()
				m.cursor = 0
				m.yOffset = 0
			}
		case "enter":
			if m.overlay.filter.idx == len(statuses) {
				if err := m.saveDefaultFilters(); err != nil {
					m.showNotice("Save Filter Defaults Failed", err.Error(), false)
					return m, nil
				}
			} else {
				status := statuses[m.overlay.filter.idx]
				m.overlay.filter.values[status] = !m.overlay.filter.values[status]
				m.overlay.filter.saved = false
				m.activeFilters = cloneFilterMap(m.overlay.filter.values)
				m.applyFilterAndSort()
				m.cursor = 0
				m.yOffset = 0
			}
		}
		return m, nil
	}

	if m.overlay.kind == overlayHealth && !m.overlay.health.minimized {
		switch msg.String() {
		case "esc":
			m.clearOverlay()
			return m, nil
		case "up", "k":
			m.overlay.health.scrollOffset = clampPopupScroll(m.overlay.health.scrollOffset-1, m.getMaxHealthScroll())
		case "down", "j":
			m.overlay.health.scrollOffset = clampPopupScroll(m.overlay.health.scrollOffset+1, m.getMaxHealthScroll())
		case "pgup":
			m.overlay.health.scrollOffset = clampPopupScroll(m.overlay.health.scrollOffset-10, m.getMaxHealthScroll())
		case "pgdown":
			m.overlay.health.scrollOffset = clampPopupScroll(m.overlay.health.scrollOffset+10, m.getMaxHealthScroll())
		case "home":
			m.overlay.health.scrollOffset = 0
		case "end":
			m.overlay.health.scrollOffset = m.getMaxHealthScroll()
		case "u":
			job := m.filteredJobs[m.cursor]
			cmd := m.startSingleHealthTask(job, true, true)
			if cmd == nil {
				return m, nil
			}
			return m, tea.Batch(cmd, m.restartLoadingIndicator())
		case "q", "ctrl+c":
			return m, m.quitCommand()
		}
		return m, nil
	}

	if m.overlay.kind == overlayDetail {
		switch msg.String() {
		case "esc":
			m.clearOverlay()
		case "up", "p":
			if m.cursor > 0 {
				m.moveCursor(-1)
				m.overlay.detail.scrollOffset = 0
			}
		case "down", "n":
			if m.cursor < len(m.filteredJobs)-1 {
				m.moveCursor(1)
				m.overlay.detail.scrollOffset = 0
			}
		case "k":
			m.overlay.detail.scrollOffset = clampPopupScroll(m.overlay.detail.scrollOffset-1, m.getMaxDetailScroll())
		case "j":
			m.overlay.detail.scrollOffset = clampPopupScroll(m.overlay.detail.scrollOffset+1, m.getMaxDetailScroll())
		case "pgup":
			m.overlay.detail.scrollOffset = clampPopupScroll(m.overlay.detail.scrollOffset-10, m.getMaxDetailScroll())
		case "pgdown":
			m.overlay.detail.scrollOffset = clampPopupScroll(m.overlay.detail.scrollOffset+10, m.getMaxDetailScroll())
		case "home":
			m.overlay.detail.scrollOffset = 0
		case "end":
			m.overlay.detail.scrollOffset = m.getMaxDetailScroll()
		case "o":
			if len(m.filteredJobs) > 0 {
				return m, openURL(m.filteredJobs[m.cursor].ApplyURL)
			}
		case "u":
			if len(m.filteredJobs) > 0 {
				m.nextBackgroundTask++
				taskID := m.nextBackgroundTask
				job := m.filteredJobs[m.cursor]
				progressCh := make(chan string, 8)
				jobCh := make(chan Job, 1)
				m.backgroundTask = backgroundTaskState{
					active:        true,
					expanded:      false,
					animProgress:  0,
					id:            taskID,
					title:         "Updating Job Details",
					progress:      "Updating missing job and company fields in the background...",
					pendingFields: pendingFieldsForJobs([]Job{job}),
				}
				return m, tea.Batch(
					enrichAcceptedFetchCmd(taskID, m.sessionLLMDisabled, []Job{job}, progressCh, jobCh),
					waitForBackgroundTaskProgress(taskID, progressCh, jobCh),
					m.restartLoadingIndicator(),
				)
			}
		case "q", "ctrl+c":
			return m, m.quitCommand()
		}
		return m, nil
	}

	mainListKeysAvailable := m.overlay.kind == overlayNone || (m.overlay.kind == overlayHealth && m.overlay.health.minimized)

	switch msg.String() {
	case "q", "ctrl+c":
		return m, m.quitCommand()
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.yOffset {
				m.yOffset = m.cursor
			}
		}
	case "u":
		// (Handled above in showHealth, but this is the main list view fallback if needed)
		// No action defined for 'u' on main list currently.
		return m, nil
	case "down", "j":
		if m.cursor < len(m.filteredJobs)-1 {
			m.cursor++
			if m.cursor >= m.yOffset+m.tableHeight {
				m.yOffset = m.cursor - m.tableHeight + 1
			}
		}
	case "pgup":
		m.cursor -= m.tableHeight
		if m.cursor < 0 {
			m.cursor = 0
		}
		if m.cursor < m.yOffset {
			m.yOffset = m.cursor
		}
	case "pgdown":
		m.cursor += m.tableHeight
		if m.cursor >= len(m.filteredJobs) {
			m.cursor = len(m.filteredJobs) - 1
		}
		if m.cursor >= m.yOffset+m.tableHeight {
			m.yOffset = m.cursor - m.tableHeight + 1
		}
	case "home":
		m.cursor = 0
		m.yOffset = 0
	case "end":
		m.cursor = len(m.filteredJobs) - 1
		m.yOffset = m.cursor - m.tableHeight + 1
		if m.yOffset < 0 {
			m.yOffset = 0
		}
	case "enter":
		if len(m.filteredJobs) > 0 {
			m.openDetailOverlay()
		}
		return m, nil
	case "esc":
		if m.overlay.kind == overlayDetail {
			m.clearOverlay()
		}
		return m, nil
	case "s":
		if mainListKeysAvailable && len(m.filteredJobs) > 0 {
			m.openStatusMenu()
			return m, nil
		}
	case "1", "2", "3", "4", "5":
		if mainListKeysAvailable {
			newSort := int(msg.String()[0] - '1')
			if m.sortBy == newSort {
				m.sortDesc = !m.sortDesc // Toggle direction
			} else {
				m.sortBy = newSort
				// Default directions
				if newSort == 0 || newSort == 4 {
					m.sortDesc = true // Health and Date default to Descending
				} else {
					m.sortDesc = false // Strings default to Ascending
				}
			}
			m.applyFilterAndSort()
		}
	case "r":
		if mainListKeysAvailable && !m.fetchingJobs {
			progressCh := make(chan string, 8)
			m.fetchingJobs = true
			m.fetchProgress = progressCh

			m.activeFetch = activeFetchState{
				expanded:     true,
				animProgress: 1.0,
				title:        "Fetching Jobs",
				progress:     fetchStartMessage(m.sessionLLMDisabled),
			}
			return m, tea.Batch(fetchJobsCmd(m.sessionLLMDisabled, append([]Job(nil), m.allJobs...), progressCh), waitForFetchProgress(progressCh), m.restartLoadingIndicator())
		}
	case "c":
		if mainListKeysAvailable {
			m.openSetupOverlay(setupModeEdit, setupSectionNone)
			return m, nil
		}
	case "f":
		if mainListKeysAvailable {
			m.openFilterMenu()
		}
	case "/":
		if mainListKeysAvailable {
			m.isFiltering = true
			m.textInput.Focus()
			return m, textinput.Blink
		}
	case "h":
		if mainListKeysAvailable && len(m.filteredJobs) > 0 {
			job := m.filteredJobs[m.cursor]
			if cached := healthpkg.StoredHealthForJob(m.healthCache, job); cached != nil {
				m.openHealthOverlay(false, "", cached, "")
				return m, nil
			}
			showPopup := m.overlay.kind == overlayNone && (!m.singleHealthTasksActive() || m.backgroundHealth.expanded)
			cmd := m.startSingleHealthTask(job, showPopup, false)
			if cmd == nil {
				return m, nil
			}
			return m, tea.Batch(cmd, m.restartLoadingIndicator())
		}
	case "H":
		if mainListKeysAvailable && !m.bulkHealthFetching && len(m.allJobs) > 0 {
			jobs := healthpkg.UniqueJobsFromJobs(m.allJobs)
			jobs = m.excludeRunningSingleHealthJobs(jobs)
			if len(jobs) == 0 {
				return m, nil
			}

			// Clear the cache to force a full refresh.
			m.healthCache = make(HealthCache)
			_ = runtimeHealthStore.ClearHealthCache()
			m.bulkHealthFetching = true
			m.bulkHealthCompanies = healthpkg.UniqueCompaniesFromJobs(jobs)
			m.bulkHealthJobs = jobs
			m.bulkHealthIdx = 0
			m.bulkHealthUpdated = 0
			m.bulkHealthCompleted = 0
			m.bulkHealthFailed = 0
			m.bulkHealthSkipped = 0
			m.bulkHealthInFlight = 0
			m.clearOverlay()
			logBulkHealthDebug(
				"start jobs=%d unique_jobs=%d unique_companies=%d concurrency=%d termux=%t mem=%s",
				len(m.allJobs),
				len(jobs),
				len(m.bulkHealthCompanies),
				bulkHealthConcurrency(len(jobs)),
				isTermuxRuntime(),
				currentBulkHealthMemStats(),
			)
			m.backgroundTask = backgroundTaskState{
				active:       true,
				expanded:     true,
				animProgress: 1.0,
				title:        "Refreshing Health Data",
				progress:     bulkHealthProgressMessage(0, len(jobs), 0, 0, 0, bulkHealthConcurrency(len(jobs)), ""),
			}
			return m, tea.Batch(m.scheduleBulkHealthCommands(), m.restartLoadingIndicator())
		}
	case "D":
		if mainListKeysAvailable && len(m.filteredJobs) > 0 {
			// Delete current job
			// We need to map cursor (filtered index) to allJobs index
			current := m.filteredJobs[m.cursor]
			idx := -1
			for i, j := range m.allJobs {
				if j.Company == current.Company && j.Title == current.Title {
					idx = i
					break
				}
			}
			if idx != -1 {
				newJobs, err := deleteJob(m.allJobs, idx)
				if err == nil {
					m.allJobs = newJobs
					m.applyFilterAndSort()
				} else {
					m.openHealthOverlay(false, "", nil, fmt.Sprintf("Delete failed: %v", err))
				}
			}
			return m, nil
		}
	case "m":
		if mainListKeysAvailable && len(m.filteredJobs) > 0 {
			// Mark as Viewed
			current := m.filteredJobs[m.cursor]
			idx := -1
			for i, j := range m.allJobs {
				if j.Company == current.Company && j.Title == current.Title {
					idx = i
					break
				}
			}
			if idx != -1 {
				m.allJobs[idx].Status = "Viewed"
				_ = saveRuntimeJobs(m.allJobs)

				// Record current cursor position
				oldCursor := m.cursor

				m.applyFilterAndSort()

				// Advance cursor if we are still viewing the same list roughly
				// Or if the item disappeared due to filtering, the cursor stays or shifts.
				// For simplicity, just advance if not at the end.
				m.cursor = oldCursor
				if m.cursor < len(m.filteredJobs)-1 {
					m.cursor++
					if m.cursor >= m.yOffset+m.tableHeight {
						m.yOffset = m.cursor - m.tableHeight + 1
					}
				}
			}
			return m, nil
		}
	case "E":
		if mainListKeysAvailable && len(m.filteredJobs) > 0 {
			current := m.filteredJobs[m.cursor]
			return m, tea.ExecProcess(exec.Command("true"), func(err error) tea.Msg {
				return editJob(current)()
			})
		}
	case "o":
		if m.overlay.kind == overlayDetail {
			return m, openURL(m.filteredJobs[m.cursor].ApplyURL)
		}
	}
	return m, nil
}

func (m model) setupAcceptsTextInput() bool {
	switch m.setup.step {
	case setupStepModelValueField, setupStepTaskModelValueField, setupStepAuthValueField, setupStepResumePathField:
		return true
	case setupStepCriteriaField:
		field, ok := setupFieldSpecAt(m.setup.fieldIdx)
		if !ok {
			return false
		}
		return len(m.currentSetupCriteriaChoiceOptions(field.Key)) == 0
	default:
		return false
	}
}
