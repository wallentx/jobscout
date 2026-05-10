package fetcher

import (
	"context"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

func normalizeRSSJobIdentity(sourceName string, item *gofeed.Item) (string, string) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return "Unknown", title
	}

	prefix := strings.TrimSpace(sourceName)
	if prefix != "" {
		for _, sep := range []string{" - ", ": "} {
			candidate := prefix + sep
			if strings.HasPrefix(strings.ToLower(title), strings.ToLower(candidate)) {
				title = strings.TrimSpace(title[len(candidate):])
				break
			}
		}
	}

	company := "Unknown"

	if parsedTitle, parsedCompany, ok := splitJobTitleCompanyAt(title); ok {
		company = parsedCompany
		title = parsedTitle
	} else if parsedCompany, parsedTitle, ok := splitWeWorkRemotelyCompanyTitleColon(sourceName, item, title); ok {
		company = parsedCompany
		title = parsedTitle
	} else if parsedCompany, parsedTitle, ok := splitJobCompanyTitleColon(title); ok {
		company = parsedCompany
		title = parsedTitle
	} else {
		if item.Author != nil && strings.TrimSpace(item.Author.Name) != "" {
			company = strings.TrimSpace(item.Author.Name)
		} else if len(item.Authors) > 0 && item.Authors[0] != nil && strings.TrimSpace(item.Authors[0].Name) != "" {
			company = strings.TrimSpace(item.Authors[0].Name)
		} else {
			if exts, ok := item.Extensions["dc"]; ok {
				if creators, ok := exts["creator"]; ok && len(creators) > 0 {
					company = strings.TrimSpace(creators[0].Value)
				}
			}
		}
	}

	return company, title
}

func splitWeWorkRemotelyCompanyTitleColon(sourceName string, item *gofeed.Item, text string) (string, string, bool) {
	source := strings.ToLower(strings.TrimSpace(sourceName))
	link := ""
	if item != nil {
		link = strings.ToLower(strings.TrimSpace(item.Link))
	}
	if !strings.Contains(source, "we work remotely") && !strings.Contains(link, "weworkremotely.com") {
		return "", "", false
	}
	return splitCompanyTitleColon(text, false)
}

func splitJobTitleCompanyAt(text string) (string, string, bool) {
	lower := strings.ToLower(text)

	separators := []string{" at ", " with "}
	bestIdx := -1
	var bestSep string

	for _, sep := range separators {
		idx := strings.LastIndex(lower, sep)
		if idx > bestIdx {
			bestIdx = idx
			bestSep = sep
		}
	}

	if bestIdx <= 0 {
		return "", "", false
	}
	title := strings.TrimSpace(text[:bestIdx])
	company := strings.TrimSpace(text[bestIdx+len(bestSep):])
	if title == "" || company == "" || !looksLikeJobTitle(strings.ToLower(title)) {
		return "", "", false
	}
	return title, company, true
}

func splitJobCompanyTitleColon(text string) (string, string, bool) {
	return splitCompanyTitleColon(text, true)
}

func splitCompanyTitleColon(text string, requireJobTitle bool) (string, string, bool) {
	idx := strings.Index(text, ":")
	if idx <= 0 {
		return "", "", false
	}
	company := strings.TrimSpace(text[:idx])
	title := strings.TrimSpace(text[idx+1:])
	if company == "" || title == "" {
		return "", "", false
	}
	if requireJobTitle && !looksLikeJobTitle(strings.ToLower(title)) {
		return "", "", false
	}
	return company, title, true
}

func fetchRSS(ctx context.Context, source RSSSource, criteria *CriteriaConfig) ([]Job, map[string][]Job, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(source.URL, ctx)
	if err != nil {
		return nil, nil, err
	}
	logDebug("rss %s: fetched %d feed items from %s", source.Name, len(feed.Items), source.URL)

	var jobs []Job
	filtered := make(map[string][]Job)
	for _, item := range feed.Items {
		company, title := normalizeRSSJobIdentity(source.Name, item)
		job := Job{
			Company:      company,
			Title:        title,
			ApplyURL:     item.Link,
			Source:       formatSearchSource(fetchSearchRSS, source.Name),
			Status:       "Unopened",
			Remote:       "Unknown",
			Compensation: "Not listed",
			Description:  item.Description,
		}
		enrichJobFromDescription(&job)
		job.SetDateAdded(time.Now().Unix())

		if item.Description != "" {
			desc := strings.ToLower(item.Description)
			if strings.Contains(desc, "remote") || strings.Contains(desc, "anywhere") {
				job.Remote = "Remote"
			}
		}

		if reason := filterJobReason(&job, criteria); reason != "" {
			logDebug("rss %s: filtered %s - %s: %s", source.Name, job.Company, job.Title, reason)
			filtered[reason] = append(filtered[reason], job)
			continue
		}
		jobs = append(jobs, job)
	}

	logDebug("rss %s: accepted %d; filtered %d", source.Name, len(jobs), countFilteredJobs(filtered))
	return jobs, filtered, nil
}
