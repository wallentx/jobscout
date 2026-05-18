package tuiapp

import (
	"maps"
	"sort"
	"strings"

	healthpkg "github.com/wallentx/jobscout/internal/health"
)

func calculateTableHeight(termHeight int) int {
	// Account for:
	// - table border: 2 lines
	// - table vertical margins: 2 lines
	// - spacing between table/footer: 2 lines
	// - footer: up to ~2 lines
	// - small safety buffer: 1 line
	height := termHeight - 10
	if height < 5 {
		return 5
	}
	return height
}

func cloneFilterMap(in map[string]bool) map[string]bool {
	if in == nil {
		return map[string]bool{}
	}
	return maps.Clone(in)
}

func filterValuesFromStatuses(selected []string) map[string]bool {
	values := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		values[status] = false
	}
	for _, status := range selected {
		if _, ok := values[status]; ok {
			values[status] = true
		}
	}
	return values
}

func selectedStatuses(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for _, status := range statuses {
		if values[status] {
			out = append(out, status)
		}
	}
	return out
}

func statusEmoji(status string) string {
	if emoji, ok := statusEmojis[status]; ok {
		return emoji
	}
	return ""
}

func enabledFilterEmojiSummary(values map[string]bool) string {
	selected := selectedStatuses(values)
	if len(selected) == 0 {
		return ""
	}

	emojis := make([]string, 0, len(selected))
	for _, status := range selected {
		if emoji := statusEmoji(status); emoji != "" {
			emojis = append(emojis, emoji)
		}
	}
	return strings.Join(emojis, " ")
}

func hasActiveFilters(values map[string]bool) bool {
	for _, enabled := range values {
		if enabled {
			return true
		}
	}
	return false
}

func (m *model) applyFilterAndSort() {
	m.filteredJobs = []Job{}
	searchLower := strings.TrimSpace(strings.ToLower(m.searchQuery))
	searching := searchLower != ""

	for _, job := range m.allJobs {
		statusMatch := searching || !hasActiveFilters(m.activeFilters) || m.activeFilters[job.Status]
		searchMatch := true
		if searching {
			searchMatch = strings.Contains(strings.ToLower(job.Company), searchLower) ||
				strings.Contains(strings.ToLower(job.Title), searchLower)
		}

		if statusMatch && searchMatch {
			m.filteredJobs = append(m.filteredJobs, job)
		}
	}

	sort.SliceStable(m.filteredJobs, func(i, j int) bool {
		cmp := 0
		switch m.sortBy {
		case 0:
			// Sort by Health Score
			scoreI := 0
			if cached := healthpkg.CachedHealthForJob(m.healthCache, m.filteredJobs[i]); cached != nil {
				scoreI = cached.Score
			}
			scoreJ := 0
			if cached := healthpkg.CachedHealthForJob(m.healthCache, m.filteredJobs[j]); cached != nil {
				scoreJ = cached.Score
			}
			switch {
			case scoreI < scoreJ:
				cmp = -1
			case scoreI > scoreJ:
				cmp = 1
			}
		case 1:
			cmp = strings.Compare(strings.ToLower(m.filteredJobs[i].Company), strings.ToLower(m.filteredJobs[j].Company))
		case 2:
			cmp = strings.Compare(strings.ToLower(m.filteredJobs[i].Title), strings.ToLower(m.filteredJobs[j].Title))
		case 3:
			cmp = strings.Compare(strings.ToLower(m.filteredJobs[i].Status), strings.ToLower(m.filteredJobs[j].Status))
		case 4:
			switch {
			case m.filteredJobs[i].DateAdded < m.filteredJobs[j].DateAdded:
				cmp = -1
			case m.filteredJobs[i].DateAdded > m.filteredJobs[j].DateAdded:
				cmp = 1
			}
		}

		if m.sortDesc {
			return cmp > 0
		}
		return cmp < 0
	})

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filteredJobs) && len(m.filteredJobs) > 0 {
		m.cursor = len(m.filteredJobs) - 1
	}
	if len(m.filteredJobs) == 0 {
		m.cursor = 0
	}
}
