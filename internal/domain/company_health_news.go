package domain

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// RSSFeed represents an RSS feed structure
type RSSFeed struct {
	Channel struct {
		Items []RSSItem `xml:"item"`
	} `xml:"channel"`
}

// RSSItem represents a single RSS item
type RSSItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
}

// fetchGoogleNewsRSS fetches and parses Google News RSS feed
func fetchGoogleNewsRSS(query string) ([]RSSItem, error) {
	rssURL := fmt.Sprintf(googleNewsRSSURL, url.QueryEscape(fmt.Sprintf(`"%s"`, query)))

	client := &http.Client{Timeout: requestTimeout}
	req, err := http.NewRequest("GET", rssURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", companyHealthUserAgent())
	req.Header.Set("Accept", "application/rss+xml,application/xml,text/xml,*/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var feed RSSFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	return feed.Channel.Items, nil
}

// fetchLayoffSignals searches news specifically for layoff mentions
func fetchLayoffSignals(company string) []LayoffSignal {
	query := fmt.Sprintf(`"%s" layoffs`, company)
	items, err := fetchGoogleNewsRSS(query)
	if err != nil {
		return nil
	}

	signals := []LayoffSignal{}
	layoffPattern := regexp.MustCompile(`(?i)(\b\d{1,3}(?:,\d{3})*|\d+%)\s+(?:jobs|employees|staff|workers|people|roles|positions|cuts|layoffs|reduction in force|rif)`)

	// Exclusion patterns for "industry trend" headlines where the company is mentioned but not the one laying off
	exclusionPattern := regexp.MustCompile(`(?i)(?:as layoffs sweep|amid layoffs|despite layoffs|industry layoffs|tech layoffs|layoffs sweep|market layoffs|sector layoffs)`)

	for _, item := range items {
		title := item.Title
		titleLower := strings.ToLower(title)

		// Skip if it matches an exclusion pattern (e.g. "Nvidia grows AS layoffs sweep")
		if exclusionPattern.MatchString(title) {
			continue
		}

		if layoffPattern.MatchString(title) ||
			strings.Contains(titleLower, "layoff") ||
			strings.Contains(titleLower, "job cut") ||
			strings.Contains(titleLower, "cutting jobs") ||
			strings.Contains(titleLower, "downsize") ||
			strings.Contains(titleLower, "rif") ||
			strings.Contains(titleLower, "reduction in force") ||
			strings.Contains(titleLower, "restructuring") {

			// Strict check: Ensure company name is actually in the title (Google sometimes returns loose matches)
			// and treat it as a word boundary match.
			// Use strictly case-sensitive matching to avoid dictionary word collisions.
			pattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(company))
			companyRe := regexp.MustCompile(pattern)
			if !companyRe.MatchString(title) {
				continue
			}

			signal := LayoffSignal{
				Title: title,
				URL:   item.Link,
			}

			// Try to parse date
			if item.PubDate != "" {
				if t, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
					signal.Date = &t
				}
			}

			// Extract employee count or percentage
			matches := layoffPattern.FindStringSubmatch(title)
			if len(matches) > 1 {
				numStr := strings.ReplaceAll(matches[1], ",", "")
				if strings.HasSuffix(numStr, "%") {
					signal.PercentageStr = numStr
				} else {
					var empCount int
					if _, err := fmt.Sscanf(numStr, "%d", &empCount); err == nil && empCount > 0 {
						signal.EmployeeCount = new(empCount)
					}
				}
			}

			signals = append(signals, signal)
			if len(signals) >= 5 {
				break
			}
		}
	}

	return signals
}

func fetchLayoffSignalsForContextWithRejected(identity CompanyHealthContext) ([]LayoffSignal, []CompanyHealthEvidence) {
	signals := fetchLayoffSignals(identity.Company)
	return filterLayoffSignalsForContext(signals, identity)
}

func filterLayoffSignalsForContext(signals []LayoffSignal, identity CompanyHealthContext) ([]LayoffSignal, []CompanyHealthEvidence) {
	if len(signals) == 0 {
		return nil, nil
	}
	out := make([]LayoffSignal, 0, len(signals))
	rejected := make([]CompanyHealthEvidence, 0)
	for _, signal := range signals {
		if ok, reason := healthEvidenceMatchesCompanyContext(signal.Title, signal.URL, identity); ok {
			out = append(out, signal)
		} else {
			rejected = append(rejected, rejectedHealthEvidence(signal.Title, "layoff_news", signal.URL, reason))
		}
	}
	return out, rejected
}

// googleNewsSentiment fetches news and counts positive/negative keywords
func googleNewsSentimentForContext(identity CompanyHealthContext) (titles []string, negHits, posHits int, rejected []CompanyHealthEvidence, err error) {
	items, err := fetchGoogleNewsRSS(identity.Company)
	if err != nil {
		return nil, 0, 0, nil, err
	}

	titles = []string{}
	for _, item := range items {
		ok, reason := healthEvidenceMatchesCompanyContext(item.Title, item.Link, identity)
		if !ok {
			rejected = append(rejected, rejectedHealthEvidence(item.Title, "google_news_rss", item.Link, reason))
			continue
		}
		titles = append(titles, item.Title)
		if len(titles) >= 25 {
			break
		}
	}

	blob := strings.Join(titles, " ")
	negHits = wordHitCount(blob, negNewsKeywords)
	posHits = wordHitCount(blob, posNewsKeywords)

	return titles, negHits, posHits, rejected, nil
}
