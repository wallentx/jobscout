package fetcher

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wallentx/jobscout/internal/domain"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
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
	Title   string
	Company string
	URL     string
	Score   int
}

var siteSearchNetworkErrorPattern = regexp.MustCompile(`net::[A-Z_]+`)
var cityStateLinePattern = regexp.MustCompile(`^[A-Za-z .'-]+,\s*[A-Z]{2}(?:\b|$)`)
var relativeTimeLinePattern = regexp.MustCompile(`(?i)^\d+\s+(minute|hour|day|week|month)s?\s+ago$`)
var siteSearchBrowserLookPath = launcher.LookPath

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
	return ""
}

func FindSiteSearchBrowserBinary() string {
	return findSiteSearchBrowserBinary()
}

func fetchGenericSiteSearch(ctx context.Context, browser *rod.Browser, target string, targetURL string, sourceName string, criteria *CriteriaConfig) ([]Job, map[string][]Job, error) {
	candidates, err := probeSiteSearchCandidates(ctx, browser, targetURL, criteria)
	if err != nil {
		logDebug("site search %s: browser candidate probe failed: %v", target, err)
		return nil, nil, err
	}
	logDebug("site search %s: browser candidate probe returned %d candidates", target, len(candidates))

	jobs := make([]Job, 0, len(candidates))
	filtered := make(map[string][]Job)
	seen := make(map[string]bool)

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
			Remote:       inferWorkSetting(candidate.Title+" "+candidate.URL, criteria),
			Compensation: "Not listed",
			Description:  candidate.URL,
		}
		job.SetDateAdded(time.Now().Unix())
		enrichJobFromDescription(&job)
		enrichSiteSearchJobIdentityBeforeFilter(ctx, &job)
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
	return jobs, filtered, nil
}

func enrichSiteSearchJobIdentityBeforeFilter(ctx context.Context, job *Job) {
	if job == nil || !siteSearchJobNeedsPreFilterDetail(*job) {
		return
	}
	rawHTML, finalURL, err := fetchApplyPage(ctx, job.ApplyURL)
	if err != nil || strings.TrimSpace(rawHTML) == "" {
		if err != nil {
			logDebug("site search pre-filter identity %s: fetch failed: %v", job.ApplyURL, err)
		} else {
			logDebug("site search pre-filter identity %s: empty detail page", job.ApplyURL)
		}
		return
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
	if isIndeedURL(job.ApplyURL) {
		return false
	}
	return siteSearchJobNeedsPreFilterIdentity(job) ||
		jobDescriptionMissingOrURL(job.Description) ||
		jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) ||
		jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) ||
		jobCompanyIndustryNeedsEnrichment(job) ||
		jobCompensationMissing(job.Compensation)
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

func fetchBuiltInSiteSearch(ctx context.Context, targetURL string, sourceName string, criteria *CriteriaConfig, detailCache *builtInDetailCache, profileEnricher *sourceProfileEnricher, existing *existingJobIndex) ([]Job, map[string][]Job, bool, error) {
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
	return parseBuiltInSiteSearchHTML(ctx, rawHTML, finalURL, sourceName, criteria, detailCache, profileEnricher, existing)
}

func parseBuiltInSiteSearchHTML(ctx context.Context, rawHTML string, finalURL string, sourceName string, criteria *CriteriaConfig, detailCache *builtInDetailCache, profileEnricher *sourceProfileEnricher, existing *existingJobIndex) ([]Job, map[string][]Job, bool, error) {
	hrefs := extractHTMLHrefs(rawHTML)
	listingJobs, cardCount := extractBuiltInListingJobs(rawHTML, finalURL, sourceName, criteria)
	if cardCount > 0 {
		acceptedListingJobs, cardFiltered := filterBuiltInListingJobsWithProfiles(listingJobs, criteria)
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
	filtered := make(map[string][]Job)
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

	prefixCount := len(prefixes)
	titleCount := len(titles)
	queryCap := 0
	maxInt := int(^uint(0) >> 1)
	if titleCount > 0 && prefixCount <= maxInt/titleCount {
		queryCap = prefixCount * titleCount
	}

	queries := make([]string, 0, queryCap)
	seen := make(map[string]bool)
	for _, prefix := range prefixes {
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

func isIndeedURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	return isIndeedHost(parsed.Hostname())
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
	case host == "himalayas.app":
		return isHimalayasJobURL(rawURL)
	case isBuiltInHost(host):
		return isBuiltInJobCandidate(title, resolved)
	case isSharedATSDirectoryHost(host):
		return !isSharedATSDirectoryHost(baseHost)
	default:
		return true
	}
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

func isHimalayasJobURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	path := strings.ToLower(parsed.EscapedPath())
	return host == "himalayas.app" && strings.HasPrefix(path, "/companies/") && strings.Contains(path, "/jobs/")
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
	case isHimalayasJobURL(rawURL):
		return companyNameFromPathSegment(rawURL, "companies")
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

func companyNameFromPathSegment(rawURL string, marker string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	for i, part := range parts {
		if part == marker && i+1 < len(parts) {
			return titleCaseSlug(parts[i+1])
		}
	}
	return ""
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

		page := browser.Timeout(timeout).MustPage(targetURL)
		defer page.MustClose()

		page.MustWaitLoad()
		time.Sleep(siteSearchSettleDelay)

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
				let card = el.closest('[data-jk], [data-testid*="job"], .job_seen_beacon, .cardOutline, .result, article, li');
				if (card && card !== document.body && card !== document.documentElement) {
					addContext(card);
				}
				let curr = el.parentElement;
				for (let j = 0; j < 6; j++) {
					if (!curr) break;
					addContext(curr);
					curr = curr.parentElement;
				}
				results.push({
					title: title,
					href: href,
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

			score := scoreSiteSearchCandidate(title, resolved.String(), criteria)
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

			if siteSearchCandidateMissingRequiredIdentity(baseURL.Hostname(), company, resolved.String()) {
				badIdentityLinks++
				continue
			}

			raw = append(raw, siteSearchCandidate{
				Title:   title,
				Company: company,
				URL:     resolved.String(),
				Score:   score,
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

func simplifySiteSearchError(err error) error {
	if err == nil {
		return nil
	}

	message := strings.TrimSpace(err.Error())
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
