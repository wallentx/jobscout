package fetcher

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	builtInSourceProfileInterval   = 3 * time.Second
	errBuiltInSourceProfileBlocked = errors.New("built in profile hydration disabled after HTTP block")
	sourceProfileRunMu             sync.Mutex
	sourceProfileRun               *sourceProfileEnricher
)

type sourceProfileEnricher struct {
	mu        sync.Mutex
	entries   map[string]*sourceProfileEntry
	limiters  map[string]*sourceProfileHostLimiter
	fetchHTML func(context.Context, string) (string, error)
}

type sourceProfileEntry struct {
	done   chan struct{}
	mu     sync.Mutex
	record CompanyIdentityRecord
	html   string
	ok     bool
}

type sourceProfileHostLimiter struct {
	mu        sync.Mutex
	isBuiltIn bool
	nextFetch time.Time
	blocked   bool
}

func newSourceProfileEnricher() *sourceProfileEnricher {
	return &sourceProfileEnricher{
		entries:   make(map[string]*sourceProfileEntry),
		limiters:  make(map[string]*sourceProfileHostLimiter),
		fetchHTML: fetchApplyPageHTML,
	}
}

func beginSourceProfileRun() *sourceProfileEnricher {
	enricher := newSourceProfileEnricher()
	sourceProfileRunMu.Lock()
	sourceProfileRun = enricher
	sourceProfileRunMu.Unlock()
	return enricher
}

func currentSourceProfileRun() *sourceProfileEnricher {
	sourceProfileRunMu.Lock()
	defer sourceProfileRunMu.Unlock()
	if sourceProfileRun == nil {
		sourceProfileRun = newSourceProfileEnricher()
	}
	return sourceProfileRun
}

func (e *sourceProfileEnricher) Enrich(ctx context.Context, job *Job, profileURL string, llmEnrich jobIdentityPageEnrichFunc, llmSource string) bool {
	return e.EnrichWithStats(ctx, job, profileURL, llmEnrich, llmSource, nil)
}

func (e *sourceProfileEnricher) EnrichWithStats(ctx context.Context, job *Job, profileURL string, llmEnrich jobIdentityPageEnrichFunc, llmSource string, stats *acceptedEnrichmentStats) bool {
	if job == nil {
		return false
	}
	if e == nil {
		e = newSourceProfileEnricher()
	}
	if stats == nil {
		stats = newAcceptedEnrichmentStats(0)
	}
	key := canonicalSourceProfileURL(profileURL)
	if key == "" {
		return false
	}
	stats.inc(&stats.sourceProfileAttempts)

	e.mu.Lock()
	if entry, ok := e.entries[key]; ok {
		e.mu.Unlock()
		select {
		case <-entry.done:
			if entry.ok {
				entry.applyCached(ctx, job, key, llmEnrich, llmSource, stats)
				stats.inc(&stats.sourceProfileCacheHits)
				logDebug("identity enrichment source profile: cache hit profile_url=%q company=%q title=%q", key, job.Company, job.Title)
			}
			return entry.ok
		case <-ctx.Done():
			return false
		}
	}
	entry := &sourceProfileEntry{done: make(chan struct{})}
	e.entries[key] = entry
	limiter := e.hostLimiterLocked(key)
	e.mu.Unlock()

	ok, record, html := e.fetchAndApply(ctx, job, key, llmEnrich, llmSource, limiter, stats)

	e.mu.Lock()
	entry.mu.Lock()
	entry.ok = ok
	entry.record = record
	entry.html = html
	entry.mu.Unlock()
	close(entry.done)
	e.mu.Unlock()
	return ok
}

func (entry *sourceProfileEntry) applyCached(ctx context.Context, job *Job, profileURL string, llmEnrich jobIdentityPageEnrichFunc, llmSource string, stats *acceptedEnrichmentStats) {
	entry.mu.Lock()
	record := entry.record
	html := entry.html
	entry.mu.Unlock()

	ApplyCachedIdentity(job, record)
	if strings.TrimSpace(html) == "" || !jobNeedsLLMIdentityEnrichment(*job) {
		return
	}
	stats.addLLMUsage(applyLLMJobIdentityEnrichment(ctx, job, buildJobIdentityPage(html, profileURL), llmEnrich, llmSource))
	if !recordHasIdentity(CompanyIdentityRecord{
		Website:  job.CompanyWebsite,
		Summary:  job.CompanySummary,
		Industry: job.CompanyIndustry,
		Identity: CloneJobIdentityMetadata(job.CompanyIdentity),
	}) {
		return
	}
	entry.mu.Lock()
	entry.record = CompanyIdentityRecord{
		Website:  job.CompanyWebsite,
		Summary:  job.CompanySummary,
		Industry: job.CompanyIndustry,
		Identity: CloneJobIdentityMetadata(job.CompanyIdentity),
	}
	entry.mu.Unlock()
}

func (e *sourceProfileEnricher) fetchAndApply(ctx context.Context, job *Job, profileURL string, llmEnrich jobIdentityPageEnrichFunc, llmSource string, limiter *sourceProfileHostLimiter, stats *acceptedEnrichmentStats) (bool, CompanyIdentityRecord, string) {
	if err := limiter.beforeFetch(ctx); err != nil {
		stats.inc(&stats.sourceProfileSkipped)
		logDebug("identity enrichment source profile: skipped profile_url=%q company=%q title=%q reason=%v", profileURL, job.Company, job.Title, err)
		return false, CompanyIdentityRecord{}, ""
	}

	logDebug("identity enrichment source profile: fetching profile_url=%q company=%q title=%q", profileURL, job.Company, job.Title)
	stats.inc(&stats.sourceProfileFetches)
	profileHTML, profileErr := e.fetchHTML(ctx, profileURL)
	limiter.afterFetch(profileErr)
	if profileErr != nil {
		stats.inc(&stats.sourceProfileFailed)
		if isBlockedFetchError(profileErr) {
			stats.inc(&stats.sourceProfileBlocked)
		}
		if limiter.isBuiltIn && isSourceProfileBlockError(profileErr) {
			logDebug("identity enrichment source profile: blocked profile_url=%q error=%v; disabling Built In profile hydration for this fetch run", profileURL, profileErr)
			return false, CompanyIdentityRecord{}, ""
		}
		logDebug("identity enrichment source profile: fetch failed profile_url=%q error=%v", profileURL, profileErr)
		return false, CompanyIdentityRecord{}, ""
	}
	if strings.TrimSpace(profileHTML) == "" {
		stats.inc(&stats.sourceProfileFailed)
		logDebug("identity enrichment source profile: empty profile profile_url=%q", profileURL)
		return false, CompanyIdentityRecord{}, ""
	}
	stats.inc(&stats.sourceProfileSuccess)
	logDebug("identity enrichment source profile: fetched profile_url=%q bytes=%d llm=%t", profileURL, len(profileHTML), llmEnrich != nil)
	enrichJobFromCompanyProfileHTML(job, profileHTML, profileURL)
	if jobNeedsLLMIdentityEnrichment(*job) {
		stats.addLLMUsage(applyLLMJobIdentityEnrichment(ctx, job, buildJobIdentityPage(profileHTML, profileURL), llmEnrich, llmSource))
	}
	record := CompanyIdentityRecord{
		Website:  job.CompanyWebsite,
		Summary:  job.CompanySummary,
		Industry: job.CompanyIndustry,
		Identity: CloneJobIdentityMetadata(job.CompanyIdentity),
	}
	return recordHasIdentity(record), record, profileHTML
}

func (e *sourceProfileEnricher) hostLimiterLocked(rawURL string) *sourceProfileHostLimiter {
	parsed, err := url.Parse(rawURL)
	host := ""
	if err == nil {
		host = strings.ToLower(parsed.Hostname())
	}
	if host == "" {
		host = "unknown"
	}
	if limiter, ok := e.limiters[host]; ok {
		return limiter
	}
	limiter := &sourceProfileHostLimiter{isBuiltIn: isBuiltInHost(host)}
	e.limiters[host] = limiter
	return limiter
}

func (l *sourceProfileHostLimiter) beforeFetch(ctx context.Context) error {
	if l == nil || !l.isBuiltIn {
		return nil
	}
	l.mu.Lock()
	if l.blocked {
		l.mu.Unlock()
		return errBuiltInSourceProfileBlocked
	}
	now := time.Now()
	waitUntil := now
	if waitUntil.Before(l.nextFetch) {
		waitUntil = l.nextFetch
	}
	l.nextFetch = waitUntil.Add(builtInSourceProfileInterval)
	l.mu.Unlock()

	if delay := time.Until(waitUntil); delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
		timer.Stop()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.blocked {
		return errBuiltInSourceProfileBlocked
	}
	return nil
}

func (l *sourceProfileHostLimiter) afterFetch(err error) {
	if l == nil || !l.isBuiltIn || !isSourceProfileBlockError(err) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.blocked = true
}

func canonicalSourceProfileURL(rawURL string) string {
	if profileURL := normalizeBuiltInCompanyProfileURL(rawURL); profileURL != "" {
		return profileURL
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	parsed.Fragment = ""
	return parsed.String()
}

func isSourceProfileBlockError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "HTTP 401") || strings.Contains(text, "HTTP 403") || strings.Contains(text, "HTTP 429")
}

func recordHasIdentity(record CompanyIdentityRecord) bool {
	return strings.TrimSpace(record.Website) != "" ||
		strings.TrimSpace(record.Summary) != "" ||
		strings.TrimSpace(record.Industry) != "" ||
		record.Identity != nil
}
