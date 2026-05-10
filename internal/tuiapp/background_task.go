package tuiapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/fetcher"
	llmpkg "github.com/wallentx/jobscout/internal/llm"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const enrichmentLoadingText = "Enrichment loading"

const (
	taskActivityGlyph       = "✨"
	taskActivityGlyphHidden = "  "
)

type taskActivityGlyphState int

const (
	taskActivityGlyphBright taskActivityGlyphState = iota
	taskActivityGlyphDim
	taskActivityGlyphOff
)

type taskActivityPulseSegment struct {
	state  taskActivityGlyphState
	frames int
}

var (
	taskActivityGlyphDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	taskActivityPulse         = []taskActivityPulseSegment{
		{state: taskActivityGlyphBright, frames: 20},
		{state: taskActivityGlyphDim, frames: 4},
		{state: taskActivityGlyphOff, frames: 4},
		{state: taskActivityGlyphDim, frames: 4},
		{state: taskActivityGlyphBright, frames: 12},
		{state: taskActivityGlyphOff, frames: 3},
		{state: taskActivityGlyphBright, frames: 8},
		{state: taskActivityGlyphOff, frames: 2},
		{state: taskActivityGlyphBright, frames: 4},
		{state: taskActivityGlyphOff, frames: 1},
		{state: taskActivityGlyphBright, frames: 2},
		{state: taskActivityGlyphOff, frames: 1},
		{state: taskActivityGlyphBright, frames: 4},
		{state: taskActivityGlyphOff, frames: 1},
		{state: taskActivityGlyphBright, frames: 8},
		{state: taskActivityGlyphOff, frames: 2},
		{state: taskActivityGlyphBright, frames: 12},
		{state: taskActivityGlyphOff, frames: 3},
		{state: taskActivityGlyphDim, frames: 4},
		{state: taskActivityGlyphOff, frames: 4},
		{state: taskActivityGlyphDim, frames: 4},
	}
)

func backgroundJobKey(job Job) string {
	if applyURL := strings.TrimSpace(job.ApplyURL); applyURL != "" {
		return "url:" + strings.ToLower(applyURL)
	}
	return "merge:" + domain.JobMergeKey(job)
}

func pendingEnrichmentFields(job Job) map[string]bool {
	fields := make(map[string]bool)
	if domain.JobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		fields["company_website"] = true
	}
	if domain.JobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
		fields["company_summary"] = true
	}
	if strings.TrimSpace(job.CompanyIndustry) == "" {
		fields["company_industry"] = true
	}
	if domain.JobCompensationMissing(job.Compensation) {
		fields["compensation"] = true
	}
	return fields
}

func pendingFieldsForJobs(jobs []Job) map[string]map[string]bool {
	pending := make(map[string]map[string]bool)
	for _, job := range jobs {
		fields := pendingEnrichmentFields(job)
		if len(fields) == 0 {
			continue
		}
		pending[backgroundJobKey(job)] = fields
	}
	return pending
}

func (m model) pendingField(job Job, field string) bool {
	if !m.backgroundTask.active || len(m.backgroundTask.pendingFields) == 0 {
		return false
	}
	fields := m.backgroundTask.pendingFields[backgroundJobKey(job)]
	return fields[field]
}

func (m model) backgroundTaskActivityView() string {
	title := m.taskActivityTitle()
	if title == "" {
		return ""
	}

	return taskActivityGlyphView(m.loading.frame) + " " + renderLoadingTitle(title, m.loading.frame)
}

func (m model) taskActivityTitle() string {
	if m.blockingLoadingOverlayMinimized() {
		if m.overlay.kind == overlayHealth && m.singleHealthTasksActive() {
			return m.singleHealthTaskTitle()
		}
		return m.blockingLoadingOverlayTitle()
	} else if m.fetchingJobs {
		if title := strings.TrimSpace(m.activeFetch.title); title != "" {
			return title
		}
		return "Fetching Jobs"
	} else if m.backgroundTask.active {
		if title := strings.TrimSpace(m.backgroundTask.title); title != "" {
			return title
		}
		return "Background task"
	} else if title := m.singleHealthTaskTitle(); title != "" {
		return title
	}

	return ""
}

func taskActivityGlyphView(frame int) string {
	switch taskActivityGlyphStateForFrame(frame) {
	case taskActivityGlyphDim:
		return taskActivityGlyphDimStyle.Render(taskActivityGlyph)
	case taskActivityGlyphOff:
		return taskActivityGlyphHidden
	default:
		return taskActivityGlyph
	}
}

func taskActivityGlyphStateForFrame(frame int) taskActivityGlyphState {
	if frame < 0 || len(taskActivityPulse) == 0 {
		return taskActivityGlyphBright
	}

	cycleFrames := 0
	for _, segment := range taskActivityPulse {
		if segment.frames > 0 {
			cycleFrames += segment.frames
		}
	}
	if cycleFrames == 0 {
		return taskActivityGlyphBright
	}

	step := frame % cycleFrames
	for _, segment := range taskActivityPulse {
		if segment.frames <= 0 {
			continue
		}
		if step < segment.frames {
			return segment.state
		}
		step -= segment.frames
	}

	return taskActivityGlyphBright
}

func (m model) backgroundTaskMessage() string {
	if !m.backgroundTask.active {
		return ""
	}
	lines := []string{m.backgroundTask.progress}
	if strings.TrimSpace(lines[0]) == "" {
		lines[0] = "Working in the background..."
	}
	pendingJobs := len(m.backgroundTask.pendingFields)
	if pendingJobs > 0 {
		lines = append(lines, "", fmt.Sprintf("Pending enrichment fields for %d jobs.", pendingJobs))
	}
	return strings.Join(lines, "\n")
}

func waitForBackgroundTaskProgress(taskID int, ch <-chan string, jobCh <-chan Job) tea.Cmd {
	if ch == nil && jobCh == nil {
		return nil
	}
	return func() tea.Msg {
		for {
			if jobCh != nil {
				select {
				case job, ok := <-jobCh:
					if ok {
						return backgroundJobEnrichedMsg{
							taskID: taskID,
							job:    job,
							ch:     ch,
							jobCh:  jobCh,
						}
					}
					jobCh = nil
					continue
				default:
				}
			}

			if ch != nil {
				select {
				case text, ok := <-ch:
					if ok {
						return backgroundTaskProgressMsg{
							taskID: taskID,
							text:   text,
							ch:     ch,
							jobCh:  jobCh,
						}
					}
					ch = nil
					if jobCh == nil {
						return backgroundTaskProgressMsg{
							taskID: taskID,
							done:   true,
						}
					}
					continue
				default:
				}
			}

			if ch == nil && jobCh == nil {
				return backgroundTaskProgressMsg{
					taskID: taskID,
					done:   true,
				}
			}

			select {
			case job, ok := <-jobCh:
				if ok {
					return backgroundJobEnrichedMsg{
						taskID: taskID,
						job:    job,
						ch:     ch,
						jobCh:  jobCh,
					}
				}
				jobCh = nil
				if ch == nil {
					return backgroundTaskProgressMsg{
						taskID: taskID,
						done:   true,
					}
				}
			case text, ok := <-ch:
				if ok {
					return backgroundTaskProgressMsg{
						taskID: taskID,
						text:   text,
						ch:     ch,
						jobCh:  jobCh,
					}
				}
				ch = nil
				if jobCh == nil {
					return backgroundTaskProgressMsg{
						taskID: taskID,
						done:   true,
					}
				}
			}
		}
	}
}

func enrichAcceptedFetchCmd(taskID int, disableLLM bool, jobs []Job, progressCh chan<- string, jobCh chan<- Job) tea.Cmd {
	return func() tea.Msg {
		if progressCh != nil {
			defer close(progressCh)
		}
		if jobCh != nil {
			defer close(jobCh)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		appCfg, err := config.LoadAppConfig(runtimeConfigPath)
		if err != nil {
			reportAcceptedFetchProgress(progressCh, "Config unavailable; enriching accepted jobs without LLM...")
		} else {
			appCfg = sessionFetchConfig(disableLLM, appCfg)
		}

		fetcher.ConfigureLLM(llmpkg.InitConfiguredLLMForTask, llmpkg.ExecuteLLMSearch, llmpkg.EnrichJobIdentityWithLLMUsage)
		fetcher.ConfigureLLMWebSearch(llmpkg.ExecuteLLMWebSearch)
		reportAcceptedFetchProgress(progressCh, "Enriching accepted jobs in the background...")
		enriched := fetcher.EnrichJobsFromApplyPagesWithConfigStoreAndProgress(ctx, append([]Job(nil), jobs...), appCfg, runtimeCompanyIdentityStore, func(message string) {
			reportAcceptedFetchProgress(progressCh, message)
		}, func(job Job) {
			if jobCh != nil {
				jobCh <- job
			}
		})

		return acceptedFetchEnrichedMsg{
			taskID: taskID,
			jobs:   enriched,
		}
	}
}
