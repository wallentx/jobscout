package fetcher

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"
)

type ReviewSession struct {
	jobs      []Job
	summary   FetchSummary
	addedJobs []Job
	fetched   int
	added     int
	message   string
}

type mergePlan struct {
	merged        []Job
	addedJobs     []Job
	duplicateJobs []Job
}

func NewReviewSession(existing []Job, fetchedJobs []Job, summary FetchSummary, debug bool) *ReviewSession {
	plan := planJobMerge(existing, fetchedJobs)
	reviewSummary := cloneFetchSummary(summary)
	if len(plan.duplicateJobs) > 0 {
		reviewSummary.Filtered["duplicate"] = append(reviewSummary.Filtered["duplicate"], plan.duplicateJobs...)
	}

	session := &ReviewSession{
		jobs:      slices.Clone(fetchedJobs),
		summary:   reviewSummary,
		addedJobs: slices.Clone(plan.addedJobs),
		fetched:   len(fetchedJobs),
		added:     len(plan.addedJobs),
	}
	session.message = session.Summary(debug)
	return session
}

func (r *ReviewSession) Summary(debug bool) string {
	if r == nil {
		return ""
	}
	return formatFetchReviewSummary(r, r.addedJobs, debug)
}

func (r *ReviewSession) Message() string {
	if r == nil {
		return ""
	}
	return r.message
}

func (r *ReviewSession) FetchedCount() int {
	if r == nil {
		return 0
	}
	return r.fetched
}

func (r *ReviewSession) AddedCount() int {
	if r == nil {
		return 0
	}
	return r.added
}

func (r *ReviewSession) HasNewJobs() bool {
	return r != nil && r.added > 0
}

func (r *ReviewSession) AddedJobs() []Job {
	if r == nil {
		return nil
	}
	return slices.Clone(r.addedJobs)
}

func (r *ReviewSession) MergeInto(existing []Job) ([]Job, []Job) {
	if r == nil {
		return slices.Clone(existing), nil
	}
	plan := planJobMerge(existing, r.jobs)
	return plan.merged, plan.addedJobs
}

func planJobMerge(existing []Job, newJobs []Job) mergePlan {
	existingMap := make(map[string]bool, len(existing))
	for _, job := range existing {
		existingMap[domain.JobMergeKey(job)] = true
	}

	plan := mergePlan{
		merged: slices.Clone(existing),
	}
	for _, job := range newJobs {
		key := domain.JobMergeKey(job)
		if existingMap[key] {
			plan.duplicateJobs = append(plan.duplicateJobs, job)
			continue
		}
		if job.Status == "" || job.Status == "New" {
			job.Status = "Unopened"
		}
		plan.merged = append(plan.merged, job)
		plan.addedJobs = append(plan.addedJobs, job)
		existingMap[key] = true
	}

	return plan
}

func cloneFetchSummary(summary FetchSummary) FetchSummary {
	cloned := FetchSummary{
		Notices:         slices.Clone(summary.Notices),
		Rejected:        make(map[string]map[string][]string, len(summary.Rejected)),
		Filtered:        make(map[string][]Job, len(summary.Filtered)),
		LLMFilterBypass: make(map[string][]Job, len(summary.LLMFilterBypass)),
		Searches:        make(map[string]string, len(summary.Searches)),
	}
	for searchType, grouped := range summary.Rejected {
		reasons := make(map[string][]string, len(grouped))
		for reason, entries := range grouped {
			reasons[reason] = slices.Clone(entries)
		}
		cloned.Rejected[searchType] = reasons
	}
	for reason, entries := range summary.Filtered {
		cloned.Filtered[reason] = slices.Clone(entries)
	}
	for reason, entries := range summary.LLMFilterBypass {
		cloned.LLMFilterBypass[reason] = slices.Clone(entries)
	}
	for searchType, status := range summary.Searches {
		cloned.Searches[searchType] = status
	}
	return cloned
}

func formatFetchReviewSummary(state *ReviewSession, addedJobs []Job, debug bool) string {
	lines := []string{
		fmt.Sprintf("Fetched %d results. Added %d new jobs.", state.fetched, state.added),
	}
	if state.added > 0 {
		lines = append(lines, "", "Press Enter to keep these results, or Esc to discard them.")
	}

	addedBySearch := groupJobsBySearch(addedJobs)
	if len(addedBySearch) > 0 || debug {
		lines = append(lines, "", "Added")
		searchTypes := mapKeys(addedBySearch)
		if debug {
			searchTypes = FetchSearchKinds()
		}
		for _, searchType := range orderedSearchTypes(searchTypes) {
			entries := addedBySearch[searchType]
			if len(entries) == 0 {
				if debug {
					lines = append(lines, "  "+searchType)
					lines = append(lines, searchStatusPlaceholder(state.summary.Searches[searchType], "no jobs added")...)
				}
				continue
			}

			lines = append(lines, fmt.Sprintf("  %s (%d)", searchType, len(entries)))
			for _, entry := range entries {
				lines = append(lines, "    + "+entry)
			}
		}
	}

	if debug {
		lines = append(lines, "", "Rejected")
		for _, searchType := range orderedSearchTypes(FetchSearchKinds()) {
			reasons := state.summary.Rejected[searchType]
			if len(reasons) == 0 {
				lines = append(lines, "  "+searchType)
				lines = append(lines, searchStatusPlaceholder(state.summary.Searches[searchType], "no rejected jobs")...)
				continue
			}

			lines = append(lines, "  "+searchType)
			for _, reason := range sortedMapKeys(reasons) {
				entries := reasons[reason]
				lines = append(lines, fmt.Sprintf("    %s (%d)", reason, len(entries)))
				for _, entry := range entries {
					lines = append(lines, "      - "+entry)
				}
			}
		}
	}

	if debug {
		lines = append(lines, "", "Filtered")
		if len(state.summary.Filtered) == 0 {
			lines = append(lines, "  none")
		}
		for _, reason := range sortedMapKeys(state.summary.Filtered) {
			entries := state.summary.Filtered[reason]
			lines = append(lines, fmt.Sprintf("  %s (%d)", reason, len(entries)))
			grouped := groupJobsBySearchAndSource(entries)
			for _, searchType := range orderedSearchTypes(mapKeys(grouped)) {
				sources := grouped[searchType]
				lines = append(lines, "    "+searchType)
				for _, sourceName := range sortedMapKeys(sources) {
					items := sources[sourceName]
					lines = append(lines, fmt.Sprintf("      %s (%d)", sourceName, len(items)))
					for _, entry := range items {
						lines = append(lines, "        - "+entry)
					}
				}
			}
		}
	}

	if debug && len(state.summary.LLMFilterBypass) > 0 {
		lines = append(lines, "", "LLM Filter Bypass")
		for _, reason := range sortedMapKeys(state.summary.LLMFilterBypass) {
			entries := state.summary.LLMFilterBypass[reason]
			lines = append(lines, fmt.Sprintf("  %s (%d)", reason, len(entries)))
			grouped := groupJobsBySearchAndSource(entries)
			for _, searchType := range orderedSearchTypes(mapKeys(grouped)) {
				sources := grouped[searchType]
				lines = append(lines, "    "+searchType)
				for _, sourceName := range sortedMapKeys(sources) {
					items := sources[sourceName]
					lines = append(lines, fmt.Sprintf("      %s (%d)", sourceName, len(items)))
					for _, entry := range items {
						lines = append(lines, "        - "+entry)
					}
				}
			}
		}
	}

	if debug {
		lines = append(lines, "", "Searches")
		for _, searchType := range orderedSearchTypes(mapKeys(state.summary.Searches)) {
			status := strings.TrimSpace(state.summary.Searches[searchType])
			if status == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %s: %s", searchType, status))
		}
	}

	if debug && len(state.summary.Notices) > 0 {
		lines = append(lines, "", "Notes")
		for _, notice := range state.summary.Notices {
			lines = append(lines, "  "+notice)
		}
	}

	return strings.Join(lines, "\n")
}

func searchTypeForSource(source string) string {
	sourceLower := strings.ToLower(strings.TrimSpace(source))
	switch {
	case strings.HasPrefix(sourceLower, strings.ToLower(FetchSearchLLMWeb)+":"):
		return FetchSearchLLMWeb
	case strings.HasPrefix(sourceLower, "llm"):
		return FetchSearchLLM
	case strings.HasPrefix(sourceLower, strings.ToLower(FetchSearchRSS)+":"):
		return FetchSearchRSS
	case strings.HasPrefix(sourceLower, strings.ToLower(FetchSearchAPI)+":"), strings.HasPrefix(sourceLower, "api:"):
		return FetchSearchAPI
	case strings.HasPrefix(sourceLower, strings.ToLower(FetchSearchSite)+":"), strings.HasPrefix(sourceLower, "site:"):
		return FetchSearchSite
	default:
		return "Other"
	}
}

func searchStatusPlaceholder(status string, fallback string) []string {
	status = strings.TrimSpace(status)
	switch {
	case status == "":
		return []string{"    " + fallback}
	case searchStatusSkipped(status):
		return []string{"    skipped: " + status}
	case searchStatusFailed(status):
		return []string{"    failed: " + status}
	case strings.Contains(status, "found 0 results"):
		return []string{"    no results"}
	default:
		return []string{"    " + fallback}
	}
}

func searchStatusSkipped(status string) bool {
	switch {
	case strings.HasPrefix(status, "disabled"):
		return true
	case strings.HasPrefix(status, "LLM job search disabled"):
		return true
	case strings.HasPrefix(status, "config unavailable"):
		return true
	case strings.HasPrefix(status, "enabled, but no "):
		return true
	case strings.HasPrefix(status, "enabled, but all "):
		return true
	default:
		return false
	}
}

func searchStatusFailed(status string) bool {
	switch {
	case strings.HasPrefix(status, "failed"):
		return true
	case strings.Contains(status, "execution failed"):
		return true
	default:
		return false
	}
}

func groupJobsBySearch(jobs []Job) map[string][]string {
	grouped := make(map[string][]string)
	for _, job := range jobs {
		searchType := searchTypeForSource(job.Source)
		grouped[searchType] = append(grouped[searchType], fmt.Sprintf("%s - %s", job.Company, job.Title))
	}
	return grouped
}

func groupJobsBySearchAndSource(jobs []Job) map[string]map[string][]string {
	grouped := make(map[string]map[string][]string)
	for _, job := range jobs {
		searchType := searchTypeForSource(job.Source)
		sourceName := sourceLabelForSource(job.Source)
		if grouped[searchType] == nil {
			grouped[searchType] = make(map[string][]string)
		}
		grouped[searchType][sourceName] = append(grouped[searchType][sourceName], fmt.Sprintf("%s - %s", job.Company, job.Title))
	}
	return grouped
}

func sourceLabelForSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "unknown"
	}

	parts := strings.SplitN(source, ":", 2)
	if len(parts) == 2 {
		label := strings.TrimSpace(parts[1])
		if label != "" {
			if strings.HasSuffix(strings.ToUpper(label), " RSS") {
				label = strings.TrimSpace(label[:len(label)-4])
			}
			return label
		}
	}

	return source
}

func orderedSearchTypes(values []string) []string {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		seen[value] = true
	}

	ordered := make([]string, 0, len(values))
	for _, searchType := range FetchSearchKinds() {
		if seen[searchType] {
			ordered = append(ordered, searchType)
			delete(seen, searchType)
		}
	}

	extras := slices.Sorted(maps.Keys(seen))
	return append(ordered, extras...)
}

func sortedMapKeys[V any](items map[string]V) []string {
	return slices.Sorted(maps.Keys(items))
}

func mapKeys[V any](items map[string]V) []string {
	return sortedMapKeys(items)
}
