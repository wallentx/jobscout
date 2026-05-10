package fetcher

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func enrichJobFromStructuredJobPosting(job *Job, rawHTML string, pageURL string) {
	if job == nil || strings.TrimSpace(rawHTML) == "" {
		return
	}
	for _, posting := range extractStructuredJobPostings(rawHTML) {
		company := structuredHiringOrganizationName(posting)
		if company != "" && jobCompanyMissingOrUnknown(job.Company) {
			job.Company = company
		}

		if title := structuredStringField(posting, "title"); title != "" && strings.TrimSpace(job.Title) == "" {
			job.Title = title
		}

		if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
			if website := structuredHiringOrganizationWebsite(posting); website != "" && looksLikeCompanyWebsite(website, pageURL) {
				website = normalizeCompanyWebsiteURL(website)
				if candidateWebsiteMatchesCompany(website, job.Company) {
					job.CompanyWebsite = website
					setJobIdentityEvidence(job, "website", website, "structured_job_posting", pageURL, "high", false, "Website extracted from JobPosting structured data.")
				}
			}
		}

		if jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
			if description := structuredStringField(posting, "description"); description != "" {
				if summary := extractCompanySummaryFromHTML(description, job.Company); summary != "" {
					job.CompanySummary = summary
					setJobIdentityEvidence(job, "summary", summary, "structured_job_posting", pageURL, "medium", false, "Company summary extracted from JobPosting description.")
				}
			}
		}

		if jobCompensationMissing(job.Compensation) {
			if compensation := structuredJobPostingCompensation(posting); compensation != "" {
				job.Compensation = compensation
			}
		}

		if jobCompanyIndustryNeedsEnrichment(*job) {
			if industry := structuredJobPostingIndustry(posting); industry != "" {
				job.CompanyIndustry = industry
				setJobIdentityEvidence(job, "industry", industry, "structured_job_posting", pageURL, "high", false, "Industry extracted from JobPosting structured data.")
			}
		}
	}
}

func extractStructuredJobPostings(rawHTML string) []map[string]any {
	var postings []map[string]any
	for _, script := range extractJSONLDScripts(rawHTML) {
		var payload any
		if err := json.Unmarshal([]byte(strings.TrimSpace(script)), &payload); err != nil {
			continue
		}
		collectStructuredJobPostings(payload, &postings)
	}
	return postings
}

func collectStructuredJobPostings(value any, postings *[]map[string]any) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectStructuredJobPostings(item, postings)
		}
	case map[string]any:
		if structuredTypeMatches(typed["@type"], "JobPosting") {
			*postings = append(*postings, typed)
		}
		if graph, ok := typed["@graph"]; ok {
			collectStructuredJobPostings(graph, postings)
		}
		if item, ok := typed["item"]; ok {
			collectStructuredJobPostings(item, postings)
		}
	}
}

func structuredTypeMatches(value any, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	switch typed := value.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), want)
	case []any:
		for _, item := range typed {
			if structuredTypeMatches(item, want) {
				return true
			}
		}
	}
	return false
}

func structuredHiringOrganizationName(posting map[string]any) string {
	org, ok := posting["hiringOrganization"]
	if !ok {
		return ""
	}
	return structuredStringField(org, "name")
}

func structuredHiringOrganizationWebsite(posting map[string]any) string {
	org, ok := posting["hiringOrganization"]
	if !ok {
		return ""
	}
	if website := structuredStringField(org, "sameAs"); website != "" {
		return website
	}
	return structuredStringField(org, "url")
}

func structuredJobPostingCompensation(posting map[string]any) string {
	baseSalary, ok := posting["baseSalary"]
	if !ok {
		return ""
	}
	salary, ok := baseSalary.(map[string]any)
	if !ok {
		return ""
	}
	currency := strings.ToUpper(strings.TrimSpace(structuredStringField(salary, "currency")))
	if currency == "" {
		currency = "USD"
	}
	value, ok := salary["value"].(map[string]any)
	if !ok {
		return ""
	}
	minValue, hasMin := structuredNumberField(value, "minValue")
	maxValue, hasMax := structuredNumberField(value, "maxValue")
	exactValue, hasExact := structuredNumberField(value, "value")
	unit := structuredSalaryUnit(structuredStringField(value, "unitText"))

	switch {
	case hasMin && hasMax && minValue > 0 && maxValue > 0:
		return fmt.Sprintf("%s - %s %s%s", formatSalaryAmount(minValue, currency), formatSalaryAmount(maxValue, currency), currency, unit)
	case hasExact && exactValue > 0:
		return fmt.Sprintf("%s %s%s", formatSalaryAmount(exactValue, currency), currency, unit)
	default:
		return ""
	}
}

func structuredJobPostingIndustry(posting map[string]any) string {
	raw, ok := posting["industry"]
	if !ok {
		return ""
	}
	values := structuredStringValues(raw)
	for _, value := range values {
		value = normalizeHTMLText(value)
		if looksLikeCompanyIndustry(value) {
			return truncateAtSentence(value, 80)
		}
	}
	return ""
}

func structuredStringValues(value any) []string {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{typed}
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, structuredStringValues(item)...)
		}
		return values
	default:
		return nil
	}
}

func structuredNumberField(value map[string]any, key string) (float64, bool) {
	raw, ok := value[key]
	if !ok {
		return 0, false
	}
	switch typed := raw.(type) {
	case float64:
		return typed, true
	case json.Number:
		value, err := typed.Float64()
		return value, err == nil
	case string:
		value, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(typed), ",", ""), 64)
		return value, err == nil
	default:
		return 0, false
	}
}

func structuredSalaryUnit(unitText string) string {
	switch strings.ToUpper(strings.TrimSpace(unitText)) {
	case "YEAR", "YEARLY", "ANNUAL", "ANNUALLY":
		return "/year"
	case "HOUR", "HOURLY":
		return "/hour"
	default:
		return ""
	}
}

func formatSalaryAmount(amount float64, currency string) string {
	rounded := int(math.Round(amount))
	value := strconv.Itoa(rounded)
	for i := len(value) - 3; i > 0; i -= 3 {
		value = value[:i] + "," + value[i:]
	}
	if currency == "USD" {
		return "$" + value
	}
	return value
}

func jobCompanyMissingOrUnknown(company string) bool {
	company = strings.TrimSpace(company)
	return company == "" || strings.EqualFold(company, "unknown")
}

func extractBuiltInJobDetailURLs(rawHTML string, pageURL string) []string {
	page, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil || page.Host == "" || !isBuiltInHost(page.Hostname()) {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(candidate string) {
		resolved := resolveURL(pageURL, candidate)
		parsed, err := url.Parse(resolved)
		if err != nil || parsed.Host == "" || !isBuiltInHost(parsed.Hostname()) || !isBuiltInJobDetailPath(parsed.EscapedPath()) {
			return
		}
		parsed.RawQuery = ""
		parsed.Fragment = ""
		normalized := parsed.String()
		if seen[normalized] {
			return
		}
		seen[normalized] = true
		out = append(out, normalized)
	}

	for _, script := range extractJSONLDScripts(rawHTML) {
		var payload any
		if err := json.Unmarshal([]byte(strings.TrimSpace(script)), &payload); err != nil {
			continue
		}
		collectStructuredURLs(payload, add)
	}
	for _, href := range extractHTMLHrefs(rawHTML) {
		add(href)
	}
	return out
}

func collectStructuredURLs(value any, add func(string)) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectStructuredURLs(item, add)
		}
	case map[string]any:
		for key, item := range typed {
			if strings.EqualFold(key, "url") || strings.EqualFold(key, "@id") {
				if raw, ok := item.(string); ok {
					add(raw)
				}
			}
			collectStructuredURLs(item, add)
		}
	}
}

func isBuiltInJobDetailPath(rawPath string) bool {
	path := strings.Trim(strings.ToLower(rawPath), "/")
	if path == "" || !strings.Contains("/"+path, "/job") {
		return false
	}
	parts := strings.Split(path, "/")
	last := parts[len(parts)-1]
	if last == "" {
		return false
	}
	for _, r := range last {
		if r < '0' || r > '9' {
			return false
		}
	}
	return strings.HasPrefix(path, "job/") ||
		strings.HasPrefix(path, "jobs/") ||
		strings.Contains(path, "/job/") ||
		strings.Contains(path, "/jobs/")
}

func extractQuotedJSField(rawHTML string, field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	pattern := regexp.MustCompile(`"` + regexp.QuoteMeta(field) + `"\s*:\s*"((?:\\.|[^"\\])*)"`)
	match := pattern.FindStringSubmatch(rawHTML)
	if len(match) < 2 {
		return ""
	}
	value, err := strconv.Unquote(`"` + match[1] + `"`)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}
