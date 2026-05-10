package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// rdapResponse represents minimal RDAP data
type rdapResponse struct {
	Events []struct {
		Action string `json:"eventAction"`
		Date   string `json:"eventDate"`
	} `json:"events"`
}

// fetchWhoisAge attempts to get domain creation year via RDAP
func fetchWhoisAge(domain string) *int {
	if domain == "" {
		return nil
	}

	// Clean domain
	u, err := url.Parse("https://" + domain)
	if err == nil {
		domain = u.Hostname()
	}
	domain = strings.TrimPrefix(domain, "www.")

	// Public free RDAP
	rdapURL := fmt.Sprintf("https://rdap.org/domain/%s", domain)
	data, err := httpGet(rdapURL)
	if err != nil {
		return nil
	}

	var res rdapResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return nil
	}

	for _, event := range res.Events {
		if event.Action == "registration" || event.Action == "last changed" { // registration is best
			// Format usually "2021-01-27T16:47:04Z"
			if len(event.Date) >= 4 {
				var year int
				if _, err := fmt.Sscanf(event.Date[:4], "%d", &year); err == nil {
					// Sanity check: Startups aren't founded in 1980 usually unless it's a parked domain
					// But we return what we find.
					return new(year)
				}
			}
		}
	}
	return nil
}

func FetchWhoisAge(domain string) *int {
	return fetchWhoisAge(domain)
}
