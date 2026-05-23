package fetcher

import (
	"strings"

	"github.com/wallentx/jobscout/internal/domain"
)

func enrichJobFromDescription(job *Job) {
	if job == nil || strings.TrimSpace(job.Description) == "" {
		return
	}
	originalApplyURL := job.ApplyURL
	if directApplyURL := extractDirectApplyURLFromHTML(job.Description, job.ApplyURL); directApplyURL != "" {
		job.ApplyURL = directApplyURL
	}
	if domain.JobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		if website := extractCompanyWebsiteFromHTML(job.Description, originalApplyURL, job.Company); website != "" {
			job.CompanyWebsite = website
			setJobIdentityEvidence(job, "website", website, "job_description", originalApplyURL, "medium", false, "Website extracted from job description HTML.")
		}
	}
	if domain.JobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
		if summary := extractCompanySummaryFromHTML(job.Description, job.Company); summary != "" {
			job.CompanySummary = summary
			setJobIdentityEvidence(job, "summary", summary, "job_description", originalApplyURL, "medium", false, "Company summary extracted from job description HTML.")
		} else if strings.TrimSpace(job.CompanySummary) != "" {
			job.CompanySummary = ""
		}
	}
	if explicitIndustry := extractExplicitCompanyIndustryFromHTML(job.Description); explicitIndustry != "" {
		job.CompanyIndustry = explicitIndustry
		setJobIdentityEvidence(job, "industry", explicitIndustry, "job_description", originalApplyURL, "medium", false, "Industry came from an explicit page label.")
	} else if summaryIndustry := inferCompanyIndustry(job.CompanySummary); summaryIndustry != "" {
		job.CompanyIndustry = summaryIndustry
		setJobIdentityEvidence(job, "industry", summaryIndustry, "company_summary_inference", originalApplyURL, "low", true, "Industry inferred from company summary text.")
	}
	if compensation := extractCompensationFromHTML(job.Description); domain.JobCompensationMissing(job.Compensation) && compensation != "" {
		job.Compensation = compensation
	}
}
