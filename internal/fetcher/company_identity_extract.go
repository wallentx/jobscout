package fetcher

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"

	"github.com/PuerkitoBio/goquery"
)

func enrichJobIndustryFromExistingSummary(job *Job) {
	if job == nil || strings.TrimSpace(job.CompanyIndustry) != "" || jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
		return
	}
	industry := inferCompanyIndustry(job.CompanySummary)
	if industry == "" {
		return
	}
	job.CompanyIndustry = industry
	evidenceURL := firstNonEmptyString(job.CompanyWebsite, job.ApplyURL)
	setJobIdentityEvidence(job, "industry", industry, "company_summary_inference", evidenceURL, "low", true, "Industry inferred from existing company summary text.")
}

func enrichJobFromCompanyWebsitePages(ctx context.Context, job *Job, llmEnrich jobIdentityPageEnrichFunc) {
	enrichJobFromCompanyWebsitePagesWithStats(ctx, job, llmEnrich, nil)
}

func enrichJobFromCompanyWebsitePagesWithStats(ctx context.Context, job *Job, llmEnrich jobIdentityPageEnrichFunc, stats *acceptedEnrichmentStats) {
	if job == nil || !jobNeedsCompanyPageEnrichment(*job) {
		return
	}
	if stats == nil {
		stats = newAcceptedEnrichmentStats(0)
	}
	homepageURL := canonicalCompanySiteURL(job.CompanyWebsite)
	if homepageURL == "" {
		homepageURL = strings.TrimSpace(job.CompanyWebsite)
	}
	if homepageURL == "" {
		return
	}

	stats.inc(&stats.companyHomepageAttempts)
	homepageHTML, finalURL, err := fetchApplyPage(ctx, homepageURL)
	if err != nil || strings.TrimSpace(homepageHTML) == "" {
		if err != nil {
			stats.inc(&stats.companyHomepageFailed)
			if isBlockedFetchError(err) {
				stats.inc(&stats.companyHomepageBlocked)
			}
		} else {
			stats.inc(&stats.companyHomepageEmpty)
		}
		return
	}
	stats.inc(&stats.companyHomepageSuccess)
	enrichJobFromCompanySiteHTML(job, homepageHTML, finalURL, "company_homepage")
	if jobNeedsLLMIdentityEnrichment(*job) {
		stats.addLLMUsage(applyLLMJobIdentityEnrichment(ctx, job, buildJobIdentityPage(homepageHTML, finalURL), llmEnrich, "llm_company_homepage"))
	}

	if !jobNeedsCompanyPageEnrichment(*job) {
		return
	}
	if aboutURL := chooseCompanyAboutURL(finalURL, extractPageLinksFromHTML(homepageHTML, finalURL)); aboutURL != "" {
		stats.inc(&stats.companyAboutAttempts)
		aboutHTML, aboutFinalURL, err := fetchApplyPage(ctx, aboutURL)
		if err == nil && strings.TrimSpace(aboutHTML) != "" {
			stats.inc(&stats.companyAboutSuccess)
			enrichJobFromCompanySiteHTML(job, aboutHTML, aboutFinalURL, "company_about")
			if jobNeedsLLMIdentityEnrichment(*job) {
				stats.addLLMUsage(applyLLMJobIdentityEnrichment(ctx, job, buildJobIdentityPage(aboutHTML, aboutFinalURL), llmEnrich, "llm_company_about"))
			}
		} else if err != nil {
			stats.inc(&stats.companyAboutFailed)
			if isBlockedFetchError(err) {
				stats.inc(&stats.companyAboutBlocked)
			}
		} else {
			stats.inc(&stats.companyAboutEmpty)
		}
	}
}

func enrichJobFromCompanySiteHTML(job *Job, rawHTML string, pageURL string, source string) {
	if job == nil || strings.TrimSpace(rawHTML) == "" {
		return
	}
	if jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
		if summary := extractCompanyProfileSummary(rawHTML, job.Company); summary != "" {
			job.CompanySummary = summary
			setJobIdentityEvidence(job, "summary", summary, source, pageURL, "high", false, "Company summary extracted from company site.")
		}
	}
	if explicitIndustry := extractExplicitCompanyIndustryFromHTML(rawHTML); explicitIndustry != "" {
		job.CompanyIndustry = explicitIndustry
		setJobIdentityEvidence(job, "industry", explicitIndustry, source, pageURL, "high", false, "Industry came from an explicit company-site label.")
	} else if jobCompanyIndustryNeedsEnrichment(*job) {
		if summaryIndustry := inferCompanyIndustry(job.CompanySummary); summaryIndustry != "" {
			job.CompanyIndustry = summaryIndustry
			setJobIdentityEvidence(job, "industry", summaryIndustry, source+"_inference", pageURL, "low", true, "Industry inferred from company summary text.")
		}
	}
}

func enrichJobFromCompanySiteText(ctx context.Context, job *Job, text string, pageURL string, source string, llmEnrich jobIdentityPageEnrichFunc) {
	if job == nil || strings.TrimSpace(text) == "" {
		return
	}
	page := JobIdentityPage{URL: pageURL, Text: text}
	if llmEnrich != nil {
		applyLLMJobIdentityEnrichment(ctx, job, page, llmEnrich, "llm_"+source)
	}
	if jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) && looksLikeCompanySummary(text, job.Company) {
		summary := truncateAtSentence(text, 420)
		job.CompanySummary = summary
		setJobIdentityEvidence(job, "summary", summary, source, pageURL, "medium", false, "Company summary extracted from company site text.")
	}
	enrichJobFromCompanySiteHTML(job, text, pageURL, source)
}

func extractPageLinksFromHTML(rawHTML string, pageURL string) []pageLink {
	doc, err := newHTMLDocument(rawHTML)
	if err != nil {
		return nil
	}
	var links []pageLink
	doc.Find("a[href]").Each(func(_ int, selection *goquery.Selection) {
		href, _ := selection.Attr("href")
		linkURL := resolveURL(pageURL, href)
		if strings.TrimSpace(linkURL) == "" {
			return
		}
		links = append(links, pageLink{
			Text: normalizeHTMLText(selection.Text()),
			URL:  linkURL,
		})
	})
	return dedupePageLinks(links)
}

func enrichJobFromHTML(job *Job, rawHTML string, pageURL string) {
	if job == nil || strings.TrimSpace(rawHTML) == "" {
		return
	}
	enrichJobFromStructuredJobPosting(job, rawHTML, pageURL)
	enrichJobFromKnownJobBoardHTML(job, rawHTML, pageURL)
	if jobCompensationMissing(job.Compensation) {
		if compensation := extractCompensationFromHTML(rawHTML); compensation != "" {
			job.Compensation = compensation
		}
	}
	if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		if website := extractCompanyWebsiteFromHTML(rawHTML, pageURL, job.Company); website != "" {
			job.CompanyWebsite = website
			setJobIdentityEvidence(job, "website", website, "apply_page", pageURL, "medium", false, "Website extracted from apply page HTML.")
		}
	}
	if jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
		if summary := extractCompanySummaryFromHTML(rawHTML, job.Company); summary != "" {
			job.CompanySummary = summary
			setJobIdentityEvidence(job, "summary", summary, "apply_page", pageURL, "medium", false, "Company summary extracted from apply page HTML.")
			setJobCompanyIfMissing(job, companyNameFromSummary(summary))
		} else if strings.TrimSpace(job.CompanySummary) != "" {
			job.CompanySummary = ""
		}
	} else {
		setJobCompanyIfMissing(job, companyNameFromSummary(job.CompanySummary))
	}
	if explicitIndustry := extractExplicitCompanyIndustryFromHTML(rawHTML); explicitIndustry != "" && jobCompanyIndustryNeedsEnrichment(*job) {
		job.CompanyIndustry = explicitIndustry
		setJobIdentityEvidence(job, "industry", explicitIndustry, "apply_page", pageURL, "medium", false, "Industry came from an explicit page label.")
	} else if summaryIndustry := inferCompanyIndustry(job.CompanySummary); summaryIndustry != "" && jobCompanyIndustryNeedsEnrichment(*job) {
		job.CompanyIndustry = summaryIndustry
		setJobIdentityEvidence(job, "industry", summaryIndustry, "company_summary_inference", pageURL, "low", true, "Industry inferred from company summary text.")
	}
	if strings.TrimSpace(job.Description) == "" {
		job.Description = truncateAtSentence(normalizeHTMLText(rawHTML), 1200)
	}
}

func jobNeedsApplyPageEnrichment(job Job) bool {
	if isKnownNonJobApplyURL(job.ApplyURL) {
		return false
	}
	return jobHasSourceCompanyProfile(job.ApplyURL) ||
		jobCompanyMissingOrUnknown(job.Company) ||
		jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) ||
		jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) ||
		jobCompanyIndustryNeedsEnrichment(job) ||
		jobCompensationMissing(job.Compensation)
}

func jobNeedsLLMIdentityEnrichment(job Job) bool {
	return jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) ||
		jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) ||
		jobCompanyIndustryNeedsEnrichment(job)
}

func jobNeedsCompanyPageEnrichment(job Job) bool {
	if isKnownNonJobApplyURL(job.ApplyURL) {
		return false
	}
	return !jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) &&
		(jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) || jobCompanyIndustryNeedsEnrichment(job))
}

func jobCompanyIndustryNeedsEnrichment(job Job) bool {
	return strings.TrimSpace(job.CompanyIndustry) == "" ||
		(job.CompanyIdentity != nil && job.CompanyIdentity.Industry != nil && job.CompanyIdentity.Industry.Provisional)
}

func jobHasSourceCompanyProfile(applyURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(applyURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return strings.Contains(host, "weworkremotely.com") || strings.Contains(host, "realworkfromanywhere.com")
}

func inferCompanyWebsiteFromApplyURL(job Job) string {
	if strings.TrimSpace(job.Company) == "" || strings.Contains(strings.ToLower(job.Company), "client role") || isKnownNonJobApplyURL(job.ApplyURL) {
		return ""
	}
	parsed, err := url.Parse(strings.TrimSpace(job.ApplyURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	if isApplyURLCompanyWebsiteHostExcluded(host) {
		return ""
	}
	candidate := normalizeCompanyWebsiteURL(parsed.Scheme + "://" + parsed.Host)
	if !looksLikeCompanyWebsite(candidate, "") || !candidateWebsiteMatchesCompany(candidate, job.Company) {
		return ""
	}
	return candidate
}

func isApplyURLCompanyWebsiteHostExcluded(host string) bool {
	if host == "" {
		return true
	}
	if isBuiltInHost(host) || isSharedATSDirectoryHost(host) {
		return true
	}
	if strings.HasPrefix(host, "apply.") || strings.Contains(host, ".jobs.") {
		return true
	}
	for _, blocked := range []string{
		"ashbyhq.com",
		"careervault.io",
		"devopsprojectshq.com",
		"greenhouse.io",
		"ku.bz",
		"kube.careers",
		"lever.co",
		"motionrecruitment.com",
		"myworkdayjobs.com",
		"remotive.com",
		"smartrecruiters.com",
		"theladders.com",
		"workable.com",
		"workdayjobs.com",
	} {
		if host == blocked || strings.HasSuffix(host, "."+blocked) {
			return true
		}
	}
	return false
}

func looksLikeCompanyWebsite(candidate string, applyURL string) bool {
	return domain.LooksLikeCompanyWebsite(candidate, applyURL)
}

func looksLikeCompanySummary(text string, company string) bool {
	return domain.LooksLikeCompanySummary(text, company)
}

func extractCompanySummaryFromHTML(rawHTML string, company string) string {
	if summary := extractRemotiveCompanySummary(rawHTML, company); summary != "" {
		return summary
	}
	if description := extractJobPostingDescriptionHTML(rawHTML); description != "" {
		if summary := extractCompanySummaryFromHTML(description, company); summary != "" {
			return summary
		}
	}
	if summary := extractMetaContent(rawHTML, "description"); looksLikeCompanySummary(summary, company) {
		return truncateAtSentence(summary, 360)
	}
	for _, paragraph := range htmlParagraphs(rawHTML) {
		text := normalizeHTMLText(paragraph.Text)
		if !looksLikeCompanySummary(text, company) {
			continue
		}
		return truncateAtSentence(text, 360)
	}
	return ""
}

func extractRemotiveCompanySummary(rawHTML string, company string) string {
	text := normalizeHTMLText(rawHTML)
	pattern := regexp.MustCompile(`(?i)\bAbout The Company\s+` + regexp.QuoteMeta(strings.TrimSpace(company)) + `\s+(.+?)(?:\s+Read more|\s+Similar Remote Jobs|\s+Before You Apply|\s+Kickstart Your Job Search|$)`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	summary := cleanCompanyProfileSummary(match[1], company)
	if summary != "" {
		return summary
	}
	return truncateAtSentence(strings.TrimSpace(match[1]), 420)
}

func normalizeCompanyWebsiteURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return strings.TrimSpace(rawURL)
	}
	host := normalizeCompanyWebsiteHost(parsed.Hostname())
	return parsed.Scheme + "://" + host
}

func normalizeCompanyWebsiteHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	for {
		changed := false
		for _, prefix := range []string{"careers.", "jobs.", "go.", "discover.", "info.", "redirect.", "pages.", "about.", "forum."} {
			if strings.HasPrefix(host, prefix) {
				host = strings.TrimPrefix(host, prefix)
				changed = true
			}
		}
		pagePrefix := regexp.MustCompile(`^pages\d+\.`).FindString(host)
		if pagePrefix != "" {
			host = strings.TrimPrefix(host, pagePrefix)
			changed = true
		}
		if !changed {
			return host
		}
	}
}

func candidateWebsiteMatchesCompany(candidate string, company string) bool {
	if strings.TrimSpace(company) == "" {
		return true
	}
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Host == "" {
		return false
	}
	return companyHostMatchesName(parsed.Hostname(), company)
}

func companyHostMatchesName(host string, company string) bool {
	hostSlug := strings.ReplaceAll(Slugify(strings.TrimPrefix(strings.ToLower(host), "www.")), "-", "")
	companyKey := Slugify(CleanCompanyName(company))
	companySlug := strings.ReplaceAll(companyKey, "-", "")
	if hostSlug == "" || companySlug == "" {
		return false
	}
	if strings.Contains(hostSlug, companySlug) {
		return true
	}
	for _, token := range strings.Split(companyKey, "-") {
		if isGenericCompanyNameToken(token) {
			continue
		}
		if len(token) >= 4 && strings.Contains(hostSlug, token) {
			return true
		}
	}
	return false
}

func isGenericCompanyNameToken(token string) bool {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "company", "consulting", "digital", "global", "gmbh", "group", "inc", "labs", "network", "security", "services", "solutions", "systems", "technology", "technologies":
		return true
	default:
		return false
	}
}

func extractExplicitCompanyIndustryFromHTML(rawHTML string) string {
	text := normalizeHTMLText(rawHTML)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bindustry\s*:\s*([A-Za-z0-9 &,/+\-]{3,80})`),
		regexp.MustCompile(`(?i)\bindustry\s+-\s+([A-Za-z0-9 &,/+\-]{3,80})`),
		regexp.MustCompile(`(?i)\bsector\s*:\s*([A-Za-z0-9 &,/+\-]{3,80})`),
		regexp.MustCompile(`(?i)\bsector\s+-\s+([A-Za-z0-9 &,/+\-]{3,80})`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		industry := strings.TrimSpace(match[1])
		for _, stop := range []string{" Company website", " Website", " Headquarters", " About", " Salary", " Job type", " Location", " Skills"} {
			if idx := strings.Index(strings.ToLower(industry), strings.ToLower(stop)); idx >= 0 {
				industry = industry[:idx]
			}
		}
		industry = strings.Trim(industry, " .•|")
		if looksLikeCompanyIndustry(industry) {
			return truncateAtSentence(industry, 80)
		}
	}
	return ""
}

func looksLikeCompanyIndustry(industry string) bool {
	industry = strings.TrimSpace(industry)
	if len([]rune(industry)) < 3 || len([]rune(industry)) > 80 {
		return false
	}
	lower := strings.ToLower(industry)
	switch strings.Trim(lower, " .,;:") {
	case "alt", "before", "building a path to compliance 3", "carousel", "icon-30x30", "link", "member trust", "wide":
		return false
	}
	for _, token := range []string{
		"401",
		"apply",
		"candidate",
		"compensation",
		"defense and space mid",
		"here are",
		"icon",
		"job ",
		"leading,",
		"leading accuracy",
		"leading models",
		"login",
		"option",
		"standard security",
		"remote united states",
		"salary",
		"search",
		"west coast",
	} {
		if strings.Contains(lower, token) {
			return false
		}
	}
	return len(strings.Fields(industry)) <= 7
}

func inferCompanyIndustry(text string) string {
	lower := strings.ToLower(text)
	industrySignals := []struct {
		industry string
		tokens   []string
	}{
		{"Healthcare Technology", []string{"healthcare", "fertility clinic", "patient intake", "clinical", "medical"}},
		{"Financial Technology", []string{"fintech", "payments", "stablecoin", "banking", "financial infrastructure", "cryptocurrency", "crypto infrastructure", "digital assets"}},
		{"Financial Services", []string{"investment management", "wealth management", "brokerage", "retirement planning", "financial services"}},
		{"Developer Tools", []string{"developer experience", "infrastructure as code", "observability", "api platform", "git repository", "software development platform", "devsecops platform"}},
		{"Cybersecurity", []string{"cybersecurity", "security platform", "threat", "vulnerability"}},
		{"Cloud Infrastructure", []string{"cloud native", "cloud infrastructure", "devops", "kubernetes", "aws services", "managed hosting", "platform services"}},
		{"Government Technology", []string{"government technology", "local government", "government agencies", "government contractor", "mission customers", "tax assessors"}},
		{"Artificial Intelligence", []string{"artificial intelligence", "machine learning", "ai infrastructure", "ai teams", "ai safety", "ai research", "large language model", "llm"}},
		{"Data Infrastructure", []string{"big data", "data lakehouse", "data warehouse", "data platform", "analytics platform"}},
		{"Legal Technology", []string{"contract lifecycle management", "clm software", "legal operations", "legal technology"}},
		{"Autonomous Vehicle Technology", []string{"self-driving", "autonomous vehicle", "autonomous trucking", "driverless"}},
		{"Transportation / Logistics", []string{"railway", "railroad", "freight", "logistics and supply chain", "supply chain operations"}},
		{"Agriculture Technology", []string{"agriculture", "agricultural", "farm", "farming", "precision agriculture"}},
		{"Advertising Technology", []string{"purchase data", "retail media", "advertising platform", "marketing platform", "consumer engagement"}},
		{"Consumer Software", []string{"consumer brands", "digital freedom", "identity protection", "privacy protection"}},
		{"Media / Communications", []string{"public relations", "communications professionals", "earned media"}},
		{"Online Communities / SaaS", []string{"online communities", "creators", "courses", "live streams", "saas platform"}},
		{"E-commerce", []string{"e-commerce", "ecommerce", "merchants", "commerce"}},
	}
	for _, signal := range industrySignals {
		for _, token := range signal.tokens {
			if strings.Contains(lower, token) {
				return signal.industry
			}
		}
	}
	return ""
}

func extractCompensationFromHTML(rawHTML string) string {
	text := normalizeHTMLText(rawHTML)
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "salary") && !strings.Contains(lower, "compensation") && !strings.Contains(lower, "$") && !strings.Contains(lower, "usd") {
		return ""
	}
	match := salaryRangeRe.FindString(text)
	if strings.TrimSpace(match) == "" {
		return ""
	}
	return strings.TrimSpace(match)
}
