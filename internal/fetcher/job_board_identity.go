package fetcher

import (
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func enrichJobFromKnownJobBoardHTML(job *Job, rawHTML string, pageURL string) {
	if job == nil || strings.TrimSpace(rawHTML) == "" {
		return
	}
	switch {
	case isBuiltInJobURL(pageURL):
		enrichJobFromBuiltInJobHTML(job, rawHTML, pageURL)
	case isYCombinatorJobURL(pageURL):
		enrichJobFromYCombinatorJobHTML(job, rawHTML, pageURL)
	case isGreenhouseJobURL(pageURL):
		enrichJobFromGreenhouseJobHTML(job, rawHTML, pageURL)
	}
}

func enrichJobFromBuiltInJobHTML(job *Job, rawHTML string, pageURL string) {
	if company := extractQuotedJSField(rawHTML, "companyName"); company != "" {
		setJobCompanyIfMissing(job, company)
	}
	if strings.TrimSpace(job.Title) == "" {
		job.Title = extractQuotedJSField(rawHTML, "title")
	}
	if !jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		return
	}
	applyURL := extractQuotedJSField(rawHTML, "howToApply")
	if applyURL == "" {
		return
	}
	website := normalizeCompanyWebsiteURL(applyURL)
	if !looksLikeCompanyWebsite(website, pageURL) {
		return
	}
	if strings.TrimSpace(job.Company) != "" && !candidateWebsiteMatchesCompany(website, job.Company) {
		return
	}
	job.CompanyWebsite = website
	setJobIdentityEvidence(job, "website", website, "builtin_how_to_apply", pageURL, "high", false, "Company website inferred from Built In's external apply URL.")
}

func isBuiltInJobURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	return isBuiltInHost(parsed.Hostname()) && isBuiltInJobDetailPath(parsed.EscapedPath())
}

func enrichJobFromYCombinatorJobHTML(job *Job, rawHTML string, pageURL string) {
	company := extractYCombinatorCompanyName(rawHTML, pageURL)
	setJobCompanyIfMissing(job, company)
	if jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
		if summary := extractYCombinatorCompanySummary(rawHTML, job.Company); summary != "" {
			job.CompanySummary = summary
			setJobIdentityEvidence(job, "summary", summary, "ycombinator_job_page", pageURL, "high", false, "Company summary extracted from Y Combinator job page.")
		}
	}
	if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		if website := extractYCombinatorCompanyWebsite(rawHTML, pageURL, job.Company); website != "" {
			job.CompanyWebsite = website
			setJobIdentityEvidence(job, "website", website, "ycombinator_job_page", pageURL, "high", false, "Company website extracted from Y Combinator job page.")
		}
	}
}

func enrichJobFromGreenhouseJobHTML(job *Job, rawHTML string, pageURL string) {
	doc, err := newHTMLDocument(rawHTML)
	if err != nil {
		return
	}
	logo := greenhouseLogoSelection(doc)
	if logo != nil && logo.Length() > 0 {
		if company := cleanGreenhouseLogoCompanyName(logoImageAlt(logo)); company != "" {
			setJobCompanyIfMissing(job, company)
		}
	}
	if company := greenhouseCompanyNameFromTitle(rawHTML); company != "" {
		setJobCompanyIfMissing(job, company)
	}
	setJobCompanyIfMissing(job, companyNameFromGreenhouseURL(pageURL))
	if !jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		return
	}
	if logo == nil || logo.Length() == 0 {
		return
	}
	if website := greenhouseLogoWebsite(logo, pageURL, job.Company); website != "" {
		job.CompanyWebsite = website
		setJobIdentityEvidence(job, "website", website, "greenhouse_logo", pageURL, "high", false, "Company website extracted from Greenhouse logo link.")
	}
}

func isYCombinatorJobURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.EscapedPath())
	return (host == "ycombinator.com" || host == "www.ycombinator.com") &&
		strings.HasPrefix(path, "/companies/") &&
		strings.Contains(path, "/jobs/")
}

func isGreenhouseJobURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.EscapedPath())
	return (host == "job-boards.greenhouse.io" || strings.HasSuffix(host, ".greenhouse.io")) &&
		strings.Contains(path, "/jobs/")
}

func extractYCombinatorCompanyName(rawHTML string, pageURL string) string {
	doc, err := newHTMLDocument(rawHTML)
	if err == nil {
		if name := ycCompanyNameFromLinks(doc); name != "" {
			return name
		}
		if name := ycCompanyNameFromTitle(rawHTML); name != "" {
			return name
		}
	}
	return ycCompanyNameFromURL(pageURL)
}

func ycCompanyNameFromLinks(doc *goquery.Document) string {
	var company string
	doc.Find("a[href]").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		href, _ := selection.Attr("href")
		if !ycCompanyProfilePath(href) {
			return true
		}
		text := cleanYCombinatorCompanyName(selectionTextWithSpaces(selection))
		if text == "" {
			return true
		}
		company = text
		return false
	})
	return company
}

func ycCompanyProfilePath(href string) bool {
	parsed, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return false
	}
	path := strings.ToLower(parsed.EscapedPath())
	if !strings.HasPrefix(path, "/companies/") || strings.Contains(path, "/jobs/") {
		return false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 2 && parts[0] == "companies" && parts[1] != ""
}

func ycCompanyNameFromTitle(rawHTML string) string {
	title := extractMetaContent(rawHTML, "title")
	if title == "" {
		titlePattern := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
		match := titlePattern.FindStringSubmatch(rawHTML)
		if len(match) > 1 {
			title = normalizeHTMLText(html.UnescapeString(match[1]))
		}
	}
	pattern := regexp.MustCompile(`(?i)\bat\s+(.+?)\s+\|\s+Y Combinator\b`)
	match := pattern.FindStringSubmatch(title)
	if len(match) < 2 {
		return ""
	}
	return cleanYCombinatorCompanyName(match[1])
}

func ycCompanyNameFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) < 2 || parts[0] != "companies" {
		return ""
	}
	return titleCaseSlug(parts[1])
}

func cleanYCombinatorCompanyName(name string) string {
	name = normalizeHTMLText(name)
	name = strings.TrimSuffix(name, " | Y Combinator")
	if strings.EqualFold(name, "Image") || strings.HasPrefix(strings.ToLower(name), "image:") {
		return ""
	}
	return strings.TrimSpace(name)
}

func extractYCombinatorCompanySummary(rawHTML string, company string) string {
	if strings.TrimSpace(company) == "" {
		return ""
	}
	doc, err := newHTMLDocument(rawHTML)
	if err != nil {
		return ""
	}
	var summary string
	doc.Find("p").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		text := normalizeHTMLText(selectionTextWithSpaces(selection))
		if !strings.Contains(strings.ToLower(text), strings.ToLower(company)) {
			return true
		}
		if !looksLikeCompanySummary(text, company) {
			return true
		}
		summary = truncateAtSentence(text, 360)
		return false
	})
	return summary
}

func extractYCombinatorCompanyWebsite(rawHTML string, pageURL string, company string) string {
	for _, href := range extractHTMLHrefs(rawHTML) {
		resolved := resolveURL(pageURL, href)
		if !looksLikeCompanyWebsite(resolved, pageURL) {
			continue
		}
		if !candidateWebsiteMatchesCompany(resolved, company) {
			continue
		}
		return normalizeCompanyWebsiteURL(resolved)
	}
	return ""
}

func greenhouseLogoSelection(doc *goquery.Document) *goquery.Selection {
	var logo *goquery.Selection
	doc.Find(".image-container a.logo, a.logo, .image-container img, img[alt]").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		if selection.Is("img") {
			alt, _ := selection.Attr("alt")
			if cleanGreenhouseLogoCompanyName(alt) == "" {
				return true
			}
		} else if logoImageAlt(selection) == "" {
			return true
		}
		logo = selection
		return false
	})
	return logo
}

func greenhouseLogoWebsite(selection *goquery.Selection, pageURL string, company string) string {
	href, _ := selection.Attr("href")
	if strings.TrimSpace(href) == "" {
		href, _ = selection.Closest("a[href]").Attr("href")
	}
	resolved := resolveURL(pageURL, href)
	if !looksLikeCompanyWebsite(resolved, pageURL) {
		return ""
	}
	if strings.TrimSpace(company) != "" && !candidateWebsiteMatchesCompany(resolved, company) {
		return ""
	}
	return normalizeCompanyWebsiteURL(resolved)
}

func logoImageAlt(selection *goquery.Selection) string {
	if selection.Is("img") {
		alt, _ := selection.Attr("alt")
		return alt
	}
	alt := ""
	selection.Find("img[alt]").EachWithBreak(func(_ int, img *goquery.Selection) bool {
		alt, _ = img.Attr("alt")
		return strings.TrimSpace(alt) == ""
	})
	return alt
}

func cleanGreenhouseLogoCompanyName(alt string) string {
	name := normalizeHTMLText(alt)
	name = regexp.MustCompile(`(?i)\s+logo$`).ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "logo") {
		return ""
	}
	return name
}

func greenhouseCompanyNameFromTitle(rawHTML string) string {
	title := extractMetaContent(rawHTML, "title")
	if title == "" {
		titlePattern := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
		match := titlePattern.FindStringSubmatch(rawHTML)
		if len(match) > 1 {
			title = normalizeHTMLText(html.UnescapeString(match[1]))
		}
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bjob application for .+?\s+at\s+(.+?)(?:\s+\||$)`),
		regexp.MustCompile(`(?i)^.+?\s+at\s+(.+?)(?:\s+\||$)`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(title)
		if len(match) < 2 {
			continue
		}
		name := cleanJobBoardCompanyName(match[1])
		if name != "" {
			return name
		}
	}
	return ""
}

func companyNameFromSummary(summary string) string {
	summary = normalizeHTMLText(summary)
	if summary == "" {
		return ""
	}
	pattern := regexp.MustCompile(`^([A-Z][A-Za-z0-9&.'’+\- ]{1,70})\s+(?:is|are|builds|provides|offers|develops|creates|operates|helps|uses|makes|delivers|specializes|focuses)\b`)
	match := pattern.FindStringSubmatch(summary)
	if len(match) < 2 {
		return ""
	}
	return cleanJobBoardCompanyName(match[1])
}

func cleanJobBoardCompanyName(name string) string {
	name = normalizeHTMLText(name)
	name = strings.Trim(name, " .•|:-")
	switch strings.ToLower(name) {
	case "", "the company", "company", "we", "our team":
		return ""
	default:
		return name
	}
}

func setJobCompanyIfMissing(job *Job, company string) {
	if job == nil || strings.TrimSpace(company) == "" || !jobCompanyMissingOrUnknown(job.Company) {
		return
	}
	job.Company = company
}

func titleCaseSlug(slug string) string {
	parts := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(slug, "-", " "), "_", " "))
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
