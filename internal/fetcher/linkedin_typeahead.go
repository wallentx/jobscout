package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const linkedInTypeaheadDefaultURL = "https://www.linkedin.com/jobs-guest/api/typeaheadHits"

var linkedInTypeaheadURL = linkedInTypeaheadDefaultURL

type linkedInTypeaheadHit struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
}

type linkedInTypeaheadCacheEntry struct {
	Kind        string    `json:"kind"`
	Query       string    `json:"query"`
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	FetchedAt   time.Time `json:"fetched_at"`
}

type linkedInTypeaheadCache map[string]linkedInTypeaheadCacheEntry

func refreshLinkedInCriteriaHints(ctx context.Context, criteria *CriteriaConfig) error {
	if criteria == nil {
		return nil
	}
	var firstErr error
	query := linkedInLocationQuery(criteria)
	if strings.TrimSpace(query) != "" && linkedInCachedGeoID(query) == "" {
		hit, err := resolveLinkedInGeo(ctx, query)
		if err != nil {
			firstErr = err
		} else if err := saveLinkedInTypeaheadCacheEntry("geo", query, hit); err != nil {
			firstErr = err
		}
	}
	if err := refreshLinkedInTitleHints(ctx, criteria); err != nil && firstErr == nil {
		return err
	}
	return firstErr
}

func RefreshLinkedInCriteriaHints(ctx context.Context, criteria *CriteriaConfig) error {
	return refreshLinkedInCriteriaHints(ctx, criteria)
}

func refreshLinkedInTitleHints(ctx context.Context, criteria *CriteriaConfig) error {
	for _, query := range targetedSiteSearchQueries(criteria) {
		if linkedInCachedTitle(query) != "" {
			continue
		}
		hit, err := resolveLinkedInTitle(ctx, query)
		if err != nil {
			return err
		}
		if err := saveLinkedInTypeaheadCacheEntry("title", query, hit); err != nil {
			return err
		}
	}
	return nil
}

func linkedInCachedGeoID(locationQuery string) string {
	entry, ok := loadLinkedInTypeaheadCacheEntry("geo", locationQuery)
	if !ok || !validLinkedInID(entry.ID) {
		return ""
	}
	return strings.TrimSpace(entry.ID)
}

func linkedInCachedTitle(query string) string {
	entry, ok := loadLinkedInTypeaheadCacheEntry("title", query)
	if !ok || !validLinkedInID(entry.ID) || strings.TrimSpace(entry.DisplayName) == "" {
		return ""
	}
	return strings.TrimSpace(entry.DisplayName)
}

func loadLinkedInTypeaheadCacheEntry(kind string, query string) (linkedInTypeaheadCacheEntry, bool) {
	cache, err := loadLinkedInTypeaheadCache()
	if err != nil {
		return linkedInTypeaheadCacheEntry{}, false
	}
	entry, ok := cache[linkedInTypeaheadCacheKey(kind, query)]
	if !ok {
		return linkedInTypeaheadCacheEntry{}, false
	}
	if !strings.EqualFold(strings.TrimSpace(entry.Kind), strings.TrimSpace(kind)) ||
		!strings.EqualFold(strings.TrimSpace(entry.Query), strings.TrimSpace(query)) {
		return linkedInTypeaheadCacheEntry{}, false
	}
	return entry, true
}

func saveLinkedInTypeaheadCacheEntry(kind string, query string, hit linkedInTypeaheadHit) error {
	if strings.TrimSpace(runtimeLinkedInTypeaheadCachePath) == "" {
		return nil
	}
	if !validLinkedInID(hit.ID) {
		return fmt.Errorf("LinkedIn returned invalid %s id %q for %q", kind, hit.ID, query)
	}
	cache, err := loadLinkedInTypeaheadCache()
	if err != nil {
		return err
	}
	cache[linkedInTypeaheadCacheKey(kind, query)] = linkedInTypeaheadCacheEntry{
		Kind:        strings.TrimSpace(kind),
		Query:       strings.TrimSpace(query),
		ID:          strings.TrimSpace(hit.ID),
		DisplayName: strings.TrimSpace(hit.DisplayName),
		FetchedAt:   time.Now(),
	}
	return saveLinkedInTypeaheadCache(cache)
}

func loadLinkedInTypeaheadCache() (linkedInTypeaheadCache, error) {
	if strings.TrimSpace(runtimeLinkedInTypeaheadCachePath) == "" {
		return linkedInTypeaheadCache{}, nil
	}
	data, err := os.ReadFile(runtimeLinkedInTypeaheadCachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return linkedInTypeaheadCache{}, nil
		}
		return nil, err
	}
	var cache linkedInTypeaheadCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return linkedInTypeaheadCache{}, nil
	}
	if cache == nil {
		cache = linkedInTypeaheadCache{}
	}
	return cache, nil
}

func saveLinkedInTypeaheadCache(cache linkedInTypeaheadCache) error {
	if strings.TrimSpace(runtimeLinkedInTypeaheadCachePath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(runtimeLinkedInTypeaheadCachePath), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(runtimeLinkedInTypeaheadCachePath, data, 0600)
}

func linkedInTypeaheadCacheKey(kind string, query string) string {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	normalizedQuery := strings.ToLower(whitespaceRe.ReplaceAllString(strings.TrimSpace(query), " "))
	return normalizedKind + ":" + normalizedQuery
}

func validLinkedInID(value string) bool {
	return regexp.MustCompile(`^\d+$`).MatchString(strings.TrimSpace(value))
}

func resolveLinkedInGeo(ctx context.Context, query string) (linkedInTypeaheadHit, error) {
	return resolveLinkedInTypeahead(ctx, "GEO", query, func(values url.Values) {
		values.Set("geoTypes", "POPULATED_PLACE,ADMIN_DIVISION_2,MARKET_AREA,COUNTRY_REGION")
	}, bestLinkedInTypeaheadHit)
}

func resolveLinkedInTitle(ctx context.Context, query string) (linkedInTypeaheadHit, error) {
	return resolveLinkedInTypeahead(ctx, "JOB_TITLE", query, nil, bestLinkedInTitleHit)
}

func resolveLinkedInTypeahead(ctx context.Context, typeaheadType string, query string, configure func(url.Values), choose func(string, []linkedInTypeaheadHit) linkedInTypeaheadHit) (linkedInTypeaheadHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return linkedInTypeaheadHit{}, nil
	}
	endpoint, err := url.Parse(linkedInTypeaheadURL)
	if err != nil {
		return linkedInTypeaheadHit{}, fmt.Errorf("parse LinkedIn typeahead URL: %w", err)
	}
	values := endpoint.Query()
	values.Set("typeaheadType", typeaheadType)
	values.Set("query", query)
	if configure != nil {
		configure(values)
	}
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return linkedInTypeaheadHit{}, fmt.Errorf("build LinkedIn typeahead request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", jobscoutUserAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return linkedInTypeaheadHit{}, fmt.Errorf("fetch LinkedIn %s suggestion for %q: %w", typeaheadType, query, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return linkedInTypeaheadHit{}, fmt.Errorf("fetch LinkedIn %s suggestion for %q: HTTP %d", typeaheadType, query, resp.StatusCode)
	}
	var hits []linkedInTypeaheadHit
	if err := json.NewDecoder(resp.Body).Decode(&hits); err != nil {
		return linkedInTypeaheadHit{}, fmt.Errorf("decode LinkedIn %s suggestion for %q: %w", typeaheadType, query, err)
	}
	if len(hits) == 0 {
		return linkedInTypeaheadHit{}, fmt.Errorf("no LinkedIn %s suggestion found for %q", typeaheadType, query)
	}
	if hit := choose(query, hits); hit.ID != "" {
		return hit, nil
	}
	return linkedInTypeaheadHit{}, fmt.Errorf("no usable LinkedIn %s suggestion found for %q", typeaheadType, query)
}

func bestLinkedInTypeaheadHit(query string, hits []linkedInTypeaheadHit) linkedInTypeaheadHit {
	tokens := linkedInTypeaheadMatchTokens(query)
	var best linkedInTypeaheadHit
	bestScore := 0
	for _, hit := range hits {
		if strings.TrimSpace(hit.ID) == "" {
			continue
		}
		display := strings.ToLower(strings.TrimSpace(hit.DisplayName))
		score := 0
		for _, token := range tokens {
			if strings.Contains(display, token) {
				score += 2
			}
		}
		if strings.Contains(display, "metropolitan area") || strings.Contains(display, " county,") {
			score--
		}
		if strings.Contains(display, "united states") {
			score++
		}
		if score > bestScore {
			best = hit
			bestScore = score
		}
	}
	if bestScore > 0 {
		return best
	}
	for _, hit := range hits {
		if strings.TrimSpace(hit.ID) != "" {
			return hit
		}
	}
	return linkedInTypeaheadHit{}
}

func bestLinkedInTitleHit(query string, hits []linkedInTypeaheadHit) linkedInTypeaheadHit {
	query = strings.TrimSpace(query)
	for _, hit := range hits {
		if !strings.EqualFold(strings.TrimSpace(hit.Type), "TITLE") || strings.TrimSpace(hit.ID) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(hit.DisplayName), query) {
			return hit
		}
	}
	tokens := linkedInTypeaheadMatchTokens(query)
	var best linkedInTypeaheadHit
	bestScore := 0
	for _, hit := range hits {
		if !strings.EqualFold(strings.TrimSpace(hit.Type), "TITLE") || strings.TrimSpace(hit.ID) == "" {
			continue
		}
		display := strings.ToLower(strings.TrimSpace(hit.DisplayName))
		score := 0
		for _, token := range tokens {
			if strings.Contains(display, token) {
				score += 2
			}
		}
		if score > bestScore {
			best = hit
			bestScore = score
		}
	}
	return best
}

func linkedInTypeaheadMatchTokens(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len(field) < 3 {
			if expanded := linkedInUSStateToken(field); expanded != "" {
				tokens = append(tokens, expanded)
			}
			continue
		}
		tokens = append(tokens, field)
	}
	return tokens
}

func linkedInUSStateToken(token string) string {
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "AL":
		return "alabama"
	case "AK":
		return "alaska"
	case "AZ":
		return "arizona"
	case "AR":
		return "arkansas"
	case "CA":
		return "california"
	case "CO":
		return "colorado"
	case "CT":
		return "connecticut"
	case "DE":
		return "delaware"
	case "FL":
		return "florida"
	case "GA":
		return "georgia"
	case "HI":
		return "hawaii"
	case "ID":
		return "idaho"
	case "IL":
		return "illinois"
	case "IN":
		return "indiana"
	case "IA":
		return "iowa"
	case "KS":
		return "kansas"
	case "KY":
		return "kentucky"
	case "LA":
		return "louisiana"
	case "ME":
		return "maine"
	case "MD":
		return "maryland"
	case "MA":
		return "massachusetts"
	case "MI":
		return "michigan"
	case "MN":
		return "minnesota"
	case "MS":
		return "mississippi"
	case "MO":
		return "missouri"
	case "MT":
		return "montana"
	case "NE":
		return "nebraska"
	case "NV":
		return "nevada"
	case "NH":
		return "new hampshire"
	case "NJ":
		return "new jersey"
	case "NM":
		return "new mexico"
	case "NY":
		return "new york"
	case "NC":
		return "north carolina"
	case "ND":
		return "north dakota"
	case "OH":
		return "ohio"
	case "OK":
		return "oklahoma"
	case "OR":
		return "oregon"
	case "PA":
		return "pennsylvania"
	case "RI":
		return "rhode island"
	case "SC":
		return "south carolina"
	case "SD":
		return "south dakota"
	case "TN":
		return "tennessee"
	case "TX":
		return "texas"
	case "UT":
		return "utah"
	case "VT":
		return "vermont"
	case "VA":
		return "virginia"
	case "WA":
		return "washington"
	case "WV":
		return "west virginia"
	case "WI":
		return "wisconsin"
	case "WY":
		return "wyoming"
	default:
		return ""
	}
}
