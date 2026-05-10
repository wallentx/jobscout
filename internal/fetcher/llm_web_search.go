package fetcher

import (
	"fmt"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"
)

const maxLLMWebSearchQueries = 18

func llmWebSearchTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(target), "site:") {
		return target
	}
	return "site:" + target
}

func buildLLMWebSearchPrompt(criteria *CriteriaConfig, targets []string) (string, []string) {
	queries := llmWebSearchQueries(criteria, targets, maxLLMWebSearchQueries)
	if len(queries) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("Use provider-backed web search, if available, to find current public job postings.\n")
	b.WriteString("If web search is not available in this runtime, return exactly [] with no explanation.\n")
	b.WriteString("Do not answer with prose, caveats, markdown, or citations outside the JSON array.\n")
	b.WriteString("When a provider web search tool is available, use it before deciding there are no matching jobs.\n")
	b.WriteString("Search only these public-web queries:\n")
	for _, query := range queries {
		fmt.Fprintf(&b, "- %s\n", query)
	}
	if domains := llmWebSearchDomains(targets); len(domains) > 0 {
		b.WriteString("\nAllowed source domains for providers that support domain filters:\n")
		for _, domain := range domains {
			fmt.Fprintf(&b, "- %s\n", domain)
		}
	}
	b.WriteString("\nOnly include roles that match the criteria below. Prefer direct employer or ATS application pages. Exclude stale, closed, unrelated, senior/lead/manager roles when those are excluded by criteria.\n")
	b.WriteString("For each job, include the actual company website, a brief factual company summary, and the company industry when available.\n\n")
	writeLLMWebCriteria(&b, criteria)

	return b.String(), queries
}

func llmWebSearchQueries(criteria *CriteriaConfig, targets []string, limit int) []string {
	titleQueries := targetedSiteSearchQueries(criteria)
	if len(titleQueries) == 0 {
		titleQueries = []string{"software engineer"}
	}

	queries := make([]string, 0)
	seen := make(map[string]bool)
	normalizedTargets := make([]string, 0, len(targets))
	for _, rawTarget := range targets {
		if target := llmWebSearchTarget(rawTarget); target != "" {
			normalizedTargets = append(normalizedTargets, target)
		}
	}

	for _, titleQuery := range titleQueries {
		for _, target := range normalizedTargets {
			query := strings.TrimSpace(target + " " + strings.TrimSpace(titleQuery))
			if query == "" {
				continue
			}
			key := strings.ToLower(query)
			if seen[key] {
				continue
			}
			seen[key] = true
			queries = append(queries, query)
			if limit > 0 && len(queries) >= limit {
				return queries
			}
		}
	}
	return queries
}

func llmWebSearchDomains(targets []string) []string {
	domains := make([]string, 0, len(targets))
	seen := make(map[string]bool)
	for _, target := range targets {
		domain := llmWebSearchDomain(target)
		if domain == "" {
			continue
		}
		key := strings.ToLower(domain)
		if seen[key] {
			continue
		}
		seen[key] = true
		domains = append(domains, domain)
	}
	return domains
}

func llmWebSearchDomain(target string) string {
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(strings.ToLower(target), "site:")
	if target == "" {
		return ""
	}
	if idx := strings.IndexAny(target, " /?"); idx >= 0 {
		target = target[:idx]
	}
	target = strings.Trim(target, ".")
	if target == "" {
		return ""
	}
	if strings.Contains(target, "*") {
		parts := strings.Split(target, ".")
		cleaned := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" || strings.Contains(part, "*") {
				continue
			}
			cleaned = append(cleaned, part)
		}
		if len(cleaned) >= 2 {
			return strings.Join(cleaned[len(cleaned)-2:], ".")
		}
		return strings.Join(cleaned, ".")
	}
	return target
}

func writeLLMWebCriteria(b *strings.Builder, criteria *CriteriaConfig) {
	if criteria == nil {
		b.WriteString("Criteria: none provided.\n")
		return
	}
	location := strings.TrimSpace(strings.Join(nonEmptyStrings(
		criteria.Candidate.City,
		criteria.Candidate.State,
		criteria.Candidate.CountryCode,
	), ", "))

	fmt.Fprintf(b, "Location: %s\n", fallbackString(location, "unspecified"))
	fmt.Fprintf(b, "Years of experience: %d\n", criteria.Candidate.YearsOfExperience)
	fmt.Fprintf(b, "Minimum base salary USD: %d\n", criteria.Filters.MinBaseUSD)
	fmt.Fprintf(b, "Role families: %s\n", fallbackString(domain.FormatRoleFamilyLabels(criteria.RoleFamilies), "unspecified"))
	fmt.Fprintf(b, "Title prefixes: %s\n", fallbackString(strings.Join(criteria.Filters.TitleRequires, ", "), "none"))
	fmt.Fprintf(b, "Target titles: %s\n", fallbackString(strings.Join(criteria.Filters.TitleIncludes, ", "), "unspecified"))
	fmt.Fprintf(b, "Excluded title terms: %s\n", fallbackString(strings.Join(criteria.Filters.TitleExcludes, ", "), "none"))
	fmt.Fprintf(b, "Work settings: %s\n", fallbackString(strings.Join(domain.SelectedWorkSettings(criteria.Filters.WorkSettings), ", "), "unspecified"))
	fmt.Fprintf(b, "Priority signals: %s\n", fallbackString(strings.Join(criteria.PrioritySignals, ", "), "none"))
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func fallbackString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
