package fetcher

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
)

const maxConcurrentPublicProfileIndustry = 4
const publicProfileSearchFetchTimeout = 5 * time.Second
const publicProfilePageFetchTimeout = 4 * time.Second

type publicProfileIndustryTarget struct {
	Key     string
	Job     Job
	Indexes []int
}

type publicProfileIndustryCandidate struct {
	Source  string
	Title   string
	URL     string
	Snippet string
	Score   int
}

type publicProfileIndustryResult struct {
	Industry string
	Source   string
	URL      string
}

var (
	publicProfileSearchURLFunc = companySearchURL
	fetchPublicProfileHTML     = fetchApplyPage
)

func enrichJobsFromPublicProfileIndustryWithProgress(ctx context.Context, jobs []Job, progress func(string)) []Job {
	targets := publicProfileIndustryTargets(jobs)
	if len(targets) == 0 {
		logDebug("public profile industry: skipped; no eligible companies among %d jobs", len(jobs))
		return jobs
	}

	start := time.Now()
	logDebug("public profile industry: start targets=%d jobs=%d concurrency=%d", len(targets), len(jobs), maxConcurrentPublicProfileIndustry)
	reportFetchProgress(progress, "Checking public company profiles for %d companies with missing industry...", len(targets))
	results := make([]*publicProfileIndustryResult, len(targets))
	sem := make(chan struct{}, maxConcurrentPublicProfileIndustry)
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	completed := 0

	for i, target := range targets {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(idx int, target publicProfileIndustryTarget) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			results[idx] = discoverPublicProfileIndustry(ctx, target.Job)
			if results[idx] == nil {
				logDebug("public profile industry: no industry found company=%q website=%q indexes=%d", target.Job.Company, target.Job.CompanyWebsite, len(target.Indexes))
			} else {
				logDebug("public profile industry: found company=%q industry=%q source=%q url=%q indexes=%d", target.Job.Company, results[idx].Industry, results[idx].Source, results[idx].URL, len(target.Indexes))
			}
			progressMu.Lock()
			completed++
			if completed == len(targets) || completed%10 == 0 {
				reportFetchProgress(progress, "Public profile industry lookup checked %d of %d companies...", completed, len(targets))
			}
			progressMu.Unlock()
		}(i, target)
	}
	wg.Wait()

	for i, result := range results {
		if result == nil || strings.TrimSpace(result.Industry) == "" {
			continue
		}
		for _, idx := range targets[i].Indexes {
			applyPublicProfileIndustry(&jobs[idx], result)
		}
	}
	logDebug("public profile industry: complete targets=%d duration=%s", len(targets), time.Since(start).Round(time.Millisecond))
	return jobs
}

func EnrichJobsFromPublicProfileIndustryWithProgress(ctx context.Context, jobs []Job, progress func(string)) []Job {
	return enrichJobsFromPublicProfileIndustryWithProgress(ctx, jobs, progress)
}

func publicProfileIndustryTargets(jobs []Job) []publicProfileIndustryTarget {
	targets := make([]publicProfileIndustryTarget, 0)
	targetByKey := make(map[string]int)
	for i, job := range jobs {
		if !jobNeedsPublicProfileIndustry(job) {
			continue
		}
		key := publicProfileIndustryKey(job)
		if key == "" {
			continue
		}
		if existing, ok := targetByKey[key]; ok {
			targets[existing].Indexes = append(targets[existing].Indexes, i)
			continue
		}
		targetByKey[key] = len(targets)
		targets = append(targets, publicProfileIndustryTarget{
			Key:     key,
			Job:     job,
			Indexes: []int{i},
		})
	}
	return targets
}

func jobNeedsPublicProfileIndustry(job Job) bool {
	return !strings.EqualFold(strings.TrimSpace(job.Status), "Expired") &&
		strings.TrimSpace(job.Company) != "" &&
		!strings.EqualFold(strings.TrimSpace(job.Company), "Unknown") &&
		!isKnownNonJobApplyURL(job.ApplyURL) &&
		!jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) &&
		jobCompanyIndustryNeedsEnrichment(job)
}

func publicProfileIndustryKey(job Job) string {
	company := browserCompanySearchKey(job.Company)
	domain := companyWebsiteDomainForSearch(job.CompanyWebsite)
	if company == "" || domain == "" {
		return ""
	}
	return company + "|" + domain
}

func discoverPublicProfileIndustry(ctx context.Context, job Job) *publicProfileIndustryResult {
	for _, query := range publicProfileIndustryQueries(job) {
		if err := ctx.Err(); err != nil {
			logDebug("public profile industry: stopped company=%q query=%q error=%v", job.Company, query, err)
			return nil
		}
		searchURL := publicProfileSearchURLFunc(query)
		logDebug("public profile industry: query company=%q query=%q url=%q", job.Company, query, searchURL)
		searchCtx, cancel := context.WithTimeout(ctx, publicProfileSearchFetchTimeout)
		rawHTML, finalURL, err := fetchPublicProfileHTML(searchCtx, searchURL)
		cancel()
		if err != nil || strings.TrimSpace(rawHTML) == "" {
			if err != nil {
				logDebug("public profile industry: query failed company=%q query=%q error=%v", job.Company, query, err)
			} else {
				logDebug("public profile industry: query empty company=%q query=%q final_url=%q", job.Company, query, finalURL)
			}
			continue
		}
		pageText := normalizeHTMLText(rawHTML)
		candidates := publicProfileIndustryCandidates(job, extractPageLinksFromHTML(rawHTML, finalURL), pageText)
		logDebug("public profile industry: query candidates company=%q query=%q candidates=%d final_url=%q", job.Company, query, len(candidates), finalURL)
		profileFetches := 0
		for _, candidate := range candidates {
			if industry := extractPublicProfileIndustryFromText(candidate.Snippet); industry != "" {
				logDebug("public profile industry: snippet hit company=%q source=%q industry=%q url=%q", job.Company, candidate.Source, industry, candidate.URL)
				return &publicProfileIndustryResult{Industry: industry, Source: candidate.Source, URL: candidate.URL}
			}
			if profileFetches >= 1 {
				continue
			}
			profileFetches++
			profileCtx, cancel := context.WithTimeout(ctx, publicProfilePageFetchTimeout)
			profileHTML, profileURL, err := fetchPublicProfileHTML(profileCtx, candidate.URL)
			cancel()
			if err != nil || strings.TrimSpace(profileHTML) == "" {
				if err != nil {
					logDebug("public profile industry: profile fetch failed company=%q source=%q url=%q error=%v", job.Company, candidate.Source, candidate.URL, err)
				} else {
					logDebug("public profile industry: profile fetch empty company=%q source=%q url=%q final_url=%q", job.Company, candidate.Source, candidate.URL, profileURL)
				}
				continue
			}
			profileText := normalizeHTMLText(profileHTML)
			if !publicProfileMatchesCompany(job, profileText, profileURL) {
				logDebug("public profile industry: profile rejected company=%q source=%q url=%q reason=company_mismatch", job.Company, candidate.Source, profileURL)
				continue
			}
			if industry := extractPublicProfileIndustryFromText(profileText); industry != "" {
				logDebug("public profile industry: profile hit company=%q source=%q industry=%q url=%q", job.Company, candidate.Source, industry, profileURL)
				return &publicProfileIndustryResult{Industry: industry, Source: candidate.Source, URL: profileURL}
			}
			logDebug("public profile industry: profile matched without industry company=%q source=%q url=%q", job.Company, candidate.Source, profileURL)
		}
	}
	return nil
}

func publicProfileIndustryQueries(job Job) []string {
	company := CleanCompanyName(job.Company)
	if strings.TrimSpace(company) == "" {
		company = strings.TrimSpace(job.Company)
	}
	domain := companyWebsiteDomainForSearch(job.CompanyWebsite)
	if domain == "" {
		return nil
	}
	return []string{
		fmt.Sprintf(`"%s" "%s" industry (LinkedIn OR Glassdoor OR Indeed) company profile`, company, domain),
	}
}

func companyWebsiteDomainForSearch(companyWebsite string) string {
	parsed, err := url.Parse(strings.TrimSpace(companyWebsite))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return companyHostRoot(parsed.Hostname())
}

func publicProfileIndustryCandidates(job Job, links []pageLink, pageText string) []publicProfileIndustryCandidate {
	candidates := make([]publicProfileIndustryCandidate, 0, len(links))
	for _, link := range links {
		source := publicProfileSource(link.URL)
		if source == "" {
			continue
		}
		snippet := publicProfileSnippet(pageText, link)
		score := scorePublicProfileIndustryCandidate(job, source, link, snippet)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, publicProfileIndustryCandidate{
			Source:  source,
			Title:   NormalizeWhitespace(link.Text),
			URL:     link.URL,
			Snippet: snippet,
			Score:   score,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].URL < candidates[j].URL
		}
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return candidates
}

func publicProfileSource(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "linkedin.com" || strings.HasSuffix(host, ".linkedin.com"):
		if strings.Contains(strings.ToLower(parsed.EscapedPath()), "/company/") {
			return "linkedin"
		}
	case host == "glassdoor.com" || strings.HasSuffix(host, ".glassdoor.com"):
		return "glassdoor"
	case host == "indeed.com" || strings.HasSuffix(host, ".indeed.com"):
		if strings.Contains(strings.ToLower(parsed.EscapedPath()), "/cmp/") {
			return "indeed"
		}
	}
	return ""
}

func publicProfileSnippet(pageText string, link pageLink) string {
	pageText = NormalizeWhitespace(pageText)
	title := NormalizeWhitespace(link.Text)
	if pageText == "" {
		return title
	}
	index := -1
	for _, needle := range []string{title, link.URL} {
		needle = strings.TrimSpace(needle)
		if needle == "" {
			continue
		}
		index = strings.Index(strings.ToLower(pageText), strings.ToLower(needle))
		if index >= 0 {
			break
		}
	}
	if index < 0 {
		return title
	}
	start := max(0, index-120)
	end := min(len(pageText), index+520)
	return strings.TrimSpace(pageText[start:end])
}

func scorePublicProfileIndustryCandidate(job Job, source string, link pageLink, snippet string) int {
	if !publicProfileMatchesCompany(job, strings.Join([]string{link.Text, snippet}, " "), link.URL) {
		return 0
	}
	parsed, err := url.Parse(strings.TrimSpace(link.URL))
	if err != nil || parsed.Host == "" {
		return 0
	}
	pathLower := strings.ToLower(parsed.EscapedPath())
	textLower := strings.ToLower(strings.Join([]string{link.Text, snippet, link.URL}, " "))

	score := 5
	switch source {
	case "linkedin":
		score += 3
	case "glassdoor", "indeed":
		score += 2
	}
	if extractPublicProfileIndustryFromText(snippet) != "" {
		score += 5
	}
	if strings.Contains(pathLower, "/jobs") || strings.Contains(pathLower, "/job") || strings.Contains(textLower, "jobs at ") {
		score -= 6
	}
	if strings.Contains(pathLower, "reviews") || strings.Contains(textLower, "reviews") {
		score -= 1
	}
	return score
}

func publicProfileMatchesCompany(job Job, text string, rawURL string) bool {
	combined := strings.ToLower(strings.Join([]string{text, rawURL}, " "))
	urlSlug := strings.ReplaceAll(Slugify(rawURL), "-", "")
	for _, token := range publicProfileCompanyTokens(job.Company) {
		if strings.Contains(combined, token) || strings.Contains(urlSlug, token) {
			return true
		}
	}
	domain := companyWebsiteDomainForSearch(job.CompanyWebsite)
	domainToken := strings.Split(domain, ".")[0]
	domainToken = strings.ReplaceAll(Slugify(domainToken), "-", "")
	return len(domainToken) >= 4 && (strings.Contains(combined, domainToken) || strings.Contains(urlSlug, domainToken))
}

func publicProfileCompanyTokens(company string) []string {
	tokens := companySearchTokens(company)
	if len(tokens) > 0 {
		return tokens
	}
	cleaned := strings.ReplaceAll(Slugify(CleanCompanyName(company)), "-", "")
	if len(cleaned) >= 2 {
		return []string{cleaned}
	}
	return nil
}

func extractPublicProfileIndustryFromText(text string) string {
	text = NormalizeWhitespace(text)
	if text == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bIndustries\s*[:\-]\s*([A-Za-z][A-Za-z0-9 &,/+\-]{2,80})`),
		regexp.MustCompile(`(?i)\bIndustry\s*[:\-]\s*([A-Za-z][A-Za-z0-9 &,/+\-]{2,80})`),
		regexp.MustCompile(`(?i)\bIndustries\s+([A-Za-z][A-Za-z0-9 &,/+\-]{2,80}?)(?:\s+(?:Company size|Headquarters|Founded|Type|Specialties|Website|Revenue|Employees|Overview|Reviews|Jobs|Salaries|Benefits|Photos|Questions|Interviews|Locations)\b|$)`),
		regexp.MustCompile(`(?i)\bIndustry\s+([A-Za-z][A-Za-z0-9 &,/+\-]{2,80}?)(?:\s+(?:Company size|Headquarters|Founded|Type|Specialties|Website|Revenue|Employees|Overview|Reviews|Jobs|Salaries|Benefits|Photos|Questions|Interviews|Locations)\b|$)`),
		regexp.MustCompile(`(?i)\bLinkedIn\s+([A-Za-z][A-Za-z0-9 &,/+\-|]{2,80}?)(?:\s+Company size\b|$)`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		industry := cleanPublicProfileIndustry(match[1])
		if looksLikeCompanyIndustry(industry) {
			return industry
		}
	}
	return ""
}

func cleanPublicProfileIndustry(industry string) string {
	industry = strings.TrimSpace(industry)
	if strings.Contains(industry, "|") {
		parts := strings.Split(industry, "|")
		industry = strings.TrimSpace(parts[len(parts)-1])
	}
	industry = strings.TrimPrefix(industry, "LinkedIn ")
	for _, stop := range []string{
		" Company size",
		" Headquarters",
		" Founded",
		" Type",
		" Specialties",
		" Website",
		" Revenue",
		" Employees",
		" Overview",
		" Reviews",
		" Jobs",
		" Salaries",
		" Benefits",
		" Photos",
		" Questions",
		" Interviews",
		" Locations",
		" followers",
		" employees",
	} {
		if idx := strings.Index(strings.ToLower(industry), strings.ToLower(stop)); idx >= 0 {
			industry = industry[:idx]
		}
	}
	return strings.Trim(industry, " .•|:-")
}

func applyPublicProfileIndustry(job *Job, result *publicProfileIndustryResult) {
	if job == nil || result == nil || !jobCompanyIndustryNeedsEnrichment(*job) {
		return
	}
	industry := strings.TrimSpace(result.Industry)
	if !looksLikeCompanyIndustry(industry) {
		return
	}
	job.CompanyIndustry = industry
	setJobIdentityEvidence(job, "industry", industry, "public_profile_"+result.Source, result.URL, "medium", false, "Industry extracted from a public company profile.")
}

func companyPublicProfileEvidenceFromText(source string, profileURL string, title string, text string) domain.CompanyPublicProfile {
	text = NormalizeWhitespace(strings.Join([]string{title, text}, " "))
	evidence := domain.CompanyPublicProfile{
		Source:  strings.TrimSpace(strings.ToLower(source)),
		URL:     strings.TrimSpace(profileURL),
		Title:   NormalizeWhitespace(title),
		Snippet: truncatePublicProfileSnippet(text),
	}
	evidence.Industry = extractPublicProfileIndustryFromText(text)
	if employeeRange, estimate := extractPublicProfileEmployeeRange(text); employeeRange != "" {
		evidence.EmployeeRange = employeeRange
		evidence.EstimatedEmployees = estimate
	}
	evidence.FoundedYear = extractPublicProfileFoundedYear(text)
	evidence.Headquarters = extractPublicProfileLabeledField(text, "Headquarters")
	evidence.Revenue = extractPublicProfileLabeledField(text, "Revenue")
	evidence.Rating = extractEmployerReviewRating(text)
	evidence.ReviewCount = extractPublicProfileReviewCount(text)
	evidence.RecommendPercent = extractPublicProfilePercent(text, []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(\d{1,3})%\s+(?:recommend|would recommend|recommend to a friend)\b`),
	})
	evidence.CEOApprovalPercent = extractPublicProfilePercent(text, []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(\d{1,3})%\s+(?:approve of ceo|ceo approval|approve)\b`),
	})
	return evidence
}

func companyPublicProfileEvidenceUseful(evidence domain.CompanyPublicProfile) bool {
	return strings.TrimSpace(evidence.Industry) != "" ||
		strings.TrimSpace(evidence.EmployeeRange) != "" ||
		evidence.EstimatedEmployees != nil ||
		evidence.FoundedYear != nil ||
		strings.TrimSpace(evidence.Headquarters) != "" ||
		strings.TrimSpace(evidence.Revenue) != "" ||
		strings.TrimSpace(evidence.Rating) != "" ||
		evidence.ReviewCount != nil ||
		evidence.RecommendPercent != nil ||
		evidence.CEOApprovalPercent != nil
}

func extractPublicProfileEmployeeRange(text string) (string, *int) {
	text = NormalizeWhitespace(text)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:company size|size)\s*[:\-]?\s*([0-9][0-9,.k+\s]*(?:(?:-|–|—|to)\s*[0-9][0-9,.k+\s]*)?\s*employees?)\b`),
		regexp.MustCompile(`(?i)\b([0-9][0-9,.k+\s]*(?:(?:-|–|—|to)\s*[0-9][0-9,.k+\s]*)?\s*employees?)\b`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		employeeRange := strings.TrimSpace(match[1])
		if employeeRange == "" {
			continue
		}
		return employeeRange, estimateEmployeeCountFromRange(employeeRange)
	}
	return "", nil
}

func estimateEmployeeCountFromRange(employeeRange string) *int {
	matches := regexp.MustCompile(`(?i)[0-9][0-9,.]*(?:\s*k)?\+?`).FindAllString(employeeRange, -1)
	if len(matches) == 0 {
		return nil
	}
	values := make([]int, 0, len(matches))
	for _, match := range matches {
		if value, ok := parsePublicProfileNumber(match); ok {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil
	}
	count := values[0]
	if len(values) >= 2 {
		count = (values[0] + values[1]) / 2
	}
	return &count
}

func parsePublicProfileNumber(value string) (int, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, "+")
	multiplier := 1.0
	if strings.HasSuffix(value, "k") {
		multiplier = 1000
		value = strings.TrimSpace(strings.TrimSuffix(value, "k"))
	}
	value = strings.ReplaceAll(value, ",", "")
	value = strings.ReplaceAll(value, " ", "")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return int(parsed * multiplier), true
}

func extractPublicProfileFoundedYear(text string) *int {
	if year := domain.ParseYearFromText(text); year != nil {
		return year
	}
	match := regexp.MustCompile(`(?i)\bFounded\s*[:\-]?\s*(19\d{2}|20\d{2})\b`).FindStringSubmatch(text)
	if len(match) < 2 {
		return nil
	}
	year, err := strconv.Atoi(match[1])
	if err != nil {
		return nil
	}
	return &year
}

func extractPublicProfileLabeledField(text string, label string) string {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(label) + `\s*[:\-]?\s*([^|•]{2,120}?)(?:\s+(?:Company size|Size|Headquarters|Founded|Industry|Industries|Revenue|Type|Website|Specialties|Overview|Reviews|Jobs|Salaries|Benefits|Photos|Questions|Interviews|Locations)\b|$)`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return cleanPublicProfileValue(match[1])
}

func extractPublicProfileReviewCount(text string) *int {
	match := regexp.MustCompile(`(?i)\b([0-9][0-9,]*)\s+(?:reviews|review)\b`).FindStringSubmatch(text)
	if len(match) < 2 {
		return nil
	}
	value, err := strconv.Atoi(strings.ReplaceAll(match[1], ",", ""))
	if err != nil || value <= 0 {
		return nil
	}
	return &value
}

func extractPublicProfilePercent(text string, patterns []*regexp.Regexp) *int {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		value, err := strconv.Atoi(match[1])
		if err == nil && value >= 0 && value <= 100 {
			return &value
		}
	}
	return nil
}

func cleanPublicProfileValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), " .•|:-")
}

func truncatePublicProfileSnippet(text string) string {
	text = NormalizeWhitespace(text)
	if len(text) <= 520 {
		return text
	}
	return strings.TrimSpace(text[:520])
}
