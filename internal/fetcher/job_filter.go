package fetcher

import (
	"fmt"
	"regexp"
	"strings"
)

var nonRemoteWorkSignalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bnot\s+remote\b`),
	regexp.MustCompile(`\bnon[-\s]?remote\b`),
	regexp.MustCompile(`\bno\s+remote\b`),
	regexp.MustCompile(`\bremote\s*[:=]\s*(false|no|0)\b`),
}

func workSettingsConfigured(settings WorkSettingsConfig) bool {
	return settings.Remote || settings.Hybrid || settings.Onsite
}

func jobMatchesWorkSettings(job *Job, settings WorkSettingsConfig) bool {
	if !workSettingsConfigured(settings) {
		return true
	}

	remoteSignals := detectWorkSettingSignals(job.Remote)
	if remoteSignals.hasAny() {
		return remoteSignals.matches(settings)
	}

	descSignals := detectWorkSettingSignals(job.Description)
	if descSignals.hasAny() {
		return descSignals.matches(settings)
	}

	return true
}

type workSettingSignals struct {
	remote bool
	hybrid bool
	onsite bool
}

func detectWorkSettingSignals(text string) workSettingSignals {
	text = strings.ToLower(strings.TrimSpace(text))
	nonRemote := containsNonRemoteWorkSignal(text)
	return workSettingSignals{
		remote: !nonRemote && strings.Contains(text, "remote"),
		hybrid: containsHybridWorkSignal(text),
		onsite: nonRemote ||
			strings.Contains(text, "on-site") ||
			strings.Contains(text, "onsite") ||
			strings.Contains(text, "in-office") ||
			strings.Contains(text, "office based") ||
			strings.Contains(text, "in office"),
	}
}

func (s workSettingSignals) hasAny() bool {
	return s.remote || s.hybrid || s.onsite
}

func (s workSettingSignals) matches(settings WorkSettingsConfig) bool {
	return (s.remote && settings.Remote) ||
		(s.hybrid && settings.Hybrid) ||
		(s.onsite && settings.Onsite)
}

func containsHybridWorkSignal(text string) bool {
	if !strings.Contains(text, "hybrid") {
		return false
	}
	incidental := []string{
		"hybrid cloud",
		"hybrid-cloud",
		"hybrid infrastructure",
		"hybrid-infrastructure",
		"hybrid environment",
		"hybrid-environment",
	}
	for _, phrase := range incidental {
		text = strings.ReplaceAll(text, phrase, "")
	}
	return strings.Contains(text, "hybrid")
}

func containsNonRemoteWorkSignal(text string) bool {
	if !strings.Contains(text, "remote") {
		return false
	}
	for _, pattern := range nonRemoteWorkSignalPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// extractSalary parses a messy string and looks for numbers resembling a tech salary (> $50k)
// It returns true if it finds a number >= minBase
func extractAndCheckSalary(text string, minBase int) bool {
	if minBase <= 0 {
		return true // No minimum set
	}

	// Remove commas for easier parsing: "$180,000" -> "$180000"
	cleanText := strings.ReplaceAll(text, ",", "")

	// Look for explicitly high numbers (e.g., 180000)
	reFull := regexp.MustCompile(`\b[1-9]\d{4,5}\b`)
	matchesFull := reFull.FindAllString(cleanText, -1)
	for _, m := range matchesFull {
		var val int
		_, _ = fmt.Sscanf(m, "%d", &val)
		if val >= minBase {
			return true
		}
	}

	// Look for "K" notation (e.g., 180k, 180K)
	reK := regexp.MustCompile(`\b([1-9]\d{1,2})[kK]\b`)
	matchesK := reK.FindAllStringSubmatch(cleanText, -1)
	for _, m := range matchesK {
		var val int
		_, _ = fmt.Sscanf(m[1], "%d", &val)
		if val*1000 >= minBase {
			return true
		}
	}

	// We couldn't definitively prove it meets the criteria.
	// We return false, meaning we aggressively filter out jobs missing compensation details.
	return false
}

func filterJobReason(job *Job, criteria *CriteriaConfig) string {
	if criteria == nil {
		return ""
	}

	titleLower := strings.ToLower(job.Title)
	descLower := strings.ToLower(job.Description)

	// 1. Title Excludes (Fail fast)
	if len(criteria.Filters.TitleExcludes) > 0 {
		for _, exclude := range criteria.Filters.TitleExcludes {
			if strings.Contains(titleLower, strings.ToLower(exclude)) {
				return "title excludes"
			}
		}
	}

	// 2. Title Requires (Strict scope enforcement - MUST match as whole word)
	if len(criteria.Filters.TitleRequires) > 0 {
		match := false
		for _, req := range criteria.Filters.TitleRequires {
			pattern := `(?i)(^|[^a-zA-Z])` + regexp.QuoteMeta(req) + `([^a-zA-Z]|$)`
			re, err := regexp.Compile(pattern)
			if err == nil && re.MatchString(job.Title) {
				match = true
				break
			}
		}
		if !match {
			return "title requirements"
		}
	}

	// 3. Title Includes (General role targeting - MUST match as whole word/boundary)
	if len(criteria.Filters.TitleIncludes) > 0 {
		match := false
		for _, include := range criteria.Filters.TitleIncludes {
			pattern := `(?i)(^|[^a-zA-Z])` + regexp.QuoteMeta(include) + `([^a-zA-Z]|$)`
			re, err := regexp.Compile(pattern)
			if err == nil && re.MatchString(job.Title) {
				match = true
				break
			}
		}
		if !match {
			return "title includes"
		}
	}

	// 4. Industry Excludes (Scan description)
	if len(criteria.Filters.IndustryExcludes) > 0 {
		for _, exclude := range criteria.Filters.IndustryExcludes {
			if strings.Contains(descLower, strings.ToLower(exclude)) {
				return "industry excludes"
			}
		}
	}

	// 5. Compensation Check (if compensation string exists and a minimum is set)
	if criteria.Filters.MinBaseUSD > 0 && job.Compensation != "" && job.Compensation != "Not listed" {
		if !extractAndCheckSalary(job.Compensation, criteria.Filters.MinBaseUSD) {
			return "pay"
		}
	}

	// 6. Work Settings / Remote check (Basic heuristic)
	if !jobMatchesWorkSettings(job, criteria.Filters.WorkSettings) {
		return "work setting"
	}

	return ""
}
