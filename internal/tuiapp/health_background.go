package tuiapp

import (
	"context"
	"fmt"
	"sort"
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

type singleHealthTaskItem struct {
	key     string
	company string
}

func (m model) singleHealthTaskRunning(job Job) bool {
	if len(m.backgroundHealth.tasks) == 0 {
		return false
	}
	_, ok := m.backgroundHealth.tasks[healthTaskKeyForJob(job)]
	return ok
}

func (m model) singleHealthTaskItems() []singleHealthTaskItem {
	if len(m.backgroundHealth.tasks) == 0 {
		return nil
	}
	items := make([]singleHealthTaskItem, 0, len(m.backgroundHealth.tasks))
	for key, task := range m.backgroundHealth.tasks {
		company := strings.TrimSpace(task.company)
		if company == "" {
			company = strings.TrimSpace(task.job.Company)
		}
		if company == "" {
			company = key
		}
		items = append(items, singleHealthTaskItem{
			key:     key,
			company: company,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		left := strings.ToLower(items[i].company)
		right := strings.ToLower(items[j].company)
		if left == right {
			return items[i].key < items[j].key
		}
		return left < right
	})
	return items
}

func (m model) selectedSingleHealthTask() (singleHealthTaskItem, int, int, bool) {
	items := m.singleHealthTaskItems()
	if len(items) == 0 {
		return singleHealthTaskItem{}, 0, 0, false
	}
	idx := m.backgroundHealth.selected
	if idx < 0 {
		idx = 0
	}
	if idx >= len(items) {
		idx = len(items) - 1
	}
	return items[idx], idx, len(items), true
}

func (m *model) selectSingleHealthTaskKey(key string) {
	items := m.singleHealthTaskItems()
	for i, item := range items {
		if item.key == key {
			m.backgroundHealth.selected = i
			return
		}
	}
	m.clampSingleHealthTaskSelection()
}

func (m *model) clampSingleHealthTaskSelection() {
	count := len(m.backgroundHealth.tasks)
	if count == 0 {
		m.backgroundHealth.selected = 0
		return
	}
	if m.backgroundHealth.selected < 0 {
		m.backgroundHealth.selected = 0
	}
	if m.backgroundHealth.selected >= count {
		m.backgroundHealth.selected = count - 1
	}
}

func (m *model) cycleSingleHealthTask(delta int) bool {
	count := len(m.backgroundHealth.tasks)
	if count < 2 {
		return false
	}
	m.backgroundHealth.selected = (m.backgroundHealth.selected + delta) % count
	if m.backgroundHealth.selected < 0 {
		m.backgroundHealth.selected += count
	}
	return true
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
	if !m.backgroundHealth.expanded {
		m.selectSingleHealthTaskKey(key)
	} else {
		m.clampSingleHealthTaskSelection()
	}
	if showPopup {
		m.clearOverlay()
		m.backgroundHealth.expanded = true
		m.backgroundHealth.animating = false
		m.backgroundHealth.progress = 1
	} else {
		m.backgroundHealth.expanded = false
		m.backgroundHealth.progress = 0
	}
	m.backgroundHealth.last = fmt.Sprintf("Started %s", company)
	logBulkHealthDebug("single start company=%q cache_key=%q popup=%t force_refresh=%t running=%d", company, key, showPopup, forceRefresh, len(m.backgroundHealth.tasks))

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
	m.clampSingleHealthTaskSelection()

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
	selected, _, _, ok := m.selectedSingleHealthTask()
	company := "Unknown"
	if ok && strings.TrimSpace(selected.company) != "" {
		company = selected.company
	}
	lines := []string{
		"Refreshing selected company health",
		"",
		fmt.Sprintf("Company  %s", company),
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
	return "Refreshing Health Data"
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
