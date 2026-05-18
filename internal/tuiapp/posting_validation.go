package tuiapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/fetcher"

	tea "github.com/charmbracelet/bubbletea"
)

type postingValidationTarget struct {
	key string
	job Job
}

type postingValidationResult struct {
	key    string
	active bool
	reason string
}

type postingValidationCompleteMsg struct {
	taskID  int
	checked int
	results []postingValidationResult
	err     error
}

var verifyJobPosting = fetcher.VerifyJobPosting

func postingValidationTargets(jobs []Job) []postingValidationTarget {
	targets := make([]postingValidationTarget, 0, len(jobs))
	seen := make(map[string]bool, len(jobs))
	for _, job := range jobs {
		if !postingValidationStatus(job.Status) {
			continue
		}
		key := backgroundJobKey(job)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, postingValidationTarget{
			key: key,
			job: job,
		})
	}
	return targets
}

func postingValidationStatus(status string) bool {
	status = strings.TrimSpace(status)
	return strings.EqualFold(status, "Unopened") || strings.EqualFold(status, "Viewed")
}

func validatePostingsCmd(taskID int, targets []postingValidationTarget, progressCh chan<- string) tea.Cmd {
	return func() tea.Msg {
		if progressCh != nil {
			defer close(progressCh)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		results := make([]postingValidationResult, 0, len(targets))
		for i, target := range targets {
			reportPostingValidationProgress(progressCh, "Checking active postings %d of %d: %s - %s", i+1, len(targets), target.job.Company, target.job.Title)
			active, reason := verifyJobPosting(ctx, target.job)
			if err := ctx.Err(); err != nil {
				return postingValidationCompleteMsg{
					taskID:  taskID,
					checked: len(results),
					results: results,
					err:     err,
				}
			}
			results = append(results, postingValidationResult{
				key:    target.key,
				active: active,
				reason: reason,
			})
		}

		return postingValidationCompleteMsg{
			taskID:  taskID,
			checked: len(results),
			results: results,
		}
	}
}

func (m model) handlePostingValidationCompleteMsg(msg postingValidationCompleteMsg) (tea.Model, tea.Cmd) {
	if !m.backgroundTask.active || msg.taskID != m.backgroundTask.id {
		return m, nil
	}

	selectedKey := m.selectedJobKey()
	expired := m.expireInactivePostings(msg.results)

	m.backgroundTask.active = false
	m.backgroundTask.expanded = false
	m.backgroundTask.progress = ""
	m.backgroundTask.pendingFields = nil

	if expired > 0 {
		if err := saveRuntimeJobs(m.allJobs); err != nil {
			m.showNotice("Posting Check Save Failed", err.Error(), false)
			return m, nil
		}
	}

	m.applyFilterAndSort()
	m.restoreCursorToJobKey(selectedKey)

	title := "Posting Check Complete"
	if msg.err != nil {
		title = "Posting Check Stopped"
	}
	message := postingValidationSummary(msg.checked, expired, msg.err)
	m.showNotice(title, message, false)
	return m, nil
}

func (m *model) expireInactivePostings(results []postingValidationResult) int {
	if len(results) == 0 {
		return 0
	}

	inactive := make(map[string]string, len(results))
	for _, result := range results {
		if result.active {
			continue
		}
		inactive[result.key] = result.reason
	}
	if len(inactive) == 0 {
		return 0
	}

	expired := 0
	for i := range m.allJobs {
		job := m.allJobs[i]
		if !postingValidationStatus(job.Status) {
			continue
		}
		reason, ok := inactive[backgroundJobKey(job)]
		if !ok {
			continue
		}
		m.allJobs[i].Status = "Expired"
		expired++
		if reason != "" {
			logDebug("posting validation expired company=%q title=%q apply_url=%q reason=%q", job.Company, job.Title, job.ApplyURL, reason)
		}
	}
	return expired
}

func postingValidationSummary(checked int, expired int, err error) string {
	lines := []string{
		fmt.Sprintf("Checked %d active %s.", checked, pluralize(checked, "posting", "postings")),
		fmt.Sprintf("Marked %d %s as Expired.", expired, pluralize(expired, "posting", "postings")),
	}
	if err != nil {
		lines = append(lines, "", fmt.Sprintf("Stopped before finishing: %v", err))
	}
	return strings.Join(lines, "\n")
}

func pluralize(count int, singular string, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func reportPostingValidationProgress(progressCh chan<- string, format string, args ...any) {
	if progressCh == nil {
		return
	}
	message := strings.TrimSpace(fmt.Sprintf(format, args...))
	if message == "" {
		return
	}
	select {
	case progressCh <- message:
	default:
	}
}
