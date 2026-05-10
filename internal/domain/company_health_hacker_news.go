package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"time"
)

// HNAlgoliaResult represents the response from HN Algolia API
type HNAlgoliaResult struct {
	Hits []struct {
		Title       string    `json:"title"`
		URL         string    `json:"url"`
		Points      int       `json:"points"`
		NumComments int       `json:"num_comments"`
		CreatedAt   time.Time `json:"created_at"`
	} `json:"hits"`
}

// fetchHNVibeCheck searches Hacker News for company mentions
func fetchHNVibeCheck(company string) ([]HNSignal, int, int) {
	query := url.QueryEscape(company)
	data, err := httpGet(fmt.Sprintf(hnSearchURL, query))
	if err != nil {
		return nil, 0, 0
	}

	var res HNAlgoliaResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, 0, 0
	}

	signals := []HNSignal{}
	negHits := 0
	posHits := 0

	// Compile a strict regex for the company name to filter noise
	// e.g. "Braze" should not match "Brazen"
	// We handle "Inc", "Corp" removal elsewhere, so 'company' is likely just the name.
	// Use case-sensitive matching for the company name to avoid dictionary word collisions.
	companyRegex := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(company)))

	for _, hit := range res.Hits {
		// Only consider stories from the last 2 years for "vibe"
		if time.Since(hit.CreatedAt) > 2*365*24*time.Hour {
			continue
		}

		// STICT FILTER: Title must contain the company name as a whole word
		if !companyRegex.MatchString(hit.Title) {
			continue
		}

		sig := HNSignal{
			Title:       hit.Title,
			URL:         hit.URL,
			Points:      hit.Points,
			NumComments: hit.NumComments,
			Date:        hit.CreatedAt,
		}
		signals = append(signals, sig)

		// Check for negative sentiment
		if wordHitCount(hit.Title, hnNegKeywords) > 0 {
			// Higher weight for stories with many comments/points
			if hit.NumComments > 50 || hit.Points > 100 {
				negHits += 2
			} else {
				negHits++
			}
		}

		// Check for positive sentiment
		if wordHitCount(hit.Title, posNewsKeywords) > 0 {
			posHits++
		}
	}

	return signals, negHits, posHits
}

func fetchHNVibeCheckForContext(identity CompanyHealthContext) ([]HNSignal, int, int, []CompanyHealthEvidence) {
	signals, _, _ := fetchHNVibeCheck(identity.Company)
	if len(signals) == 0 {
		return nil, 0, 0, nil
	}

	filtered := make([]HNSignal, 0, len(signals))
	rejected := make([]CompanyHealthEvidence, 0)
	negHits := 0
	posHits := 0
	for _, signal := range signals {
		ok, reason := healthEvidenceMatchesCompanyContext(signal.Title, signal.URL, identity)
		if !ok {
			rejected = append(rejected, rejectedHealthEvidence(signal.Title, "hacker_news", signal.URL, reason))
			continue
		}
		filtered = append(filtered, signal)
		if wordHitCount(signal.Title, hnNegKeywords) > 0 {
			if signal.NumComments > 50 || signal.Points > 100 {
				negHits += 2
			} else {
				negHits++
			}
		}
		if wordHitCount(signal.Title, posNewsKeywords) > 0 {
			posHits++
		}
	}
	return filtered, negHits, posHits, rejected
}
