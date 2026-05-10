package fetcher

import "strings"

func cleanCompanyName(name string) string {
	n := strings.ToLower(name)
	n = strings.ReplaceAll(n, ",", "")
	for _, suffix := range []string{
		" inc", " inc.", " corp", " corp.", " corporation", " co", " co.",
		" ltd", " ltd.", " limited", " plc", " ag", " sa", " se", " holdings", " group",
	} {
		if strings.HasSuffix(n, suffix) {
			n = strings.TrimSpace(strings.TrimSuffix(n, suffix))
		}
	}
	return n
}

func CleanCompanyName(name string) string {
	return cleanCompanyName(name)
}
