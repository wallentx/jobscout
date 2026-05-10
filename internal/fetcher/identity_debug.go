package fetcher

import (
	"strconv"
	"strings"
)

type jobIdentityDebugSnapshot struct {
	Company  string
	Title    string
	Website  string
	Summary  string
	Industry string
}

func debugJobIdentitySnapshot(job Job) jobIdentityDebugSnapshot {
	return jobIdentityDebugSnapshot{
		Company:  strings.TrimSpace(job.Company),
		Title:    strings.TrimSpace(job.Title),
		Website:  strings.TrimSpace(job.CompanyWebsite),
		Summary:  strings.TrimSpace(job.CompanySummary),
		Industry: strings.TrimSpace(job.CompanyIndustry),
	}
}

func logJobIdentityOutcome(stage string, before jobIdentityDebugSnapshot, job Job) {
	after := debugJobIdentitySnapshot(job)
	changes := debugJobIdentityChanges(before, after)
	missing := debugMissingIdentityFields(job)
	if len(changes) == 0 {
		changes = []string{"none"}
	}
	if len(missing) == 0 {
		missing = []string{"none"}
	}
	logDebug(
		"identity enrichment %s: source=%q title=%q company=%q apply_url=%q changes=%s missing=%s",
		stage,
		job.Source,
		after.Title,
		after.Company,
		job.ApplyURL,
		strings.Join(changes, "; "),
		strings.Join(missing, ","),
	)
}

func debugJobIdentityChanges(before, after jobIdentityDebugSnapshot) []string {
	var changes []string
	if before.Company != after.Company {
		changes = append(changes, "company:"+debugFieldChange(before.Company, after.Company))
	}
	if before.Website != after.Website {
		changes = append(changes, "website:"+debugFieldChange(before.Website, after.Website))
	}
	if before.Summary != after.Summary {
		changes = append(changes, "summary_len:"+debugLengthChange(before.Summary, after.Summary))
	}
	if before.Industry != after.Industry {
		changes = append(changes, "industry:"+debugFieldChange(before.Industry, after.Industry))
	}
	return changes
}

func debugFieldChange(before, after string) string {
	if before == "" {
		before = "<empty>"
	}
	if after == "" {
		after = "<empty>"
	}
	return debugSafeField(before) + "->" + debugSafeField(after)
}

func debugLengthChange(before, after string) string {
	return debugIntString(len(before)) + "->" + debugIntString(len(after))
}

func debugIntString(value int) string {
	return strconv.Itoa(value)
}

func debugSafeField(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if len(value) > 160 {
		return value[:160] + "...[truncated]"
	}
	return value
}

func debugMissingIdentityFields(job Job) []string {
	var missing []string
	if jobCompanyMissingOrUnknown(job.Company) {
		missing = append(missing, "company")
	}
	if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		missing = append(missing, "website")
	}
	if jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
		missing = append(missing, "summary")
	}
	if jobCompanyIndustryNeedsEnrichment(job) {
		missing = append(missing, "industry")
	}
	return missing
}

func logJobIdentityBatchSummary(stage string, jobs []Job, elapsed string) {
	missingCompany := 0
	missingWebsite := 0
	missingSummary := 0
	missingIndustry := 0
	for _, job := range jobs {
		if jobCompanyMissingOrUnknown(job.Company) {
			missingCompany++
		}
		if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
			missingWebsite++
		}
		if jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) {
			missingSummary++
		}
		if jobCompanyIndustryNeedsEnrichment(job) {
			missingIndustry++
		}
	}
	logDebug(
		"identity enrichment %s summary: jobs=%d missing_company=%d missing_website=%d missing_summary=%d missing_industry=%d duration=%s",
		stage,
		len(jobs),
		missingCompany,
		missingWebsite,
		missingSummary,
		missingIndustry,
		elapsed,
	)
}
