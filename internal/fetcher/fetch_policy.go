package fetcher

import (
	"math/rand"
	"strings"
	"sync"
	"time"
)

type fetchPolicy struct {
	CandidateLimitPerSource int
	AcceptedLimit           int
}

func fetchPolicyFromConfig(cfg *AppConfig) fetchPolicy {
	if cfg == nil {
		return fetchPolicy{}
	}
	return fetchPolicy{
		CandidateLimitPerSource: nonNegativeInt(cfg.Fetch.CandidateLimitPerSource),
		AcceptedLimit:           nonNegativeInt(cfg.Fetch.AcceptedLimit),
	}
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func randomizeResolvedSources(sources *ResolvedSourceSet) {
	if sources == nil {
		return
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(sources.RSSFeeds), func(i, j int) {
		sources.RSSFeeds[i], sources.RSSFeeds[j] = sources.RSSFeeds[j], sources.RSSFeeds[i]
	})
	r.Shuffle(len(sources.APISources), func(i, j int) {
		sources.APISources[i], sources.APISources[j] = sources.APISources[j], sources.APISources[i]
	})
	r.Shuffle(len(sources.SiteTargets), func(i, j int) {
		sources.SiteTargets[i], sources.SiteTargets[j] = sources.SiteTargets[j], sources.SiteTargets[i]
	})
	r.Shuffle(len(sources.LLMWebTargets), func(i, j int) {
		sources.LLMWebTargets[i], sources.LLMWebTargets[j] = sources.LLMWebTargets[j], sources.LLMWebTargets[i]
	})
}

type siteCandidateLimiter struct {
	limit int
	mu    sync.Mutex
	used  map[string]int
}

func newSiteCandidateLimiter(limit int) *siteCandidateLimiter {
	if limit <= 0 {
		return nil
	}
	return &siteCandidateLimiter{
		limit: limit,
		used:  make(map[string]int),
	}
}

func (l *siteCandidateLimiter) exhausted(source string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.used[siteCandidateLimitKey(source)] >= l.limit
}

func (l *siteCandidateLimiter) take(source string, candidates []siteSearchCandidate) ([]siteSearchCandidate, int) {
	if l == nil || len(candidates) == 0 {
		return candidates, 0
	}
	kept := l.takeCount(source, len(candidates))
	if kept >= len(candidates) {
		return candidates, 0
	}
	return candidates[:kept], len(candidates) - kept
}

func (l *siteCandidateLimiter) takeCount(source string, count int) int {
	if l == nil || count <= 0 {
		return count
	}
	key := siteCandidateLimitKey(source)
	l.mu.Lock()
	defer l.mu.Unlock()

	remaining := l.limit - l.used[key]
	if remaining <= 0 {
		return 0
	}
	if count <= remaining {
		l.used[key] += count
		return count
	}
	l.used[key] += remaining
	return remaining
}

func siteCandidateLimitKey(source string) string {
	return strings.ToLower(strings.TrimSpace(source))
}

func capAcceptedFetchedJobs(jobs []Job, limit int, summary *FetchSummary) []Job {
	if limit <= 0 || len(jobs) <= limit {
		return jobs
	}
	kept := jobs[:limit]
	omitted := append([]Job(nil), jobs[limit:]...)
	if summary != nil {
		if summary.Filtered == nil {
			summary.Filtered = make(map[string][]Job)
		}
		summary.Filtered["accepted limit reached"] = append(summary.Filtered["accepted limit reached"], omitted...)
		appendFetchNotice(summary, "Accepted result limit reached; additional matching jobs were omitted from this fetch.")
	}
	logDebug("fetch policy: accepted result limit kept=%d omitted=%d", len(kept), len(omitted))
	return kept
}
