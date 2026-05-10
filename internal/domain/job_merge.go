package domain

import (
	"net/url"
	"strings"
)

func JobMergeKey(job Job) string {
	return strings.ToLower(strings.TrimSpace(job.Company) + "|" + strings.TrimSpace(job.Title))
}

func MergeJobs(existing []Job, newJobs []Job) (int, []Job) {
	added := 0
	existingMap := make(map[string]int, len(existing)+len(newJobs))
	for idx, job := range existing {
		existingMap[JobMergeKey(job)] = idx
	}

	finalJobs := append([]Job(nil), existing...)
	for _, job := range newJobs {
		key := JobMergeKey(job)
		if existingIdx, exists := existingMap[key]; exists {
			MergeJobIdentityFields(&finalJobs[existingIdx], job)
			continue
		}
		if job.Status == "" || job.Status == "New" {
			job.Status = "Unopened"
		}
		finalJobs = append(finalJobs, job)
		existingMap[key] = len(finalJobs) - 1
		added++
	}
	return added, finalJobs
}

func MergeJobIdentityFields(existing *Job, incoming Job) {
	if existing == nil {
		return
	}
	if JobCompanyWebsiteMissingOrInvalid(existing.CompanyWebsite) && !JobCompanyWebsiteMissingOrInvalid(incoming.CompanyWebsite) {
		existing.CompanyWebsite = incoming.CompanyWebsite
	}
	if JobCompanySummaryMissingOrInvalid(existing.CompanySummary, existing.Company) && !JobCompanySummaryMissingOrInvalid(incoming.CompanySummary, incoming.Company) {
		existing.CompanySummary = incoming.CompanySummary
	}
	if strings.TrimSpace(existing.CompanyIndustry) == "" && strings.TrimSpace(incoming.CompanyIndustry) != "" {
		existing.CompanyIndustry = incoming.CompanyIndustry
	}
	if JobCompensationMissing(existing.Compensation) && !JobCompensationMissing(incoming.Compensation) {
		existing.Compensation = incoming.Compensation
	}
	if strings.TrimSpace(existing.Description) == "" && strings.TrimSpace(incoming.Description) != "" {
		existing.Description = incoming.Description
	}
}

func JobCompanyWebsiteMissingOrInvalid(website string) bool {
	return strings.TrimSpace(website) == "" || !LooksLikeCompanyWebsite(website, "")
}

func LooksLikeCompanyWebsite(candidate string, applyURL string) bool {
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if strings.Contains(strings.ToLower(parsed.Host), "weworkremotely.com") {
		return false
	}
	if blockedCompanyWebsiteHost(parsed.Host) {
		return false
	}
	path := strings.ToLower(parsed.EscapedPath())
	for _, suffix := range []string{".apng", ".avif", ".css", ".gif", ".ico", ".jpg", ".jpeg", ".js", ".json", ".pdf", ".png", ".svg", ".ttf", ".webp", ".woff", ".woff2"} {
		if strings.HasSuffix(path, suffix) {
			return false
		}
	}
	for _, pathPart := range []string{"/apple-icon", "/favicon", "/icon-", "/privacy", "/legal", "/site-policy"} {
		if strings.Contains(path, pathPart) {
			return false
		}
	}
	if strings.HasPrefix(path, "/docs") || strings.HasPrefix(path, "/documentation") || strings.HasPrefix(path, "/support") {
		return false
	}
	applyParsed, err := url.Parse(applyURL)
	if err == nil && strings.EqualFold(parsed.Host, applyParsed.Host) {
		return false
	}
	return true
}

func blockedCompanyWebsiteHost(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(host, "www."))
	for _, prefix := range []string{
		"click.",
		"clicks.",
		"docs.",
		"email.",
		"help.",
		"link.",
		"links.",
		"support.",
		"track.",
		"tracking.",
		"trk.",
	} {
		if strings.HasPrefix(host, prefix) {
			return true
		}
	}
	if strings.HasPrefix(host, "go2.") {
		return true
	}
	blockedExact := []string{
		"app.link",
		"avatars.githubusercontent.com",
		"bit.ly",
		"bnc.lt",
		"branch.io",
		"cdn.segment.com",
		"cdnjs.cloudflare.com",
		"cdn.jsdelivr.net",
		"cdn.optimizely.com",
		"cdn.sift.com",
		"docs.github.com",
		"dol.gov",
		"facebook.com",
		"fonts.googleapis.com",
		"fonts.gstatic.com",
		"gmpg.org",
		"googletagmanager.com",
		"instagram.com",
		"job-boards.greenhouse.io",
		"jobs.ashbyhq.com",
		"jobs.lever.co",
		"linktr.ee",
		"linkedin.com",
		"lnkd.in",
		"netflixhouse.com",
		"onelink.me",
		"pagead2.googlesyndication.com",
		"q.stripe.com",
		"cybersecurityjobslist.com",
		"t.co",
		"tinyurl.com",
		"twitter.com",
		"x.com",
		"youtu.be",
		"youtube.com",
	}
	for _, blocked := range blockedExact {
		if host == blocked || strings.HasSuffix(host, "."+blocked) {
			return true
		}
	}
	blockedParts := []string{
		"ashbyprd.com",
		"cloudflare.com",
		"cloudfront.net",
		"phenompeople.com",
		"static.vscdn.net",
		"website-files.com",
		"workdayjobs.com",
	}
	for _, blocked := range blockedParts {
		if strings.Contains(host, blocked) {
			return true
		}
	}
	return false
}

func JobCompanySummaryMissingOrInvalid(summary string, company string) bool {
	return strings.TrimSpace(summary) == "" || !LooksLikeCompanySummary(summary, company)
}

func LooksLikeCompanySummary(text string, company string) bool {
	text = strings.TrimSpace(text)
	if len([]rune(text)) < 50 {
		return false
	}
	lower := strings.ToLower(text)
	metadataTokens := []string{
		"headquarters:",
		"url:",
		"location:",
		"compensation:",
		"schedule:",
		"apply at",
		"to apply:",
		"<p>",
		"<strong>",
		"learn more about",
		"find jobs, explore benefits",
		"research company culture",
		" is hiring ",
		"the range represents",
		"reasonably expects to pay",
		"this fully remote role",
		"this role ",
		"reports to our",
		"candidates are required",
		"unable to sponsor",
		"you are ",
		"you're ",
		"you'll ",
		"you will ",
		"your role",
		"you'll deploy",
		"we are seeking",
		"we're seeking",
		"we're looking for engineers",
		"about the role",
		"experienced engineer",
		"leadership ability",
		"personal information",
		"candidate privacy",
		"to continue, please login",
		"a web browser is a piece of software",
		"we're sure there's lots more to know",
		"we don't have all the info",
		"@media",
		"transition:",
		"data-hover-features",
	}
	for _, token := range metadataTokens {
		if strings.Contains(lower, token) {
			return false
		}
	}
	company = strings.ToLower(strings.TrimSpace(company))
	if company != "" && strings.Contains(lower, company) {
		return true
	}
	companyPatterns := []string{
		" is a ",
		" is an ",
		" is the ",
		" builds ",
		" provides ",
		" helps ",
		" focused on ",
	}
	for _, pattern := range companyPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func JobCompensationMissing(compensation string) bool {
	compensation = strings.TrimSpace(strings.ToLower(compensation))
	return compensation == "" || compensation == "not listed" || compensation == "n/a" || compensation == "na"
}

func IdentityRepairTargets(jobs []Job) ([]Job, []int) {
	targets := make([]Job, 0, len(jobs))
	indexes := make([]int, 0, len(jobs))
	for i, job := range jobs {
		if !JobNeedsIdentityRepair(job) {
			continue
		}
		targets = append(targets, job)
		indexes = append(indexes, i)
	}
	return targets, indexes
}

func JobNeedsIdentityRepair(job Job) bool {
	if strings.EqualFold(strings.TrimSpace(job.Status), "Expired") {
		return false
	}
	return strings.TrimSpace(job.CompanyWebsite) == "" ||
		strings.TrimSpace(job.CompanySummary) == "" ||
		strings.TrimSpace(job.CompanyIndustry) == "" ||
		JobCompensationMissing(job.Compensation) ||
		JobHasProvisionalIdentityEvidence(job)
}

func JobHasProvisionalIdentityEvidence(job Job) bool {
	if job.CompanyIdentity == nil {
		return false
	}
	return IdentityEvidenceProvisional(job.CompanyIdentity.Website) ||
		IdentityEvidenceProvisional(job.CompanyIdentity.Summary) ||
		IdentityEvidenceProvisional(job.CompanyIdentity.Industry)
}

func IdentityEvidenceProvisional(evidence *JobIdentityEvidence) bool {
	return evidence != nil && evidence.Provisional
}
