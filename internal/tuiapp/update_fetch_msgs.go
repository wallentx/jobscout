package tuiapp

import (
	"fmt"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/fetcher"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleJobEditedMsg(msg jobEditedMsg) (tea.Model, tea.Cmd) {
	// Find the original job (which we edited) and replace it
	// Problem: We only have the NEW job. We assume the user didn't change the Company/Title so much we can't find it?
	// Actually, editJob takes the OLD job as input. But the callback only returns the NEW one.
	// We should probably rely on the cursor. The cursor points to the job we just edited (in filtered view).
	// BUT the user might have changed the Company name, breaking the sort/filter position.

	// Strategy: Ideally, we'd pass the original ID/Index.
	// Since we don't have IDs, let's just assume m.cursor matches m.filteredJobs[m.cursor] BEFORE the edit.
	// BUT `exec.Command` might have cleared the screen or something? No, logic state is preserved.

	if len(m.filteredJobs) > 0 && m.cursor < len(m.filteredJobs) {
		current := m.filteredJobs[m.cursor] // This is the OLD version

		// Find it in allJobs
		idx := -1
		for i, j := range m.allJobs {
			if j.Company == current.Company && j.Title == current.Title {
				idx = i
				break
			}
		}

		if idx != -1 {
			// Update
			m.allJobs[idx] = msg.job
			_ = saveRuntimeJobs(m.allJobs)
			m.applyFilterAndSort()
		}
	}
	return m, nil
}

func (m model) handleFetchJobsMsg(msg fetchJobsMsg) (tea.Model, tea.Cmd) {
	m.fetchingJobs = false
	m.fetchProgress = nil
	if msg.err != nil {
		m.showNotice("Fetch Failed", msg.err.Error(), false)
	} else {
		m.pendingFetch = fetcher.NewReviewSession(m.allJobs, msg.jobs, msg.summary, runtimeDebugEnabled)
		logDebug("Fetched jobs ready for review: fetched %d, added %d", m.pendingFetch.FetchedCount(), m.pendingFetch.AddedCount())
		if !m.pendingFetch.HasNewJobs() {
			m.showNotice("Fetch Complete", m.pendingFetch.Message(), false)
			m.pendingFetch = nil
			return m, nil
		}
		m.showNotice("Fetch Review", m.pendingFetch.Message(), false)
	}
	return m, nil
}

func (m model) handleAcceptedFetchSavedMsg(msg acceptedFetchSavedMsg) (tea.Model, tea.Cmd) {
	m.fetchingJobs = false
	m.fetchProgress = nil
	if msg.err != nil {
		m.showNotice("Fetch Save Failed", msg.err.Error(), false)
		return m, nil
	}
	m.allJobs = append([]Job(nil), msg.jobs...)
	m.applyFilterAndSort()
	if msg.saved == 0 {
		m.showNotice("Fetch Saved", "No new jobs were saved from the reviewed results.", false)
		return m, nil
	}
	m.showNotice("Fetch Saved", fmt.Sprintf("Saved %d new jobs from the reviewed results.", msg.saved), false)
	return m, nil
}

func (m model) handleAcceptedFetchEnrichedMsg(msg acceptedFetchEnrichedMsg) (tea.Model, tea.Cmd) {
	if !m.backgroundTask.active || msg.taskID != m.backgroundTask.id {
		return m, nil
	}
	selectedKey := m.selectedJobKey()
	if msg.err != nil {
		m.backgroundTask.active = false
		m.backgroundTask.expanded = false
		m.backgroundTask.progress = ""
		m.backgroundTask.pendingFields = nil
		m.showNotice("Background Task Failed", msg.err.Error(), false)
		return m, nil
	}

	_, merged := domain.MergeJobs(m.allJobs, msg.jobs)
	if err := saveRuntimeJobs(merged); err != nil {
		m.backgroundTask.active = false
		m.backgroundTask.expanded = false
		m.backgroundTask.progress = ""
		m.backgroundTask.pendingFields = nil
		m.showNotice("Background Save Failed", err.Error(), false)
		return m, nil
	}

	m.allJobs = append([]Job(nil), merged...)
	m.backgroundTask.active = false
	m.backgroundTask.expanded = false
	m.backgroundTask.progress = ""
	m.backgroundTask.pendingFields = nil
	m.applyFilterAndSort()
	m.restoreCursorToJobKey(selectedKey)
	logDebug("Accepted fetch background enrichment completed for %d jobs", len(msg.jobs))
	return m, nil
}

func (m model) handleBackgroundTaskProgressMsg(msg backgroundTaskProgressMsg) (tea.Model, tea.Cmd) {
	if !m.backgroundTask.active || msg.taskID != m.backgroundTask.id {
		return m, nil
	}
	if msg.done {
		return m, nil
	}
	if strings.TrimSpace(msg.text) != "" {
		m.backgroundTask.progress = msg.text
	}
	return m, waitForBackgroundTaskProgress(msg.taskID, msg.ch, msg.jobCh)
}

func (m model) handleBackgroundJobEnrichedMsg(msg backgroundJobEnrichedMsg) (tea.Model, tea.Cmd) {
	if !m.backgroundTask.active || msg.taskID != m.backgroundTask.id {
		return m, nil
	}

	// Update the job in allJobs
	idx := -1
	for i, j := range m.allJobs {
		if domain.JobMergeKey(j) == domain.JobMergeKey(msg.job) {
			idx = i
			break
		}
	}

	if idx != -1 {
		m.allJobs[idx] = msg.job
		// We don't save to disk on every single job enrichment to avoid heavy IO,
		// but we could if we want durability. The final completion saves everything.
		m.applyFilterAndSort()
	}

	m.updatePendingEnrichmentFields(msg.job)

	return m, waitForBackgroundTaskProgress(msg.taskID, msg.ch, msg.jobCh)
}

func (m *model) updatePendingEnrichmentFields(job Job) {
	if len(m.backgroundTask.pendingFields) == 0 {
		return
	}
	jobKey := backgroundJobKey(job)
	fields := m.backgroundTask.pendingFields[jobKey]
	if len(fields) == 0 {
		return
	}
	if !domain.JobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		delete(fields, "company_website")
	}
	if !domain.JobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
		delete(fields, "company_summary")
	}
	if strings.TrimSpace(job.CompanyIndustry) != "" {
		delete(fields, "company_industry")
	}
	if !domain.JobCompensationMissing(job.Compensation) {
		delete(fields, "compensation")
	}
	if len(fields) == 0 {
		delete(m.backgroundTask.pendingFields, jobKey)
		return
	}
	m.backgroundTask.pendingFields[jobKey] = fields
}

func (m model) selectedJobKey() string {
	if len(m.filteredJobs) == 0 || m.cursor < 0 || m.cursor >= len(m.filteredJobs) {
		return ""
	}
	return backgroundJobKey(m.filteredJobs[m.cursor])
}

func (m *model) restoreCursorToJobKey(key string) {
	if key == "" {
		return
	}
	for i, job := range m.filteredJobs {
		if backgroundJobKey(job) != key {
			continue
		}
		m.cursor = i
		if m.cursor < m.yOffset {
			m.yOffset = m.cursor
		}
		if m.tableHeight > 0 && m.cursor >= m.yOffset+m.tableHeight {
			m.yOffset = m.cursor - m.tableHeight + 1
		}
		if m.yOffset < 0 {
			m.yOffset = 0
		}
		return
	}
}

func (m model) handleFetchJobsProgressMsg(msg fetchJobsProgressMsg) (tea.Model, tea.Cmd) {
	if msg.ch == nil || msg.ch != m.fetchProgress {
		return m, nil
	}
	if msg.done {
		m.fetchProgress = nil
		return m, nil
	}
	if strings.TrimSpace(msg.text) != "" {
		if m.fetchingJobs {
			m.activeFetch.progress = msg.text
		}
	}
	return m, waitForFetchProgress(msg.ch)
}
