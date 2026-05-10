package fetcher

import (
	"fmt"
	"strings"
)

type FetchSummary struct {
	Notices         []string
	Rejected        map[string]map[string][]string
	Filtered        map[string][]Job
	LLMFilterBypass map[string][]Job
	Searches        map[string]string
}

const (
	fetchSearchLLM    = "LLM"
	fetchSearchLLMWeb = "LLM Web"
	fetchSearchRSS    = "RSS"
	fetchSearchAPI    = "Configured Source"
	fetchSearchSite   = "Site Search"
)

const (
	FetchSearchLLM    = fetchSearchLLM
	FetchSearchLLMWeb = fetchSearchLLMWeb
	FetchSearchRSS    = fetchSearchRSS
	FetchSearchAPI    = fetchSearchAPI
	FetchSearchSite   = fetchSearchSite
)

var fetchSearchKinds = []string{
	fetchSearchLLM,
	fetchSearchLLMWeb,
	fetchSearchRSS,
	fetchSearchAPI,
	fetchSearchSite,
}

func FetchSearchKinds() []string {
	return append([]string(nil), fetchSearchKinds...)
}

func newFetchSummary() FetchSummary {
	summary := FetchSummary{
		Rejected: make(map[string]map[string][]string),
		Filtered: make(map[string][]Job),
		Searches: make(map[string]string, len(fetchSearchKinds)),
	}
	for _, searchType := range fetchSearchKinds {
		summary.Searches[searchType] = "disabled in config"
	}
	return summary
}

func setFetchSearchStatus(summary *FetchSummary, searchType string, status string) {
	if summary.Searches == nil {
		summary.Searches = make(map[string]string, len(fetchSearchKinds))
	}
	summary.Searches[searchType] = status
	logDebug("%s search: %s", searchType, status)
}

func formatExecutedSearchStatus(accepted int, filtered int, rejected int) string {
	if filtered == 0 && rejected == 0 {
		return fmt.Sprintf("executed; found %d results", accepted)
	}
	return fmt.Sprintf("executed; accepted %d results; filtered %d; rejected %d", accepted, filtered, rejected)
}

func countFilteredJobs(filtered map[string][]Job) int {
	total := 0
	for _, jobs := range filtered {
		total += len(jobs)
	}
	return total
}

func countRejectedJobs(rejected map[string][]string) int {
	total := 0
	for _, entries := range rejected {
		total += len(entries)
	}
	return total
}

func countRejectedSearchSummary(rejected map[string]map[string][]string) int {
	total := 0
	for _, byReason := range rejected {
		total += countRejectedJobs(byReason)
	}
	return total
}

func sampleDebugHrefs(hrefs []string, limit int) string {
	if limit <= 0 || len(hrefs) == 0 {
		return ""
	}
	if len(hrefs) > limit {
		hrefs = hrefs[:limit]
	}
	return strings.Join(hrefs, ", ")
}

func debugRoleFamilies(roleFamilies []RoleFamilyID) string {
	values := make([]string, 0, len(roleFamilies))
	for _, role := range roleFamilies {
		values = append(values, string(role))
	}
	return debugStringList(values)
}

func debugRSSSources(sources []RSSSource) string {
	values := make([]string, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.Name) != "" {
			values = append(values, fmt.Sprintf("%s=%s", source.Name, source.URL))
		} else {
			values = append(values, source.URL)
		}
	}
	return debugStringList(values)
}

func debugAPISources(sources []APISource) string {
	values := make([]string, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.Name) != "" {
			values = append(values, fmt.Sprintf("%s=%s", source.Name, source.URL))
		} else {
			values = append(values, source.URL)
		}
	}
	return debugStringList(values)
}

func debugStringList(values []string) string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return "<none>"
	}
	return strings.Join(cleaned, ", ")
}

func reportFetchProgress(progress func(string), format string, args ...interface{}) {
	if progress == nil {
		return
	}
	progress(fmt.Sprintf(format, args...))
}
