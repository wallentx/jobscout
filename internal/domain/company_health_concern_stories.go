package domain

import (
	"strings"
	"time"
)

const companyHealthConcernStoryWindow = 365 * 24 * time.Hour

func companyHealthConcernReason(title string, keywords []string) string {
	for _, keyword := range keywords {
		if wordHitCount(title, []string{keyword}) > 0 {
			return "negative news keyword: " + keyword
		}
	}
	return ""
}

func companyHealthConcernStoryRecent(date *time.Time) bool {
	if date == nil {
		return false
	}
	now := time.Now()
	if date.After(now.Add(24 * time.Hour)) {
		return false
	}
	return now.Sub(*date) <= companyHealthConcernStoryWindow
}

func companyHealthConcernStoryFromRSSItem(source string, item RSSItem, keywords []string) (CompanyHealthConcernStory, bool) {
	concern := companyHealthConcernReason(item.Title, keywords)
	if concern == "" {
		return CompanyHealthConcernStory{}, false
	}
	date := parseRSSItemPubDate(item.PubDate)
	if !companyHealthConcernStoryRecent(date) {
		return CompanyHealthConcernStory{}, false
	}
	return CompanyHealthConcernStory{
		Source:  source,
		Title:   strings.TrimSpace(item.Title),
		URL:     strings.TrimSpace(item.Link),
		Date:    date,
		Concern: concern,
	}, true
}

func concernStoriesFromHNSignals(signals []HNSignal) []CompanyHealthConcernStory {
	stories := make([]CompanyHealthConcernStory, 0)
	for _, signal := range signals {
		concern := companyHealthConcernReason(signal.Title, hnNegKeywords)
		date := signal.Date
		if concern == "" || !companyHealthConcernStoryRecent(&date) {
			continue
		}
		stories = append(stories, CompanyHealthConcernStory{
			Source:  "hacker_news",
			Title:   strings.TrimSpace(signal.Title),
			URL:     strings.TrimSpace(signal.URL),
			Date:    &date,
			Concern: concern,
		})
	}
	return stories
}

func concernStoriesFromLayoffSignals(signals []LayoffSignal) []CompanyHealthConcernStory {
	stories := make([]CompanyHealthConcernStory, 0, len(signals))
	for _, signal := range signals {
		if !companyHealthConcernStoryRecent(signal.Date) {
			continue
		}
		stories = append(stories, CompanyHealthConcernStory{
			Source:  "layoff_news",
			Title:   strings.TrimSpace(signal.Title),
			URL:     strings.TrimSpace(signal.URL),
			Date:    signal.Date,
			Concern: "layoff signal",
		})
	}
	return stories
}

func appendUniqueCompanyHealthConcernStories(existing []CompanyHealthConcernStory, stories ...CompanyHealthConcernStory) []CompanyHealthConcernStory {
	if len(stories) == 0 {
		return existing
	}
	seen := make(map[string]bool, len(existing)+len(stories))
	for _, story := range existing {
		seen[companyHealthConcernStoryKey(story)] = true
	}
	for _, story := range stories {
		key := companyHealthConcernStoryKey(story)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		existing = append(existing, story)
	}
	return existing
}

func companyHealthConcernStoryKey(story CompanyHealthConcernStory) string {
	title := strings.ToLower(strings.TrimSpace(story.Title))
	url := strings.ToLower(strings.TrimSpace(story.URL))
	if title == "" && url == "" {
		return ""
	}
	return title + "\x00" + url
}

func parseRSSItemPubDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed
		}
	}
	return nil
}
