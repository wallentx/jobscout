package fetcher

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
)

const (
	maxConcurrentRSSFetch        = 6
	maxConcurrentAPIFetch        = 4
	maxConcurrentSiteSearchFetch = 4
	maxConcurrentBrowserSearch   = 1
)

func fetchAllJobs(ctx context.Context, appCfg *AppConfig, criteriaCfg *CriteriaConfig, progress func(string), existingJobs ...[]Job) ([]Job, FetchSummary, error) {
	fetchStart := time.Now()
	var allFetched []Job
	summary := newFetchSummary()
	var existing []Job
	if len(existingJobs) > 0 {
		existing = existingJobs[0]
	}
	existingIndex := newExistingJobIndex(existing)
	defer func() {
		logDebug(
			"fetch complete: total=%d filtered=%d rejected=%d notices=%d duration=%s",
			len(allFetched),
			countFilteredJobs(summary.Filtered),
			countRejectedSearchSummary(summary.Rejected),
			len(summary.Notices),
			time.Since(fetchStart).Round(time.Millisecond),
		)
	}()
	if appCfg == nil {
		for _, searchType := range fetchSearchKinds {
			setFetchSearchStatus(&summary, searchType, "config unavailable")
		}
		return allFetched, summary, nil
	}
	if err := refreshLinkedInCriteriaHints(ctx, criteriaCfg); err != nil {
		logDebug("linkedin criteria hint refresh failed: %v", err)
	}
	urlVisits := beginURLVisitRun()
	sourceProfiles := beginSourceProfileRun()
	defer func() {
		fetches, hits, htmlEntries, probeEntries := urlVisits.Stats()
		logDebug("url visit registry summary: fetches=%d hits=%d html_entries=%d probe_entries=%d", fetches, hits, htmlEntries, probeEntries)
	}()
	effectiveSources := resolveEffectiveSources(appCfg, criteriaCfg)
	logDebug(
		"fetch start: llm_enabled=%t llm_job_search=%t llm_job_filtering=%t llm_company_health=%t sources_enabled=%t rss_enabled=%t rss_feeds=%d api_sources=%d site_enabled=%t site_targets=%d llm_web_enabled=%t llm_web_targets=%d",
		appCfg.LLM.Enabled,
		appCfg.LLM.JobSearch,
		appCfg.LLM.JobFiltering,
		appCfg.LLM.CompanyHealth,
		appCfg.Sources.Enabled,
		appCfg.Sources.RSS.Enabled,
		len(effectiveSources.RSSFeeds),
		len(effectiveSources.APISources),
		appCfg.Sources.SiteSearch.Enabled,
		len(effectiveSources.SiteTargets),
		appCfg.Sources.LLMWeb.Enabled,
		len(effectiveSources.LLMWebTargets),
	)
	logDebug("source resolution: role_families=%s", debugRoleFamilies(effectiveRoleFamilies(appCfg, criteriaCfg)))
	logDebug("source resolution: rss=%s", debugRSSSources(effectiveSources.RSSFeeds))
	logDebug("source resolution: api=%s", debugAPISources(effectiveSources.APISources))
	logDebug("source resolution: site_targets=%s", debugStringList(effectiveSources.SiteTargets))
	logDebug("source resolution: llm_web_targets=%s", debugStringList(effectiveSources.LLMWebTargets))

	var mu sync.Mutex

	// 1. Fetch via LLM job search when enabled.
	if appCfg.LLM.Enabled && appCfg.LLM.JobSearch {
		if fetchAllJobsInitConfiguredLLM == nil || fetchAllJobsExecuteLLMSearch == nil {
			mu.Lock()
			setFetchSearchStatus(&summary, fetchSearchLLM, "disabled: LLM job search runner unavailable")
			mu.Unlock()
		} else {
			reportFetchProgress(progress, "Preparing LLM job search...")
			llm, restoreAuth, initErr := fetchAllJobsInitConfiguredLLM(ctx, appCfg, llmTaskJobSearch)
			if initErr == nil {
				defer restoreAuth()
				promptBytes, err := fetchAllJobsReadFile(runtimeSearchPromptPath)
				if err != nil {
					mu.Lock()
					summary.Notices = append(summary.Notices, fmt.Sprintf("Could not read SEARCH_PROMPT.md: %v", err))
					setFetchSearchStatus(&summary, fetchSearchLLM, fmt.Sprintf("failed before execution: %v", err))
					mu.Unlock()
				} else {
					reportFetchProgress(progress, "Running LLM job search...")
					jobs, err := fetchAllJobsExecuteLLMSearch(ctx, llm, string(promptBytes))
					if err != nil {
						mu.Lock()
						summary.Notices = append(summary.Notices, fmt.Sprintf("LLM job search failed: %v", err))
						setFetchSearchStatus(&summary, fetchSearchLLM, fmt.Sprintf("execution failed: %v", err))
						mu.Unlock()
					} else {
						var dropped map[string][]string
						jobs, dropped = validateFetchedJobs(ctx, jobs)
						mu.Lock()
						summary.Rejected = mergeRejectedBySearch(summary.Rejected, fetchSearchLLM, dropped)
						for i := range jobs {
							switch {
							case strings.TrimSpace(jobs[i].Source) == "":
								jobs[i].Source = formatSearchSource(fetchSearchLLM, "llm_job_search")
							case !strings.HasPrefix(strings.ToLower(strings.TrimSpace(jobs[i].Source)), strings.ToLower(fetchSearchLLM)+":"):
								jobs[i].Source = formatSearchSource(fetchSearchLLM, jobs[i].Source)
							}
							jobs[i].Status = "Unopened"
							jobs[i].SetDateAdded(time.Now().Unix())
							allFetched = append(allFetched, jobs[i])
						}
						setFetchSearchStatus(&summary, fetchSearchLLM, formatExecutedSearchStatus(len(jobs), 0, countRejectedJobs(dropped)))
						mu.Unlock()
					}
				}
			} else {
				mu.Lock()
				summary.Notices = append(summary.Notices, fmt.Sprintf("Failed to init LLM for llm_job_search, falling back to configured sources: %v", initErr))
				setFetchSearchStatus(&summary, fetchSearchLLM, fmt.Sprintf("failed to initialize: %v", initErr))
				mu.Unlock()
			}
		}
	} else if !appCfg.LLM.Enabled {
		setFetchSearchStatus(&summary, fetchSearchLLM, "disabled in config")
	} else {
		setFetchSearchStatus(&summary, fetchSearchLLM, "LLM job search disabled in config")
	}

	if appCfg.Sources.LLMWeb.Enabled {
		switch {
		case !appCfg.LLM.Enabled:
			setFetchSearchStatus(&summary, fetchSearchLLMWeb, "disabled: LLM is disabled in config")
		case fetchAllJobsExecuteLLMWebSearch == nil && (fetchAllJobsInitConfiguredLLM == nil || fetchAllJobsExecuteLLMSearch == nil):
			setFetchSearchStatus(&summary, fetchSearchLLMWeb, "disabled: LLM search runner unavailable")
		case len(effectiveSources.LLMWebTargets) == 0:
			setFetchSearchStatus(&summary, fetchSearchLLMWeb, "enabled, but no llm_web targets were configured or resolved")
		default:
			prompt, queries := buildLLMWebSearchPrompt(criteriaCfg, effectiveSources.LLMWebTargets)
			if strings.TrimSpace(prompt) == "" {
				setFetchSearchStatus(&summary, fetchSearchLLMWeb, "enabled, but no llm_web queries could be built")
				break
			}
			logDebug("llm web search: running %d provider-web query prompts: %s", len(queries), debugStringList(queries))
			reportFetchProgress(progress, "Running LLM web search...")
			jobs, err := executeLLMWebSearchForFetch(ctx, appCfg, prompt)
			if err != nil {
				if isLLMSearchParseError(err) {
					setFetchSearchStatus(&summary, fetchSearchLLMWeb, "enabled, but provider did not return JSON; selected model may not support web search")
					logDebug("llm web search: provider returned non-JSON response: %v", err)
					break
				}
				mu.Lock()
				summary.Notices = append(summary.Notices, fmt.Sprintf("LLM web search failed: %v", err))
				setFetchSearchStatus(&summary, fetchSearchLLMWeb, fmt.Sprintf("execution failed: %v", err))
				mu.Unlock()
				break
			}

			var dropped map[string][]string
			var filtered map[string][]Job
			markLLMWebJobsForValidation(jobs)
			jobs, filtered = filterLLMWebJobsBeforeEnrichment(jobs, criteriaCfg)
			jobs = enrichLLMWebJobsBeforeValidation(ctx, appCfg, jobs, progress)
			jobs, dropped = validateFetchedJobs(ctx, jobs)
			mu.Lock()
			summary.Filtered = mergeFiltered(summary.Filtered, filtered)
			summary.Rejected = mergeRejectedBySearch(summary.Rejected, fetchSearchLLMWeb, dropped)
			for i := range jobs {
				switch {
				case strings.TrimSpace(jobs[i].Source) == "":
					jobs[i].Source = formatSearchSource(fetchSearchLLMWeb, "llm_web")
				case !strings.HasPrefix(strings.ToLower(strings.TrimSpace(jobs[i].Source)), strings.ToLower(fetchSearchLLMWeb)+":"):
					jobs[i].Source = formatSearchSource(fetchSearchLLMWeb, jobs[i].Source)
				}
				jobs[i].Status = "Unopened"
				jobs[i].SetDateAdded(time.Now().Unix())
				allFetched = append(allFetched, jobs[i])
			}
			setFetchSearchStatus(&summary, fetchSearchLLMWeb, formatExecutedSearchStatus(len(jobs), countFilteredJobs(filtered), countRejectedJobs(dropped)))
			mu.Unlock()
		}
	} else {
		setFetchSearchStatus(&summary, fetchSearchLLMWeb, "disabled in config")
	}

	// 2. Fetch configured non-LLM sources when enabled.
	if !appCfg.Sources.Enabled {
		mu.Lock()
		setFetchSearchStatus(&summary, fetchSearchRSS, "disabled in config")
		setFetchSearchStatus(&summary, fetchSearchAPI, "disabled in config")
		setFetchSearchStatus(&summary, fetchSearchSite, "disabled in config")
		mu.Unlock()
		return allFetched, summary, nil
	}

	var wg sync.WaitGroup

	// Fetch RSS
	wg.Add(1)
	go func() {
		defer wg.Done()
		if appCfg.Sources.RSS.Enabled {
			if len(effectiveSources.RSSFeeds) == 0 {
				mu.Lock()
				setFetchSearchStatus(&summary, fetchSearchRSS, "enabled, but no RSS feeds were configured or resolved")
				mu.Unlock()
			} else {
				logDebug("rss search: fetching %d feeds with concurrency=%d", len(effectiveSources.RSSFeeds), maxConcurrentRSSFetch)
				results := make([]sourceFetchResult, len(effectiveSources.RSSFeeds))
				runBounded(ctx, len(effectiveSources.RSSFeeds), maxConcurrentRSSFetch, func(i int) {
					source := effectiveSources.RSSFeeds[i]
					reportFetchProgress(progress, "Checking RSS feed: %s", source.Name)
					jobs, filtered, err := fetchRSS(ctx, source, criteriaCfg)
					if err != nil {
						results[i].err = err
						results[i].notice = fmt.Sprintf("Failed to fetch RSS %s: %v", source.Name, err)
						return
					}
					validated, dropped := validateFetchedJobs(ctx, jobs)
					results[i] = newSourceFetchResult(validated, filtered, dropped)
				})

				rssCount := 0
				rssFiltered := 0
				rssRejected := 0
				rssErrors := 0
				for _, result := range results {
					if result.err != nil {
						if result.notice != "" {
							mu.Lock()
							summary.Notices = append(summary.Notices, result.notice)
							mu.Unlock()
						}
						rssErrors++
						continue
					}
					mu.Lock()
					summary.Filtered = mergeFiltered(summary.Filtered, result.filtered)
					summary.Rejected = mergeRejectedBySearch(summary.Rejected, fetchSearchRSS, result.dropped)
					rssCount += len(result.jobs)
					rssFiltered += countFilteredJobs(result.filtered)
					rssRejected += countRejectedJobs(result.dropped)
					allFetched = append(allFetched, result.jobs...)
					mu.Unlock()
				}
				status := formatExecutedSearchStatus(rssCount, rssFiltered, rssRejected)
				if rssErrors > 0 {
					status += fmt.Sprintf("; %d source errors", rssErrors)
				}
				mu.Lock()
				setFetchSearchStatus(&summary, fetchSearchRSS, status)
				mu.Unlock()
			}
		} else {
			mu.Lock()
			setFetchSearchStatus(&summary, fetchSearchRSS, "disabled in config")
			mu.Unlock()
		}
	}()

	// Fetch configured source hooks.
	wg.Add(1)
	go func() {
		defer wg.Done()
		apiSourceCount := 0
		apiCount := 0
		apiFiltered := 0
		apiRejected := 0
		apiErrors := 0
		apiSources := make([]APISource, 0, len(effectiveSources.APISources))
		for _, source := range effectiveSources.APISources {
			if source.Enabled {
				apiSources = append(apiSources, source)
			}
		}
		apiSourceCount = len(apiSources)
		logDebug("api search: fetching %d sources with concurrency=%d", len(apiSources), maxConcurrentAPIFetch)
		results := make([]sourceFetchResult, len(apiSources))
		runBounded(ctx, len(apiSources), maxConcurrentAPIFetch, func(i int) {
			source := apiSources[i]
			reportFetchProgress(progress, "Checking configured source: %s", source.Name)
			jobs, filtered, err := fetchJobsFromAPISource(ctx, source, criteriaCfg)
			if err != nil {
				results[i].err = err
				results[i].notice = fmt.Sprintf("Failed to fetch configured source %s: %v", source.Name, err)
				return
			}
			validated, dropped := validateFetchedJobs(ctx, jobs)
			results[i] = newSourceFetchResult(validated, filtered, dropped)
		})

		for _, result := range results {
			if result.err != nil {
				if result.notice != "" {
					mu.Lock()
					summary.Notices = append(summary.Notices, result.notice)
					mu.Unlock()
				}
				apiErrors++
				continue
			}
			mu.Lock()
			summary.Filtered = mergeFiltered(summary.Filtered, result.filtered)
			summary.Rejected = mergeRejectedBySearch(summary.Rejected, fetchSearchAPI, result.dropped)
			apiCount += len(result.jobs)
			apiFiltered += countFilteredJobs(result.filtered)
			apiRejected += countRejectedJobs(result.dropped)
			allFetched = append(allFetched, result.jobs...)
			mu.Unlock()
		}

		mu.Lock()
		switch {
		case len(appCfg.Sources.APIs) == 0:
			setFetchSearchStatus(&summary, fetchSearchAPI, "enabled, but no configured sources were available")
		case apiSourceCount == 0:
			setFetchSearchStatus(&summary, fetchSearchAPI, "enabled, but all configured sources were disabled")
		default:
			status := formatExecutedSearchStatus(apiCount, apiFiltered, apiRejected)
			if apiErrors > 0 {
				status += fmt.Sprintf("; %d source errors", apiErrors)
			}
			setFetchSearchStatus(&summary, fetchSearchAPI, status)
		}
		mu.Unlock()
	}()

	// Fetch site-search sources
	wg.Add(1)
	go func() {
		defer wg.Done()
		if appCfg.Sources.SiteSearch.Enabled {
			if len(effectiveSources.SiteTargets) == 0 {
				mu.Lock()
				setFetchSearchStatus(&summary, fetchSearchSite, "enabled, but no site-search targets were configured or resolved")
				mu.Unlock()
				return
			}
			var siteBrowser *rod.Browser
			var closeBrowser func()
			var siteBrowserMu sync.Mutex
			defer func() {
				if closeBrowser != nil {
					logDebug("site search: closing browser")
					closeBrowser()
				}
			}()
			ensureSiteBrowser := func() (*rod.Browser, error) {
				siteBrowserMu.Lock()
				defer siteBrowserMu.Unlock()
				if siteBrowser != nil {
					return siteBrowser, nil
				}
				logDebug("site search: launching browser for dynamic target probing")
				browser, cleanup, err := newSiteSearchBrowser()
				if err != nil {
					logDebug("site search: browser launch failed: %v", err)
					return nil, err
				}
				logDebug("site search: browser launched")
				siteBrowser = browser
				closeBrowser = cleanup
				return siteBrowser, nil
			}
			siteCount := 0
			siteFiltered := 0
			siteRejected := 0
			siteErrors := 0
			logDebug("site search: evaluating %d resolved targets", len(effectiveSources.SiteTargets))
			tasks := make([]siteSearchTask, 0, len(effectiveSources.SiteTargets))
			for i, site := range effectiveSources.SiteTargets {
				if strings.TrimSpace(site) == "" {
					continue
				}

				sourceName := formatSearchSource(fetchSearchSite, strings.TrimSpace(site))
				targetURLs := siteSearchURLsForCriteria(site, criteriaCfg)
				logDebug("site search target %d/%d %s: target URLs %s", i+1, len(effectiveSources.SiteTargets), site, debugStringList(targetURLs))
				if len(targetURLs) == 0 {
					logDebug("site search %s: skipped because no searchable target URL could be resolved", site)
					continue
				}
				for queryIdx, targetURL := range targetURLs {
					tasks = append(tasks, siteSearchTask{
						site:       site,
						sourceName: sourceName,
						targetURL:  targetURL,
						queryIndex: queryIdx,
						queryCount: len(targetURLs),
					})
				}
			}
			logDebug("site search: running %d query URLs with concurrency=%d browser_concurrency=%d", len(tasks), maxConcurrentSiteSearchFetch, maxConcurrentBrowserSearch)
			results := make([]sourceFetchResult, len(tasks))
			browserSem := make(chan struct{}, maxConcurrentBrowserSearch)
			builtInDetails := newBuiltInDetailCache()
			builtInProfiles := sourceProfiles
			blockedSiteSearches := newSiteSearchBlocklist()
			runBounded(ctx, len(tasks), maxConcurrentSiteSearchFetch, func(i int) {
				task := tasks[i]
				reportFetchProgress(progress, "Searching site target: %s", strings.TrimSpace(task.site))
				results[i] = fetchSiteSearchTask(ctx, task, criteriaCfg, ensureSiteBrowser, browserSem, builtInDetails, builtInProfiles, existingIndex, blockedSiteSearches)
			})

			for i, result := range results {
				if result.err != nil {
					if result.notice != "" {
						mu.Lock()
						summary.Notices = append(summary.Notices, result.notice)
						mu.Unlock()
					}
					siteErrors++
					continue
				}
				if i >= len(tasks) {
					continue
				}
				mu.Lock()
				summary.Filtered = mergeFiltered(summary.Filtered, result.filtered)
				summary.Rejected = mergeRejectedBySearch(summary.Rejected, fetchSearchSite, result.dropped)
				siteCount += len(result.jobs)
				siteFiltered += countFilteredJobs(result.filtered)
				siteRejected += countRejectedJobs(result.dropped)
				allFetched = append(allFetched, result.jobs...)
				mu.Unlock()
			}
			status := formatExecutedSearchStatus(siteCount, siteFiltered, siteRejected)
			if siteErrors > 0 {
				status += fmt.Sprintf("; %d target errors", siteErrors)
			}
			mu.Lock()
			setFetchSearchStatus(&summary, fetchSearchSite, status)
			mu.Unlock()
		} else {
			mu.Lock()
			setFetchSearchStatus(&summary, fetchSearchSite, "disabled in config")
			mu.Unlock()
		}
	}()

	wg.Wait()

	var skippedExisting []Job
	allFetched, skippedExisting = skipExistingFetchedJobs(allFetched, existingIndex)
	if len(skippedExisting) > 0 {
		mu.Lock()
		if summary.Filtered == nil {
			summary.Filtered = make(map[string][]Job)
		}
		summary.Filtered["already saved"] = append(summary.Filtered["already saved"], skippedExisting...)
		mu.Unlock()
		logDebug("fetch finalization: skipped %d already saved fetched jobs before dedupe/LLM filtering", len(skippedExisting))
	}

	var duplicates []Job
	allFetched, duplicates = dedupeFetchedJobs(allFetched)
	if len(duplicates) > 0 {
		mu.Lock()
		if summary.Filtered == nil {
			summary.Filtered = make(map[string][]Job)
		}
		summary.Filtered["duplicate"] = append(summary.Filtered["duplicate"], duplicates...)
		mu.Unlock()
		logDebug("fetch finalization: deduped %d duplicate fetched jobs before review/LLM filtering", len(duplicates))
	}

	return allFetched, summary, nil
}

func executeLLMWebSearchForFetch(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error) {
	if fetchAllJobsExecuteLLMWebSearch != nil {
		return fetchAllJobsExecuteLLMWebSearch(ctx, appCfg, prompt)
	}
	llm, restoreAuth, err := fetchAllJobsInitConfiguredLLM(ctx, appCfg, llmTaskJobSearch)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}
	defer restoreAuth()
	return fetchAllJobsExecuteLLMSearch(ctx, llm, prompt)
}

func markLLMWebJobsForValidation(jobs []Job) {
	for i := range jobs {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(jobs[i].Source)), strings.ToLower(fetchSearchLLMWeb)+":") {
			continue
		}
		jobs[i].Source = formatSearchSource(fetchSearchLLMWeb, jobs[i].Source)
	}
}

func filterLLMWebJobsBeforeEnrichment(jobs []Job, criteria *CriteriaConfig) ([]Job, map[string][]Job) {
	if len(jobs) == 0 {
		return jobs, nil
	}
	kept := make([]Job, 0, len(jobs))
	filtered := make(map[string][]Job)
	for _, job := range jobs {
		if reason := filterJobReason(&job, criteria); reason != "" {
			logDebug("llm web search: pre-enrichment filtered %s - %s: %s", job.Company, job.Title, reason)
			filtered[reason] = append(filtered[reason], job)
			continue
		}
		kept = append(kept, job)
	}
	if len(filtered) == 0 {
		return jobs, nil
	}
	logDebug("llm web search: pre-enrichment filtered %d/%d jobs", countFilteredJobs(filtered), len(jobs))
	return kept, filtered
}

func enrichLLMWebJobsBeforeValidation(ctx context.Context, appCfg *AppConfig, jobs []Job, progress func(string)) []Job {
	if len(jobs) == 0 {
		return jobs
	}

	indexes := make([]int, 0, len(jobs))
	candidates := make([]Job, 0, len(jobs))
	for i, job := range jobs {
		if !llmWebJobNeedsPreValidationIdentity(job) {
			continue
		}
		indexes = append(indexes, i)
		candidates = append(candidates, job)
	}
	if len(candidates) == 0 {
		return jobs
	}

	start := time.Now()
	logDebug("llm web search: repairing identity for %d/%d jobs before validation", len(candidates), len(jobs))
	enriched := enrichJobsFromApplyPagesForValidation(ctx, candidates, progress)
	repaired := 0
	fallbackIndexes := make([]int, 0, len(enriched))
	fallbackCandidates := make([]Job, 0, len(enriched))
	for i, job := range enriched {
		if jobHasRequiredCompanyIdentity(job) {
			repaired++
			continue
		}
		fallbackIndexes = append(fallbackIndexes, i)
		fallbackCandidates = append(fallbackCandidates, job)
	}
	logDebug(
		"llm web search: fast identity repair checked %d jobs; repaired=%d fallback=%d duration=%s",
		len(enriched),
		repaired,
		len(fallbackCandidates),
		time.Since(start).Round(time.Millisecond),
	)

	if len(fallbackCandidates) > 0 {
		fallbackStart := time.Now()
		logDebug("llm web search: running full identity enrichment fallback for %d jobs", len(fallbackCandidates))
		fallbackEnriched := EnrichJobsFromApplyPagesWithConfigAndProgress(ctx, fallbackCandidates, appCfg, progress, nil)
		for i, idx := range fallbackIndexes {
			enriched[idx] = fallbackEnriched[i]
		}
		logDebug("llm web search: full identity enrichment fallback finished in %s", time.Since(fallbackStart).Round(time.Millisecond))
	}

	for i, idx := range indexes {
		jobs[idx] = enriched[i]
	}
	return jobs
}

func llmWebJobNeedsPreValidationIdentity(job Job) bool {
	if !isLLMGeneratedJob(job) {
		return false
	}
	if strings.TrimSpace(job.ApplyURL) == "" || isKnownNonJobApplyURL(job.ApplyURL) {
		return false
	}
	return !jobHasRequiredCompanyIdentity(job)
}

func isLLMSearchParseError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "failed to parse LLM JSON output")
}

func FetchAllJobs(ctx context.Context, appCfg *AppConfig, criteriaCfg *CriteriaConfig, progress func(string)) ([]Job, FetchSummary, error) {
	return fetchAllJobs(ctx, appCfg, criteriaCfg, progress)
}

func FetchAllJobsSkippingExisting(ctx context.Context, appCfg *AppConfig, criteriaCfg *CriteriaConfig, existingJobs []Job, progress func(string)) ([]Job, FetchSummary, error) {
	return fetchAllJobs(ctx, appCfg, criteriaCfg, progress, existingJobs)
}

type sourceFetchResult struct {
	jobs     []Job
	filtered map[string][]Job
	dropped  map[string][]string
	err      error
	notice   string
}

type siteSearchTask struct {
	site       string
	sourceName string
	targetURL  string
	queryIndex int
	queryCount int
}

func newSourceFetchResult(jobs []Job, filtered map[string][]Job, dropped map[string][]string) sourceFetchResult {
	return sourceFetchResult{
		jobs:     jobs,
		filtered: filtered,
		dropped:  dropped,
	}
}

type siteSearchBlocklist struct {
	mu      sync.Mutex
	blocked map[string]string
}

func newSiteSearchBlocklist() *siteSearchBlocklist {
	return &siteSearchBlocklist{blocked: make(map[string]string)}
}

func (b *siteSearchBlocklist) reasonFor(rawURL string) string {
	if b == nil {
		return ""
	}
	host := normalizedSiteSearchHost(rawURL)
	if host == "" {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.blocked[host]
}

func (b *siteSearchBlocklist) block(rawURL string, reason string) bool {
	if b == nil {
		return false
	}
	host := normalizedSiteSearchHost(rawURL)
	if host == "" {
		return false
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "blocked"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.blocked[host]; exists {
		return false
	}
	b.blocked[host] = reason
	return true
}

func normalizedSiteSearchHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}

func fetchSiteSearchTask(ctx context.Context, task siteSearchTask, criteria *CriteriaConfig, ensureBrowser func() (*rod.Browser, error), browserSem chan struct{}, builtInDetails *builtInDetailCache, builtInProfiles *sourceProfileEnricher, existing *existingJobIndex, blocked *siteSearchBlocklist) sourceFetchResult {
	var jobs []Job
	var filtered map[string][]Job
	var err error
	handled := false
	logDebug("site search %s query %d/%d: target URL %s", task.site, task.queryIndex+1, task.queryCount, task.targetURL)
	if reason := blocked.reasonFor(task.targetURL); reason != "" {
		logDebug("site search %s query %d/%d: skipped because host was blocked earlier in this fetch: %s", task.site, task.queryIndex+1, task.queryCount, reason)
		return sourceFetchResult{}
	}
	logDebug("site search %s query %d/%d: trying static handler", task.site, task.queryIndex+1, task.queryCount)
	jobs, filtered, handled, err = fetchBuiltInSiteSearch(ctx, task.targetURL, task.sourceName, criteria, builtInDetails, builtInProfiles, existing)
	if handled {
		logDebug("site search %s query %d/%d: static handler completed with %d accepted and %d filtered", task.site, task.queryIndex+1, task.queryCount, len(jobs), countFilteredJobs(filtered))
	} else if err != nil {
		logDebug("site search %s query %d/%d: static handler failed: %v", task.site, task.queryIndex+1, task.queryCount, err)
	} else {
		logDebug("site search %s query %d/%d: static handler did not handle target", task.site, task.queryIndex+1, task.queryCount)
	}
	if !handled && err == nil {
		select {
		case browserSem <- struct{}{}:
			defer func() {
				<-browserSem
			}()
		case <-ctx.Done():
			err = ctx.Err()
		}
		if err == nil {
			if reason := blocked.reasonFor(task.targetURL); reason != "" {
				logDebug("site search %s query %d/%d: skipped browser probe because host was blocked while waiting: %s", task.site, task.queryIndex+1, task.queryCount, reason)
				return sourceFetchResult{}
			}
			var browser *rod.Browser
			browser, err = ensureBrowser()
			if err == nil {
				jobs, filtered, err = fetchGenericSiteSearch(ctx, browser, task.site, task.targetURL, task.sourceName, criteria)
				if errors.Is(err, errSiteSearchVerificationRequired) {
					if blocked.block(task.targetURL, err.Error()) {
						logDebug("site search %s query %d/%d: blocked remaining searches for host after verification page: %v", task.site, task.queryIndex+1, task.queryCount, err)
					}
					return sourceFetchResult{}
				}
			}
		}
	}
	if err != nil {
		return sourceFetchResult{
			err:    err,
			notice: fmt.Sprintf("Failed to search site %s: %v", task.site, err),
		}
	}
	validated, dropped := validateFetchedJobs(ctx, jobs)
	validated, skippedExisting := skipExistingFetchedJobs(validated, existing)
	if len(skippedExisting) > 0 {
		if filtered == nil {
			filtered = make(map[string][]Job)
		}
		filtered["already saved"] = append(filtered["already saved"], skippedExisting...)
		logDebug("site search %s query %d/%d: skipped %d already saved jobs after validation", task.site, task.queryIndex+1, task.queryCount, len(skippedExisting))
	}
	return newSourceFetchResult(validated, filtered, dropped)
}

func runBounded(ctx context.Context, total int, limit int, work func(int)) {
	if total <= 0 {
		return
	}
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() {
				<-sem
			}()
			work(i)
		}(i)
	}
	wg.Wait()
}
