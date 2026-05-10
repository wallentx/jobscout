package fetcher

import (
	"net/url"
	"strings"
)

func normalizeBuiltInCompanyProfileURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	if !isBuiltInHost(parsed.Hostname()) {
		return ""
	}
	path := strings.ToLower(parsed.EscapedPath())
	if !strings.HasPrefix(path, "/company/") || strings.Trim(path, "/") == "company" {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func builtInCompanyProfileURLFromJob(job Job) string {
	if !isBuiltInJobURL(job.ApplyURL) {
		return ""
	}
	if job.CompanyIdentity == nil {
		return ""
	}
	for _, evidence := range []*JobIdentityEvidence{
		job.CompanyIdentity.Website,
		job.CompanyIdentity.Summary,
		job.CompanyIdentity.Industry,
	} {
		if evidence == nil {
			continue
		}
		if profileURL := normalizeBuiltInCompanyProfileURL(evidence.URL); profileURL != "" {
			return profileURL
		}
	}
	return ""
}
