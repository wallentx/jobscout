package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var errSECCompanyNotFound = errors.New("company not found in SEC database")

// SECCompanyTicker represents an entry in the SEC tickers JSON
type SECCompanyTicker struct {
	CIKStr int    `json:"cik_str"`
	Ticker string `json:"ticker"`
	Title  string `json:"title"`
}

// SECFiling represents a single SEC filing
type SECFiling struct {
	Form       string `json:"form"`
	FilingDate string `json:"filingDate"`
	Desc       string `json:"desc"`
}

// SECSubmission represents SEC submissions data
type SECSubmission struct {
	CIK            string   `json:"cik"`
	Name           string   `json:"name"`
	Tickers        []string `json:"tickers"`
	Exchanges      []string `json:"exchanges"`
	SIC            string   `json:"sic"`
	SICDescription string   `json:"sicDescription"`
	Filings        struct {
		Recent struct {
			AccessionNumber       []string `json:"accessionNumber"`
			Form                  []string `json:"form"`
			FilingDate            []string `json:"filingDate"`
			PrimaryDocument       []string `json:"primaryDocument"`
			PrimaryDocDescription []string `json:"primaryDocDescription"`
		} `json:"recent"`
	} `json:"filings"`
}

// secLookupCIK looks up a company's CIK in the SEC database
func secLookupCIK(company string, ticker string) (cik10, foundTicker, foundName string, err error) {
	data, err := httpGet(secTickersURL)
	if err != nil {
		return "", "", "", err
	}

	var tickers map[string]SECCompanyTicker
	if err := json.Unmarshal(data, &tickers); err != nil {
		return "", "", "", err
	}

	companyLower := strings.ToLower(strings.TrimSpace(company))
	companyClean := cleanCompanyName(companyLower)
	tickerLower := strings.ToLower(strings.TrimSpace(ticker))

	// Try exact ticker match first
	if tickerLower != "" {
		for _, entry := range tickers {
			if strings.ToLower(entry.Ticker) == tickerLower {
				return fmt.Sprintf("%010d", entry.CIKStr), entry.Ticker, entry.Title, nil
			}
		}
	}

	// Try exact name match
	for _, entry := range tickers {
		if strings.ToLower(entry.Title) == companyLower {
			return fmt.Sprintf("%010d", entry.CIKStr), entry.Ticker, entry.Title, nil
		}
	}

	// Try clean name match
	for _, entry := range tickers {
		if cleanCompanyName(strings.ToLower(entry.Title)) == companyClean {
			return fmt.Sprintf("%010d", entry.CIKStr), entry.Ticker, entry.Title, nil
		}
	}

	return "", "", "", errSECCompanyNotFound
}

// secGetSubmissions fetches SEC submissions for a CIK
func secGetSubmissions(cik10 string) (*SECSubmission, error) {
	data, err := httpGet(fmt.Sprintf(secSubmissionsURL, cik10))
	if err != nil {
		return nil, err
	}

	var sub SECSubmission
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, err
	}

	return &sub, nil
}

// secGetHeadcount attempts to parse employee count from the latest 10-K filing
func secGetHeadcount(cik10 string, sub *SECSubmission) *int {
	if sub == nil {
		return nil
	}

	// Find the most recent 10-K
	idx := -1
	for i, form := range sub.Filings.Recent.Form {
		if form == "10-K" {
			idx = i
			break
		}
	}

	if idx == -1 {
		return nil
	}

	accession := sub.Filings.Recent.AccessionNumber[idx]
	accessionClean := strings.ReplaceAll(accession, "-", "")
	primaryDoc := sub.Filings.Recent.PrimaryDocument[idx]

	// CIK in URL is often without leading zeros or with them, SEC usually works with 10-digit
	// But the directory structure uses the raw integer often.
	// We'll try the cleaned cik (no leading zeros) which is common in Archives path
	cikInt := 0
	_, _ = fmt.Sscanf(cik10, "%d", &cikInt)

	docURL := fmt.Sprintf(secDocumentURL, fmt.Sprintf("%d", cikInt), accessionClean, primaryDoc)

	// Fetch the 10-K. These are HUGE, so we only read the beginning.
	// Actually httpGet reads the whole thing. For now, let's limit it if possible.
	// But our httpGet doesn't support range. Let's just try to read it.
	data, err := httpGet(docURL)
	if err != nil {
		return nil
	}

	// 10-Ks are often many MBs. Let's look for headcount patterns.
	// Pattern 1: "we had approximately 12,345 employees"
	// Pattern 2: "employed approximately 12,300 people"
	// Pattern 3: "approximately 12,345 full-time employees"
	text := string(data)
	if len(text) > 500000 {
		// Just look in first 500KB to save memory/CPU
		text = text[:500000]
	}

	re := regexp.MustCompile(`(?i)(?:had|employed|approximately|total|of)\s+([\d,]+)\s+(?:full-time\s+)?(?:employees|people|workers|persons)`)
	matches := re.FindAllStringSubmatch(text, 5) // Get first few matches

	for _, match := range matches {
		if len(match) > 1 {
			numStr := strings.ReplaceAll(match[1], ",", "")
			var count int
			if _, err := fmt.Sscanf(numStr, "%d", &count); err == nil && count > 10 {
				return new(count)
			}
		}
	}

	return nil
}
