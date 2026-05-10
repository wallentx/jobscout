package fetcher

import (
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type builtInListingJob struct {
	job        Job
	profileURL string
}

func extractBuiltInJobsFromListing(rawHTML string, pageURL string, sourceName string, criteria *CriteriaConfig) ([]Job, map[string][]Job, int) {
	listingJobs, cardCount := extractBuiltInListingJobs(rawHTML, pageURL, sourceName, criteria)
	accepted, filtered := filterBuiltInListingJobsWithProfiles(listingJobs, criteria)
	jobs := builtInListingJobsToJobs(accepted)
	return jobs, filtered, cardCount
}

func filterBuiltInListingJobsWithProfiles(listingJobs []builtInListingJob, criteria *CriteriaConfig) ([]builtInListingJob, map[string][]Job) {
	accepted := make([]builtInListingJob, 0, len(listingJobs))
	filtered := make(map[string][]Job)
	for _, listingJob := range listingJobs {
		if reason := filterJobReason(&listingJob.job, criteria); reason != "" {
			filtered[reason] = append(filtered[reason], listingJob.job)
			continue
		}
		accepted = append(accepted, listingJob)
	}
	if len(filtered) == 0 {
		filtered = nil
	}
	return accepted, filtered
}

func builtInListingJobsToJobs(listingJobs []builtInListingJob) []Job {
	jobs := make([]Job, 0, len(listingJobs))
	for _, listingJob := range listingJobs {
		jobs = append(jobs, listingJob.job)
	}
	return jobs
}

func extractBuiltInListingJobs(rawHTML string, pageURL string, sourceName string, criteria *CriteriaConfig) ([]builtInListingJob, int) {
	doc, err := newHTMLDocument(rawHTML)
	if err != nil {
		return nil, 0
	}

	var jobs []builtInListingJob
	cardCount := 0
	doc.Find(`[data-id="job-card"]`).Each(func(_ int, card *goquery.Selection) {
		cardCount++
		job, profileURL, ok := builtInJobFromListingCard(card, pageURL, sourceName, criteria)
		if !ok {
			return
		}
		jobs = append(jobs, builtInListingJob{job: job, profileURL: profileURL})
	})

	return jobs, cardCount
}

func builtInJobFromListingCard(card *goquery.Selection, pageURL string, sourceName string, criteria *CriteriaConfig) (Job, string, bool) {
	titleLink := card.Find(`[data-id="job-card-title"]`).First()
	title := selectionTextWithSpaces(titleLink)
	companyLink := card.Find(`[data-id="company-title"]`).First()
	company := selectionTextWithSpaces(companyLink)
	applyURL := builtInListingCardApplyURL(titleLink, pageURL)
	companyProfileURL := builtInListingCardCompanyProfileURL(companyLink, pageURL)
	if strings.TrimSpace(title) == "" || strings.TrimSpace(company) == "" || strings.TrimSpace(applyURL) == "" {
		return Job{}, "", false
	}

	job := Job{
		Company:         strings.TrimSpace(company),
		Title:           strings.TrimSpace(title),
		Remote:          builtInListingCardWorkSetting(card, criteria),
		Compensation:    firstNonEmptyString(builtInListingCardCompensation(card), "Not listed"),
		Source:          sourceName,
		ApplyURL:        applyURL,
		Status:          "Unopened",
		Description:     builtInListingCardDescription(card),
		CompanyIndustry: builtInListingCardIndustry(card),
	}
	job.SetDateAdded(time.Now().Unix())
	if website := builtInListingCardCompanyWebsite(company); website != "" {
		job.CompanyWebsite = website
		setJobIdentityEvidence(&job, "website", website, "builtin_card_company_label", firstNonEmptyString(companyProfileURL, pageURL), "medium", false, "Company website inferred from Built In card company label.")
	}
	if job.CompanyIndustry != "" {
		setJobIdentityEvidence(&job, "industry", job.CompanyIndustry, "builtin_card_industry", firstNonEmptyString(companyProfileURL, pageURL), "medium", false, "Industry extracted from Built In listing card.")
	}
	return job, companyProfileURL, true
}

func builtInListingCardApplyURL(titleLink *goquery.Selection, pageURL string) string {
	if titleLink == nil || titleLink.Length() == 0 {
		return ""
	}
	href, ok := titleLink.Attr("href")
	if !ok {
		return ""
	}
	resolved := resolveURL(pageURL, href)
	parsed, err := url.Parse(resolved)
	if err != nil || parsed.Host == "" || !isBuiltInHost(parsed.Hostname()) || !isBuiltInJobDetailPath(parsed.EscapedPath()) {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func builtInListingCardCompanyProfileURL(companyLink *goquery.Selection, pageURL string) string {
	if companyLink == nil || companyLink.Length() == 0 {
		return ""
	}
	href, ok := companyLink.Attr("href")
	if !ok {
		return ""
	}
	return normalizeBuiltInCompanyProfileURL(resolveURL(pageURL, href))
}

func builtInListingCardWorkSetting(card *goquery.Selection, criteria *CriteriaConfig) string {
	if card == nil || card.Length() == 0 {
		return inferWorkSetting("", criteria)
	}
	var best string
	card.Find("span").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		text := normalizeWhitespace(selectionTextWithSpaces(selection))
		if setting := builtInWorkSettingLabel(text); setting != "" {
			best = setting
			return false
		}
		return true
	})
	if best != "" {
		return best
	}
	return inferWorkSetting(selectionTextWithSpaces(card), criteria)
}

func builtInWorkSettingLabel(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case lower == "remote", lower == "fully remote", strings.HasPrefix(lower, "hiring remotely"):
		return "Remote"
	case lower == "remote or hybrid", lower == "hybrid or remote":
		return "Remote or Hybrid"
	case lower == "in-office or remote", lower == "in office or remote", lower == "remote or in-office", lower == "remote or in office":
		return "In-Office or Remote"
	case lower == "hybrid", strings.HasPrefix(lower, "hybrid in "):
		return "Hybrid"
	case lower == "on site", lower == "on-site", lower == "onsite", lower == "in-office", lower == "in office":
		return "Onsite"
	default:
		return ""
	}
}

func builtInListingCardCompensation(card *goquery.Selection) string {
	if card == nil || card.Length() == 0 {
		return ""
	}
	var compensation string
	card.Find("span").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		text := normalizeWhitespace(selectionTextWithSpaces(selection))
		lower := strings.ToLower(text)
		if strings.Contains(lower, "annually") || strings.Contains(lower, "hourly") || strings.Contains(lower, "salary") {
			compensation = text
			return false
		}
		return true
	})
	return compensation
}

func builtInListingCardIndustry(card *goquery.Selection) string {
	text := normalizeWhitespace(selectionTextWithSpaces(card.Find(".mb-md.fs-xs.fw-bold").First()))
	if text == "" {
		return ""
	}
	for _, part := range splitBuiltInBulletList(text) {
		if looksLikeCompanyIndustry(part) {
			return part
		}
	}
	if looksLikeCompanyIndustry(text) {
		return text
	}
	return ""
}

func builtInListingCardDescription(card *goquery.Selection) string {
	parts := []string{}
	if summary := normalizeWhitespace(selectionTextWithSpaces(card.Find(".fs-sm.fw-regular.mb-md.text-gray-04").First())); summary != "" {
		parts = append(parts, summary)
	}
	if industry := normalizeWhitespace(selectionTextWithSpaces(card.Find(".mb-md.fs-xs.fw-bold").First())); industry != "" {
		parts = append(parts, "Industries: "+strings.Join(splitBuiltInBulletList(industry), ", "))
	}
	if skills := builtInListingCardSkills(card); len(skills) > 0 {
		parts = append(parts, "Top skills: "+strings.Join(skills, ", "))
	}
	return strings.Join(parts, "\n")
}

func builtInListingCardSkills(card *goquery.Selection) []string {
	var skills []string
	card.Find(`[id^="drop-data-"] span.fs-xs.text-gray-04.mx-sm`).Each(func(_ int, selection *goquery.Selection) {
		skill := normalizeWhitespace(selectionTextWithSpaces(selection))
		if skill == "" || strings.EqualFold(skill, "Top Skills:") {
			return
		}
		skills = appendUniqueString(skills, skill)
	})
	return skills
}

func splitBuiltInBulletList(text string) []string {
	text = strings.ReplaceAll(text, "\u2022", "•")
	raw := strings.Split(text, "•")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = normalizeWhitespace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func builtInListingCardCompanyWebsite(company string) string {
	start := strings.LastIndex(company, "(")
	end := strings.LastIndex(company, ")")
	if start < 0 || end <= start {
		return ""
	}
	candidate := strings.TrimSpace(company[start+1 : end])
	if !strings.Contains(candidate, ".") || strings.Contains(candidate, " ") {
		return ""
	}
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	website := normalizeCompanyWebsiteURL(candidate)
	if !looksLikeCompanyWebsite(website, "") {
		return ""
	}
	return website
}
