package domain

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ShouldUseBrowserCompanyProfile reports whether a company-site profile can fill result gaps.
func ShouldUseBrowserCompanyProfile(result *CompanyHealthResult) bool {
	if result == nil {
		return false
	}
	return result.FoundedYear == nil || result.EstimatedEmployees == nil || result.Confidence == "low"
}

// ApplyCompanySiteProfile applies discovered company-site evidence to a health result.
func ApplyCompanySiteProfile(result *CompanyHealthResult, profile *CompanySiteProfile) {
	if result == nil || profile == nil {
		return
	}

	combinedText := strings.TrimSpace(strings.Join([]string{profile.WebsiteText, profile.AboutText}, "\n\n"))
	if combinedText == "" && profile.WebsiteURL == "" && len(profile.PublicProfiles) == 0 {
		return
	}

	result.SignalsUsed = append(result.SignalsUsed, "browser_company_site")
	siteSource := map[string]any{
		"search_query": profile.SearchQuery,
		"search_url":   profile.SearchURL,
		"website_url":  profile.WebsiteURL,
		"about_url":    profile.AboutURL,
	}
	if strings.TrimSpace(profile.Summary) != "" {
		siteSource["summary"] = strings.TrimSpace(profile.Summary)
	}
	if strings.TrimSpace(profile.Industry) != "" {
		siteSource["industry"] = strings.TrimSpace(profile.Industry)
	}
	result.Sources["company_site"] = siteSource
	recordCompanyPublicProfiles(result, profile.PublicProfiles)

	if result.FoundedYear == nil {
		if year := parseYearFromText(combinedText); year != nil {
			ageYears := time.Now().Year() - *year
			observeFoundedYear(result, *year, "company_site", preferredCompanySiteURL(profile), "medium", "Founded year inferred from the company site.")
			result.Notes = append(result.Notes, fmt.Sprintf("Estimated age: ~%d years (company site).", ageYears))
			if result.Confidence == "low" {
				result.Confidence = "medium"
			}
		} else if websiteHost := extractURLHostname(profile.WebsiteURL); websiteHost != "" {
			if year := fetchWhoisAge(websiteHost); year != nil {
				ageYears := time.Now().Year() - *year
				observeFoundedYear(result, *year, "company_site_domain_age", profile.WebsiteURL, "low", "Founded year estimated from the discovered company domain age.")
				result.Notes = append(result.Notes, fmt.Sprintf("Estimated age: ~%d years (site domain age).", ageYears))
				if result.Confidence == "low" {
					result.Confidence = "medium"
				}
			}
		}
		if result.FoundedYear == nil {
			applyCompanyPublicProfileFoundedYears(result, profile.PublicProfiles)
		}
		if result.FoundedYear == nil {
			noteFieldGap(result, "founded_year", "Browser company-site lookup found no trustworthy founded-year evidence.")
		}
	}

	if result.EstimatedEmployees == nil {
		if count := parseEmployeeCount(combinedText); count != nil {
			observeEmployeeCount(result, *count, "company_site", preferredCompanySiteURL(profile), "medium", "Employee count inferred from the company site.")
			result.Notes = append(result.Notes, fmt.Sprintf("Estimated size: ~%d employees (company site).", *count))
			if result.Confidence == "low" {
				result.Confidence = "medium"
			}
		} else {
			applyCompanyPublicProfileEmployeeCounts(result, profile.PublicProfiles)
		}
		if result.EstimatedEmployees == nil {
			noteFieldGap(result, "estimated_employees", "Browser company-site lookup found no trustworthy employee-count evidence.")
		}
	}
}

func recordCompanyPublicProfiles(result *CompanyHealthResult, profiles []CompanyPublicProfile) {
	if result == nil || len(profiles) == 0 {
		return
	}
	if result.Sources == nil {
		result.Sources = make(map[string]any)
	}
	result.Sources["public_profiles"] = profiles
	if !companyHealthSignalUsed(result.SignalsUsed, "public_company_profile") {
		result.SignalsUsed = append(result.SignalsUsed, "public_company_profile")
	}

	for _, profile := range profiles {
		if signal, ok := employerReviewSignalFromPublicProfile(profile); ok {
			result.EmployerReviews = appendUniqueEmployerReviewSignal(result.EmployerReviews, signal)
		}
	}
}

func applyCompanyPublicProfileFoundedYears(result *CompanyHealthResult, profiles []CompanyPublicProfile) {
	for _, profile := range profiles {
		if profile.FoundedYear == nil {
			continue
		}
		observeFoundedYear(
			result,
			*profile.FoundedYear,
			publicProfileHealthSource(profile.Source),
			profile.URL,
			"medium",
			fmt.Sprintf("Founded year extracted from %s company profile.", profile.Source),
		)
		if result.FoundedYear != nil {
			return
		}
	}
}

func applyCompanyPublicProfileEmployeeCounts(result *CompanyHealthResult, profiles []CompanyPublicProfile) {
	for _, profile := range profiles {
		if profile.EstimatedEmployees == nil {
			continue
		}
		reason := fmt.Sprintf("Employee count estimated from %s company profile.", profile.Source)
		if strings.TrimSpace(profile.EmployeeRange) != "" {
			reason = fmt.Sprintf("Employee count estimated from %s public range: %s.", profile.Source, profile.EmployeeRange)
		}
		observeEmployeeCount(result, *profile.EstimatedEmployees, publicProfileHealthSource(profile.Source), profile.URL, "medium", reason)
		if result.EstimatedEmployees != nil {
			return
		}
	}
}

func publicProfileHealthSource(source string) string {
	source = strings.TrimSpace(strings.ToLower(source))
	if source == "" {
		return "public_profile"
	}
	return "public_profile_" + source
}

func employerReviewSignalFromPublicProfile(profile CompanyPublicProfile) (EmployerReviewSignal, bool) {
	if strings.TrimSpace(profile.Rating) == "" &&
		profile.ReviewCount == nil &&
		profile.RecommendPercent == nil &&
		profile.CEOApprovalPercent == nil &&
		strings.TrimSpace(profile.Snippet) == "" {
		return EmployerReviewSignal{}, false
	}
	source := strings.TrimSpace(profile.Source)
	title := strings.TrimSpace(profile.Title)
	if title == "" {
		title = strings.TrimSpace(reviewSourceLabel(source) + " employer reviews")
	}
	signal := EmployerReviewSignal{
		Source:             source,
		Title:              title,
		URL:                strings.TrimSpace(profile.URL),
		Rating:             strings.TrimSpace(profile.Rating),
		ReviewCount:        profile.ReviewCount,
		RecommendPercent:   profile.RecommendPercent,
		CEOApprovalPercent: profile.CEOApprovalPercent,
		Snippet:            strings.TrimSpace(profile.Snippet),
	}
	return signal, true
}

func appendUniqueEmployerReviewSignal(signals []EmployerReviewSignal, signal EmployerReviewSignal) []EmployerReviewSignal {
	key := strings.TrimSpace(signal.Source) + "|" + strings.TrimSpace(signal.URL)
	if key == "|" {
		return append(signals, signal)
	}
	for _, existing := range signals {
		existingKey := strings.TrimSpace(existing.Source) + "|" + strings.TrimSpace(existing.URL)
		if strings.EqualFold(existingKey, key) {
			return signals
		}
	}
	return append(signals, signal)
}

func EnrichCompanyHealthContextFromSiteProfile(identity CompanyHealthContext, profile *CompanySiteProfile) CompanyHealthContext {
	if profile == nil {
		return identity
	}
	if strings.TrimSpace(identity.Website) == "" {
		identity.Website = strings.TrimSpace(profile.WebsiteURL)
	}
	if strings.TrimSpace(identity.Summary) == "" {
		identity.Summary = strings.TrimSpace(profile.Summary)
	}
	if strings.TrimSpace(identity.Industry) == "" {
		identity.Industry = strings.TrimSpace(profile.Industry)
	}
	if strings.TrimSpace(profile.Industry) != "" {
		identity.Industries = appendUniqueNonEmptyStrings(identity.Industries, profile.Industry)
	}
	return identity
}

func recordCompanySiteProfileFetchError(result *CompanyHealthResult, err error) {
	if result == nil || err == nil {
		return
	}
	notice := "Company-site health signals unavailable; company-site lookup failed."
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "browser binary") || strings.Contains(lower, "chrome") || strings.Contains(lower, "chromium") {
		notice = "Install Chrome or Chromium to enable company-site health signals."
	}
	for _, existing := range result.Notices {
		if strings.EqualFold(existing, notice) {
			return
		}
	}
	result.Notices = append(result.Notices, notice)
}

func preferredCompanySiteURL(profile *CompanySiteProfile) string {
	if profile == nil {
		return ""
	}
	if profile.AboutURL != "" {
		return profile.AboutURL
	}
	return profile.WebsiteURL
}

func extractURLHostname(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
