package domain

import (
	"net/url"
	"regexp"
	"strings"
)

func healthEvidenceMatchesCompanyContext(title string, evidenceURL string, identity CompanyHealthContext) (bool, string) {
	company := strings.ToLower(strings.TrimSpace(identity.Company))
	titleLower := strings.ToLower(strings.TrimSpace(title))
	if company == "" || title == "" {
		return false, "missing company or evidence title"
	}
	domain := companyHealthContextDomain(identity)
	evidenceText := titleLower + " " + strings.ToLower(evidenceURL)
	if domain != "" && strings.Contains(evidenceText, domain) {
		return true, ""
	}
	companyPattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(company) + `\b`)
	if !companyPattern.MatchString(title) {
		return false, "company name not present"
	}

	industry := strings.ToLower(identity.Industry)
	if industryLooksFinancial(industry) && containsAny(evidenceText, []string{"skate", "game", "gaming", "gamesindustry", "shacknews", "developer full circle", "video game", "publisher"}) {
		return false, "evidence industry conflicts with company industry"
	}
	if containsAny(industry, []string{"healthcare", "medical", "clinical"}) && containsAny(evidenceText, []string{"game", "gaming", "crypto", "banking"}) {
		return false, "evidence industry conflicts with company industry"
	}
	return true, ""
}

func industryLooksFinancial(industry string) bool {
	return containsAny(industry, []string{"financ", "fintech", "payments", "banking", "crypto", "stablecoin"})
}

func rejectedHealthEvidence(value string, source string, evidenceURL string, reason string) CompanyHealthEvidence {
	return CompanyHealthEvidence{
		Value:      strings.TrimSpace(value),
		Source:     source,
		URL:        strings.TrimSpace(evidenceURL),
		Confidence: "low",
		Accepted:   false,
		Reason:     strings.TrimSpace(reason),
	}
}

func companyHealthContextDomain(identity CompanyHealthContext) string {
	website := strings.TrimSpace(identity.Website)
	if website == "" {
		return ""
	}
	parsed, err := url.Parse(website)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.TrimPrefix(host, "www.")
}

func CompanyHealthContextDomain(identity CompanyHealthContext) string {
	return companyHealthContextDomain(identity)
}
