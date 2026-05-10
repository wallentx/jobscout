package fetcher

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strings"
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
		}
	}
	return ""
}

func enrichJobFromCompanyProfileHTML(job *Job, rawHTML string, profileURL string) {
	if job == nil || strings.TrimSpace(rawHTML) == "" {
		return
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
}

func extractCompanyWebsiteFromHTML(rawHTML string, applyURL string, company string) string {
	if website := extractStructuredCompanyWebsite(rawHTML); website != "" && looksLikeCompanyWebsite(website, applyURL) {
		return normalizeCompanyWebsiteURL(website)
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
		if strings.Contains(strings.ToLower(rawHTML), "url:</strong>") {
			return normalizeCompanyWebsiteURL(href)
		}
	}
	return ""
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
	for _, href := range extractHTMLHrefs(rawHTML) {
		if !looksLikeCompanyWebsite(href, applyURL) {
			continue
		}
		needle := strings.ToLower(href)
		idx := strings.Index(strings.ToLower(rawHTML), needle)
		if idx < 0 {
			continue
		}
		start := idx - 200
		if start < 0 {
			start = 0
		}
		end := idx + len(href) + 200
		if end > len(rawHTML) {
			end = len(rawHTML)
		}
		contextText := strings.ToLower(normalizeHTMLText(rawHTML[start:end]))
		for _, label := range labels {
			if strings.Contains(contextText, strings.ToLower(label)) {
				return href
			}
		}
	}
	return ""
}
