package tuiapp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/fetcher"
	llmpkg "github.com/wallentx/jobscout/internal/llm"

	tea "github.com/charmbracelet/bubbletea"
)

const llmJobFilteringReason = "llm job filtering"
const llmJobFilteringTimeout = 180 * time.Second

var filterJobsWithLLM = llmpkg.FilterJobsWithLLM

func sessionFetchConfig(disableLLM bool, appCfg *AppConfig) *AppConfig {
	if appCfg == nil {
		return nil
	}
	cfgCopy := *appCfg
	config.ApplyFetchSourceSelection(&cfgCopy, runtimeSourceSelection)
	if disableLLM {
		cfgCopy.LLM.Enabled = false
		cfgCopy.LLM.JobSearch = false
		cfgCopy.LLM.JobFiltering = false
		cfgCopy.LLM.CompanyHealth = false
	}
	return &cfgCopy
}

func fetchStartMessage(disableLLM bool) string {
	appCfg, err := config.LoadAppConfig(runtimeConfigPath)
	if err != nil {
		return "Fetching jobs with your current configuration..."
	}
	appCfg = sessionFetchConfig(disableLLM, appCfg)
	if disableLLM {
		if appCfg.Sources.Enabled {
			return "Starting configured source search with LLM disabled for this session..."
		}
		return "LLM is disabled for this session, and no non-LLM source is enabled."
	}

	if appCfg.LLM.Enabled && appCfg.LLM.JobSearch {
		provider := appCfg.LLM.Provider
		model := appCfg.LLM.Model
		if appCfg.Sources.Enabled {
			if provider != "" && model != "" {
				return fmt.Sprintf("Starting LLM job search via %s (%s) plus configured sources...", provider, model)
			}
			if provider != "" {
				return fmt.Sprintf("Starting LLM job search via %s plus configured sources...", provider)
			}
			return "Starting LLM job search plus configured sources..."
		}
		if provider != "" && model != "" {
			return fmt.Sprintf("Starting LLM job search prompt via %s (%s)...", provider, model)
		}
		if provider != "" {
			return fmt.Sprintf("Starting LLM job search prompt via %s...", provider)
		}
		return "Starting LLM job search prompt..."
	}

	if appCfg.Sources.Enabled {
		if appCfg.LLM.Enabled && appCfg.LLM.JobFiltering {
			return "Starting configured source search with LLM job filtering..."
		}
		return "Starting configured source search..."
	}

	return "No fetch source is enabled in config. Attempting fetch..."
}

func formatFetchSummary(fetched int, added int, addedEntries []string, notices []string, rejected map[string][]string) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("Fetched %d results. Added %d new jobs.", fetched, added))

	if len(addedEntries) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Added")
		for _, entry := range addedEntries {
			lines = append(lines, "  + "+entry)
		}
	}

	if len(rejected) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Rejected")
		for _, reason := range sortedRejectedReasons(rejected) {
			entries := rejected[reason]
			lines = append(lines, fmt.Sprintf("  %s (%d)", reason, len(entries)))
			for _, entry := range entries {
				lines = append(lines, "    - "+entry)
			}
		}
	}

	if len(notices) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Notes")
		for _, notice := range notices {
			lines = append(lines, "  "+notice)
		}
	}

	return strings.Join(lines, "\n")
}

func flattenRejectedSummary(rejected map[string]map[string][]string) map[string][]string {
	if len(rejected) == 0 {
		return nil
	}
	flat := make(map[string][]string)
	for _, grouped := range rejected {
		for reason, entries := range grouped {
			flat[reason] = append(flat[reason], entries...)
		}
	}
	return flat
}

func sortedRejectedReasons(rejected map[string][]string) []string {
	reasons := make([]string, 0, len(rejected))
	for reason := range rejected {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	return reasons
}

func summarizeJobs(jobs []Job) []string {
	out := make([]string, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, fmt.Sprintf("%s - %s", job.Company, job.Title))
	}
	return out
}

func previewFetchCmd(appCfg AppConfig, criteriaCfg *CriteriaConfig) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		config.ApplyFetchSourceSelection(&appCfg, runtimeSourceSelection)
		newJobs, summary, err := fetchAllJobs(ctx, &appCfg, criteriaCfg, nil, nil)
		if err != nil {
			return setupPreviewMsg{err: err}
		}

		newJobs, notices := applyOptionalLLMJobFilteringWithFreshTimeout(&appCfg, criteriaCfg, newJobs)
		summary.Notices = append(summary.Notices, notices...)

		return setupPreviewMsg{jobs: newJobs, notices: summary.Notices, rejected: flattenRejectedSummary(summary.Rejected)}
	}
}

func applyOptionalLLMJobFiltering(ctx context.Context, appCfg *AppConfig, criteriaCfg *CriteriaConfig, jobs []Job) ([]Job, []string) {
	return filterJobsWithLLM(ctx, appCfg, criteriaCfg, jobs)
}

func applyOptionalLLMJobFilteringWithFreshTimeout(appCfg *AppConfig, criteriaCfg *CriteriaConfig, jobs []Job) ([]Job, []string) {
	ctx, cancel := context.WithTimeout(context.Background(), llmJobFilteringTimeout)
	defer cancel()
	return applyOptionalLLMJobFiltering(ctx, appCfg, criteriaCfg, jobs)
}

func fetchAllJobs(ctx context.Context, appCfg *AppConfig, criteriaCfg *CriteriaConfig, existingJobs []Job, progress func(string)) ([]Job, FetchSummary, error) {
	fetcher.ConfigureLLM(llmpkg.InitConfiguredLLMForTask, llmpkg.ExecuteLLMSearch, llmpkg.EnrichJobIdentityWithLLMUsage)
	fetcher.ConfigureLLMWebSearch(llmpkg.ExecuteLLMWebSearch)
	return fetcher.FetchAllJobsSkippingExisting(ctx, appCfg, criteriaCfg, existingJobs, progress)
}

func setupPreviewNotice(existingCount int, previewCount int, added int) string {
	switch {
	case previewCount == 0 && existingCount > 0:
		return fmt.Sprintf("Setup saved. No preview jobs were returned, so %d saved jobs were left unchanged.", existingCount)
	case previewCount == 0:
		return "Setup saved. No preview jobs were returned, so the database was left unchanged."
	case added == 0:
		if existingCount > 0 {
			return fmt.Sprintf("Setup saved. All %d temporary preview jobs were already present in your %d saved jobs, so nothing new was added.", previewCount, existingCount)
		}
		return fmt.Sprintf("Setup saved. All %d temporary preview jobs collapsed into duplicates, so nothing new was written.", previewCount)
	default:
		skipped := previewCount - added
		if existingCount > 0 {
			if skipped > 0 {
				return fmt.Sprintf("Setup saved. Merged %d new jobs from %d temporary preview jobs into %d saved jobs. %d duplicates were skipped.", added, previewCount, existingCount, skipped)
			}
			return fmt.Sprintf("Setup saved. Merged %d new jobs from the temporary preview list into %d saved jobs.", added, existingCount)
		}
		if skipped > 0 {
			return fmt.Sprintf("Setup saved. Added %d jobs from %d temporary preview jobs. %d duplicates were skipped.", added, previewCount, skipped)
		}
		return fmt.Sprintf("Setup saved. Added %d jobs from the temporary preview list.", added)
	}
}

func waitForFetchProgress(ch <-chan string) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		text, ok := <-ch
		return fetchJobsProgressMsg{
			text: text,
			ch:   ch,
			done: !ok,
		}
	}
}

func fetchJobsCmd(disableLLM bool, existingJobs []Job, progressCh chan<- string) tea.Cmd {
	return func() tea.Msg {
		fetchStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()
		defer close(progressCh)

		appCfg, err := config.LoadAppConfig(runtimeConfigPath)
		if err != nil {
			return fetchJobsMsg{err: fmt.Errorf("failed to load config: %v", err)}
		}
		appCfg = sessionFetchConfig(disableLLM, appCfg)

		criteriaCfg, err := config.LoadCriteriaConfig(runtimeConfigPath)
		if err != nil {
			criteriaCfg = nil // fail open
		}

		sourceFetchStart := time.Now()
		newJobs, summary, err := fetchAllJobs(ctx, appCfg, criteriaCfg, existingJobs, func(message string) {
			if strings.TrimSpace(message) == "" {
				return
			}
			select {
			case progressCh <- message:
			default:
			}
		})
		if err != nil {
			return fetchJobsMsg{err: err}
		}
		logDebug(
			"fetch finalization: source fetch complete jobs=%d filtered=%d rejected=%d notices=%d duration=%s",
			len(newJobs),
			countFetchSummaryFiltered(summary.Filtered),
			countFetchSummaryRejected(summary.Rejected),
			len(summary.Notices),
			time.Since(sourceFetchStart).Round(time.Millisecond),
		)

		if llmJobFilteringShouldRun(appCfg, newJobs) {
			select {
			case progressCh <- "Running LLM job filtering before review...":
			default:
			}
		} else {
			select {
			case progressCh <- "Preparing fetch results for review...":
			default:
			}
		}
		beforeLLMFilter := append([]Job(nil), newJobs...)
		recordLLMJobFilteringBypassReasons(appCfg, criteriaCfg, &summary, beforeLLMFilter)
		llmFilterStart := time.Now()
		newJobs, notices := applyOptionalLLMJobFilteringWithFreshTimeout(appCfg, criteriaCfg, newJobs)
		recordLLMJobFilteringOutcome(appCfg, &summary, beforeLLMFilter, newJobs, notices)
		summary.Notices = append(summary.Notices, notices...)
		logDebug(
			"fetch finalization: llm filtering complete before=%d after=%d dropped=%d notices=%d duration=%s",
			len(beforeLLMFilter),
			len(newJobs),
			len(beforeLLMFilter)-len(newJobs),
			len(notices),
			time.Since(llmFilterStart).Round(time.Millisecond),
		)
		newJobs = removeExistingJobsBeforeReview(newJobs, existingJobs, &summary)
		logDebug("fetch finalization: review handoff jobs=%d total_duration=%s", len(newJobs), time.Since(fetchStart).Round(time.Millisecond))

		return fetchJobsMsg{
			jobs:    newJobs,
			summary: summary,
		}
	}
}

func removeExistingJobsBeforeReview(newJobs []Job, existingJobs []Job, summary *FetchSummary) []Job {
	kept, skipped := fetcher.SkipExistingFetchedJobs(newJobs, existingJobs)
	kept, mergeSkipped := skipExistingJobsByMergeKey(kept, existingJobs)
	skipped = append(skipped, mergeSkipped...)
	if len(skipped) == 0 {
		return kept
	}
	if summary != nil {
		if summary.Filtered == nil {
			summary.Filtered = make(map[string][]Job)
		}
		summary.Filtered["already saved"] = append(summary.Filtered["already saved"], skipped...)
	}
	logDebug("fetch finalization: skipped %d already saved fetched jobs before review handoff", len(skipped))
	return kept
}

func skipExistingJobsByMergeKey(jobs []Job, existingJobs []Job) ([]Job, []Job) {
	if len(jobs) == 0 || len(existingJobs) == 0 {
		return jobs, nil
	}
	existingKeys := make(map[string]struct{}, len(existingJobs))
	for _, job := range existingJobs {
		key := domain.JobMergeKey(job)
		if key == "|" {
			continue
		}
		existingKeys[key] = struct{}{}
	}
	if len(existingKeys) == 0 {
		return jobs, nil
	}

	kept := make([]Job, 0, len(jobs))
	skipped := make([]Job, 0)
	for _, job := range jobs {
		key := domain.JobMergeKey(job)
		if _, ok := existingKeys[key]; ok {
			skipped = append(skipped, job)
			continue
		}
		kept = append(kept, job)
	}
	return kept, skipped
}

func countFetchSummaryFiltered(filtered map[string][]Job) int {
	total := 0
	for _, jobs := range filtered {
		total += len(jobs)
	}
	return total
}

func countFetchSummaryRejected(rejected map[string]map[string][]string) int {
	total := 0
	for _, reasons := range rejected {
		for _, entries := range reasons {
			total += len(entries)
		}
	}
	return total
}

func reportAcceptedFetchProgress(progressCh chan<- string, message string) {
	if strings.TrimSpace(message) == "" || progressCh == nil {
		return
	}
	select {
	case progressCh <- message:
	default:
	}
}

func recordLLMJobFilteringOutcome(appCfg *AppConfig, summary *FetchSummary, before []Job, after []Job, notices []string) {
	if !llmJobFilteringShouldRun(appCfg, before) {
		return
	}

	dropped := jobsDroppedByLLMFilter(before, after)
	logDebug("LLM job filtering kept %d of %d fetched jobs; dropped %d", len(after), len(before), len(dropped))
	if len(notices) > 0 || len(dropped) == 0 {
		return
	}
	if summary.Filtered == nil {
		summary.Filtered = make(map[string][]Job)
	}
	summary.Filtered[llmJobFilteringReason] = append(summary.Filtered[llmJobFilteringReason], dropped...)
}

func recordLLMJobFilteringBypassReasons(appCfg *AppConfig, criteriaCfg *CriteriaConfig, summary *FetchSummary, jobs []Job) {
	if summary == nil || !llmJobFilteringShouldRun(appCfg, jobs) {
		return
	}
	bypassed := llmpkg.JobFilterBypassGroups(appCfg, criteriaCfg, jobs)
	if len(bypassed) == 0 {
		return
	}
	if summary.LLMFilterBypass == nil {
		summary.LLMFilterBypass = make(map[string][]Job, len(bypassed))
	}
	total := 0
	for reason, entries := range bypassed {
		summary.LLMFilterBypass[reason] = append(summary.LLMFilterBypass[reason], entries...)
		total += len(entries)
	}
	logDebug("LLM job filtering bypassed %d jobs before review; reasons=%s", total, formatJobCountMap(bypassed))
}

func llmJobFilteringShouldRun(appCfg *AppConfig, jobs []Job) bool {
	return appCfg != nil &&
		appCfg.LLM.Enabled &&
		appCfg.LLM.JobFiltering &&
		len(jobs) > 0
}

func jobsDroppedByLLMFilter(before []Job, after []Job) []Job {
	remaining := make(map[string]int, len(after))
	for _, job := range after {
		remaining[domain.JobMergeKey(job)]++
	}

	dropped := make([]Job, 0, len(before))
	for _, job := range before {
		key := domain.JobMergeKey(job)
		if remaining[key] > 0 {
			remaining[key]--
			continue
		}
		dropped = append(dropped, job)
	}
	return dropped
}

func formatJobCountMap(groups map[string][]Job) string {
	if len(groups) == 0 {
		return "<none>"
	}
	reasons := make([]string, 0, len(groups))
	for reason := range groups {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		parts = append(parts, fmt.Sprintf("%s=%d", reason, len(groups[reason])))
	}
	return strings.Join(parts, ", ")
}
