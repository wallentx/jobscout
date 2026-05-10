package fetcher

import (
	"fmt"
	"strings"
)

func buildSiteSearchQuery(site string, criteria *CriteriaConfig) string {
	site = strings.TrimSpace(site)
	if site == "" {
		return ""
	}

	var queryParts []string
	queryParts = append(queryParts, fmt.Sprintf("site:%s", site))

	if criteria != nil && len(criteria.Filters.TitleIncludes) > 0 {
		queryParts = append(queryParts, "("+strings.Join(criteria.Filters.TitleIncludes, " OR ")+")")
	} else {
		queryParts = append(queryParts, "jobs")
	}

	if criteria != nil && criteria.Filters.WorkSettings.Remote {
		queryParts = append(queryParts, "Remote")
	}

	return strings.Join(queryParts, " ")
}
