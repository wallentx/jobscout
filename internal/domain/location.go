package domain

import "strings"

var usStateAbbreviations = map[string]string{
	"ALABAMA": "AL", "ALASKA": "AK", "ARIZONA": "AZ", "ARKANSAS": "AR", "CALIFORNIA": "CA",
	"COLORADO": "CO", "CONNECTICUT": "CT", "DELAWARE": "DE", "DISTRICT OF COLUMBIA": "DC",
	"FLORIDA": "FL", "GEORGIA": "GA", "HAWAII": "HI", "IDAHO": "ID", "ILLINOIS": "IL",
	"INDIANA": "IN", "IOWA": "IA", "KANSAS": "KS", "KENTUCKY": "KY", "LOUISIANA": "LA",
	"MAINE": "ME", "MARYLAND": "MD", "MASSACHUSETTS": "MA", "MICHIGAN": "MI",
	"MINNESOTA": "MN", "MISSISSIPPI": "MS", "MISSOURI": "MO", "MONTANA": "MT",
	"NEBRASKA": "NE", "NEVADA": "NV", "NEW HAMPSHIRE": "NH", "NEW JERSEY": "NJ",
	"NEW MEXICO": "NM", "NEW YORK": "NY", "NORTH CAROLINA": "NC", "NORTH DAKOTA": "ND",
	"OHIO": "OH", "OKLAHOMA": "OK", "OREGON": "OR", "PENNSYLVANIA": "PA",
	"RHODE ISLAND": "RI", "SOUTH CAROLINA": "SC", "SOUTH DAKOTA": "SD", "TENNESSEE": "TN",
	"TEXAS": "TX", "UTAH": "UT", "VERMONT": "VT", "VIRGINIA": "VA", "WASHINGTON": "WA",
	"WEST VIRGINIA": "WV", "WISCONSIN": "WI", "WYOMING": "WY",
}

func NormalizeCountryCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "UNITED STATES", "UNITED STATES OF AMERICA", "USA", "U.S.", "U.S.A.":
		return "US"
	case "AUSTRALIA", "AUS":
		return "AU"
	case "SINGAPORE", "SGP":
		return "SG"
	default:
		return value
	}
}

func NormalizeStateRegion(value string, countryCode string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	upper := strings.ToUpper(value)
	countryCode = NormalizeCountryCode(countryCode)
	if countryCode == "" || countryCode == "US" {
		if abbreviation, ok := usStateAbbreviations[upper]; ok {
			return abbreviation
		}
		if len(upper) == 2 {
			return upper
		}
	}
	return strings.ToUpper(value)
}

func NormalizeCriteriaLocation(cfg *CriteriaConfig) {
	if cfg == nil {
		return
	}
	cfg.Candidate.CountryCode = NormalizeCountryCode(cfg.Candidate.CountryCode)
	cfg.Candidate.State = NormalizeStateRegion(cfg.Candidate.State, cfg.Candidate.CountryCode)
}

// ParseCSVList parses comma- or newline-separated user input.
func ParseCSVList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}

	return out
}

func ParseWorkSettings(value string) WorkSettingsConfig {
	var settings WorkSettingsConfig
	for _, item := range ParseCSVList(value) {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "remote":
			settings.Remote = true
		case "hybrid":
			settings.Hybrid = true
		case "onsite", "on-site", "office":
			settings.Onsite = true
		}
	}
	return settings
}

func SelectedWorkSettings(settings WorkSettingsConfig) []string {
	values := make([]string, 0, 3)
	if settings.Remote {
		values = append(values, "remote")
	}
	if settings.Hybrid {
		values = append(values, "hybrid")
	}
	if settings.Onsite {
		values = append(values, "onsite")
	}
	return values
}
