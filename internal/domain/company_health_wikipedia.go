package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// WikiSearchResult represents Wikipedia search results
type WikiSearchResult struct {
	Pages []struct {
		Title string `json:"title"`
	} `json:"pages"`
}

// WikiSummary represents Wikipedia page summary
type WikiSummary struct {
	Title        string `json:"title"`
	Extract      string `json:"extract"`
	WikibaseItem string `json:"wikibase_item"`
}

type wikidataEntityResponse struct {
	Entities map[string]wikidataEntity `json:"entities"`
}

type wikidataEntity struct {
	Claims map[string][]wikidataClaim `json:"claims"`
}

type wikidataClaim struct {
	Rank       string                    `json:"rank"`
	Mainsnak   wikidataSnak              `json:"mainsnak"`
	Qualifiers map[string][]wikidataSnak `json:"qualifiers"`
}

type wikidataSnak struct {
	Datavalue *wikidataDataValue `json:"datavalue"`
}

type wikidataDataValue struct {
	Value json.RawMessage `json:"value"`
}

type WikidataCompanyFacts struct {
	EntityID                string
	EntityURL               string
	FoundedYear             *int
	EmployeeCount           *int
	FoundedYearClaimCount   int
	EmployeeCountClaimCount int
}

// validateWikiRelevance checks if the found page is likely the correct tech company
func validateWikiRelevance(query string, summary *WikiSummary) bool {
	titleLower := strings.ToLower(summary.Title)
	extractLower := strings.ToLower(summary.Extract)
	queryLower := strings.ToLower(query)

	// 1. If query contains "AI", "Labs", "Tech", but title doesn't match roughly
	techIndicators := []string{"ai", "labs", "technology", "software", "systems", "cloud", "data", "security", "robotics"}

	isTechQuery := false
	for _, kw := range techIndicators {
		if strings.Contains(queryLower, kw) {
			isTechQuery = true
			break
		}
	}

	// If it's a tech query, the summary MUST contain tech keywords
	if isTechQuery {
		hasTechContext := false
		techContexts := []string{
			"technology", "software", "computer", "intelligence", "platform",
			"startup", "app", "mobile", "web", "internet", "saas", "tech",
		}
		for _, ctx := range techContexts {
			if strings.Contains(extractLower, ctx) {
				hasTechContext = true
				break
			}
		}
		if !hasTechContext {
			return false
		}
	}

	// 2. Sanity Check: "AI" company founded before 1990? Suspect.
	// Only apply if the name explicitly says "AI"
	if strings.Contains(queryLower, "ai") {
		year := parseYearFromText(summary.Extract)
		if year != nil && *year < 1990 {
			// Exception: Huge pivots (Nvidia, etc) but they usually match title exactly.
			// If title doesn't contain AI but query does, and it's old, it's likely wrong.
			if !strings.Contains(titleLower, "ai") {
				return false
			}
		}
	}

	return true
}

// wikiGetSummary searches for and retrieves a Wikipedia summary
func wikiGetSummary(company string) (summary *WikiSummary, err error) {
	// Search for title
	searchURL := fmt.Sprintf("%s?q=%s&limit=1", wikiTitleSearchURL, url.QueryEscape(company))
	data, err := httpGet(searchURL)
	if err != nil {
		return nil, err
	}

	var searchResult WikiSearchResult
	if err := json.Unmarshal(data, &searchResult); err != nil {
		return nil, err
	}

	if len(searchResult.Pages) == 0 {
		return nil, fmt.Errorf("no Wikipedia page found")
	}

	// Get first result
	title := searchResult.Pages[0].Title

	// Get summary
	summaryURL := fmt.Sprintf(wikiSummaryURL, url.PathEscape(title))
	data, err = httpGet(summaryURL)
	if err != nil {
		return nil, err
	}

	var wikiSum WikiSummary
	if err := json.Unmarshal(data, &wikiSum); err != nil {
		return nil, err
	}

	// Validate
	if !validateWikiRelevance(company, &wikiSum) {
		return nil, fmt.Errorf("wikipedia result deemed irrelevant")
	}

	return &wikiSum, nil
}

func wikiGetCompanyFacts(summary *WikiSummary) (*WikidataCompanyFacts, error) {
	if summary == nil || strings.TrimSpace(summary.WikibaseItem) == "" {
		return nil, fmt.Errorf("missing wikidata entity id")
	}
	entityID := strings.TrimSpace(summary.WikibaseItem)
	data, err := httpGet(fmt.Sprintf(wikidataEntityURL, url.PathEscape(entityID)))
	if err != nil {
		return nil, err
	}
	return parseWikidataCompanyFacts(entityID, data)
}

func parseWikidataCompanyFacts(entityID string, data []byte) (*WikidataCompanyFacts, error) {
	var response wikidataEntityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	entity, ok := response.Entities[entityID]
	if !ok {
		return nil, fmt.Errorf("wikidata entity %s not found", entityID)
	}

	facts := &WikidataCompanyFacts{
		EntityID:                entityID,
		EntityURL:               "https://www.wikidata.org/wiki/" + url.PathEscape(entityID),
		FoundedYearClaimCount:   len(entity.Claims["P571"]),
		EmployeeCountClaimCount: len(entity.Claims["P1128"]),
	}
	facts.FoundedYear = bestWikidataTimeYear(entity.Claims["P571"])
	facts.EmployeeCount = bestWikidataEmployeeCount(entity.Claims["P1128"])
	return facts, nil
}

func bestWikidataTimeYear(claims []wikidataClaim) *int {
	var best *int
	bestRank := -1
	for _, claim := range claims {
		if claim.Rank == "deprecated" {
			continue
		}
		year := wikidataTimeYear(claim.Mainsnak)
		if year == nil {
			continue
		}
		rank := wikidataRankScore(claim.Rank)
		if best == nil || rank > bestRank {
			value := *year
			best = &value
			bestRank = rank
		}
	}
	return best
}

func bestWikidataEmployeeCount(claims []wikidataClaim) *int {
	type candidate struct {
		count int
		rank  int
		year  int
	}

	var best *candidate
	for _, claim := range claims {
		if claim.Rank == "deprecated" {
			continue
		}
		count := wikidataQuantityInt(claim.Mainsnak)
		if count == nil || *count <= 0 {
			continue
		}
		candidate := candidate{
			count: *count,
			rank:  wikidataRankScore(claim.Rank),
			year:  wikidataQualifierYear(claim, "P585"),
		}
		if best == nil ||
			candidate.rank > best.rank ||
			(candidate.rank == best.rank && candidate.year > best.year) {
			best = &candidate
		}
	}
	if best == nil {
		return nil
	}
	return &best.count
}

func wikidataQualifierYear(claim wikidataClaim, property string) int {
	for _, snak := range claim.Qualifiers[property] {
		if year := wikidataTimeYear(snak); year != nil {
			return *year
		}
	}
	return 0
}

func wikidataTimeYear(snak wikidataSnak) *int {
	if snak.Datavalue == nil {
		return nil
	}
	var value struct {
		Time string `json:"time"`
	}
	if err := json.Unmarshal(snak.Datavalue.Value, &value); err != nil {
		return nil
	}
	text := strings.TrimPrefix(strings.TrimSpace(value.Time), "+")
	if len(text) < 4 {
		return nil
	}
	year, err := strconv.Atoi(text[:4])
	if err != nil {
		return nil
	}
	return &year
}

func wikidataQuantityInt(snak wikidataSnak) *int {
	if snak.Datavalue == nil {
		return nil
	}
	var value struct {
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal(snak.Datavalue.Value, &value); err != nil {
		return nil
	}
	amount := strings.TrimPrefix(strings.TrimSpace(value.Amount), "+")
	number, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return nil
	}
	count := int(number + 0.5)
	return &count
}

func wikidataRankScore(rank string) int {
	switch rank {
	case "preferred":
		return 2
	case "normal":
		return 1
	default:
		return 0
	}
}
