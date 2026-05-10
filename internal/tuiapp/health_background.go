package tuiapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
	healthpkg "github.com/wallentx/jobscout/internal/health"

	tea "github.com/charmbracelet/bubbletea"
)

func modelTaskContext(m model) context.Context {
	if m.taskCtx != nil {
		return m.taskCtx
	}
	return context.Background()
}

func (m *model) cancelRunningTasks() {
	if m.cancelTasks != nil {
		m.cancelTasks()
	}
}

func healthTaskKeyForJob(job Job) string {
	if key := healthpkg.CacheKeyForJob(job); strings.TrimSpace(key) != "" {
		return key
	}
	if key := domain.JobMergeKey(job); strings.TrimSpace(key) != "" {
		return key
	}
	return strings.ToLower(strings.TrimSpace(job.Company))
}

func (m model) singleHealthTaskRunning(job Job) bool {
	if len(m.backgroundHealth.tasks) == 0 {
		return false
	}
	_, ok := m.backgroundHealth.tasks[healthTaskKeyForJob(job)]
	return ok
}

func (m model) excludeRunningSingleHealthJobs(jobs []Job) []Job {
	if len(jobs) == 0 || len(m.backgroundHealth.tasks) == 0 {
		return jobs
	}
	filtered := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		key := healthTaskKeyForJob(job)
		if _, running := m.backgroundHealth.tasks[key]; running {
			logBulkHealthDebug("full refresh excluding single in-flight company=%q cache_key=%q", job.Company, key)
			continue
		}
		filtered = append(filtered, job)
	}
	return filtered
}

func (m *model) startSingleHealthTask(job Job, showPopup bool, forceRefresh bool) tea.Cmd {
	key := healthTaskKeyForJob(job)
	if key == "" || m.singleHealthTaskRunning(job) {
		if key != "" {
			logBulkHealthDebug("single skipped company=%q cache_key=%q reason=already_running", job.Company, key)
		}
		return nil
	}

	if m.backgroundHealth.tasks == nil {
		m.backgroundHealth.tasks = make(map[string]singleHealthTaskState)
	}
	company := strings.TrimSpace(job.Company)
	m.backgroundHealth.tasks[key] = singleHealthTaskState{job: job, company: company}
	if !showPopup {
		m.backgroundHealth.expanded = false
		m.backgroundHealth.progress = 0
	}
	m.backgroundHealth.last = fmt.Sprintf("Started %s", company)
	logBulkHealthDebug("single start company=%q cache_key=%q popup=%t force_refresh=%t running=%d", company, key, showPopup, forceRefresh, len(m.backgroundHealth.tasks))

	if showPopup {
		m.openHealthOverlay(true, fmt.Sprintf("Checking %s", company), nil, "")
	}
	return loadCompanyHealthForJobWithContext(modelTaskContext(*m), job, forceRefresh, key, !showPopup)
}

func (m *model) finishSingleHealthTask(msg healthLoadedMsg) {
	key := strings.TrimSpace(msg.taskKey)
	if key == "" {
		return
	}
	if len(m.backgroundHealth.tasks) > 0 {
		delete(m.backgroundHealth.tasks, key)
	}

	company := strings.TrimSpace(msg.company)
	if company == "" {
		company = key
	}
	switch {
	case msg.err == nil && msg.result != nil:
		m.backgroundHealth.updated++
		m.backgroundHealth.last = fmt.Sprintf("Updated %s", company)
	case msg.err != nil && healthpkg.IsIdentityUnresolvedText(msg.err.Error()):
		m.backgroundHealth.skipped++
		m.backgroundHealth.last = fmt.Sprintf("Skipped %s", company)
	case msg.err != nil:
		m.backgroundHealth.failed++
		m.backgroundHealth.last = fmt.Sprintf("Failed %s", company)
	default:
		m.backgroundHealth.last = fmt.Sprintf("Finished %s", company)
	}
	logBulkHealthDebug("single finished company=%q cache_key=%q err=%v running=%d", company, key, msg.err, len(m.backgroundHealth.tasks))
}

func (m model) singleHealthTasksActive() bool {
	return len(m.backgroundHealth.tasks) > 0
}

func (m model) singleHealthTaskMessage() string {
	running := len(m.backgroundHealth.tasks)
	if running == 0 {
		return ""
	}
	lines := []string{
		"Refreshing selected company health",
		"",
		fmt.Sprintf("Running  %d", running),
		fmt.Sprintf("Updated  %d", m.backgroundHealth.updated),
		fmt.Sprintf("Skipped  %d", m.backgroundHealth.skipped),
		fmt.Sprintf("Failed   %d", m.backgroundHealth.failed),
	}
	if strings.TrimSpace(m.backgroundHealth.last) != "" {
		lines = append(lines, "", m.backgroundHealth.last)
	}
	return strings.Join(lines, "\n")
}

func (m model) singleHealthTaskTitle() string {
	if !m.singleHealthTasksActive() {
		return ""
	}
	if len(m.backgroundHealth.tasks) == 1 {
		for _, task := range m.backgroundHealth.tasks {
			if strings.TrimSpace(task.company) != "" {
				return "Refreshing " + task.company
			}
		}
	}
	return fmt.Sprintf("Refreshing Health (%d)", len(m.backgroundHealth.tasks))
}

func healthLoadedCacheKey(msg healthLoadedMsg) string {
	if msg.result == nil {
		return msg.company
	}
	cacheKey := healthpkg.CacheKeyForIdentity(CompanyHealthContext{
		Company:  msg.result.Company,
		Website:  healthpkg.SourceStringFromMap(msg.result.Sources, "company_identity", "website"),
		Summary:  healthpkg.SourceStringFromMap(msg.result.Sources, "company_identity", "summary"),
		Industry: healthpkg.SourceStringFromMap(msg.result.Sources, "company_identity", "industry"),
	})
	if cacheKey == "" {
		cacheKey = msg.company
	}
	return cacheKey
}

func healthLoadedFetchedAt(msg healthLoadedMsg) time.Time {
	if msg.fetchedAt.IsZero() {
		return time.Now()
	}
	return msg.fetchedAt
}
