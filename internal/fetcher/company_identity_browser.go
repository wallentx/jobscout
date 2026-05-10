package fetcher

import (
	"context"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"
)

type browserCompanySearchTarget struct {
	Key     string
	Company string
	Indexes []int
}

func browserCompanySearchTargets(jobs []Job) []browserCompanySearchTarget {
	targets := make([]browserCompanySearchTarget, 0)
	targetByKey := make(map[string]int)
	for i, job := range jobs {
		if !jobNeedsBrowserCompanySearch(job) {
			continue
		}
		key := browserCompanySearchKey(job.Company)
		if key == "" {
			continue
		}
		if existing, ok := targetByKey[key]; ok {
			targets[existing].Indexes = append(targets[existing].Indexes, i)
			continue
		}
		targetByKey[key] = len(targets)
		targets = append(targets, browserCompanySearchTarget{
			Key:     key,
			Company: strings.TrimSpace(job.Company),
			Indexes: []int{i},
		})
	}
	return targets
}

func browserCompanySearchKey(company string) string {
	key := Slugify(CleanCompanyName(company))
	if key != "" {
		return key
	}
	return strings.ToLower(strings.TrimSpace(company))
}

func jobNeedsBrowserCompanySearch(job Job) bool {
	company := strings.TrimSpace(job.Company)
	return company != "" && !strings.EqualFold(company, "Unknown") && !isKnownNonJobApplyURL(job.ApplyURL) && jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite)
}

func applyBrowserCompanySiteProfile(ctx context.Context, job *Job, profile *domain.CompanySiteProfile, llmEnrich jobIdentityPageEnrichFunc) {
	if job == nil || profile == nil {
		return
	}

	before := debugJobIdentitySnapshot(*job)
	defer func() {
		logJobIdentityOutcome("browser company search", before, *job)
	}()

	evidenceURL := firstNonEmptyString(profile.SearchURL, profile.WebsiteURL)
	if website := normalizeCompanyWebsiteURL(profile.WebsiteURL); website != "" && jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) {
		job.CompanyWebsite = website
		setJobIdentityEvidence(job, "website", website, "browser_company_search", evidenceURL, "medium", false, "Website discovered from a browser search for the company.")
	}

	if strings.TrimSpace(profile.WebsiteText) != "" {
		enrichJobFromCompanySiteText(ctx, job, profile.WebsiteText, profile.WebsiteURL, "browser_company_search", llmEnrich)
	}
	if strings.TrimSpace(profile.AboutText) != "" {
		enrichJobFromCompanySiteText(ctx, job, profile.AboutText, profile.AboutURL, "browser_company_about", llmEnrich)
	}
}
