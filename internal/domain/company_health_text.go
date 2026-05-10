package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// parseEmployeeCount extracts employee count from text
func parseEmployeeCount(text string) *int {
	// Look for patterns like "123,456 employees", "workforce of 5000", etc.
	// Normalize text
	lower := strings.ToLower(text)

	// Pattern 1: "X employees" or "X people"
	// Match numbers with commas, e.g., "12,000" or "100"
	re := regexp.MustCompile(`(\d{1,3}(?:,\d{3})*)\s+(?:employees|staff|people|workers)`)
	matches := re.FindStringSubmatch(lower)
	if len(matches) > 1 {
		numStr := strings.ReplaceAll(matches[1], ",", "")
		var count int
		if _, err := fmt.Sscanf(numStr, "%d", &count); err == nil {
			return new(count)
		}
	}

	// Pattern 2: "employing X", "employs X"
	re2 := regexp.MustCompile(`employ(?:s|ing)\s+(\d{1,3}(?:,\d{3})*)`)
	matches2 := re2.FindStringSubmatch(lower)
	if len(matches2) > 1 {
		numStr := strings.ReplaceAll(matches2[1], ",", "")
		var count int
		if _, err := fmt.Sscanf(numStr, "%d", &count); err == nil {
			return new(count)
		}
	}

	return nil
}

func ParseEmployeeCount(text string) *int {
	return parseEmployeeCount(text)
}

// cleanCompanyName removes common suffixes for better matching
func cleanCompanyName(name string) string {
	n := strings.ToLower(name)
	n = strings.ReplaceAll(n, ",", "")

	suffixes := []string{
		" inc", " inc.", " corp", " corp.", " corporation", " co", " co.",
		" ltd", " ltd.", " limited", " plc", " ag", " sa", " se", " holdings", " group",
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(n, suffix) {
			n = strings.TrimSuffix(n, suffix)
			n = strings.TrimSpace(n)
		}
	}

	return n
}

// parseYearFromText extracts founding year from text
func parseYearFromText(text string) *int {
	lower := strings.ToLower(text)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\b(?:founded|established|incorporated)\s+(?:in|on|around|circa|c\.)?\s*\b(19\d{2}|20\d{2})\b`),
		regexp.MustCompile(`\b(?:founded|established|incorporated)\b.{0,80}?\b(?:in|on|around|circa|c\.)\s+\b(19\d{2}|20\d{2})\b`),
		regexp.MustCompile(`\bsince\s+\b(19\d{2}|20\d{2})\b`),
	}
	for _, re := range patterns {
		matches := re.FindStringSubmatch(lower)
		if len(matches) > 1 {
			var year int
			if _, err := fmt.Sscanf(matches[1], "%d", &year); err == nil && plausibleFoundedYear(year) {
				return new(year)
			}
		}
	}

	return nil
}

func plausibleFoundedYear(year int) bool {
	return year >= 1800 && year <= time.Now().Year()
}

func ParseYearFromText(text string) *int {
	return parseYearFromText(text)
}

// wordHitCount counts how many keywords appear in text
func wordHitCount(text string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		// Use word boundaries for keywords to avoid "loss" matching "blossom"
		// or "cut" matching "cute".
		re := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(kw)))
		if re.MatchString(text) {
			count++
		}
	}
	return count
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
