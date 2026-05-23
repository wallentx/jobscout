package domain

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

func healthEvidenceMatchesCompanyContext(title string, evidenceURL string, identity CompanyHealthContext) (bool, string) {
	names := companyHealthContextNames(identity)
	titleLower := strings.ToLower(strings.TrimSpace(title))
	if len(names) == 0 || title == "" {
		return false, "missing company or evidence title"
	}
	domain := companyHealthContextDomain(identity)
	evidenceText := titleLower + " " + strings.ToLower(evidenceURL)
	if domain != "" && strings.Contains(evidenceText, domain) {
		return true, ""
	}
	if !healthEvidenceMatchesCompanyName(title, names) && !healthEvidenceMatchesIdentityContext(evidenceText, identity) {
		return false, "insufficient identity context"
	}
	return true, ""
}

func healthEvidenceMatchesCompanyName(title string, names []string) bool {
	for _, name := range names {
		companyPattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `\b`)
		shortPattern := shortBrandPattern(name)
		if companyPattern.MatchString(title) || (shortPattern != nil && shortPattern.MatchString(title)) {
			return true
		}
	}
	return false
}

func shortBrandPattern(name string) *regexp.Regexp {
	letters := compactAlphaNumeric(name)
	if len(letters) < 3 || len(letters) > 5 || companyContextStopword(letters) {
		return nil
	}
	var pattern strings.Builder
	pattern.WriteString(`(?i)\b`)
	for i, r := range letters {
		if i > 0 {
			pattern.WriteString(`\W*`)
		}
		pattern.WriteString(regexp.QuoteMeta(string(r)))
	}
	pattern.WriteString(`\b`)
	return regexp.MustCompile(pattern.String())
}

func healthEvidenceMatchesIdentityContext(evidenceText string, identity CompanyHealthContext) bool {
	terms := companyHealthContextTerms(identity)
	if len(terms) < 2 {
		return false
	}
	matches := 0
	for _, term := range terms {
		if evidenceContainsContextTerm(evidenceText, term) {
			matches++
			if matches >= 2 {
				return true
			}
		}
	}
	return false
}

func companyHealthContextTerms(identity CompanyHealthContext) []string {
	companyTerms := make(map[string]bool)
	for _, name := range companyHealthContextNames(identity) {
		for _, term := range tokenizeCompanyContext(name) {
			companyTerms[term] = true
		}
	}

	seen := make(map[string]bool)
	var terms []string
	texts := append([]string{identity.Summary, identity.Industry}, identity.Industries...)
	for _, text := range texts {
		if strings.Contains(strings.ToLower(text), "san antonio") && !seen["san_antonio"] {
			seen["san_antonio"] = true
			terms = append(terms, "san_antonio")
		}
		for _, term := range tokenizeCompanyContext(text) {
			if companyTerms[term] || companyContextStopword(term) || seen[term] {
				continue
			}
			seen[term] = true
			terms = append(terms, term)
		}
	}
	return terms
}

func tokenizeCompanyContext(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len(field) < 4 {
			continue
		}
		terms = append(terms, field)
		if strings.HasSuffix(field, "s") && len(field) > 4 {
			terms = append(terms, strings.TrimSuffix(field, "s"))
		}
	}
	return terms
}

func evidenceContainsContextTerm(evidenceText string, term string) bool {
	if term == "" {
		return false
	}
	switch term {
	case "grocery":
		return regexp.MustCompile(`(?i)\b(?:grocery|grocer|grocers)\b`).MatchString(evidenceText)
	case "supermarket":
		return regexp.MustCompile(`(?i)\b(?:supermarket|supermarkets|grocery|grocer|grocers)\b`).MatchString(evidenceText)
	case "san_antonio":
		return regexp.MustCompile(`(?i)\b(?:san\W+antonio|s\W*a\W*(?:area)?)\b`).MatchString(evidenceText)
	}
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
	return pattern.MatchString(evidenceText)
}

func compactAlphaNumeric(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func companyContextStopword(term string) bool {
	switch strings.ToLower(term) {
	case "about", "after", "also", "and", "are", "based", "build", "builds", "business", "businesses",
		"chain", "company", "companies", "corp", "corporation", "develop", "develops", "founded", "from",
		"group", "industry", "industries", "into", "jobs", "labs", "market", "markets", "news",
		"offer", "offers", "operate", "operates", "platform", "platforms", "product", "products",
		"provide", "provides", "service", "services", "store", "stores", "that", "their", "with":
		return true
	default:
		return false
	}
}

func companyHealthContextNames(identity CompanyHealthContext) []string {
	rawNames := append([]string{identity.Company}, identity.Aliases...)
	names := make([]string, 0, len(rawNames))
	seen := make(map[string]bool, len(rawNames))
	for _, raw := range rawNames {
		name := strings.TrimSpace(raw)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, name)
	}
	return names
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
	host := parsedCompanyWebsiteHost(website)
	return strings.TrimPrefix(host, "www.")
}

func parsedCompanyWebsiteHost(website string) string {
	parsed, err := url.Parse(website)
	if err == nil && parsed.Hostname() != "" {
		return strings.ToLower(parsed.Hostname())
	}
	if strings.Contains(website, "://") {
		return ""
	}
	parsed, err = url.Parse("https://" + website)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func CompanyHealthContextDomain(identity CompanyHealthContext) string {
	return companyHealthContextDomain(identity)
}
