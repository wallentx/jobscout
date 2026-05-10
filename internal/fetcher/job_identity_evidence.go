package fetcher

import (
	"net/url"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"
)

func setJobIdentityEvidence(job *Job, field string, value string, source string, evidenceURL string, confidence string, provisional bool, reason string) {
	if job == nil || strings.TrimSpace(value) == "" {
		return
	}
	job.SetCompanyIdentityEvidence(field, JobIdentityEvidence{
		Value:       value,
		Source:      source,
		URL:         evidenceURL,
		Confidence:  confidence,
		Provisional: provisional,
		Reason:      reason,
	})
}

func backfillJobIdentityEvidence(job *Job, source string, evidenceURL string) {
	if job == nil {
		return
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "source_payload"
	}
	if strings.TrimSpace(job.CompanyWebsite) != "" && (job.CompanyIdentity == nil || job.CompanyIdentity.Website == nil) {
		setJobIdentityEvidence(job, "website", job.CompanyWebsite, source, evidenceURL, "low", true, "Website came from the original job payload and was not independently verified.")
	}
	if strings.TrimSpace(job.CompanySummary) != "" && (job.CompanyIdentity == nil || job.CompanyIdentity.Summary == nil) {
		setJobIdentityEvidence(job, "summary", job.CompanySummary, source, evidenceURL, "low", true, "Summary came from the original job payload and was not independently verified.")
	}
	if strings.TrimSpace(job.CompanyIndustry) != "" && (job.CompanyIdentity == nil || job.CompanyIdentity.Industry == nil) {
		setJobIdentityEvidence(job, "industry", job.CompanyIndustry, source, evidenceURL, "low", true, "Industry came from the original job payload or inference and was not independently verified.")
	}
}

func sanitizeExistingJobIdentity(job *Job) {
	if job == nil {
		return
	}
	if website := strings.TrimSpace(job.CompanyWebsite); website != "" {
		normalized := normalizeCompanyWebsiteURL(website)
		if !looksLikeCompanyWebsite(normalized, "") || !candidateWebsiteMatchesCompany(normalized, job.Company) {
			job.CompanyWebsite = ""
			if job.CompanyIdentity != nil {
				job.CompanyIdentity.Website = nil
			}
		} else {
			job.CompanyWebsite = normalized
			if job.CompanyIdentity != nil && job.CompanyIdentity.Website != nil {
				job.CompanyIdentity.Website.Value = normalized
			}
		}
	}
	if inferred := inferCompanyWebsiteFromApplyURL(*job); inferred != "" && shouldPreferApplyURLCompanyWebsite(*job, inferred) {
		job.CompanyWebsite = inferred
		setJobIdentityEvidence(job, "website", inferred, "apply_url_host", job.ApplyURL, "medium", false, "Website inferred from a first-party apply URL host.")
	}
	if summary := strings.TrimSpace(job.CompanySummary); summary != "" && !looksLikeCompanySummary(summary, job.Company) {
		job.CompanySummary = ""
		if job.CompanyIdentity != nil {
			job.CompanyIdentity.Summary = nil
		}
	} else if summary != "" && job.CompanyIdentity != nil && job.CompanyIdentity.Summary != nil && evidenceSourceRequiresCompanyWebsiteHost(job.CompanyIdentity.Summary.Source) && !evidenceURLMatchesCompanyWebsite(job.CompanyIdentity.Summary.URL, job.CompanyWebsite) {
		job.CompanySummary = ""
		job.CompanyIdentity.Summary = nil
	}
	if industry := strings.TrimSpace(job.CompanyIndustry); industry != "" && !looksLikeCompanyIndustry(industry) {
		job.CompanyIndustry = ""
		if job.CompanyIdentity != nil {
			job.CompanyIdentity.Industry = nil
		}
	} else if industry != "" && job.CompanyIdentity != nil && job.CompanyIdentity.Industry != nil && shouldClearExistingIndustryEvidence(*job) {
		job.CompanyIndustry = ""
		job.CompanyIdentity.Industry = nil
	}
}

func evidenceSourceRequiresCompanyWebsiteHost(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "browser_company_search", "company_homepage", "company_about", "company_site":
		return true
	default:
		return false
	}
}

func evidenceURLMatchesCompanyWebsite(evidenceURL string, companyWebsite string) bool {
	evidence, err := url.Parse(strings.TrimSpace(evidenceURL))
	if err != nil || evidence.Host == "" {
		return false
	}
	website, err := url.Parse(strings.TrimSpace(companyWebsite))
	if err != nil || website.Host == "" {
		return false
	}
	return companyHostRoot(evidence.Hostname()) == companyHostRoot(website.Hostname())
}

func companyHostRoot(host string) string {
	host = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(host), "www."))
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func shouldClearExistingIndustryEvidence(job Job) bool {
	if job.CompanyIdentity == nil || job.CompanyIdentity.Industry == nil {
		return false
	}
	evidence := job.CompanyIdentity.Industry
	source := strings.ToLower(strings.TrimSpace(evidence.Source))
	switch {
	case source == "job_description_inference" || source == "apply_page_inference":
		return true
	case strings.HasSuffix(source, "_inference"):
		expected := inferCompanyIndustry(job.CompanySummary)
		return expected == "" || !strings.EqualFold(strings.TrimSpace(job.CompanyIndustry), expected)
	default:
		return false
	}
}

func shouldPreferApplyURLCompanyWebsite(job Job, inferred string) bool {
	if strings.TrimSpace(inferred) == "" || strings.EqualFold(strings.TrimSpace(job.CompanyWebsite), inferred) {
		return false
	}
	if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		return true
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Website == nil {
		return true
	}
	source := strings.ToLower(strings.TrimSpace(job.CompanyIdentity.Website.Source))
	return job.CompanyIdentity.Website.Provisional || source == "browser_company_search" || source == "source_payload"
}

func jobCompanyWebsiteMissingOrInvalid(website string) bool {
	return domain.JobCompanyWebsiteMissingOrInvalid(website)
}

func JobCompanyWebsiteMissingOrInvalid(website string) bool {
	return jobCompanyWebsiteMissingOrInvalid(website)
}

func jobCompanySummaryMissingOrInvalid(summary string, company string) bool {
	return domain.JobCompanySummaryMissingOrInvalid(summary, company)
}

func JobCompanySummaryMissingOrInvalid(summary string, company string) bool {
	return jobCompanySummaryMissingOrInvalid(summary, company)
}

func jobCompensationMissing(compensation string) bool {
	return domain.JobCompensationMissing(compensation)
}

func JobCompensationMissing(compensation string) bool {
	return jobCompensationMissing(compensation)
}
