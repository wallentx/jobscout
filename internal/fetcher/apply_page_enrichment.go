package fetcher

import (
	"context"
	"strings"
	"sync"
	"time"
)

func enrichJobFromApplyPageWithLLM(ctx context.Context, job *Job, llmEnrich jobIdentityPageEnrichFunc) {
	enrichJobFromApplyPageWithProfiles(ctx, job, llmEnrich, newSourceProfileEnricher())
}

func enrichJobFromApplyPageWithProfiles(ctx context.Context, job *Job, llmEnrich jobIdentityPageEnrichFunc, profileEnricher *sourceProfileEnricher) {
	enrichJobFromApplyPageWithProfilesAndStats(ctx, job, llmEnrich, profileEnricher, nil)
}

func enrichJobFromApplyPageWithProfilesAndStats(ctx context.Context, job *Job, llmEnrich jobIdentityPageEnrichFunc, profileEnricher *sourceProfileEnricher, stats *acceptedEnrichmentStats) {
	if job == nil {
		return
	}
	if stats == nil {
		stats = newAcceptedEnrichmentStats(0)
	}
	before := debugJobIdentitySnapshot(*job)
	defer func() {
		logJobIdentityOutcome("apply/source pages", before, *job)
	}()
	sanitizeExistingJobIdentity(job)
	setJobCompanyIfMissing(job, companyNameFromSummary(job.CompanySummary))
	enrichJobIndustryFromExistingSummary(job)
	if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		if website := inferCompanyWebsiteFromApplyURL(*job); website != "" {
			job.CompanyWebsite = website
			setJobIdentityEvidence(job, "website", website, "apply_url_host", job.ApplyURL, "medium", false, "Website inferred from a first-party apply URL host.")
		}
	}
	if !jobNeedsApplyPageEnrichment(*job) && !jobNeedsCompanyPageEnrichment(*job) {
		return
	}

	if isBuiltInJobURL(job.ApplyURL) {
		enrichJobFromBuiltInCompanyProfile(ctx, job, llmEnrich, profileEnricher, stats)
		enrichJobFromCompanyWebsitePagesWithStats(ctx, job, llmEnrich, stats)
		return
	}

	if jobNeedsApplyPageEnrichment(*job) {
		stats.inc(&stats.applyFetchAttempts)
		logDebug("identity enrichment apply/source pages: fetching apply_url=%q company=%q title=%q", job.ApplyURL, job.Company, job.Title)
		rawHTML, finalURL, err := fetchApplyPage(ctx, job.ApplyURL)
		if err == nil && strings.TrimSpace(rawHTML) != "" {
			stats.inc(&stats.applyFetchSuccess)
			logDebug("identity enrichment apply/source pages: fetched apply_url=%q final_url=%q bytes=%d llm=%t", job.ApplyURL, finalURL, len(rawHTML), llmEnrich != nil)
			enrichJobFromHTML(job, rawHTML, finalURL)
			if profileURL := extractSourceCompanyProfileURL(rawHTML, finalURL); profileURL != "" {
				enrichJobFromSourceCompanyProfile(ctx, job, profileURL, llmEnrich, "llm_source_profile", profileEnricher, stats)
			}
			if jobNeedsLLMIdentityEnrichment(*job) {
				stats.addLLMUsage(applyLLMJobIdentityEnrichment(ctx, job, buildJobIdentityPage(rawHTML, finalURL), llmEnrich, "llm_apply_page"))
			}
		} else if err != nil {
			stats.inc(&stats.applyFetchFailed)
			if isBlockedFetchError(err) {
				stats.inc(&stats.applyFetchBlocked)
			}
			logDebug("identity enrichment apply/source pages: fetch failed apply_url=%q error=%v", job.ApplyURL, err)
		} else {
			stats.inc(&stats.applyFetchEmpty)
			logDebug("identity enrichment apply/source pages: empty apply page apply_url=%q", job.ApplyURL)
		}
	}

	enrichJobFromCompanyWebsitePagesWithStats(ctx, job, llmEnrich, stats)
}

func enrichJobFromBuiltInCompanyProfile(ctx context.Context, job *Job, llmEnrich jobIdentityPageEnrichFunc, profileEnricher *sourceProfileEnricher, stats *acceptedEnrichmentStats) {
	profileURL := builtInCompanyProfileURLFromJob(*job)
	if profileURL == "" {
		logDebug("identity enrichment built-in profile: skipped apply_url fetch; no card company profile URL company=%q title=%q apply_url=%q", job.Company, job.Title, job.ApplyURL)
		return
	}
	enrichJobFromSourceCompanyProfile(ctx, job, profileURL, llmEnrich, "llm_builtin_company_profile", profileEnricher, stats)
}

func enrichJobFromSourceCompanyProfile(ctx context.Context, job *Job, profileURL string, llmEnrich jobIdentityPageEnrichFunc, llmSource string, profileEnricher *sourceProfileEnricher, stats *acceptedEnrichmentStats) bool {
	if profileEnricher == nil {
		profileEnricher = newSourceProfileEnricher()
	}
	return profileEnricher.EnrichWithStats(ctx, job, profileURL, llmEnrich, llmSource, stats)
}

func enrichJobsFromApplyPages(ctx context.Context, jobs []Job) []Job {
	return enrichJobsFromApplyPagesWithProgress(ctx, jobs, nil)
}

func enrichJobsFromApplyPagesWithProgress(ctx context.Context, jobs []Job, progress func(string)) []Job {
	return enrichJobsFromApplyPagesWithLLMAndProgress(ctx, jobs, nil, progress, nil)
}

func enrichJobsFromApplyPagesWithLLMAndProgress(ctx context.Context, jobs []Job, llmEnrich jobIdentityPageEnrichFunc, progress func(string), onJobEnriched func(Job)) []Job {
	return enrichJobsFromApplyPagesWithLLMStoreAndProgress(ctx, jobs, llmEnrich, nil, progress, onJobEnriched)
}

func enrichJobsFromApplyPagesWithLLMStoreAndProgress(ctx context.Context, jobs []Job, llmEnrich jobIdentityPageEnrichFunc, identityStore PersistentCompanyIdentityStore, progress func(string), onJobEnriched func(Job)) []Job {
	if len(jobs) == 0 {
		return jobs
	}
	start := time.Now()
	urlVisits, ownsURLVisits := ensureURLVisitRun()
	if ownsURLVisits {
		defer clearURLVisitRun(urlVisits)
	}
	const maxConcurrentEnrich = 6
	logDebug("identity enrichment apply/source pages batch start jobs=%d llm=%t concurrency=%d", len(jobs), llmEnrich != nil, maxConcurrentEnrich)
	stats := newAcceptedEnrichmentStats(len(jobs))
	reportFetchProgress(progress, "Enriching identity from apply/source pages for %d jobs...", len(jobs))
	sem := make(chan struct{}, maxConcurrentEnrich)
	var wg sync.WaitGroup
	completed := 0
	var progressMu sync.Mutex
	cache := NewCompanyIdentityCache()
	stats.addCopiedFields(seedCompanyIdentityFromTrustedJobs(jobs, cache))
	profileEnricher := currentSourceProfileRun()
	for i := range jobs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			stats.inc(&stats.descriptionChecks)
			enrichJobFromDescription(&jobs[idx])
			if record, ok := hydrateCompanyIdentityFromStore(ctx, &jobs[idx], identityStore, stats); ok {
				cache.Set(jobs[idx].Company, record)
			}
			if record, ok := cache.Get(jobs[idx].Company); ok {
				stats.inc(&stats.localCacheHits)
				ApplyCachedIdentity(&jobs[idx], record)
			}
			enrichJobFromApplyPageWithProfilesAndStats(ctx, &jobs[idx], llmEnrich, profileEnricher, stats)
			if cache.IsIdentityComplete(jobs[idx]) {
				cache.Set(jobs[idx].Company, CompanyIdentityRecord{
					Website:  jobs[idx].CompanyWebsite,
					Summary:  jobs[idx].CompanySummary,
					Industry: jobs[idx].CompanyIndustry,
					Identity: CloneJobIdentityMetadata(jobs[idx].CompanyIdentity),
				})
			}
			persistCompanyIdentityToStore(ctx, jobs[idx], identityStore, stats)
			progressMu.Lock()
			completed++
			if completed == len(jobs) || completed%25 == 0 {
				reportFetchProgress(progress, "Apply/source page identity enrichment checked %d of %d jobs...", completed, len(jobs))
			}
			if onJobEnriched != nil {
				onJobEnriched(jobs[idx])
			}
			progressMu.Unlock()
		}(i)
	}
	wg.Wait()
	logDebug("browser company search: disabled during accepted-job enrichment; leaving unresolved identity fields for source-specific repair")
	stats.addCopiedFields(propagateSameCompanyIdentity(jobs))
	for i := range jobs {
		backfillJobIdentityEvidence(&jobs[i], jobs[i].Source, jobs[i].ApplyURL)
	}
	elapsed := time.Since(start).Round(time.Millisecond).String()
	stats.log(elapsed)
	fetches, hits, htmlEntries, probeEntries := urlVisits.Stats()
	logDebug("url visit registry summary: fetches=%d hits=%d html_entries=%d probe_entries=%d", fetches, hits, htmlEntries, probeEntries)
	logJobIdentityBatchSummary("apply/source pages", jobs, elapsed)
	return jobs
}

func hydrateCompanyIdentityFromStore(ctx context.Context, job *Job, identityStore PersistentCompanyIdentityStore, stats *acceptedEnrichmentStats) (CompanyIdentityRecord, bool) {
	if job == nil || identityStore == nil {
		return CompanyIdentityRecord{}, false
	}
	identity, err := identityStore.GetCompanyIdentity(ctx, job.Company, job.CompanyWebsite)
	if err != nil {
		stats.inc(&stats.persistentCacheFailed)
		logDebug("identity enrichment persistent company cache: get failed company=%q website=%q error=%v", job.Company, job.CompanyWebsite, err)
		return CompanyIdentityRecord{}, false
	}
	if identity == nil {
		stats.inc(&stats.persistentCacheMisses)
		return CompanyIdentityRecord{}, false
	}
	record := companyIdentityRecordFromPersistent(identity)
	stats.inc(&stats.persistentCacheHits)
	ApplyCachedIdentity(job, record)
	logDebug("identity enrichment persistent company cache: hit company=%q website=%q", job.Company, record.Website)
	return record, true
}

func persistCompanyIdentityToStore(ctx context.Context, job Job, identityStore PersistentCompanyIdentityStore, stats *acceptedEnrichmentStats) {
	if identityStore == nil {
		return
	}
	record, ok := trustedCompanyIdentityRecordFromJob(job)
	if !ok {
		return
	}
	identity, ok := persistentCompanyIdentityFromRecord(job.Company, record)
	if !ok {
		return
	}
	if err := identityStore.UpsertCompanyIdentity(ctx, identity); err != nil {
		stats.inc(&stats.persistentCacheWriteFailed)
		logDebug("identity enrichment persistent company cache: set failed company=%q website=%q error=%v", job.Company, record.Website, err)
		return
	}
	stats.inc(&stats.persistentCacheWrites)
	logDebug("identity enrichment persistent company cache: stored company=%q website=%q summary=%t industry=%t", job.Company, record.Website, record.Summary != "", record.Industry != "")
}

func PersistTrustedCompanyIdentities(ctx context.Context, jobs []Job, identityStore PersistentCompanyIdentityStore) int {
	if identityStore == nil {
		return 0
	}
	written := 0
	for _, job := range jobs {
		record, ok := trustedCompanyIdentityRecordFromJob(job)
		if !ok {
			continue
		}
		identity, ok := persistentCompanyIdentityFromRecord(job.Company, record)
		if !ok {
			continue
		}
		if err := identityStore.UpsertCompanyIdentity(ctx, identity); err != nil {
			logDebug("identity enrichment persistent company cache: repair store failed company=%q website=%q error=%v", job.Company, record.Website, err)
			continue
		}
		written++
	}
	if written > 0 {
		logDebug("identity enrichment persistent company cache: repair stored identities=%d", written)
	}
	return written
}

func enrichJobsFromApplyPagesForValidation(ctx context.Context, jobs []Job, progress func(string)) []Job {
	if len(jobs) == 0 {
		return jobs
	}
	start := time.Now()
	const maxConcurrentEnrich = 6
	logDebug("identity enrichment validation batch start jobs=%d concurrency=%d", len(jobs), maxConcurrentEnrich)
	reportFetchProgress(progress, "Checking apply pages for identity on %d jobs...", len(jobs))
	sem := make(chan struct{}, maxConcurrentEnrich)
	var wg sync.WaitGroup
	profileEnricher := newSourceProfileEnricher()
	for i := range jobs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			enrichJobFromDescription(&jobs[idx])
			enrichJobFromApplyPageForValidationWithProfiles(ctx, &jobs[idx], profileEnricher)
		}(i)
	}
	wg.Wait()
	for i := range jobs {
		backfillJobIdentityEvidence(&jobs[i], jobs[i].Source, jobs[i].ApplyURL)
	}
	logJobIdentityBatchSummary("validation apply pages", jobs, time.Since(start).Round(time.Millisecond).String())
	return jobs
}

func enrichJobFromApplyPageForValidationWithProfiles(ctx context.Context, job *Job, profileEnricher *sourceProfileEnricher) {
	if job == nil {
		return
	}
	before := debugJobIdentitySnapshot(*job)
	defer func() {
		logJobIdentityOutcome("validation apply page", before, *job)
	}()
	sanitizeExistingJobIdentity(job)
	setJobCompanyIfMissing(job, companyNameFromSummary(job.CompanySummary))
	enrichJobIndustryFromExistingSummary(job)
	if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		if website := inferCompanyWebsiteFromApplyURL(*job); website != "" {
			job.CompanyWebsite = website
			setJobIdentityEvidence(job, "website", website, "apply_url_host", job.ApplyURL, "medium", false, "Website inferred from a first-party apply URL host.")
		}
	}
	if !jobNeedsApplyPageEnrichment(*job) {
		logDebug("identity enrichment validation apply page: skipped fetch; identity already sufficient company=%q title=%q", job.Company, job.Title)
		return
	}

	if isBuiltInJobURL(job.ApplyURL) {
		enrichJobFromBuiltInCompanyProfile(ctx, job, nil, profileEnricher, nil)
		return
	}

	logDebug("identity enrichment validation apply page: fetching apply_url=%q company=%q title=%q", job.ApplyURL, job.Company, job.Title)
	rawHTML, finalURL, err := fetchApplyPage(ctx, job.ApplyURL)
	if err != nil || strings.TrimSpace(rawHTML) == "" {
		if err != nil {
			logDebug("identity enrichment validation apply page: fetch failed apply_url=%q error=%v", job.ApplyURL, err)
		} else {
			logDebug("identity enrichment validation apply page: empty apply page apply_url=%q", job.ApplyURL)
		}
		return
	}
	logDebug("identity enrichment validation apply page: fetched apply_url=%q final_url=%q bytes=%d", job.ApplyURL, finalURL, len(rawHTML))
	enrichJobFromHTML(job, rawHTML, finalURL)
	if !jobNeedsApplyPageEnrichment(*job) {
		return
	}
	if profileURL := extractSourceCompanyProfileURL(rawHTML, finalURL); profileURL != "" {
		enrichJobFromSourceCompanyProfile(ctx, job, profileURL, nil, "", profileEnricher, nil)
	}
}

func EnrichJobsFromApplyPages(ctx context.Context, jobs []Job) []Job {
	return enrichJobsFromApplyPages(ctx, jobs)
}

func EnrichJobsFromApplyPagesWithProgress(ctx context.Context, jobs []Job, progress func(string)) []Job {
	return enrichJobsFromApplyPagesWithProgress(ctx, jobs, progress)
}

func EnrichJobsFromApplyPagesWithConfigAndProgress(ctx context.Context, jobs []Job, appCfg *AppConfig, progress func(string), onJobEnriched func(Job)) []Job {
	return EnrichJobsFromApplyPagesWithConfigStoreAndProgress(ctx, jobs, appCfg, nil, progress, onJobEnriched)
}

func EnrichJobsFromApplyPagesWithConfigStoreAndProgress(ctx context.Context, jobs []Job, appCfg *AppConfig, identityStore PersistentCompanyIdentityStore, progress func(string), onJobEnriched func(Job)) []Job {
	identityEnrich, restoreIdentityLLM, identityNotice := newJobIdentityLLMEnricher(ctx, appCfg)
	if restoreIdentityLLM != nil {
		defer restoreIdentityLLM()
	}
	if identityNotice != "" {
		reportFetchProgress(progress, "%s", identityNotice)
	}
	return enrichJobsFromApplyPagesWithLLMStoreAndProgress(ctx, jobs, identityEnrich, identityStore, progress, onJobEnriched)
}
