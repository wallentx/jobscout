package domain

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// CompanyHealth performs a full company health assessment
func CompanyHealth(company string, ticker string, includeNews bool) (*CompanyHealthResult, error) {
	return CompanyHealthWithContext(CompanyHealthContext{Company: company}, ticker, includeNews)
}

func CompanyHealthWithContext(identity CompanyHealthContext, ticker string, includeNews bool) (*CompanyHealthResult, error) {
	return CompanyHealthWithDataSources(identity, ticker, includeNews, CompanyHealthDataSources{})
}

func CompanyHealthWithDataSources(identity CompanyHealthContext, ticker string, includeNews bool, sources CompanyHealthDataSources) (*CompanyHealthResult, error) {
	company := strings.TrimSpace(identity.Company)

	result := &CompanyHealthResult{

		Company: company,

		Score: 50, // base score (neutral)

		Confidence: "low",

		SignalsUsed: []string{},

		Flags: []string{},

		Notes: []string{},

		Sources: make(map[string]any),
	}
	if strings.TrimSpace(identity.Website) != "" || strings.TrimSpace(identity.Summary) != "" || strings.TrimSpace(identity.Industry) != "" {
		result.Sources["company_identity"] = map[string]string{
			"website":  strings.TrimSpace(identity.Website),
			"summary":  strings.TrimSpace(identity.Summary),
			"industry": strings.TrimSpace(identity.Industry),
		}
		result.SignalsUsed = append(result.SignalsUsed, "job_company_identity")
	}
	initCompanyHealthAssessments(result)

	// Wikipedia lookup

	wikiSum, err := wikiGetSummary(company)

	if err == nil && wikiSum != nil {
		wikiURL := fmt.Sprintf(wikiSummaryURL, url.PathEscape(wikiSum.Title))

		result.SignalsUsed = append(result.SignalsUsed, "wikipedia_summary")

		result.Sources["wikipedia"] = map[string]string{

			"title": wikiSum.Title,

			"extract": wikiSum.Extract,

			"wikibase_item": wikiSum.WikibaseItem,
		}

		if year := parseYearFromText(wikiSum.Extract); year != nil {
			ageYears := time.Now().Year() - *year
			observeFoundedYear(result, *year, "wikipedia_summary", wikiURL, "medium", "Founded year inferred from Wikipedia summary text.")

			if ageYears >= 10 {

				result.Score += 10

				result.Notes = append(result.Notes, fmt.Sprintf("Age signal: founded ~%d (>=10y).", *year))

			} else if ageYears >= 5 {

				result.Score += 5

				result.Notes = append(result.Notes, fmt.Sprintf("Age signal: founded ~%d (>=5y).", *year))

			}

		}

		// Parse employee count

		if count := parseEmployeeCount(wikiSum.Extract); count != nil {
			observeEmployeeCount(result, *count, "wikipedia_summary", wikiURL, "medium", "Employee count inferred from Wikipedia summary text.")

			result.Notes = append(result.Notes, fmt.Sprintf("Estimated size: ~%d employees (Wikipedia)", *count))

		}

		result.Confidence = "medium"

		if facts, err := wikiGetCompanyFacts(wikiSum); err == nil {
			result.Sources["wikidata"] = map[string]string{
				"entity_id":                    facts.EntityID,
				"entity_url":                   facts.EntityURL,
				"founded_year_claims":          fmt.Sprintf("%d", facts.FoundedYearClaimCount),
				"employee_count_claims":        fmt.Sprintf("%d", facts.EmployeeCountClaimCount),
				"selected_founded_year":        optionalIntString(facts.FoundedYear),
				"selected_employee_count":      optionalIntString(facts.EmployeeCount),
				"founded_year_claims_accepted": fmt.Sprintf("%t", facts.FoundedYear != nil),
				"employee_claims_accepted":     fmt.Sprintf("%t", facts.EmployeeCount != nil),
			}
			if facts.FoundedYear != nil {
				if observeFoundedYear(result, *facts.FoundedYear, "wikidata_entity", facts.EntityURL, "medium", "Founded year extracted from Wikidata inception data.") {
					ageYears := time.Now().Year() - *facts.FoundedYear
					result.Notes = append(result.Notes, fmt.Sprintf("Age signal: founded %d (~%dy, Wikidata).", *facts.FoundedYear, ageYears))
				}
			}
			if facts.EmployeeCount != nil {
				if observeEmployeeCount(result, *facts.EmployeeCount, "wikidata_entity", facts.EntityURL, "medium", "Employee count extracted from Wikidata employee-count data.") {
					result.Notes = append(result.Notes, fmt.Sprintf("Estimated size: ~%d employees (Wikidata).", *facts.EmployeeCount))
				}
			}
			if facts.FoundedYear != nil || facts.EmployeeCount != nil {
				result.SignalsUsed = append(result.SignalsUsed, "wikidata_company_facts")
			}
			if facts.FoundedYearClaimCount > 0 && facts.FoundedYear == nil {
				noteFieldGap(result, "founded_year", fmt.Sprintf("Wikidata entity %s has %d inception claim(s), but none produced an accepted founded year.", facts.EntityID, facts.FoundedYearClaimCount))
			}
			if facts.EmployeeCountClaimCount > 0 && facts.EmployeeCount == nil {
				noteFieldGap(result, "estimated_employees", fmt.Sprintf("Wikidata entity %s has %d employee-count claim(s), but none produced an accepted employee count.", facts.EntityID, facts.EmployeeCountClaimCount))
			}
		}

	} else {
		// Wikipedia failed. Try Whois/RDAP if the company name looks like a domain
		// or if we can infer it.
		// Since we don't have the URL passed into CompanyHealth directly in the current signature,
		// we rely on the company name being "Name.com" from prior enrichment OR we guess.

		domain := companyHealthContextDomain(identity)
		if domain == "" && strings.Contains(company, ".") && !strings.Contains(company, " ") {
			domain = company
		}

		if domain != "" {
			if year := fetchWhoisAge(domain); year != nil {
				ageYears := time.Now().Year() - *year
				observeFoundedYear(result, *year, "rdap_domain_age", "https://rdap.org/domain/"+domain, "low", "Founded year estimated from domain registration age.")
				result.Notes = append(result.Notes, fmt.Sprintf("Estimated age: ~%d years (Domain Registered %d)", ageYears, *year))

				if ageYears >= 5 {
					result.Score += 5
				}
				result.SignalsUsed = append(result.SignalsUsed, "whois_rdap")
			}
		}
	}

	// SEC EDGAR lookup
	cik10, foundTicker, foundName, err := secLookupCIK(company, ticker)
	secRiskTerms := 0
	if err == nil {
		result.Public = new(true)
		result.DiscoveredTicker = foundTicker
		result.DiscoveredName = foundName
		result.SignalsUsed = append(result.SignalsUsed, "sec_edgar")
		result.Confidence = "high"

		sub, err := secGetSubmissions(cik10)
		if err == nil {
			result.Sources["sec"] = sub

			// Try to get precise headcount from 10-K
			if count := secGetHeadcount(cik10, sub); count != nil {
				observeEmployeeCount(result, *count, "sec_10k", "", "high", "Employee count extracted from SEC 10-K filings.")
				result.Notes = append(result.Notes, fmt.Sprintf("Precise size: %d employees (from SEC 10-K)", *count))
				result.SignalsUsed = append(result.SignalsUsed, "sec_10k_headcount")
			}

			recentFilings := sub.Filings.Recent
			if len(recentFilings.FilingDate) > 0 {
				latestDate := recentFilings.FilingDate[0]
				if t, err := time.Parse("2006-01-02", latestDate); err == nil {
					days := int(time.Since(t).Hours() / 24)
					if days <= 180 {
						result.Score += 10
						result.Notes = append(result.Notes, fmt.Sprintf("SEC filings seen recently (last filing %dd ago).", days))
					} else {
						result.Score += 4
						result.Notes = append(result.Notes, fmt.Sprintf("SEC filings exist but not recent (last filing %dd ago).", days))
					}
				}

				// Check for risk terms
				descBlob := strings.Join(recentFilings.PrimaryDocDescription[:min(15, len(recentFilings.PrimaryDocDescription))], " ")
				secRiskTerms = wordHitCount(descBlob, riskySECTerms)
				if secRiskTerms > 0 {
					result.Score -= min(12, 4*secRiskTerms)
					result.Flags = append(result.Flags, "sec_risk_terms_in_recent_filings")
					result.Notes = append(result.Notes, "Recent SEC filing descriptions contain risky terms.")
				}
			}

			if len(sub.Exchanges) > 0 {
				result.Score += 3
			}
		}
	} else {
		result.Public = new(false)
		result.Notes = append(result.Notes, "No SEC CIK match found (likely private).")
	}

	if ShouldUseBrowserCompanyProfile(result) && sources.FetchCompanySiteProfile != nil {
		searchName := company
		if result.DiscoveredName != "" {
			searchName = result.DiscoveredName
		}
		if profile, err := sources.FetchCompanySiteProfile(searchName); err == nil {
			ApplyCompanySiteProfile(result, profile)
		}
	}

	if result.Public != nil && !*result.Public && result.AgeYears != nil && *result.AgeYears >= 5 {
		result.Score += 8
		result.Notes = append(result.Notes, "Private stability bonus (+8 for >5y age).")
	}

	// News sentiment and layoffs
	var layoffSignals []LayoffSignal
	negHits := 0
	posHits := 0

	if includeNews {
		// HN Vibe Check
		hnSignals, hnNeg, hnPos, rejectedHN := fetchHNVibeCheckForContext(identity)
		result.RejectedEvidence = append(result.RejectedEvidence, rejectedHN...)
		if len(hnSignals) > 0 {
			result.SignalsUsed = append(result.SignalsUsed, "hn_vibe_check")
			result.HNSignals = hnSignals
			result.Sources["hn"] = map[string]any{
				"neg_hits": hnNeg,
				"pos_hits": hnPos,
			}

			if hnNeg >= 2 {
				result.Score -= 8
				result.Flags = append(result.Flags, "hn_negative_vibe")
				result.Notes = append(result.Notes, "Hacker News discussions suggest negative sentiment/risk.")
			} else if hnPos >= 2 && hnNeg == 0 {
				result.Score += 5
				result.Notes = append(result.Notes, "Positive mentions/vibe detected on Hacker News.")
			}
		}

		titles, neg, pos, rejectedNews, err := googleNewsSentimentForContext(identity)
		result.RejectedEvidence = append(result.RejectedEvidence, rejectedNews...)
		if err == nil {
			result.SignalsUsed = append(result.SignalsUsed, "google_news_rss")
			result.Sources["news"] = map[string]any{
				"titles":   titles,
				"neg_hits": neg,
				"pos_hits": pos,
			}
			negHits = neg
			posHits = pos
			if neg >= 3 && neg > pos {
				result.Score -= 10
				result.Flags = append(result.Flags, "negative_news_signal")
				result.Notes = append(result.Notes, "News titles contain multiple negative-risk keywords.")
			} else if neg >= 1 && neg > pos {
				result.Score -= 4
				result.Notes = append(result.Notes, "News titles contain some negative-risk keywords.")
			} else if neg == 0 && len(titles) > 5 {
				// Bonus for clean record if we actually found news
				result.Score += 5
				result.Notes = append(result.Notes, "No negative news keywords detected in recent headlines.")
			}
			if pos >= 2 && pos > neg {
				result.Score += 4
				result.Notes = append(result.Notes, "News titles contain multiple positive-momentum keywords.")
			} else if pos >= 1 && pos > neg {
				result.Score += 2
				result.Notes = append(result.Notes, "News titles contain some positive-momentum keywords.")
			}
			if result.Confidence == "low" && len(titles) > 0 {
				result.Confidence = "medium"
			}
		}
		var rejectedLayoffNews []CompanyHealthEvidence
		layoffSignals, rejectedLayoffNews = fetchLayoffSignalsForContextWithRejected(identity)
		result.RejectedEvidence = append(result.RejectedEvidence, rejectedLayoffNews...)
		var layoffsFYISignals []LayoffSignal
		if sources.FetchLayoffsFYI != nil {
			if signals, err := sources.FetchLayoffsFYI(company); err == nil && len(signals) > 0 {
				var rejectedLayoffs []CompanyHealthEvidence
				layoffsFYISignals, rejectedLayoffs = filterLayoffSignalsForContext(signals, identity)
				result.RejectedEvidence = append(result.RejectedEvidence, rejectedLayoffs...)
			}
		}
		if len(layoffsFYISignals) > 0 {
			result.SignalsUsed = append(result.SignalsUsed, "layoffs_fyi")
			result.Sources["layoffs_fyi"] = layoffsFYISignals
			layoffSignals = mergeLayoffSignals(layoffSignals, layoffsFYISignals)
		}
		if len(layoffSignals) > 0 {
			// REMOVED DIRECT PENALTY to avoid double-counting.
			// Layoffs now feed into EmploymentRisk, which caps the score downstream.
			result.Flags = append(result.Flags, "layoff_news_detected")
			result.Notes = append(result.Notes, fmt.Sprintf("Found %d recent layoff headlines (Contributing to Risk Score).", len(layoffSignals)))
			result.LayoffSignals = layoffSignals
		}
	}

	// Stock history
	var stockHistory []float64
	if result.DiscoveredTicker != "" {
		if hist, err := fetchStockHistory(result.DiscoveredTicker); err == nil {
			stockHistory = hist
			result.Sources["stock_history"] = hist
		}
	}

	// Calculate employment risk
	result.EmploymentRisk = calculateEmploymentRisk(layoffSignals, stockHistory, secRiskTerms, negHits, posHits, result.EstimatedEmployees)

	// INTELLIGENT CAP: If employment risk is high, the overall health score cannot be "Good"
	if result.EmploymentRisk.Score >= 75 { // Critical Risk
		result.Score -= 30
		result.Notes = append(result.Notes, "Heavy Penalty: Critical Employment Risk detected.")
		if result.Score > 40 {
			result.Score = 40 // Cap at Red/Orange
		}
	} else if result.EmploymentRisk.Score >= 50 { // High Risk
		result.Score -= 15
		result.Notes = append(result.Notes, "Penalty: High Employment Risk detected.")
		if result.Score > 55 {
			result.Score = 55 // Cap at Yellow/Warning
		}
	} else if result.EmploymentRisk.Score >= 25 { // Medium Risk
		result.Score -= 5
	}

	// Clamp score
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 100 {
		result.Score = 100
	}

	finalizeCompanyHealthAssessments(result)

	// Add overall assessment
	if result.Score >= 75 {
		result.Notes = append([]string{"Overall: looks relatively healthy (signals suggest stability)."}, result.Notes...)
	} else if result.Score >= 55 {
		result.Notes = append([]string{"Overall: mixed/unknown (some positive signals, limited certainty)."}, result.Notes...)
	} else {
		result.Notes = append([]string{"Overall: caution (negative signals or lack of stabilizing signals)."}, result.Notes...)
	}

	return result, nil
}

func optionalIntString(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}
