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

type googleNewsSentimentResult struct {
	Titles         []string
	ConcernStories []CompanyHealthConcernStory
	NegHits        int
	PosHits        int
	Rejected       []CompanyHealthEvidence
}

// fetchGoogleNewsRSS fetches and parses Google News RSS feed.
var fetchGoogleNewsRSS = defaultFetchGoogleNewsRSS

func defaultFetchGoogleNewsRSS(query string) ([]RSSItem, error) {
	rssURL := fmt.Sprintf(googleNewsRSSURL, url.QueryEscape(googleNewsRSSQuery(query)))

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

func googleNewsRSSQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return query
	}
	if googleNewsQueryHasExplicitSyntax(query) {
		return query
	}
	return fmt.Sprintf(`"%s"`, query)
}

func googleNewsQueryHasExplicitSyntax(query string) bool {
	if strings.Contains(query, `"`) {
		return true
	}
	for _, field := range strings.Fields(query) {
		trimmed := strings.Trim(field, "()")
		switch strings.ToUpper(trimmed) {
		case "AND", "OR", "NOT":
			return true
		}
		if strings.HasPrefix(field, "-") ||
			strings.Contains(field, ":") ||
			strings.ContainsAny(field, "()") {
			return true
		}
	}
	return false
}

func fetchLayoffSignalsForContextWithRejected(identity CompanyHealthContext) ([]LayoffSignal, []CompanyHealthEvidence) {
	signals := fetchLayoffSignalsForQueries(companyHealthLayoffQueries(identity))
	return filterLayoffSignalsForContext(signals, identity)
}

func fetchLayoffSignalsForQueries(queries []string) []LayoffSignal {
	if len(queries) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var signals []LayoffSignal
	for _, query := range queries {
		items, err := fetchGoogleNewsRSS(query)
		if err != nil {
			continue
		}
		for _, signal := range layoffSignalsFromRSSItems(items, query) {
			key := strings.TrimSpace(signal.Title) + "\x00" + strings.TrimSpace(signal.URL)
			if key == "\x00" || seen[key] {
				continue
			}
			seen[key] = true
			signals = append(signals, signal)
			if len(signals) >= 5 {
				return signals
			}
		}
	}
	return signals
}

func layoffSignalsFromRSSItems(items []RSSItem, company string) []LayoffSignal {
	company = layoffCompanyNameFromQuery(company)
	if company == "" {
		return nil
	}
	signals := []LayoffSignal{}
	layoffPattern := regexp.MustCompile(`(?i)(\b\d{1,3}(?:,\d{3})*|\d+%)\s+(?:jobs|employees|staff|workers|people|roles|positions|cuts|layoffs|reduction in force|rif)`)
	exclusionPattern := regexp.MustCompile(`(?i)(?:as layoffs sweep|amid layoffs|despite layoffs|industry layoffs|tech layoffs|layoffs sweep|market layoffs|sector layoffs)`)

	for _, item := range items {
		title := item.Title
		titleLower := strings.ToLower(title)
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
			pattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(company))
			companyRe := regexp.MustCompile(pattern)
			if !companyRe.MatchString(title) {
				continue
			}

			signal := LayoffSignal{
				Title: title,
				URL:   item.Link,
			}
			if item.PubDate != "" {
				if t, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
					signal.Date = &t
				}
			}
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

func layoffCompanyNameFromQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	if strings.HasPrefix(query, `"`) {
		rest := query[1:]
		if idx := strings.Index(rest, `"`); idx >= 0 {
			return strings.TrimSpace(rest[:idx])
		}
	}
	return strings.Trim(query, `" `)
}

func companyHealthLayoffQueries(identity CompanyHealthContext) []string {
	return companyHealthNewsQueriesWithSuffix(identity, "layoffs")
}

func companyHealthNewsQueries(identity CompanyHealthContext) []string {
	return companyHealthNewsQueriesWithSuffix(identity, "")
}

func companyHealthNewsQueriesWithSuffix(identity CompanyHealthContext, suffix string) []string {
	names := companyHealthContextNames(identity)
	if len(names) == 0 {
		return nil
	}
	contextTerms := companyHealthSearchContextTerms(identity)
	queries := make([]string, 0, len(names))
	for _, name := range names {
		var queryParts []string
		if len(contextTerms) > 0 || strings.TrimSpace(suffix) != "" {
			queryParts = append(queryParts, fmt.Sprintf(`"%s"`, name))
		} else {
			queryParts = append(queryParts, name)
		}
		queryParts = append(queryParts, contextTerms...)
		if suffix = strings.TrimSpace(suffix); suffix != "" {
			queryParts = append(queryParts, suffix)
		}
		queries = append(queries, strings.Join(queryParts, " "))
	}
	return queries
}

func companyHealthSearchContextTerms(identity CompanyHealthContext) []string {
	var terms []string
	if domain := companyHealthContextDomain(identity); domain != "" {
		terms = append(terms, domain)
	}
	for _, text := range append([]string{identity.Industry}, identity.Industries...) {
		for _, term := range tokenizeCompanyContext(text) {
			if companyContextStopword(term) {
				continue
			}
			terms = appendUniqueSearchTerm(terms, term)
			if len(terms) >= 4 {
				return terms
			}
		}
	}
	return terms
}

func appendUniqueSearchTerm(values []string, value string) []string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return values
	}
	valueRoot := strings.TrimSuffix(value, "s")
	for _, existing := range values {
		existingRoot := strings.TrimSuffix(existing, "s")
		if existing == value || existingRoot == valueRoot {
			return values
		}
	}
	return append(values, value)
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

func googleNewsSentimentForContextWithFetcher(identity CompanyHealthContext, fetch func(string) ([]RSSItem, error)) (titles []string, negHits, posHits int, rejected []CompanyHealthEvidence, err error) {
	result, err := googleNewsSentimentDetailsForContextWithFetcher(identity, fetch)
	if err != nil {
		return nil, 0, 0, nil, err
	}
	return result.Titles, result.NegHits, result.PosHits, result.Rejected, nil
}

func googleNewsSentimentDetailsForContext(identity CompanyHealthContext) (googleNewsSentimentResult, error) {
	return googleNewsSentimentDetailsForContextWithFetcher(identity, fetchGoogleNewsRSS)
}

func googleNewsSentimentDetailsForContextWithFetcher(identity CompanyHealthContext, fetch func(string) ([]RSSItem, error)) (googleNewsSentimentResult, error) {
	var items []RSSItem
	seenItems := make(map[string]bool)
	var firstErr error
	for _, query := range companyHealthNewsQueries(identity) {
		fetched, fetchErr := fetch(query)
		if fetchErr != nil {
			if firstErr == nil {
				firstErr = fetchErr
			}
			continue
		}
		for _, item := range fetched {
			key := strings.TrimSpace(item.Title) + "\x00" + strings.TrimSpace(item.Link)
			if key == "\x00" || seenItems[key] {
				continue
			}
			seenItems[key] = true
			items = append(items, item)
		}
	}
	if len(items) == 0 && firstErr != nil {
		return googleNewsSentimentResult{}, firstErr
	}

	result := googleNewsSentimentResult{Titles: []string{}}
	for _, item := range items {
		ok, reason := healthEvidenceMatchesCompanyContext(item.Title, item.Link, identity)
		if !ok {
			result.Rejected = append(result.Rejected, rejectedHealthEvidence(item.Title, "google_news_rss", item.Link, reason))
			continue
		}
		result.Titles = append(result.Titles, item.Title)
		if story, ok := companyHealthConcernStoryFromRSSItem("google_news_rss", item, negNewsKeywords); ok {
			result.ConcernStories = appendUniqueCompanyHealthConcernStories(result.ConcernStories, story)
		}
		if len(result.Titles) >= 25 {
			break
		}
	}

	blob := strings.Join(result.Titles, " ")
	result.NegHits = wordHitCount(blob, negNewsKeywords)
	result.PosHits = wordHitCount(blob, posNewsKeywords)

	return result, nil
}
