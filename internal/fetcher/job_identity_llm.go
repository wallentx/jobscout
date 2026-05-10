package fetcher

import (
	"context"
	"strings"
)

type jobIdentityPageEnrichFunc func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error)

func buildJobIdentityPage(rawHTML string, pageURL string) JobIdentityPage {
	text := normalizeHTMLText(rawHTML)
	if links := extractLLMIdentityLinks(rawHTML, pageURL, 80); len(links) > 0 {
		text = strings.TrimSpace(text) + "\n\nLinks:\n- " + strings.Join(links, "\n- ")
	}
	return JobIdentityPage{
		URL:  pageURL,
		Text: text,
	}
}

func extractLLMIdentityLinks(rawHTML string, pageURL string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	hrefs := extractHTMLHrefs(rawHTML)
	seen := make(map[string]bool, len(hrefs))
	links := make([]string, 0, min(limit, len(hrefs)))
	for _, rawHref := range hrefs {
		href := resolveURL(pageURL, rawHref)
		if strings.TrimSpace(href) == "" || seen[href] {
			continue
		}
		seen[href] = true
		links = append(links, href)
		if len(links) >= limit {
			break
		}
	}
	return links
}

func applyLLMJobIdentityEnrichment(ctx context.Context, job *Job, page JobIdentityPage, llmEnrich jobIdentityPageEnrichFunc, source string) LLMTokenUsage {
	if job == nil || llmEnrich == nil || strings.TrimSpace(page.Text) == "" || !jobNeedsLLMIdentityEnrichment(*job) {
		return LLMTokenUsage{}
	}
	hadIdentityAnchor := !jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) || !jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company)
	result, usage, err := llmEnrich(ctx, *job, page)
	if err != nil || result == nil {
		return usage
	}
	acceptedIdentityAnchor := false
	if website := strings.TrimSpace(result.CompanyWebsite); website != "" && jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) && looksLikeCompanyWebsite(website, page.URL) {
		website = normalizeCompanyWebsiteURL(website)
		job.CompanyWebsite = website
		setJobIdentityEvidence(job, "website", website, source, page.URL, confidenceOrDefault(result.WebsiteConfidence, "medium"), false, firstNonEmptyString(result.CompanyWebsiteReason, "Website extracted by LLM from supplied page text."))
		acceptedIdentityAnchor = true
	}
	if summary := strings.TrimSpace(result.CompanySummary); summary != "" && jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) && looksLikeCompanySummary(summary, job.Company) {
		job.CompanySummary = summary
		setJobIdentityEvidence(job, "summary", summary, source, page.URL, confidenceOrDefault(result.SummaryConfidence, "medium"), false, firstNonEmptyString(result.CompanySummaryReason, "Company summary extracted by LLM from supplied page text."))
		acceptedIdentityAnchor = true
	}
	if industry := strings.TrimSpace(result.CompanyIndustry); industry != "" && looksLikeCompanyIndustry(industry) && (hadIdentityAnchor || acceptedIdentityAnchor) && (strings.TrimSpace(job.CompanyIndustry) == "" || jobCompanyIndustryProvisional(job)) {
		job.CompanyIndustry = truncateAtSentence(industry, 80)
		setJobIdentityEvidence(job, "industry", job.CompanyIndustry, source, page.URL, confidenceOrDefault(result.IndustryConfidence, "medium"), result.IndustryProvisional, firstNonEmptyString(result.CompanyIndustryReason, "Industry extracted by LLM from supplied page text."))
	}
	return usage
}

func jobCompanyIndustryProvisional(job *Job) bool {
	return job != nil && job.CompanyIdentity != nil && job.CompanyIdentity.Industry != nil && job.CompanyIdentity.Industry.Provisional
}

func confidenceOrDefault(confidence string, fallback string) string {
	confidence = strings.ToLower(strings.TrimSpace(confidence))
	switch confidence {
	case "high", "medium", "low":
		return confidence
	default:
		return fallback
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
