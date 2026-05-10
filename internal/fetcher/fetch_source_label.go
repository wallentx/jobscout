package fetcher

import (
	"strings"
)

func formatSearchSource(searchType string, sourceName string) string {
	searchType = strings.TrimSpace(searchType)
	sourceName = strings.TrimSpace(sourceName)
	if searchType == "" {
		return sourceName
	}
	if sourceName == "" {
		return searchType
	}
	return searchType + ": " + sourceName
}
