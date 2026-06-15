package fetcher

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/storage"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"golang.org/x/net/publicsuffix"
)

const (
	siteSearchBrowserTimeout = 20 * time.Second
	siteSearchSettleDelay    = 1500 * time.Millisecond
)

const maxConcurrentBuiltInDetailFetch = 3

const (
	SiteSearchBrowserTimeout = siteSearchBrowserTimeout
	SiteSearchSettleDelay    = siteSearchSettleDelay
)

type siteSearchCandidate struct {
	Title       string
	Company     string
	URL         string
	Description string
	Score       int
}

var siteSearchNetworkErrorPattern = regexp.MustCompile(`net::[A-Z_]+`)
var cityStateLinePattern = regexp.MustCompile(`^[A-Za-z .'-]+,\s*[A-Z]{2}(?:\b|$)`)
var relativeTimeLinePattern = regexp.MustCompile(`(?i)^\d+\s+(minute|hour|day|week|month)s?\s+ago$`)
var siteSearchBrowserLookPath = launcher.LookPath
var siteSearchBrowserExecLookPath = exec.LookPath

var errSiteSearchVerificationRequired = errors.New("verification required")

type siteSearchBrowserCloser interface {
	Close() error
}

type siteSearchBrowserLauncherCleanup interface {
	Kill()
	Cleanup()
}

func newSiteSearchBrowser() (*rod.Browser, func(), error) {
	launch := launcher.New().
		Headless(true).
		NoSandbox(true).
		Set("ignore-certificate-errors")

	if browserBin := findSiteSearchBrowserBinary(); browserBin != "" {
		launch = launch.Bin(browserBin)
	}

	controlURL, err := launch.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("launch browser: %w", err)
	}
	logDebug("browser launch started pid=%d control_url=%q", launch.PID(), controlURL)

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		logDebug("browser connect failed pid=%d error=%v", launch.PID(), err)
		cleanupLaunchedSiteSearchBrowser(nil, launch)
		return nil, nil, fmt.Errorf("connect browser: %w", err)
	}
	logDebug("browser connected pid=%d", launch.PID())

	cleanup := func() {
		cleanupLaunchedSiteSearchBrowser(browser, launch)
	}

	return browser, cleanup, nil
}

func cleanupLaunchedSiteSearchBrowser(browser siteSearchBrowserCloser, launch siteSearchBrowserLauncherCleanup) {
	if browser != nil {
		if concrete, ok := browser.(*rod.Browser); ok {
			forgetReusableBrowserPage(concrete)
		}
		_ = browser.Close()
	}
	if launch != nil {
		if concrete, ok := launch.(interface{ PID() int }); ok {
			logDebug("browser cleanup killing pid=%d", concrete.PID())
		}
		launch.Kill()
		launch.Cleanup()
		if concrete, ok := launch.(interface{ PID() int }); ok {
			logDebug("browser cleanup complete pid=%d", concrete.PID())
		}
	}
}

func NewSiteSearchBrowser() (*rod.Browser, func(), error) {
	return newSiteSearchBrowser()
}

func findSiteSearchBrowserBinary() string {
	if value := strings.TrimSpace(os.Getenv("ROD_BROWSER_BIN")); value != "" {
		return value
	}
	if path, ok := siteSearchBrowserLookPath(); ok {
		return path
	}
	for _, candidate := range []string{
		"chromium-browser",
		"chromium",
		"google-chrome",
		"google-chrome-stable",
		"chrome",
		"microsoft-edge",
		"/data/data/com.termux/files/usr/bin/chromium-browser",
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/microsoft-edge",
		"/snap/bin/chromium",
	} {
		if path, err := siteSearchBrowserExecLookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

func FindSiteSearchBrowserBinary() string {
	return findSiteSearchBrowserBinary()
}

func fetchGenericSiteSearch(ctx context.Context, browser *rod.Browser, target string, targetURL string, sourceName string, criteria *CriteriaConfig, detailBlocked *siteSearchBlocklist, candidateLimiter *siteCandidateLimiter, candidateStore storage.CandidateStore) ([]Job, map[string][]Job, string, error) {
	candidates, err := probeSiteSearchCandidates(ctx, browser, targetURL, criteria)
	if err != nil {
		logDebug("site search %s: browser candidate probe failed: %v", target, err)
		return nil, nil, "", err
	}
	logDebug("site search %s: browser candidate probe returned %d candidates", target, len(candidates))

	jobs := make([]Job, 0, len(candidates))
	filtered := make(map[string][]Job)
	seen := make(map[string]bool)
	notice := ""
	if kept, skipped := candidateLimiter.take(target, candidates); skipped > 0 {
		logDebug("site search %s: candidate limit kept=%d skipped=%d", target, len(kept), skipped)
		candidates = kept
	}

	for _, candidate := range candidates {
		title := candidate.Title
		company := strings.TrimSpace(candidate.Company)

		if parsedTitle, parsedCompany, ok := splitJobTitleCompanyAt(candidate.Title); ok {
			title = parsedTitle
			company = parsedCompany
		} else if parsedCompany, parsedTitle, ok := splitJobCompanyTitleColon(candidate.Title); ok {
			title = parsedTitle
			company = parsedCompany
		}
		if siteSearchCompanyMissingOrInvalid(company) {
			company = inferCompanyFromSiteSearchURL(candidate.URL)
		}
		if siteSearchCompanyMissingOrInvalid(company) {
			company = "Unknown"
		}

		jobKey := strings.ToLower(strings.TrimSpace(company) + "|" + strings.TrimSpace(title))
		if seen[jobKey] {
			continue
		}
		seen[jobKey] = true

		job := Job{
			Company:      company,
			Title:        title,
			ApplyURL:     candidate.URL,
			Source:       sourceName,
			Status:       "Unopened",
			Remote:       inferWorkSetting(candidate.Title+" "+candidate.Description+" "+candidate.URL, criteria),
			Compensation: "Not listed",
			Description:  firstNonEmptyString(candidate.Description, candidate.URL),
		}
		job.SetDateAdded(time.Now().Unix())
		enrichJobFromDescription(&job)
		if cached, ok := cachedDeterministicRejectedJob(ctx, candidateStore, criteria, job); ok {
			logDebug("site search %s: skipped cached deterministic rejection %s - %s", target, cached.Company, cached.Title)
			filtered["cached rejection"] = append(filtered["cached rejection"], cached)
			continue
		}
		if reason := titleScopeFilterReason(job.Title, criteria); reason != "" {
			logDebug("site search %s: filtered candidate %s - %s before detail enrichment: %s", target, job.Company, job.Title, reason)
			filtered[reason] = append(filtered[reason], job)
			continue
		}
		if enrichmentNotice := enrichSiteSearchJobIdentityBeforeFilter(ctx, browser, &job, detailBlocked); enrichmentNotice != "" && notice == "" {
			notice = enrichmentNotice
		}
		logDebug("site search %s: candidate score=%d company=%q title=%q url=%s", target, candidate.Score, job.Company, job.Title, job.ApplyURL)

		if siteSearchCompanyMissingOrInvalid(job.Company) {
			logDebug("site search %s: filtered candidate %s - %s: missing company identity", target, job.Company, job.Title)
			filtered["missing company identity"] = append(filtered["missing company identity"], job)
			continue
		}
		if reason := filterJobReason(&job, criteria); reason != "" {
			logDebug("site search %s: filtered candidate %s - %s: %s", target, job.Company, job.Title, reason)
			filtered[reason] = append(filtered[reason], job)
			continue
		}
		logDebug("site search %s: accepted candidate %s - %s", target, job.Company, job.Title)
		jobs = append(jobs, job)
	}

	logDebug("site search %s: accepted %d; filtered %d after generic candidate filtering", target, len(jobs), countFilteredJobs(filtered))
	return jobs, filtered, notice, nil
}

func enrichSiteSearchJobIdentityBeforeFilter(ctx context.Context, browser *rod.Browser, job *Job, detailBlocked *siteSearchBlocklist) string {
	if job == nil || !siteSearchJobNeedsPreFilterDetail(*job) {
		return ""
	}
	if isIndeedURL(job.ApplyURL) {
		return enrichIndeedJobIdentityBeforeFilter(ctx, browser, job, detailBlocked)
	}
	rawHTML, finalURL, err := fetchApplyPage(ctx, job.ApplyURL)
	if err != nil || strings.TrimSpace(rawHTML) == "" {
		if err != nil {
			logDebug("site search pre-filter identity %s: fetch failed: %v", job.ApplyURL, err)
		} else {
			logDebug("site search pre-filter identity %s: empty detail page", job.ApplyURL)
		}
		return ""
	}
	if strings.TrimSpace(finalURL) != "" {
		job.ApplyURL = finalURL
	}
	beforeCompany := job.Company
	beforeWebsite := job.CompanyWebsite
	enrichJobFromHTML(job, rawHTML, job.ApplyURL)
	sanitizeExistingJobIdentity(job)
	logDebug(
		"site search pre-filter identity %s: company %q -> %q website %q -> %q",
		job.ApplyURL,
		beforeCompany,
		job.Company,
		beforeWebsite,
		job.CompanyWebsite,
	)
	return ""
}

func siteSearchJobNeedsPreFilterIdentity(job Job) bool {
	if isKnownNonJobApplyURL(job.ApplyURL) {
		return false
	}
	return siteSearchCompanyMissingOrInvalid(job.Company)
}

func siteSearchJobNeedsPreFilterDetail(job Job) bool {
	if isKnownNonJobApplyURL(job.ApplyURL) {
		return false
	}
	return siteSearchJobNeedsPreFilterIdentity(job) ||
		jobDescriptionMissingOrURL(job.Description) ||
		domain.JobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) ||
		domain.JobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) ||
		jobCompanyIndustryNeedsEnrichment(job) ||
		domain.JobCompensationMissing(job.Compensation)
}

func enrichIndeedJobIdentityBeforeFilter(ctx context.Context, browser *rod.Browser, job *Job, detailBlocked *siteSearchBlocklist) string {
	if browser == nil {
		logDebug("site search pre-filter identity %s: skipped Indeed detail enrichment: browser unavailable", job.ApplyURL)
		return ""
	}
	if reason := detailBlocked.reasonFor(job.ApplyURL); reason != "" {
		logDebug("site search pre-filter identity %s: skipped Indeed detail enrichment because detail probes are blocked: %s", job.ApplyURL, reason)
		return ""
	}
	pageText, links, err := extractBrowserPageContent(ctx, browser, job.ApplyURL)
	if err != nil || strings.TrimSpace(pageText) == "" {
		if err != nil {
			logDebug("site search pre-filter identity %s: Indeed browser fetch failed: %v", job.ApplyURL, err)
		} else {
			logDebug("site search pre-filter identity %s: Indeed browser fetch empty", job.ApplyURL)
		}
		return ""
	}
	if isSiteSearchVerificationPage("", pageText) {
		reason := "Indeed blocked detail-page access; using listing snippets only"
		if line := verificationDebugLine(pageText); line != "" {
			reason += ": " + line
		}
		if detailBlocked.block(job.ApplyURL, reason) {
			logDebug("site search pre-filter identity %s: blocked remaining Indeed detail probes after verification page: %s", job.ApplyURL, reason)
			return "Indeed blocked detail-page access; using listing snippets only"
		}
		logDebug("site search pre-filter identity %s: Indeed detail blocked by verification page: %s", job.ApplyURL, reason)
		return ""
	}

	beforeCompany := job.Company
	beforeWebsite := job.CompanyWebsite
	if source := job.EnsureSourceMetadata(); strings.TrimSpace(source.PostingURL) == "" {
		source.PostingURL = strings.TrimSpace(job.ApplyURL)
	}
	enrichJobFromIndeedDetailText(job, pageText, job.ApplyURL)
	enrichJobFromIndeedDetailLinks(job, links)
	sanitizeExistingJobIdentity(job)
	logDebug(
		"site search pre-filter identity %s: Indeed detail company %q -> %q website %q -> %q links=%d",
		job.ApplyURL,
		beforeCompany,
		job.Company,
		beforeWebsite,
		job.CompanyWebsite,
		len(links),
	)
	return ""
}

func enrichJobFromIndeedDetailText(job *Job, pageText string, pageURL string) {
	if job == nil {
		return
	}
	if isSiteSearchVerificationPage("", pageText) {
		return
	}
	if jobDescriptionMissingOrURL(job.Description) && strings.TrimSpace(pageText) != "" {
		job.Description = truncateAtSentence(pageText, 1200)
	}
	if evidence := companyPublicProfileEvidenceFromText("indeed", pageURL, job.Company, pageText); companyPublicProfileEvidenceUseful(evidence) {
		applyCompanyPublicProfileEvidence(job, evidence, "indeed_detail")
	}
}

func enrichJobFromIndeedDetailLinks(job *Job, links []pageLink) {
	if job == nil {
		return
	}
	if profileURL := indeedCompanyProfileURLFromLinks(links); profileURL != "" {
		job.EnsureSourceMetadata().CompanyProfileURL = profileURL
	}
	if externalURL := indeedExternalApplyURLFromLinks(links, job.Company); externalURL != "" {
		source := job.EnsureSourceMetadata()
		source.ExternalApplyURL = externalURL
		if domain.JobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
			if website := companyWebsiteFromExternalApplyURL(externalURL, job.ApplyURL, job.Company); website != "" {
				job.CompanyWebsite = website
				setJobIdentityEvidence(job, "website", website, "indeed_apply_link", externalURL, "high", false, "Website inferred from Indeed company-site apply link.")
			}
		}
	}
}

func indeedCompanyProfileURLFromLinks(links []pageLink) string {
	for _, link := range links {
		if publicProfileSource(link.URL) == "indeed" {
			return link.URL
		}
	}
	return ""
}

func indeedExternalApplyURLFromLinks(links []pageLink, company string) string {
	best := ""
	bestScore := 0
	for _, link := range links {
		score := scoreIndeedExternalApplyLink(link, company)
		if score > bestScore {
			bestScore = score
			best = link.URL
		}
	}
	return best
}

func companyWebsiteFromExternalApplyURL(externalURL string, applyURL string, company string) string {
	parsed, err := url.Parse(strings.TrimSpace(externalURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "https"
	}
	host := parsed.Hostname()
	if rootHost, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil && rootHost != "" {
		host = rootHost
	}
	website := (&url.URL{Scheme: scheme, Host: host, Path: "/"}).String()
	if !looksLikeCompanyWebsite(website, applyURL) || !candidateWebsiteMatchesCompany(website, company) {
		return ""
	}
	return website
}

func scoreIndeedExternalApplyLink(link pageLink, company string) int {
	parsed, err := url.Parse(strings.TrimSpace(link.URL))
	if err != nil || parsed.Host == "" {
		return 0
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	if isIndeedDomain(host) || isGoogleHost(host) {
		return 0
	}
	candidate := canonicalCompanySiteURL(link.URL)
	if candidate == "" || !looksLikeCompanyWebsite(candidate, "") {
		return 0
	}
	score := 1
	textLower := strings.ToLower(link.Text)
	urlLower := strings.ToLower(link.URL)
	if strings.Contains(textLower, "apply") || strings.Contains(textLower, "company site") || strings.Contains(urlLower, "worker_signup") || strings.Contains(urlLower, "signup") {
		score += 6
	}
	if candidateWebsiteMatchesCompany(candidate, company) {
		score += 4
	}
	if strings.Contains(urlLower, "utm_source=indeed") || strings.Contains(urlLower, "indeed") {
		score += 2
	}
	return score
}

func jobDescriptionMissingOrURL(description string) bool {
	description = strings.TrimSpace(description)
	if description == "" {
		return true
	}
	parsed, err := url.Parse(description)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

type builtInDetailCache struct {
	mu       sync.Mutex
	entries  map[string]builtInDetailResult
	inFlight map[string]*builtInDetailCall
	sem      chan struct{}
}

type builtInDetailCall struct {
	done   chan struct{}
	result builtInDetailResult
}

func newBuiltInDetailCache() *builtInDetailCache {
	return &builtInDetailCache{
		entries:  make(map[string]builtInDetailResult),
		inFlight: make(map[string]*builtInDetailCall),
		sem:      make(chan struct{}, maxConcurrentBuiltInDetailFetch),
	}
}

func (c *builtInDetailCache) getOrFetch(ctx context.Context, detailURL string, sourceName string, criteria *CriteriaConfig) builtInDetailResult {
	if c == nil {
		return fetchBuiltInJobDetail(ctx, detailURL, sourceName, criteria)
	}
	key := builtInDetailCacheKey(detailURL)

	c.mu.Lock()
	if result, ok := c.entries[key]; ok {
		c.mu.Unlock()
		logDebug("site search built-in detail %s: cache hit", detailURL)
		return builtInDetailResultForSource(result, sourceName)
	}
	if call, ok := c.inFlight[key]; ok {
		c.mu.Unlock()
		select {
		case <-call.done:
			logDebug("site search built-in detail %s: joined in-flight fetch", detailURL)
			return builtInDetailResultForSource(call.result, sourceName)
		case <-ctx.Done():
			return builtInDetailResult{}
		}
	}
	call := &builtInDetailCall{done: make(chan struct{})}
	c.inFlight[key] = call
	c.mu.Unlock()

	select {
	case c.sem <- struct{}{}:
		call.result = fetchBuiltInJobDetail(ctx, detailURL, sourceName, criteria)
		<-c.sem
	case <-ctx.Done():
		call.result = builtInDetailResult{}
	}

	c.mu.Lock()
	c.entries[key] = call.result
	delete(c.inFlight, key)
	close(call.done)
	c.mu.Unlock()

	return builtInDetailResultForSource(call.result, sourceName)
}

func builtInDetailCacheKey(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return strings.TrimSpace(rawURL)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func builtInDetailResultForSource(result builtInDetailResult, sourceName string) builtInDetailResult {
	if result.ok {
		result.job.Source = sourceName
	}
	return result
}

func fetchBuiltInSiteSearch(ctx context.Context, targetURL string, sourceName string, sourceKey string, criteria *CriteriaConfig, detailCache *builtInDetailCache, profileEnricher *sourceProfileEnricher, existing *existingJobIndex, candidateLimiter *siteCandidateLimiter, candidateStore storage.CandidateStore) ([]Job, map[string][]Job, bool, error) {
	parsed, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil || parsed.Host == "" || !isBuiltInHost(parsed.Hostname()) {
		return nil, nil, false, nil
	}
	rawHTML, finalURL, err := fetchApplyPage(ctx, targetURL)
	if err != nil || strings.TrimSpace(rawHTML) == "" {
		if err != nil {
			logDebug("site search built-in %s: listing fetch failed: %v", targetURL, err)
		} else {
			logDebug("site search built-in %s: listing page was empty", targetURL)
		}
		return nil, nil, true, err
	}
	return parseBuiltInSiteSearchHTML(ctx, rawHTML, finalURL, sourceName, sourceKey, criteria, detailCache, profileEnricher, existing, candidateLimiter, candidateStore)
}

func parseBuiltInSiteSearchHTML(ctx context.Context, rawHTML string, finalURL string, sourceName string, sourceKey string, criteria *CriteriaConfig, detailCache *builtInDetailCache, profileEnricher *sourceProfileEnricher, existing *existingJobIndex, candidateLimiter *siteCandidateLimiter, candidateStore storage.CandidateStore) ([]Job, map[string][]Job, bool, error) {
	hrefs := extractHTMLHrefs(rawHTML)
	listingJobs, cardCount := extractBuiltInListingJobs(rawHTML, finalURL, sourceName, criteria)
	if cardCount > 0 {
		var skippedByLimit []builtInListingJob
		if keptCount := candidateLimiter.takeCount(sourceKey, len(listingJobs)); keptCount < len(listingJobs) {
			skippedByLimit = append(skippedByLimit, listingJobs[keptCount:]...)
			listingJobs = listingJobs[:keptCount]
			logDebug("site search built-in %s: candidate limit kept=%d skipped=%d", finalURL, keptCount, len(skippedByLimit))
		}
		listingJobs, cachedRejected := filterBuiltInListingJobsWithCachedRejects(ctx, candidateStore, criteria, listingJobs)
		cardFiltered := make(map[string][]Job)
		if len(cachedRejected) > 0 {
			cardFiltered["cached rejection"] = append(cardFiltered["cached rejection"], cachedRejected...)
		}
		acceptedListingJobs, profileFiltered := filterBuiltInListingJobsWithProfiles(listingJobs, criteria)
		cardFiltered = mergeFiltered(cardFiltered, profileFiltered)
		if len(skippedByLimit) > 0 {
			if cardFiltered == nil {
				cardFiltered = make(map[string][]Job)
			}
			cardFiltered["candidate limit reached"] = append(cardFiltered["candidate limit reached"], builtInListingJobsToJobs(skippedByLimit)...)
		}
		var skippedExisting []Job
		acceptedListingJobs, skippedExisting = skipExistingBuiltInListingJobs(acceptedListingJobs, existing)
		if len(skippedExisting) > 0 {
			if cardFiltered == nil {
				cardFiltered = make(map[string][]Job)
			}
			cardFiltered["already saved"] = append(cardFiltered["already saved"], skippedExisting...)
			logDebug("site search built-in %s: skipped %d already saved listing cards before profile hydration", finalURL, len(skippedExisting))
		}
		enrichBuiltInListingJobProfiles(ctx, acceptedListingJobs, profileEnricher)
		cardJobs := builtInListingJobsToJobs(acceptedListingJobs)
		logDebug("site search built-in %s: parsed %d listing cards; accepted %d; filtered %d", finalURL, cardCount, len(cardJobs), countFilteredJobs(cardFiltered))
		return cardJobs, cardFiltered, true, nil
	}

	links := extractBuiltInJobDetailURLs(rawHTML, finalURL)
	if len(links) == 0 {
		logDebug("site search built-in %s: no job detail links found among %d hrefs; html bytes=%d; href sample=%s", finalURL, len(hrefs), len(rawHTML), sampleDebugHrefs(hrefs, 8))
		return nil, nil, true, nil
	}
	if len(links) > 50 {
		links = links[:50]
	}
	if keptCount := candidateLimiter.takeCount(sourceKey, len(links)); keptCount < len(links) {
		logDebug("site search built-in %s: candidate limit kept=%d skipped=%d", finalURL, keptCount, len(links)-keptCount)
		links = links[:keptCount]
	}
	preFiltered := make(map[string][]Job)
	links, cachedRejected := filterBuiltInDetailLinksWithCachedRejects(ctx, candidateStore, criteria, sourceName, links)
	if len(cachedRejected) > 0 {
		preFiltered["cached rejection"] = append(preFiltered["cached rejection"], cachedRejected...)
	}
	logDebug("site search built-in %s: found %d job detail links", finalURL, len(links))

	results := make([]builtInDetailResult, len(links))
	var wg sync.WaitGroup
	for i, link := range links {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(idx int, detailURL string) {
			defer wg.Done()
			results[idx] = detailCache.getOrFetch(ctx, detailURL, sourceName, criteria)
		}(i, link)
	}
	wg.Wait()

	jobs := make([]Job, 0, len(links))
	filtered := preFiltered
	for _, result := range results {
		if !result.ok {
			continue
		}
		if result.filterReason != "" {
			filtered[result.filterReason] = append(filtered[result.filterReason], result.job)
			continue
		}
		jobs = append(jobs, result.job)
	}
	if len(jobs) == 0 && len(filtered) == 0 {
		logDebug("site search built-in %s: detail fetches produced no usable jobs", finalURL)
		return nil, nil, true, nil
	}
	logDebug("site search built-in %s: accepted %d; filtered %d", finalURL, len(jobs), countFilteredJobs(filtered))
	return jobs, filtered, true, nil
}

func filterBuiltInListingJobsWithCachedRejects(ctx context.Context, store storage.CandidateStore, criteria *CriteriaConfig, listingJobs []builtInListingJob) ([]builtInListingJob, []Job) {
	if store == nil || len(listingJobs) == 0 {
		return listingJobs, nil
	}
	kept := make([]builtInListingJob, 0, len(listingJobs))
	rejected := make([]Job, 0)
	for _, listingJob := range listingJobs {
		cached, ok := cachedDeterministicRejectedJob(ctx, store, criteria, listingJob.job)
		if ok {
			rejected = append(rejected, cached)
			continue
		}
		kept = append(kept, listingJob)
	}
	if len(rejected) > 0 {
		logDebug("site search built-in listing cache: skipped %d cached deterministic rejections", len(rejected))
	}
	return kept, rejected
}

func filterBuiltInDetailLinksWithCachedRejects(ctx context.Context, store storage.CandidateStore, criteria *CriteriaConfig, sourceName string, links []string) ([]string, []Job) {
	if store == nil || len(links) == 0 {
		return links, nil
	}
	kept := make([]string, 0, len(links))
	rejected := make([]Job, 0)
	for _, link := range links {
		job := Job{
			Source:       sourceName,
			ApplyURL:     link,
			Status:       "Unopened",
			Compensation: "Not listed",
		}
		job.SetDateAdded(time.Now().Unix())
		cached, ok := cachedDeterministicRejectedJob(ctx, store, criteria, job)
		if ok {
			rejected = append(rejected, cached)
			continue
		}
		kept = append(kept, link)
	}
	if len(rejected) > 0 {
		logDebug("site search built-in detail cache: skipped %d cached deterministic rejections", len(rejected))
	}
	return kept, rejected
}

func skipExistingBuiltInListingJobs(listingJobs []builtInListingJob, existing *existingJobIndex) ([]builtInListingJob, []Job) {
	if existing == nil || len(listingJobs) == 0 {
		return listingJobs, nil
	}
	kept := make([]builtInListingJob, 0, len(listingJobs))
	skipped := make([]Job, 0)
	for _, listingJob := range listingJobs {
		if existing.contains(listingJob.job) {
			skipped = append(skipped, listingJob.job)
			continue
		}
		kept = append(kept, listingJob)
	}
	return kept, skipped
}

func enrichBuiltInListingJobProfiles(ctx context.Context, listingJobs []builtInListingJob, profileEnricher *sourceProfileEnricher) {
	if len(listingJobs) == 0 {
		return
	}
	if profileEnricher == nil {
		profileEnricher = newSourceProfileEnricher()
	}
	attempted := 0
	enriched := 0
	skipped := 0
	for i := range listingJobs {
		if ctx.Err() != nil {
			break
		}
		profileURL := strings.TrimSpace(listingJobs[i].profileURL)
		if profileURL == "" {
			skipped++
			continue
		}
		attempted++
		if profileEnricher.Enrich(ctx, &listingJobs[i].job, profileURL, nil, "") {
			enriched++
		}
	}
	if attempted > 0 || skipped > 0 {
		logDebug("site search built-in listing profile hydration: jobs=%d attempted=%d enriched=%d skipped=%d", len(listingJobs), attempted, enriched, skipped)
	}
}

type builtInDetailResult struct {
	job          Job
	filterReason string
	ok           bool
}

func fetchBuiltInJobDetail(ctx context.Context, detailURL string, sourceName string, criteria *CriteriaConfig) builtInDetailResult {
	detailHTML, finalURL, err := fetchApplyPage(ctx, detailURL)
	if err != nil || strings.TrimSpace(detailHTML) == "" {
		if err != nil {
			logDebug("site search built-in detail %s: fetch failed: %v", detailURL, err)
		} else {
			logDebug("site search built-in detail %s: empty detail page", detailURL)
		}
		return builtInDetailResult{}
	}
	job := Job{
		ApplyURL:     finalURL,
		Source:       sourceName,
		Status:       "Unopened",
		Remote:       inferWorkSetting(normalizeHTMLText(detailHTML)+" "+finalURL, criteria),
		Compensation: "Not listed",
	}
	enrichJobFromHTML(&job, detailHTML, finalURL)
	if profileURL := extractSourceCompanyProfileURL(detailHTML, finalURL); profileURL != "" {
		job.EnsureSourceMetadata().CompanyProfileURL = profileURL
	}
	if jobCompanyMissingOrUnknown(job.Company) || strings.TrimSpace(job.Title) == "" {
		logDebug("site search built-in detail %s: missing company/title after parsing", finalURL)
		return builtInDetailResult{}
	}
	job.SetDateAdded(time.Now().Unix())
	if reason := filterJobReason(&job, criteria); reason != "" {
		logDebug("site search built-in detail %s: filtered %s - %s: %s", finalURL, job.Company, job.Title, reason)
		return builtInDetailResult{
			job:          job,
			filterReason: reason,
			ok:           true,
		}
	}
	logDebug("site search built-in detail %s: accepted %s - %s", finalURL, job.Company, job.Title)
	return builtInDetailResult{
		job: job,
		ok:  true,
	}
}

func siteSearchURL(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(target), "site:") {
		return ""
	}

	if strings.Contains(target, "://") {
		u, err := url.Parse(target)
		if err != nil {
			return ""
		}
		if isSharedATSDirectoryHost(u.Hostname()) && (u.Path == "" || u.Path == "/") {
			return ""
		}
		if isBuiltInHost(u.Hostname()) && (u.Path == "" || u.Path == "/") {
			u.Path = "/jobs"
		}
		return u.String()
	}

	if isSharedATSDirectoryHost(target) {
		return ""
	}
	if isBuiltInHost(target) {
		return "https://" + strings.TrimSuffix(target, "/") + "/jobs"
	}
	if isIndeedHost(target) {
		return "https://www.indeed.com/jobs"
	}
	if isLinkedInHost(target) {
		return "https://www.linkedin.com/jobs/search"
	}

	return "https://" + strings.TrimSuffix(target, "/")
}

func siteSearchURLForCriteria(target string, criteria *CriteriaConfig) string {
	urls := siteSearchURLsForCriteria(target, criteria)
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func siteSearchURLsForCriteria(target string, criteria *CriteriaConfig) []string {
	targetURL := siteSearchURL(target)
	if targetURL == "" {
		return nil
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return []string{targetURL}
	}
	searchKey := ""
	var searches []string
	switch {
	case isBuiltInHost(u.Hostname()):
		if u.Path == "" || u.Path == "/" {
			u.Path = "/jobs"
		}
		query := u.Query()
		if query.Get("search") == "" {
			searchKey = "search"
			searches = targetedSiteSearchQueries(criteria)
		}
		if query.Get("country") == "" {
			if country := builtInCountryParam(criteria); country != "" {
				query.Set("country", country)
			}
		}
		if criteria != nil && criteria.Filters.WorkSettings.Remote && isBuiltInNationalHost(u.Hostname()) && query.Get("allLocations") == "" {
			query.Set("allLocations", "true")
		}
		u.RawQuery = query.Encode()
	case isIndeedHost(u.Hostname()):
		if u.Path == "" || u.Path == "/" {
			u.Path = "/jobs"
		}
		query := u.Query()
		if query.Get("q") == "" {
			searchKey = "q"
			searches = targetedSiteSearchQueries(criteria)
		}
		if query.Get("l") == "" {
			if location := siteSearchLocation(criteria); location != "" {
				query.Set("l", location)
			}
		}
		u.RawQuery = query.Encode()
	case isLinkedInHost(u.Hostname()):
		if u.Path == "" || u.Path == "/" || u.Path == "/jobs" {
			u.Path = "/jobs/search"
		}
		query := u.Query()
		if query.Get("keywords") == "" {
			searchKey = "keywords"
			searches = targetedLinkedInSearchQueries(criteria)
		}
		if query.Get("location") == "" {
			if location := linkedInLocationQuery(criteria); location != "" {
				query.Set("location", location)
			}
		}
		if query.Get("f_PP") == "" {
			if geoID := linkedInGeoID(criteria); geoID != "" {
				query.Set("f_PP", geoID)
			}
		}
		if query.Get("f_WT") == "" {
			if workplaceTypes := linkedInWorkplaceTypes(criteria); workplaceTypes != "" {
				query.Set("f_WT", workplaceTypes)
			}
		}
		if query.Get("f_E") == "" {
			if experienceLevels := linkedInExperienceLevels(criteria); experienceLevels != "" {
				query.Set("f_E", experienceLevels)
			}
		}
		if query.Get("f_SB2") == "" {
			if salaryBucket := linkedInSalaryBucket(criteria); salaryBucket != "" {
				query.Set("f_SB2", salaryBucket)
			}
		}
		u.RawQuery = query.Encode()
	default:
		return []string{targetURL}
	}
	if searchKey != "" && len(searches) > 0 {
		return siteSearchURLsWithQueryValues(u, searchKey, searches)
	}
	return []string{u.String()}
}

func siteSearchURLsForCompany(target string, criteria *CriteriaConfig, company companyFetchScope, matchCriteria bool) []string {
	targetURL := siteSearchURL(target)
	if targetURL == "" {
		return nil
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return []string{targetURL}
	}

	searches := companyTargetSearchQueries(company, criteria, matchCriteria)
	if len(searches) == 0 {
		return nil
	}
	searchCriteria := criteria
	if !matchCriteria {
		searchCriteria = nil
	}
	searchKey := ""
	switch {
	case isBuiltInHost(u.Hostname()):
		if u.Path == "" || u.Path == "/" {
			u.Path = "/jobs"
		}
		query := u.Query()
		if query.Get("search") == "" {
			searchKey = "search"
		}
		if query.Get("country") == "" {
			if country := builtInCountryParam(searchCriteria); country != "" {
				query.Set("country", country)
			}
		}
		if searchCriteria != nil && searchCriteria.Filters.WorkSettings.Remote && isBuiltInNationalHost(u.Hostname()) && query.Get("allLocations") == "" {
			query.Set("allLocations", "true")
		}
		u.RawQuery = query.Encode()
	case isIndeedHost(u.Hostname()):
		if u.Path == "" || u.Path == "/" {
			u.Path = "/jobs"
		}
		query := u.Query()
		if query.Get("q") == "" {
			searchKey = "q"
		}
		if query.Get("l") == "" {
			if location := siteSearchLocation(searchCriteria); location != "" {
				query.Set("l", location)
			}
		}
		u.RawQuery = query.Encode()
	case isLinkedInHost(u.Hostname()):
		if u.Path == "" || u.Path == "/" || u.Path == "/jobs" {
			u.Path = "/jobs/search"
		}
		query := u.Query()
		if query.Get("keywords") == "" {
			searchKey = "keywords"
		}
		if query.Get("location") == "" {
			if location := linkedInLocationQuery(searchCriteria); location != "" {
				query.Set("location", location)
			}
		}
		if query.Get("f_PP") == "" {
			if geoID := linkedInGeoID(searchCriteria); geoID != "" {
				query.Set("f_PP", geoID)
			}
		}
		if query.Get("f_WT") == "" {
			if workplaceTypes := linkedInWorkplaceTypes(searchCriteria); workplaceTypes != "" {
				query.Set("f_WT", workplaceTypes)
			}
		}
		if query.Get("f_E") == "" {
			if experienceLevels := linkedInExperienceLevels(searchCriteria); experienceLevels != "" {
				query.Set("f_E", experienceLevels)
			}
		}
		if query.Get("f_SB2") == "" {
			if salaryBucket := linkedInSalaryBucket(searchCriteria); salaryBucket != "" {
				query.Set("f_SB2", salaryBucket)
			}
		}
		u.RawQuery = query.Encode()
	default:
		return []string{u.String()}
	}
	var urls []string
	if searchKey != "" {
		urls = siteSearchURLsWithQueryValues(u, searchKey, searches)
	} else {
		urls = []string{u.String()}
	}
	if isIndeedHost(u.Hostname()) {
		for _, companyURL := range indeedCompanySearchURLs(company) {
			urls = appendUniqueString(urls, companyURL)
		}
	}
	return urls
}

func siteSearchURLsWithQueryValues(base *url.URL, key string, values []string) []string {
	if base == nil {
		return nil
	}
	urls := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		u := *base
		query := u.Query()
		query.Set(key, value)
		u.RawQuery = query.Encode()
		urls = append(urls, u.String())
	}
	return urls
}

func targetedSiteSearchQueries(criteria *CriteriaConfig) []string {
	if criteria == nil {
		return nil
	}
	prefixes := normalizedSearchTerms(domain.NormalizeTitlePrefixes(criteria.Filters.TitleRequires))
	titles := normalizedSearchTerms(domain.NormalizeTargetTitleNames(criteria.Filters.TitleIncludes, criteria.RoleFamilies))
	switch {
	case len(prefixes) == 0 && len(titles) == 0:
		return nil
	case len(prefixes) == 0:
		return titles
	case len(titles) == 0:
		return prefixes
	}
	var queries []string
	seen := make(map[string]bool)
	for _, prefix := range prefixes {
		queries = slices.Grow(queries, len(titles))
		for _, title := range titles {
			query := combinedTitleSearchQuery(prefix, title)
			key := strings.ToLower(query)
			if seen[key] {
				continue
			}
			seen[key] = true
			queries = append(queries, query)
		}
	}
	return queries
}

func companyTargetSearchQueries(company companyFetchScope, criteria *CriteriaConfig, matchCriteria bool) []string {
	names := company.names()
	if len(names) == 0 {
		return nil
	}
	domain := company.websiteDomain()
	baseQueries := make([]string, 0, len(names))
	for _, name := range names {
		query := strings.TrimSpace(name)
		if domain != "" {
			query = strings.TrimSpace(query + " " + domain)
		}
		baseQueries = appendUniqueString(baseQueries, query)
	}
	if !matchCriteria {
		return baseQueries
	}
	titleQueries := targetedSiteSearchQueries(criteria)
	if len(titleQueries) == 0 {
		return baseQueries
	}
	queries := make([]string, 0)
	seen := make(map[string]bool)
	for _, baseQuery := range baseQueries {
		for _, titleQuery := range titleQueries {
			query := strings.TrimSpace(baseQuery + " " + strings.TrimSpace(titleQuery))
			if query == "" {
				continue
			}
			key := strings.ToLower(query)
			if seen[key] {
				continue
			}
			seen[key] = true
			queries = append(queries, query)
		}
	}
	if len(queries) == 0 {
		return baseQueries
	}
	return queries
}

func indeedCompanySearchURLs(company companyFetchScope) []string {
	names := company.names()
	if len(names) == 0 {
		return nil
	}
	urls := make([]string, 0, len(names))
	for _, name := range names {
		u := url.URL{
			Scheme: "https",
			Host:   "www.indeed.com",
			Path:   "/companies/search",
		}
		query := u.Query()
		query.Set("q", name)
		u.RawQuery = query.Encode()
		urls = appendUniqueString(urls, u.String())
	}
	return urls
}

func combinedTitleSearchQuery(prefix string, title string) string {
	prefix = strings.TrimSpace(prefix)
	title = strings.TrimSpace(title)
	if prefix == "" {
		return title
	}
	if title == "" {
		return prefix
	}
	lowerTitle := strings.ToLower(title)
	lowerPrefix := strings.ToLower(prefix)
	if lowerTitle == lowerPrefix || strings.HasPrefix(lowerTitle, lowerPrefix+" ") {
		return title
	}
	return strings.TrimSpace(prefix + " " + title)
}

func targetedLinkedInSearchQueries(criteria *CriteriaConfig) []string {
	queries := targetedSiteSearchQueries(criteria)
	for i, query := range queries {
		if title := linkedInCachedTitle(query); title != "" {
			queries[i] = title
		}
	}
	return normalizedSearchTerms(queries)
}

func normalizedSearchTerms(values []string) []string {
	terms := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		terms = append(terms, value)
	}
	return terms
}

func linkedInGeoID(criteria *CriteriaConfig) string {
	if criteria == nil {
		return ""
	}
	return linkedInCachedGeoID(linkedInLocationQuery(criteria))
}

func linkedInLocationQuery(criteria *CriteriaConfig) string {
	if criteria == nil {
		return ""
	}
	values := make([]string, 0, 3)
	if city := strings.TrimSpace(criteria.Candidate.City); city != "" {
		values = append(values, city)
	}
	if state := strings.TrimSpace(criteria.Candidate.State); state != "" {
		values = append(values, state)
	}
	if len(values) > 0 {
		return strings.Join(values, ", ")
	}
	country := strings.ToUpper(strings.TrimSpace(criteria.Candidate.CountryCode))
	switch country {
	case "US", "USA", "UNITED STATES":
		return "United States"
	default:
		return strings.TrimSpace(criteria.Candidate.CountryCode)
	}
}

func linkedInWorkplaceTypes(criteria *CriteriaConfig) string {
	if criteria == nil {
		return ""
	}
	var values []string
	settings := criteria.Filters.WorkSettings
	if settings.Onsite {
		values = append(values, "1")
	}
	if settings.Remote {
		values = append(values, "2")
	}
	if settings.Hybrid {
		values = append(values, "3")
	}
	return strings.Join(values, ",")
}

func linkedInExperienceLevels(criteria *CriteriaConfig) string {
	if criteria == nil {
		return ""
	}
	years := criteria.Candidate.YearsOfExperience
	switch {
	case years <= 0:
		return ""
	case years <= 1:
		return "2"
	case years <= 3:
		return "2,3"
	case years <= 8:
		return "3,4"
	default:
		return "4"
	}
}

func linkedInSalaryBucket(criteria *CriteriaConfig) string {
	if criteria == nil {
		return ""
	}
	switch minBase := criteria.Filters.MinBaseUSD; {
	case minBase >= 120000:
		return "5"
	case minBase >= 100000:
		return "4"
	case minBase >= 80000:
		return "3"
	case minBase >= 60000:
		return "2"
	case minBase >= 40000:
		return "1"
	default:
		return ""
	}
}

func siteSearchLocation(criteria *CriteriaConfig) string {
	if criteria == nil {
		return ""
	}
	settings := criteria.Filters.WorkSettings
	if settings.Remote && !settings.Hybrid && !settings.Onsite {
		return "Remote"
	}
	values := make([]string, 0, 3)
	if city := strings.TrimSpace(criteria.Candidate.City); city != "" {
		values = append(values, city)
	}
	if state := strings.TrimSpace(criteria.Candidate.State); state != "" {
		values = append(values, state)
	}
	if len(values) > 0 {
		return strings.Join(values, ", ")
	}
	country := strings.ToUpper(strings.TrimSpace(criteria.Candidate.CountryCode))
	switch country {
	case "US", "USA", "UNITED STATES":
		return "United States"
	default:
		return strings.TrimSpace(criteria.Candidate.CountryCode)
	}
}

func builtInCountryParam(criteria *CriteriaConfig) string {
	if criteria == nil {
		return ""
	}
	country := strings.ToUpper(strings.TrimSpace(criteria.Candidate.CountryCode))
	switch country {
	case "US", "USA", "UNITED STATES":
		return "USA"
	default:
		return ""
	}
}

func isBuiltInHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return strings.Contains(host, "builtin")
}

func isBuiltInNationalHost(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(host), "www."))
	return host == "builtin.com"
}

func isIndeedHost(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(host), "www."))
	return host == "indeed.com"
}

func isIndeedDomain(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(host), "www."))
	return host == "indeed.com" || strings.HasSuffix(host, ".indeed.com")
}

func isIndeedURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	return isIndeedHost(parsed.Hostname())
}

func isIndeedPageadClickURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || !isIndeedHost(parsed.Hostname()) {
		return false
	}
	return strings.EqualFold(strings.TrimRight(parsed.EscapedPath(), "/"), "/pagead/clk")
}

func canonicalIndeedJobURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || !isIndeedHost(parsed.Hostname()) {
		return ""
	}
	path := strings.ToLower(strings.TrimRight(parsed.EscapedPath(), "/"))
	switch path {
	case "/viewjob":
		if jobKey := strings.TrimSpace(parsed.Query().Get("jk")); jobKey != "" {
			return "https://www.indeed.com/viewjob?jk=" + url.QueryEscape(jobKey)
		}
	case "/rc/clk", "/pagead/clk":
		if jobKey := strings.TrimSpace(parsed.Query().Get("jk")); jobKey != "" {
			return "https://www.indeed.com/viewjob?jk=" + url.QueryEscape(jobKey)
		}
	}
	return ""
}

func isLinkedInHost(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(host), "www."))
	return host == "linkedin.com"
}

func isGoogleHost(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(host), "www."))
	return host == "google.com"
}

func isBuiltInJobCandidate(title string, resolved *url.URL) bool {
	if resolved == nil {
		return false
	}
	titleLower := strings.ToLower(strings.TrimSpace(title))
	if titleLower == "" || strings.HasSuffix(titleLower, " jobs") || strings.Contains(titleLower, "job searches") {
		return false
	}
	return isBuiltInJobDetailPath(resolved.EscapedPath())
}

func isSiteSearchDirectJobCandidate(baseHost string, title string, resolved *url.URL) bool {
	if resolved == nil {
		return false
	}
	rawURL := resolved.String()
	if isKnownNonJobApplyURL(rawURL) {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(resolved.Hostname(), "www."))
	baseHost = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(baseHost), "www."))
	switch {
	case isIndeedHost(host):
		return isIndeedDirectJobPath(strings.ToLower(strings.TrimRight(resolved.EscapedPath(), "/")))
	case isLinkedInHost(host):
		return strings.HasPrefix(strings.ToLower(strings.TrimRight(resolved.EscapedPath(), "/")), "/jobs/view/")
	case isGoogleHost(host):
		return false
	case host == "ycombinator.com":
		return isYCombinatorJobURL(rawURL)
	case isBuiltInHost(host):
		return isBuiltInJobCandidate(title, resolved)
	case isSharedATSDirectoryHost(host):
		return !isSharedATSDirectoryHost(baseHost)
	default:
		return true
	}
}

func siteSearchCandidateApplyURL(baseHost string, resolved *url.URL, canonicalRaw string) (string, bool) {
	if resolved == nil {
		return "", false
	}
	rawURL := resolved.String()
	if canonical := safeCanonicalSiteSearchCandidateURL(rawURL, canonicalRaw); canonical != "" {
		return canonical, true
	}
	if canonical := canonicalIndeedJobURL(rawURL); canonical != "" {
		return canonical, true
	}
	if isIndeedHost(baseHost) && isIndeedPageadClickURL(rawURL) {
		return "", false
	}
	return rawURL, true
}

func safeCanonicalSiteSearchCandidateURL(rawURL string, canonicalRaw string) string {
	canonicalRaw = strings.TrimSpace(canonicalRaw)
	if canonicalRaw == "" {
		return ""
	}
	base, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	canonical, err := base.Parse(canonicalRaw)
	if err != nil || canonical.Host == "" || canonical.Scheme == "" {
		return ""
	}
	if canonical.Scheme != "http" && canonical.Scheme != "https" {
		return ""
	}
	canonicalURL := canonical.String()
	if usesReservedPlaceholderDomain(canonicalURL) || isKnownNonJobApplyURL(canonicalURL) || isIndeedPageadClickURL(canonicalURL) {
		return ""
	}
	return canonicalURL
}

func unwrapGoogleSearchResultURL(resolved *url.URL) *url.URL {
	if resolved == nil {
		return nil
	}
	if !isGoogleHost(resolved.Hostname()) {
		return resolved
	}
	raw := resolved.Query().Get("q")
	if raw == "" {
		raw = resolved.Query().Get("url")
	}
	if raw == "" {
		return resolved
	}
	unwrapped, err := url.Parse(raw)
	if err != nil || unwrapped.Scheme == "" || unwrapped.Host == "" {
		return resolved
	}
	return unwrapped
}

func isSharedATSDirectoryHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	switch host {
	case "boards.greenhouse.io", "jobs.lever.co", "myworkdayjobs.com", "icims.com", "smartrecruiters.com":
		return true
	default:
		return false
	}
}

func inferCompanyFromSiteSearchURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	switch {
	case isYCombinatorJobURL(rawURL):
		return ycCompanyNameFromURL(rawURL)
	case isLinkedInJobURL(rawURL):
		return linkedInCompanyNameFromJobURL(rawURL)
	case isGreenhouseJobURL(rawURL):
		return companyNameFromGreenhouseURL(rawURL)
	default:
		return ""
	}
}

func inferCompanyFromCandidateContext(baseHost string, title string, text string) string {
	lines := normalizedCandidateContextLines(text)
	if len(lines) == 0 {
		return ""
	}
	if isBuiltInHost(baseHost) {
		if !sameNormalizedText(lines[0], title) && looksLikeCompanyLine(lines[0]) {
			return lines[0]
		}
	}
	for i, line := range lines {
		if !candidateContextLineMatchesTitle(line, title) {
			continue
		}
		for _, candidate := range lines[i+1:] {
			if looksLikeCompanyLine(candidate) {
				return cleanCompanyLine(candidate)
			}
		}
	}
	return ""
}

func normalizedCandidateContextLines(text string) []string {
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	seen := make(map[string]bool)
	for _, line := range rawLines {
		line = normalizeWhitespace(line)
		if line == "" {
			continue
		}
		key := strings.ToLower(line)
		if seen[key] {
			continue
		}
		seen[key] = true
		lines = append(lines, line)
	}
	return lines
}

func candidateContextLineMatchesTitle(line string, title string) bool {
	line = normalizeWhitespace(line)
	title = normalizeWhitespace(title)
	if line == "" || title == "" {
		return false
	}
	if strings.EqualFold(line, title) {
		return true
	}
	lineLower := strings.ToLower(line)
	titleLower := strings.ToLower(title)
	return strings.Contains(lineLower, titleLower) || strings.Contains(titleLower, lineLower)
}

func looksLikeCompanyLine(line string) bool {
	line = normalizeWhitespace(line)
	if line == "" {
		return false
	}
	if looksLikeListingChromeLine(line) {
		return false
	}
	lower := strings.ToLower(line)
	rejectSubstrings := []string{
		"apply now",
		"be an early applicant",
		"click here",
		"company reviews",
		"easy apply",
		"hybrid work in ",
		"job alert",
		"job search",
		"jobs in ",
		"join now",
		"on-site in ",
		"posted ",
		"remote in ",
		"salary",
		"see who",
		"sign in",
		"skip to",
		"sponsored",
		"view all",
	}
	for _, reject := range rejectSubstrings {
		if strings.Contains(lower, reject) {
			return false
		}
	}
	if strings.HasSuffix(lower, " jobs") || strings.HasSuffix(lower, " salaries") {
		return false
	}
	switch lower {
	case "full-time", "part-time", "contract", "temporary", "internship", "remote", "hybrid", "on-site", "onsite":
		return false
	}
	if strings.Contains(lower, "$") || strings.Contains(lower, " per ") {
		return false
	}
	if cityStateLinePattern.MatchString(line) {
		return false
	}
	if relativeTimeLinePattern.MatchString(line) {
		return false
	}
	return true
}

func cleanCompanyLine(line string) string {
	line = normalizeWhitespace(line)
	lower := strings.ToLower(line)
	for _, suffix := range []string{" company logo", " logo"} {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSpace(line[:len(line)-len(suffix)])
		}
	}
	return line
}

func siteSearchCompanyMissingOrInvalid(company string) bool {
	return jobCompanyMissingOrUnknown(company) || looksLikeListingChromeLine(company)
}

func looksLikeListingChromeLine(line string) bool {
	lower := strings.ToLower(normalizeWhitespace(line))
	if lower == "" {
		return false
	}
	switch lower {
	case "actively hiring",
		"easily apply",
		"easy apply",
		"employer active",
		"featured",
		"hiring multiple candidates",
		"just posted",
		"multiple openings",
		"new",
		"today",
		"urgently hiring",
		"view similar jobs with this employer":
		return true
	}
	rejectSubstrings := []string{
		"company logo",
		"easily apply",
		"easy apply",
		"hiring multiple candidates",
		"image:",
		"often replies in ",
		"often responds within ",
		"transit information",
		"urgently hiring",
		"view similar jobs with this employer",
	}
	for _, reject := range rejectSubstrings {
		if strings.Contains(lower, reject) {
			return true
		}
	}
	return false
}

func sameNormalizedText(a string, b string) bool {
	return strings.EqualFold(normalizeWhitespace(a), normalizeWhitespace(b))
}

func isLinkedInJobURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	path := strings.ToLower(parsed.EscapedPath())
	return host == "linkedin.com" && strings.HasPrefix(path, "/jobs/view/")
}

func linkedInCompanyNameFromJobURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) < 3 || parts[0] != "jobs" || parts[1] != "view" {
		return ""
	}
	slug := parts[2]
	at := strings.LastIndex(slug, "-at-")
	if at < 0 {
		return ""
	}
	companySlug := slug[at+len("-at-"):]
	companySlug = regexp.MustCompile(`-\d+$`).ReplaceAllString(companySlug, "")
	return titleCaseSlug(companySlug)
}

func companyNameFromGreenhouseURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) == 0 || parts[0] == "" || parts[0] == "jobs" {
		return ""
	}
	return titleCaseSlug(parts[0])
}

func probeSiteSearchCandidates(ctx context.Context, browser *rod.Browser, targetURL string, criteria *CriteriaConfig) ([]siteSearchCandidate, error) {
	var candidates []siteSearchCandidate
	var probeErr error

	err := rod.Try(func() {
		timeout := siteSearchBrowserTimeout
		if deadline, ok := ctx.Deadline(); ok {
			if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
				timeout = remaining
			}
		}

		withReusableBrowserPage(ctx, browser, timeout, targetURL, func(page *rod.Page) {
			waitBrowserPageReady(page, siteSearchSettleDelay)

			info := page.MustInfo()
			baseURL, err := url.Parse(info.URL)
			if err != nil {
				baseURL, err = url.Parse(targetURL)
				if err != nil {
					panic(err)
				}
			}

			// Execute a single JS evaluation to extract all anchors and their contextual
			// parent text to avoid accumulating hundreds of CDP element references.
			res, err := page.Eval(`() => {
			const maxAnchors = 500;
			const maxTitleLength = 300;
			const maxContextLength = 700;
			const truncateInline = (value, max) => {
				value = (value || "").replace(/\s+/g, " ").trim();
				if (value.length <= max) return value;
				return value.slice(0, max);
			};
			const truncateContext = (value, max) => {
				value = (value || "")
					.split(/\n+/)
					.map((line) => line.replace(/[ \t\r\f\v]+/g, " ").trim())
					.filter(Boolean)
					.join("\n");
				if (value.length <= max) return value;
				return value.slice(0, max);
			};
			const contextForNode = (node) => {
				if (!node) return "";
				let text = truncateContext(node.innerText || node.textContent || "", maxContextLength);
				let logoText = Array.from(node.querySelectorAll("img[alt]"))
					.map((img) => truncateInline(img.getAttribute("alt") || "", maxTitleLength))
					.filter(Boolean)
					.join("\n");
				if (logoText) {
					text = text ? text + "\n" + logoText : logoText;
				}
				return text;
			};
				let results = [];
				const isLinkedIn = /(^|\.)linkedin\.com$/i.test(window.location.hostname || "");
				const isIndeed = /(^|\.)indeed\.com$/i.test(window.location.hostname || "");
				const normalizeURL = (value) => {
					try {
						if (!value) return "";
						return new URL(value, window.location.href).href;
					} catch (_) {
						return "";
					}
				};
				const indeedURLKey = (value) => {
					try {
						let parsed = new URL(value, window.location.href);
						if (!/(^|\.)indeed\.com$/i.test(parsed.hostname || "")) return "";
						parsed.searchParams.delete("jsa");
						return parsed.pathname + "?" + parsed.searchParams.toString();
					} catch (_) {
						return "";
					}
				};
				const indeedCanonicalByClick = new Map();
				const indeedCanonicalByKey = new Map();
				const addIndeedCanonical = (clickURL, canonicalURL) => {
					let canonical = normalizeURL(canonicalURL);
					if (!canonical) return;
					let clickKey = indeedURLKey(clickURL);
					if (clickKey) indeedCanonicalByClick.set(clickKey, canonical);
				};
				const addIndeedJob = (job) => {
					if (!job) return;
					let key = (job.key || "").trim();
					let canonical = normalizeURL(job.url || (key ? "/viewjob?jk=" + encodeURIComponent(key) : ""));
					if (!canonical) return;
					if (key) indeedCanonicalByKey.set(key, canonical);
					let clickURL = job.tracking && job.tracking.jobClick && job.tracking.jobClick.url;
					addIndeedCanonical(clickURL, canonical);
				};
				if (isIndeed) {
					try {
						let initial = window._initialData || {};
						let results = (((initial.hostQueryExecutionResult || {}).data || {}).jobData || {}).results || [];
						for (let result of results) addIndeedJob(result && result.job);
						let autoKey = ((initial.autoOpenJobAttributes || {}).jobKey || "").trim();
						let autoLink = (initial.autoOpenJobAttributes || {}).link || "";
						if (autoKey && indeedCanonicalByKey.has(autoKey)) {
							addIndeedCanonical(autoLink, indeedCanonicalByKey.get(autoKey));
						} else if (autoKey) {
							addIndeedCanonical(autoLink, "/viewjob?jk=" + encodeURIComponent(autoKey));
						}
					} catch (_) {
					}
				}
				let elements = document.querySelectorAll("a[href]");
				for (let i = 0; i < elements.length && results.length < maxAnchors; i++) {
					let el = elements[i];
					let title = truncateInline(el.innerText || el.textContent || "", maxTitleLength);
					let href = el.getAttribute("href") || "";
				let contexts = [];
				let seenContexts = new Set();
				let addContext = (node) => {
					let text = contextForNode(node);
					if (text && !seenContexts.has(text)) {
						seenContexts.add(text);
						contexts.push(text);
					}
				};
				let cardSelector = isLinkedIn
					? ".job-search-card, .base-search-card, .base-card"
					: '[data-jk], [data-testid*="job"], .job_seen_beacon, .cardOutline, .result, article, li';
				let card = el.closest(cardSelector);
				let hasCardContext = false;
				if (card && card !== document.body && card !== document.documentElement) {
					addContext(card);
					hasCardContext = true;
				}
				if (!isLinkedIn || !hasCardContext) {
					let curr = el.parentElement;
					for (let j = 0; j < 6; j++) {
						if (!curr) break;
						addContext(curr);
						curr = curr.parentElement;
					}
					}
					results.push({
						title: title,
						href: href,
						canonicalHref: isIndeed ? (indeedCanonicalByClick.get(indeedURLKey(href)) || "") : "",
						contexts: contexts
					});
				}
			const bodyText = (document.body && document.body.innerText || "")
				.replace(/[ \t\r\f\v]+/g, " ")
				.replace(/\n{3,}/g, "\n\n")
				.trim();
			return {
				title: document.title || "",
				bodyText: bodyText.slice(0, 2000),
				anchors: results
			};
			}`)
			if err != nil {
				panic(err)
			}

			pageTitle := res.Value.Get("title").Str()
			bodyText := res.Value.Get("bodyText").Str()
			anchors := res.Value.Get("anchors").Arr()
			if isSiteSearchVerificationPage(pageTitle, bodyText) {
				probeErr = fmt.Errorf("%w: %s", errSiteSearchVerificationRequired, siteSearchVerificationSummary(pageTitle, bodyText))
				logDebug("site search probe %s: final_url=%s verification required title=%q body=%q", targetURL, baseURL.String(), pageTitle, verificationDebugLine(bodyText))
				return
			}

			raw := make([]siteSearchCandidate, 0, len(anchors))
			emptyLinks := 0
			builtInNonJobLinks := 0
			nonJobLinks := 0
			lowScoreLinks := 0
			badIdentityLinks := 0

			for _, item := range anchors {
				title := normalizeWhitespace(item.Get("title").Str())
				href := strings.TrimSpace(item.Get("href").Str())
				canonicalHref := strings.TrimSpace(item.Get("canonicalHref").Str())

				if title == "" || href == "" {
					emptyLinks++
					continue
				}

				resolved, err := baseURL.Parse(href)
				if err != nil {
					continue
				}
				if isGoogleHost(baseURL.Hostname()) {
					resolved = unwrapGoogleSearchResultURL(resolved)
				}
				if isBuiltInHost(baseURL.Hostname()) && !isBuiltInJobCandidate(title, resolved) {
					builtInNonJobLinks++
					continue
				}
				if !isSiteSearchDirectJobCandidate(baseURL.Hostname(), title, resolved) {
					nonJobLinks++
					continue
				}
				applyURL, ok := siteSearchCandidateApplyURL(baseURL.Hostname(), resolved, canonicalHref)
				if !ok {
					nonJobLinks++
					continue
				}

				score := scoreSiteSearchCandidate(title, applyURL, criteria)
				if score <= 0 {
					lowScoreLinks++
					continue
				}

				var contexts []string
				for _, c := range item.Get("contexts").Arr() {
					contexts = append(contexts, c.Str())
				}

				company := ""
				if c := inferCompanyFromSiteSearchURL(resolved.String()); c != "" {
					company = c
				} else {
					for _, text := range contexts {
						if c := inferCompanyFromCandidateContext(baseURL.Hostname(), title, text); c != "" {
							company = c
							break
						}
					}
				}

				if siteSearchCandidateMissingRequiredIdentity(baseURL.Hostname(), company, applyURL) {
					badIdentityLinks++
					continue
				}

				raw = append(raw, siteSearchCandidate{
					Title:       title,
					Company:     company,
					URL:         applyURL,
					Description: siteSearchCandidateDescription(title, company, contexts),
					Score:       score,
				})
			}
			logDebug(
				"site search probe %s: final_url=%s anchors=%d raw_candidates=%d skipped_empty=%d skipped_builtin_non_job=%d skipped_non_job=%d skipped_low_score=%d skipped_bad_identity=%d",
				targetURL,
				baseURL.String(),
				len(anchors),
				len(raw),
				emptyLinks,
				builtInNonJobLinks,
				nonJobLinks,
				lowScoreLinks,
				badIdentityLinks,
			)

			candidates = dedupeSiteSearchCandidates(raw)
			sort.SliceStable(candidates, func(i, j int) bool {
				if candidates[i].Score == candidates[j].Score {
					return candidates[i].Title < candidates[j].Title
				}
				return candidates[i].Score > candidates[j].Score
			})

			if len(candidates) > 50 {
				candidates = candidates[:50]
			}
		})
	})
	if err != nil {
		return nil, simplifySiteSearchError(err)
	}
	if probeErr != nil {
		return nil, probeErr
	}

	return candidates, nil
}

func isSiteSearchVerificationPage(title string, body string) bool {
	combined := strings.ToLower(title + "\n" + body)
	if strings.Contains(combined, "additional verification required") {
		return true
	}
	if strings.Contains(combined, "cloudflare") && strings.Contains(combined, "ray id") {
		return true
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(title)), "just a moment") &&
		(strings.Contains(combined, "verify") || strings.Contains(combined, "verification")) {
		return true
	}
	return false
}

func siteSearchVerificationSummary(title string, body string) string {
	title = normalizeWhitespace(title)
	line := verificationDebugLine(body)
	if title == "" {
		return line
	}
	if line == "" {
		return title
	}
	return title + ": " + line
}

func verificationDebugLine(text string) string {
	lines := normalizedDebugLines(text)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "additional verification") ||
			strings.Contains(lower, "cloudflare") ||
			strings.Contains(lower, "ray id") {
			return truncateDebugLine(line)
		}
	}
	if len(lines) > 0 {
		return truncateDebugLine(lines[0])
	}
	return ""
}

func normalizedDebugLines(text string) []string {
	lines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		line = normalizeWhitespace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func truncateDebugLine(line string) string {
	if len(line) > 160 {
		return line[:160]
	}
	return line
}

func siteSearchCandidateMissingRequiredIdentity(baseHost string, company string, rawURL string) bool {
	if !siteSearchCompanyMissingOrInvalid(company) {
		return false
	}
	if !siteSearchCompanyMissingOrInvalid(inferCompanyFromSiteSearchURL(rawURL)) {
		return false
	}
	return siteSearchCandidateRequiresCompanyAtProbe(baseHost, rawURL)
}

func siteSearchCandidateRequiresCompanyAtProbe(baseHost string, rawURL string) bool {
	baseHost = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(baseHost), "www."))
	if isIndeedHost(baseHost) || isLinkedInHost(baseHost) {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	return isIndeedHost(host) || isLinkedInHost(host)
}

func siteSearchCandidateDescription(title string, company string, contexts []string) string {
	best := ""
	bestScore := 0
	seen := make(map[string]bool, len(contexts))
	for _, context := range contexts {
		context = normalizeWhitespace(context)
		if context == "" || seen[context] {
			continue
		}
		seen[context] = true
		if strings.EqualFold(context, title) {
			continue
		}
		if siteSearchCandidateContextTooBroad(context) {
			continue
		}

		lower := strings.ToLower(context)
		score := 1
		if title != "" && strings.Contains(lower, strings.ToLower(title)) {
			score += 2
		}
		if company != "" && !siteSearchCompanyMissingOrInvalid(company) && strings.Contains(lower, strings.ToLower(company)) {
			score += 2
		}
		signals := detectWorkSettingSignals(context)
		if signals.remote || signals.hybrid || signals.onsite {
			score++
		}
		if score > bestScore || (score == bestScore && (best == "" || len(context) < len(best))) {
			best = context
			bestScore = score
		}
	}
	return truncateAtSentence(best, 1200)
}

func siteSearchCandidateContextTooBroad(context string) bool {
	lower := strings.ToLower(context)
	return strings.Contains(lower, "get notified about new") ||
		strings.Contains(lower, "sign in to create job alert") ||
		strings.Contains(lower, "linkedin ©") ||
		strings.Contains(lower, "privacy policy") && strings.Contains(lower, "user agreement")
}

func simplifySiteSearchError(err error) error {
	if err == nil {
		return nil
	}

	message := strings.TrimSpace(err.Error())
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(message, "context.deadlineExceededError") || strings.Contains(message, "context deadline exceeded") {
		return fmt.Errorf("browser page timed out")
	}
	if match := siteSearchNetworkErrorPattern.FindString(message); match != "" {
		return fmt.Errorf("navigation failed: %s", match)
	}
	if idx := strings.IndexByte(message, '\n'); idx >= 0 {
		message = message[:idx]
	}
	return fmt.Errorf("%s", message)
}

func SimplifySiteSearchError(err error) error {
	return simplifySiteSearchError(err)
}

func scoreSiteSearchCandidate(title string, candidateURL string, criteria *CriteriaConfig) int {
	titleLower := strings.ToLower(title)
	urlLower := strings.ToLower(candidateURL)
	score := 0

	if strings.Contains(urlLower, "/job/") || strings.Contains(urlLower, "/jobs/view/") {
		score += 10
	}
	if strings.Contains(urlLower, "/viewjob") || strings.Contains(urlLower, "/rc/clk") || strings.Contains(urlLower, "/pagead/clk") {
		score += 10
	}
	if strings.Contains(urlLower, "/companies/") && strings.Contains(urlLower, "/jobs/") {
		score += 10
	}
	if strings.Contains(urlLower, "/remote-jobs/") {
		score += 8
	}
	if strings.Contains(urlLower, "/jobs/") {
		score += 4
	}
	if strings.Contains(urlLower, "careers") || strings.Contains(urlLower, "apply") {
		score += 2
	}
	if looksLikeJobTitle(titleLower) {
		score += 3
	}
	if strings.Contains(titleLower, "jobs in ") || strings.HasSuffix(titleLower, " jobs") {
		score -= 3
	}

	if criteria != nil {
		if len(criteria.Filters.TitleExcludes) > 0 {
			for _, exclude := range criteria.Filters.TitleExcludes {
				if strings.Contains(titleLower, strings.ToLower(exclude)) {
					score -= 6
				}
			}
		}
		if len(criteria.Filters.TitleIncludes) > 0 {
			matched := false
			for _, include := range criteria.Filters.TitleIncludes {
				include = strings.TrimSpace(strings.ToLower(include))
				if include == "" {
					continue
				}
				if strings.Contains(titleLower, include) || strings.Contains(urlLower, slugify(include)) {
					score += 4
					matched = true
				}
			}
			if !matched {
				score -= 2
			}
		}
		if len(criteria.Filters.TitleRequires) > 0 {
			matched := false
			for _, require := range criteria.Filters.TitleRequires {
				if strings.Contains(titleLower, strings.ToLower(require)) {
					matched = true
					break
				}
			}
			if matched {
				score += 6
			}
		}
		settings := domain.SelectedWorkSettings(criteria.Filters.WorkSettings)
		if len(settings) > 0 {
			matchedSetting := false
			for _, setting := range settings {
				if strings.Contains(titleLower, setting) || strings.Contains(urlLower, setting) {
					score += 2
					matchedSetting = true
				}
			}
			if !matchedSetting && criteria.Filters.WorkSettings.Hybrid {
				score -= 1
			}
		}
	}

	return score
}

func dedupeSiteSearchCandidates(candidates []siteSearchCandidate) []siteSearchCandidate {
	seen := make(map[string]siteSearchCandidate, len(candidates))
	order := make([]string, 0, len(candidates))

	for _, candidate := range candidates {
		existing, ok := seen[candidate.URL]
		if ok {
			if candidate.Score > existing.Score {
				seen[candidate.URL] = candidate
			}
			continue
		}
		seen[candidate.URL] = candidate
		order = append(order, candidate.URL)
	}

	out := make([]siteSearchCandidate, 0, len(order))
	for _, key := range order {
		out = append(out, seen[key])
	}
	return out
}

func inferWorkSetting(text string, criteria *CriteriaConfig) string {
	signals := detectWorkSettingSignals(text)

	if criteria != nil {
		settings := criteria.Filters.WorkSettings
		switch {
		case signals.remote && settings.Remote:
			return "Remote"
		case signals.hybrid && settings.Hybrid:
			return "Hybrid"
		case signals.onsite && settings.Onsite:
			return "Onsite"
		}
	}

	switch {
	case signals.remote:
		return "Remote"
	case signals.hybrid:
		return "Hybrid"
	case signals.onsite:
		return "Onsite"
	case criteria != nil && criteria.Filters.WorkSettings.Remote:
		return "Remote"
	case criteria != nil && criteria.Filters.WorkSettings.Hybrid:
		return "Hybrid"
	case criteria != nil && criteria.Filters.WorkSettings.Onsite:
		return "Onsite"
	default:
		return ""
	}
}

func looksLikeJobTitle(text string) bool {
	keywords := []string{
		"engineer",
		"developer",
		"architect",
		"manager",
		"designer",
		"scientist",
		"analyst",
		"administrator",
		"devops",
		"sre",
		"platform",
		"security",
		"data",
		"product",
		"frontend",
		"backend",
		"software",
		"full stack",
		"fullstack",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func NormalizeWhitespace(value string) string {
	return normalizeWhitespace(value)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func DerefString(value *string) string {
	return derefString(value)
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "/", "-", "_", "-", ",", "")
	return replacer.Replace(value)
}

func Slugify(value string) string {
	return slugify(value)
}
