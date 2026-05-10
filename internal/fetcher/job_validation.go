package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func validateFetchedJobs(ctx context.Context, jobs []Job) ([]Job, map[string][]string) {
	if len(jobs) == 0 {
		return jobs, nil
	}

	validated := make([]Job, 0, len(jobs))
	rejected := make(map[string][]string)
	for _, job := range jobs {
		if reason := unusableJobReason(job); reason != "" {
			logJobValidationRejection(job, reason)
			rejected[reason] = append(rejected[reason], fmt.Sprintf("%s - %s", job.Company, job.Title))
			continue
		}
		ok, reason := validateJobURL(ctx, job.ApplyURL)
		if !ok {
			logJobValidationRejection(job, reason)
			rejected[reason] = append(rejected[reason], fmt.Sprintf("%s - %s", job.Company, job.Title))
			continue
		}
		validated = append(validated, job)
	}

	return validated, rejected
}

func logJobValidationRejection(job Job, reason string) {
	logDebug(
		"validation rejected source=%q reason=%q company=%q title=%q apply_url=%q company_website=%q summary_len=%d",
		job.Source,
		reason,
		job.Company,
		job.Title,
		job.ApplyURL,
		job.CompanyWebsite,
		len(strings.TrimSpace(job.CompanySummary)),
	)
}

func unusableJobReason(job Job) string {
	applyURL := strings.TrimSpace(job.ApplyURL)
	if applyURL == "" {
		return "empty URL"
	}
	if isKnownNonJobApplyURL(applyURL) {
		return "not a direct job URL"
	}
	if isLLMGeneratedJob(job) && !jobHasRequiredCompanyIdentity(job) {
		return "missing company identity"
	}
	return ""
}

func UnusableJobReason(job Job) string {
	return unusableJobReason(job)
}

func isLLMGeneratedJob(job Job) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(job.Source)), "llm")
}

func jobHasRequiredCompanyIdentity(job Job) bool {
	return !jobCompanyMissingOrUnknown(job.Company) &&
		!jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) &&
		!jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company)
}

func isKnownNonJobApplyURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	path := strings.ToLower(strings.TrimRight(parsed.EscapedPath(), "/"))
	if strings.Contains(path, "/xxxxx") {
		return true
	}
	switch {
	case isBuiltInHost(host) && isKnownNonJobBuiltInPath(parsed.EscapedPath()):
		return true
	case host == "kube.careers" && (path == "" || path == "/kubernetes-jobs-in-usa"):
		return true
	case isIndeedHost(host):
		return !isIndeedDirectJobPath(path)
	case isLinkedInHost(host):
		return !strings.HasPrefix(path, "/jobs/view/")
	case host == "dice.com" || host == "glassdoor.com" || host == "news.google.com" || host == "ziprecruiter.com":
		return true
	default:
		return false
	}
}

func isKnownNonJobBuiltInPath(rawPath string) bool {
	path := strings.ToLower(strings.TrimRight(rawPath, "/"))
	return (path == "/jobs" || strings.HasPrefix(path, "/jobs/")) && !isBuiltInJobDetailPath(rawPath)
}

func isIndeedDirectJobPath(path string) bool {
	return path == "/viewjob" || path == "/rc/clk" || path == "/pagead/clk"
}

func validateJobURL(ctx context.Context, applyURL string) (bool, string) {
	if strings.TrimSpace(applyURL) == "" {
		return false, "empty URL"
	}

	parsed, err := url.Parse(applyURL)
	if err != nil {
		return false, "malformed URL"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false, "unsupported URL scheme"
	}
	if parsed.Host == "" {
		return false, "missing host"
	}

	client := &http.Client{Timeout: 8 * time.Second}
	status, err := probeURL(ctx, client, http.MethodHead, applyURL)
	if err == nil {
		if status == http.StatusNotFound || status == http.StatusGone {
			return false, fmt.Sprintf("HTTP %d", status)
		}
		if status >= 200 && status < 400 {
			return true, ""
		}
		if status == http.StatusMethodNotAllowed {
			// Some ATS endpoints reject HEAD; fall through to GET probe.
		} else {
			// Keep ambiguous statuses like 401/403/5xx so we don't drop valid jobs.
			return true, ""
		}
	}

	status, err = probeURL(ctx, client, http.MethodGet, applyURL)
	if err != nil {
		// Network errors can be transient; keep the job unless we have a definite bad URL.
		return true, ""
	}
	if status == http.StatusNotFound || status == http.StatusGone {
		return false, fmt.Sprintf("HTTP %d", status)
	}

	return true, ""
}
