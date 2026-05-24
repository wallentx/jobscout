package fetcher

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"

	"github.com/PuerkitoBio/goquery"
)

func extractSourceCompanyProfileURL(rawHTML string, pageURL string) string {
	page, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil || page.Host == "" {
		return ""
	}
	host := strings.ToLower(page.Host)
	for _, href := range extractHTMLHrefs(rawHTML) {
		resolved := resolveURL(pageURL, href)
		parsed, err := url.Parse(resolved)
		if err != nil || parsed.Host == "" {
			continue
		}
		path := strings.ToLower(parsed.EscapedPath())
		switch {
		case strings.Contains(host, "weworkremotely.com") && strings.EqualFold(parsed.Host, page.Host) && strings.HasPrefix(path, "/company/"):
			return resolved
		case strings.Contains(host, "realworkfromanywhere.com") && strings.EqualFold(parsed.Host, page.Host) && strings.HasPrefix(path, "/companies/"):
			return resolved
		case isBuiltInHost(host) && strings.EqualFold(parsed.Host, page.Host) && strings.HasPrefix(path, "/company/"):
			return resolved
		case isLinkedInHost(host) && strings.EqualFold(parsed.Host, page.Host) && strings.HasPrefix(path, "/company/"):
			parsed.RawQuery = ""
			parsed.Fragment = ""
			return parsed.String()
		}
	}
	return ""
}

func enrichJobFromCompanyProfileHTML(job *Job, rawHTML string, profileURL string) {
	if job == nil || strings.TrimSpace(rawHTML) == "" {
		return
	}
	if source := job.EnsureSourceMetadata(); strings.TrimSpace(profileURL) != "" {
		source.CompanyProfileURL = strings.TrimSpace(profileURL)
	}
	if website := extractCompanyWebsiteFromHTML(rawHTML, profileURL, job.Company); website != "" {
		job.CompanyWebsite = website
		setJobIdentityEvidence(job, "website", website, "company_profile", profileURL, "high", false, "Website extracted from source company profile.")
	}
	if summary := extractCompanyProfileSummary(rawHTML, job.Company); summary != "" {
		job.CompanySummary = summary
		setJobIdentityEvidence(job, "summary", summary, "company_profile", profileURL, "high", false, "Company summary extracted from source company profile.")
	}
	if industry := extractCompanyProfileIndustry(rawHTML); industry != "" {
		job.CompanyIndustry = industry
		setJobIdentityEvidence(job, "industry", industry, "company_profile", profileURL, "high", false, "Industry extracted from source company profile.")
	}
	applyCompanyProfileMetadata(job, rawHTML, profileURL)
}

func applyCompanyProfileMetadata(job *Job, rawHTML string, profileURL string) {
	if job == nil {
		return
	}
	text := normalizeHTMLText(rawHTML)
	evidence := companyPublicProfileEvidenceFromText("builtin", profileURL, "", text)
	applyCompanyPublicProfileEvidence(job, evidence, "company_profile")
}

func applyCompanyPublicProfileEvidence(job *Job, evidence domain.CompanyPublicProfile, source string) {
	if job == nil {
		return
	}
	if !companyPublicProfileEvidenceUseful(evidence) && strings.TrimSpace(job.CompanyIndustry) == "" {
		return
	}
	companyMetadata := job.EnsureCompanyMetadata()
	if strings.TrimSpace(evidence.Industry) != "" {
		companyMetadata.Industries = appendUniqueString(companyMetadata.Industries, evidence.Industry)
	}
	if strings.TrimSpace(job.CompanyIndustry) != "" {
		companyMetadata.Industries = appendUniqueString(companyMetadata.Industries, job.CompanyIndustry)
	}
	if strings.TrimSpace(evidence.EmployeeRange) != "" {
		companyMetadata.EmployeeRange = evidence.EmployeeRange
	}
	if evidence.EstimatedEmployees != nil {
		companyMetadata.EstimatedEmployees = evidence.EstimatedEmployees
	}
	if evidence.FoundedYear != nil {
		companyMetadata.FoundedYear = evidence.FoundedYear
	}
	if strings.TrimSpace(evidence.Headquarters) != "" {
		companyMetadata.Headquarters = evidence.Headquarters
	}
	if strings.TrimSpace(evidence.Revenue) != "" {
		companyMetadata.Revenue = evidence.Revenue
	}
	if strings.TrimSpace(evidence.Industry) != "" {
		job.EnsureSourceMetadata().Industries = appendUniqueString(job.EnsureSourceMetadata().Industries, evidence.Industry)
		if jobCompanyIndustryNeedsEnrichment(*job) {
			job.CompanyIndustry = evidence.Industry
			setJobIdentityEvidence(job, "industry", evidence.Industry, source, evidence.URL, "medium", false, "Industry extracted from public company profile evidence.")
		}
	}
}

func extractCompanyWebsiteFromHTML(rawHTML string, applyURL string, company string) string {
	if website := extractLinkedInCompanyProfileWebsite(rawHTML, applyURL); website != "" {
		return normalizeCompanyWebsiteURL(website)
	}
	if website := extractStructuredCompanyWebsite(rawHTML); website != "" && looksLikeCompanyWebsite(website, applyURL) {
		website = normalizeCompanyWebsiteURL(website)
		if structuredCompanyWebsiteAllowed(website, applyURL, company) {
			return website
		}
	}
	if website := extractLabeledHref(rawHTML, applyURL, []string{"url", "website", "company website", "view company", "home page", "learn more about us"}); website != "" {
		return normalizeCompanyWebsiteURL(website)
	}
	for _, href := range extractHTMLHrefs(rawHTML) {
		if !looksLikeCompanyWebsite(href, applyURL) {
			continue
		}
		if !candidateWebsiteMatchesCompany(href, company) {
			continue
		}
		return normalizeCompanyWebsiteURL(href)
	}
	for _, href := range extractHTMLHrefs(rawHTML) {
		if !looksLikeCompanyWebsite(href, applyURL) {
			continue
		}
		if strings.Contains(strings.ToLower(rawHTML), "url:</strong>") && candidateWebsiteMatchesCompany(href, company) {
			return normalizeCompanyWebsiteURL(href)
		}
	}
	return ""
}

func extractLinkedInCompanyProfileWebsite(rawHTML string, profileURL string) string {
	if publicProfileSource(profileURL) != "linkedin" {
		return ""
	}
	return extractLabeledHref(rawHTML, profileURL, []string{"website"})
}

func structuredCompanyWebsiteAllowed(website string, pageURL string, company string) bool {
	if sourceProfileWebsiteCandidateBlocked(website, pageURL) {
		return false
	}
	if publicProfileSource(pageURL) != "" {
		return candidateWebsiteMatchesCompany(website, company)
	}
	return true
}

func companyWebsiteFromProfileLinks(links []pageLink, profileURL string, company string) string {
	best := ""
	bestScore := 0
	for _, link := range links {
		website := companyWebsiteFromExternalApplyURL(link.URL, profileURL, company)
		if website == "" {
			continue
		}
		score := scoreCompanyProfileWebsiteLink(link, website)
		if score > bestScore {
			bestScore = score
			best = website
		}
	}
	if bestScore <= 0 {
		return ""
	}
	return best
}

func scoreCompanyProfileWebsiteLink(link pageLink, website string) int {
	parsed, err := url.Parse(website)
	if err != nil || parsed.Host == "" {
		return 0
	}
	if source := publicProfileSource(link.URL); source != "" {
		return 0
	}
	host := strings.ToLower(parsed.Hostname())
	if isIndeedHost(host) || isLinkedInHost(host) || isGoogleHost(host) || isBuiltInHost(host) {
		return 0
	}

	score := 1
	textLower := strings.ToLower(link.Text)
	urlLower := strings.ToLower(link.URL)
	for _, marker := range []string{"website", "company site", "home page", "visit", "official"} {
		if strings.Contains(textLower, marker) {
			score += 3
		}
	}
	if strings.Contains(urlLower, "utm_source=indeed") || strings.Contains(urlLower, "linkedin") {
		score++
	}
	return score
}

func extractStructuredCompanyWebsite(rawHTML string) string {
	for _, script := range extractJSONLDScripts(rawHTML) {
		var payload any
		if err := json.Unmarshal([]byte(strings.TrimSpace(script)), &payload); err != nil {
			continue
		}
		if website := findStructuredCompanyWebsite(payload); website != "" {
			return website
		}
	}
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`(?is)"publicWebsite"\s*:\s*"([^"]+)"`),
	} {
		match := pattern.FindStringSubmatch(rawHTML)
		if len(match) > 1 {
			return html.UnescapeString(match[1])
		}
	}
	return ""
}

func findStructuredCompanyWebsite(value any) string {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if website := findStructuredCompanyWebsite(item); website != "" {
				return website
			}
		}
	case map[string]any:
		if org, ok := typed["hiringOrganization"]; ok {
			if website := structuredStringField(org, "sameAs"); website != "" {
				return website
			}
			if website := structuredStringField(org, "url"); website != "" {
				return website
			}
		}
		if website := structuredStringField(typed, "publicWebsite"); website != "" {
			return website
		}
		if graph, ok := typed["@graph"]; ok {
			return findStructuredCompanyWebsite(graph)
		}
	}
	return ""
}

func structuredStringField(value any, key string) string {
	obj, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	raw, ok := obj[key]
	if !ok {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func extractCompanyProfileIndustry(rawHTML string) string {
	if industry := extractExplicitCompanyIndustryFromHTML(rawHTML); industry != "" {
		return industry
	}
	text := normalizeHTMLText(rawHTML)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bIndustry\s+([A-Za-z0-9 &,/+\-]{3,80}?)(?:\s+Years Remote\b|\s+Established\b|\s+Size\b|\s+HQ\b|\s+About\b|\s+Top 100\b|$)`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		industry := cleanCompanyProfileValue(match[1])
		if looksLikeCompanyIndustry(industry) {
			return industry
		}
	}
	return ""
}

func extractCompanyProfileSummary(rawHTML string, company string) string {
	text := normalizeHTMLText(rawHTML)
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bAbout\s+` + regexp.QuoteMeta(strings.TrimSpace(company)) + `\s+(.+?)(?:\s+Company Culture\b|\s+Our Values\b|\s+Benefits\b|\s+\d+\s+Open Jobs\b|\s+Circle Jobs\b|\s+All\s+` + regexp.QuoteMeta(strings.TrimSpace(company)) + `\s+Jobs\b|$)`),
		regexp.MustCompile(`(?i)\bAbout\s+Culture\s+Benefits\s+Hiring\s+(.+?)(?:\s+We're sure\b|\s+Be sure\b|\s+Unlimited vacation\b|\s+Healthcare coverage\b|\s+All\s+` + regexp.QuoteMeta(strings.TrimSpace(company)) + `\s+Jobs\b|$)`),
		regexp.MustCompile(`(?i)\bWhat We Do\s+(.+?)(?:\s+Why Work With Us\b|\s+Recently Posted Jobs\b|\s+` + regexp.QuoteMeta(strings.TrimSpace(company)) + `\s+Offices\b|$)`),
	} {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		summary := cleanCompanyProfileSummary(match[1], company)
		if summary != "" {
			return summary
		}
	}
	return extractCompanySummaryFromHTML(rawHTML, company)
}

func cleanCompanyProfileValue(value string) string {
	value = strings.TrimSpace(value)
	for _, stop := range []string{" Website", " About", " Years Remote", " Established", " Size", " HQ", " Top 100"} {
		if idx := strings.Index(value, stop); idx >= 0 {
			value = value[:idx]
		}
	}
	return strings.Trim(value, " .•|")
}

func cleanCompanyProfileSummary(summary string, company string) string {
	summary = strings.TrimSpace(summary)
	for _, stop := range []string{" We are known for ", " We're sure ", " Be sure ", " Unlimited vacation ", " Healthcare coverage "} {
		if idx := strings.Index(summary, stop); idx >= 0 {
			summary = summary[:idx]
		}
	}
	if !looksLikeCompanySummary(summary, company) {
		return ""
	}
	return truncateAtSentence(summary, 420)
}

func extractLabeledHref(rawHTML string, applyURL string, labels []string) string {
	doc, err := newHTMLDocument(rawHTML)
	if err != nil {
		return ""
	}
	best := ""
	bestScore := 0
	doc.Find("a[href]").Each(func(_ int, selection *goquery.Selection) {
		if bestScore >= 10 {
			return
		}
		href, _ := selection.Attr("href")
		candidate := decodeProfileExternalLink(resolveURL(applyURL, href))
		if !looksLikeCompanyWebsite(candidate, applyURL) {
			return
		}
		if sourceProfileWebsiteCandidateBlocked(candidate, applyURL) {
			return
		}
		score := scoreLabeledHrefContext(visibleLinkContext(selection), labels)
		if score <= bestScore {
			return
		}
		bestScore = score
		best = candidate
	})
	return best
}

func visibleLinkContext(selection *goquery.Selection) string {
	if selection == nil {
		return ""
	}
	parts := make([]string, 0, 5)
	add := func(text string) {
		text = NormalizeWhitespace(text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	add(selection.Text())
	for node := selection.Parent(); node != nil && node.Length() > 0 && len(parts) < 5; node = node.Parent() {
		if node.Is("body") || node.Is("html") {
			break
		}
		add(node.Text())
	}
	selection.PrevAllFiltered("dt,div,span,p,h2,h3,h4,label").EachWithBreak(func(_ int, sibling *goquery.Selection) bool {
		add(sibling.Text())
		return len(parts) < 5
	})
	return NormalizeWhitespace(strings.Join(parts, " "))
}

func scoreLabeledHrefContext(context string, labels []string) int {
	context = strings.ToLower(NormalizeWhitespace(context))
	if context == "" {
		return 0
	}
	score := 0
	for _, label := range labels {
		label = strings.ToLower(strings.TrimSpace(label))
		if label != "" && strings.Contains(context, label) {
			score += 10
		}
	}
	return score
}

func sourceProfileWebsiteCandidateBlocked(candidate string, pageURL string) bool {
	if publicProfileSource(pageURL) != "linkedin" && !isLinkedInJobURL(pageURL) {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(candidate))
	if err != nil || parsed.Host == "" {
		return false
	}
	return isLinkedInAssetHost(parsed.Hostname())
}

func isLinkedInAssetHost(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(host), "www."))
	return host == "licdn.com" || strings.HasSuffix(host, ".licdn.com")
}

func decodeProfileExternalLink(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	if isLinkedInHost(parsed.Hostname()) {
		for _, key := range []string{"url", "target", "redirectUrl"} {
			target := strings.TrimSpace(parsed.Query().Get(key))
			if target == "" {
				continue
			}
			decoded, err := url.QueryUnescape(target)
			if err == nil {
				target = decoded
			}
			targetParsed, err := url.Parse(target)
			if err == nil && targetParsed.Scheme != "" && targetParsed.Host != "" {
				return targetParsed.String()
			}
		}
	}
	if decoded := decodeSearchResultURL(rawURL); decoded != "" {
		return decoded
	}
	return rawURL
}
