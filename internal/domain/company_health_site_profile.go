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
	if combinedText == "" && profile.WebsiteURL == "" {
		return
	}

	result.SignalsUsed = append(result.SignalsUsed, "browser_company_site")
	result.Sources["company_site"] = map[string]any{
		"search_query": profile.SearchQuery,
		"search_url":   profile.SearchURL,
		"website_url":  profile.WebsiteURL,
		"about_url":    profile.AboutURL,
	}

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
		} else {
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
			noteFieldGap(result, "estimated_employees", "Browser company-site lookup found no trustworthy employee-count evidence.")
		}
	}
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
