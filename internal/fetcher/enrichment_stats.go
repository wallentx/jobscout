package fetcher

import (
	"strconv"
	"strings"
	"sync"
)

type acceptedEnrichmentStats struct {
	mu sync.Mutex

	jobs int

	descriptionChecks int
	localCacheHits    int

	persistentCacheHits        int
	persistentCacheMisses      int
	persistentCacheFailed      int
	persistentCacheWrites      int
	persistentCacheWriteFailed int

	applyFetchAttempts int
	applyFetchSuccess  int
	applyFetchFailed   int
	applyFetchBlocked  int
	applyFetchEmpty    int

	sourceProfileAttempts  int
	sourceProfileCacheHits int
	sourceProfileFetches   int
	sourceProfileSuccess   int
	sourceProfileFailed    int
	sourceProfileBlocked   int
	sourceProfileSkipped   int

	companyHomepageAttempts int
	companyHomepageSuccess  int
	companyHomepageFailed   int
	companyHomepageBlocked  int
	companyHomepageEmpty    int
	companyAboutAttempts    int
	companyAboutSuccess     int
	companyAboutFailed      int
	companyAboutBlocked     int
	companyAboutEmpty       int

	llmIdentityCalls int
	llmInputTokens   int
	llmOutputTokens  int
	llmTotalTokens   int
	llmCachedTokens  int

	sameCompanyCopiedFields int
}

func newAcceptedEnrichmentStats(jobs int) *acceptedEnrichmentStats {
	return &acceptedEnrichmentStats{jobs: jobs}
}

func (s *acceptedEnrichmentStats) inc(field *int) {
	if s == nil || field == nil {
		return
	}
	s.mu.Lock()
	(*field)++
	s.mu.Unlock()
}

func (s *acceptedEnrichmentStats) addCopiedFields(count int) {
	if s == nil || count <= 0 {
		return
	}
	s.mu.Lock()
	s.sameCompanyCopiedFields += count
	s.mu.Unlock()
}

func (s *acceptedEnrichmentStats) addLLMUsage(usage LLMTokenUsage) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.llmIdentityCalls++
	addTokenPtr(&s.llmInputTokens, usage.InputTokens)
	addTokenPtr(&s.llmOutputTokens, usage.OutputTokens)
	addTokenPtr(&s.llmTotalTokens, usage.TotalTokens)
	addTokenPtr(&s.llmCachedTokens, usage.CachedTokens)
	s.mu.Unlock()
}

func addTokenPtr(total *int, value *int) {
	if total == nil || value == nil {
		return
	}
	*total += *value
}

func (s *acceptedEnrichmentStats) log(duration string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	logDebug(
		"identity enrichment accepted stats: jobs=%d description_checks=%d local_cache_hits=%d persistent_cache hits=%d misses=%d failed=%d writes=%d write_failed=%d apply_fetches=%d/%d failed=%d blocked=%d empty=%d source_profiles attempts=%d cache_hits=%d fetches=%d success=%d failed=%d blocked=%d skipped=%d company_homepage attempts=%d success=%d failed=%d blocked=%d empty=%d company_about attempts=%d success=%d failed=%d blocked=%d empty=%d llm_identity_calls=%d token_usage=%s same_company_copied_fields=%d duration=%s",
		s.jobs,
		s.descriptionChecks,
		s.localCacheHits,
		s.persistentCacheHits,
		s.persistentCacheMisses,
		s.persistentCacheFailed,
		s.persistentCacheWrites,
		s.persistentCacheWriteFailed,
		s.applyFetchSuccess,
		s.applyFetchAttempts,
		s.applyFetchFailed,
		s.applyFetchBlocked,
		s.applyFetchEmpty,
		s.sourceProfileAttempts,
		s.sourceProfileCacheHits,
		s.sourceProfileFetches,
		s.sourceProfileSuccess,
		s.sourceProfileFailed,
		s.sourceProfileBlocked,
		s.sourceProfileSkipped,
		s.companyHomepageAttempts,
		s.companyHomepageSuccess,
		s.companyHomepageFailed,
		s.companyHomepageBlocked,
		s.companyHomepageEmpty,
		s.companyAboutAttempts,
		s.companyAboutSuccess,
		s.companyAboutFailed,
		s.companyAboutBlocked,
		s.companyAboutEmpty,
		s.llmIdentityCalls,
		formatAcceptedEnrichmentTokenUsageLocked(s),
		s.sameCompanyCopiedFields,
		duration,
	)
}

func formatAcceptedEnrichmentTokenUsageLocked(s *acceptedEnrichmentStats) string {
	if s == nil || (s.llmInputTokens == 0 && s.llmOutputTokens == 0 && s.llmTotalTokens == 0 && s.llmCachedTokens == 0) {
		return "unavailable"
	}
	parts := make([]string, 0, 4)
	if s.llmInputTokens > 0 {
		parts = append(parts, "input="+strconv.Itoa(s.llmInputTokens))
	}
	if s.llmOutputTokens > 0 {
		parts = append(parts, "output="+strconv.Itoa(s.llmOutputTokens))
	}
	if s.llmTotalTokens > 0 {
		parts = append(parts, "total="+strconv.Itoa(s.llmTotalTokens))
	}
	if s.llmCachedTokens > 0 {
		parts = append(parts, "cached="+strconv.Itoa(s.llmCachedTokens))
	}
	return strings.Join(parts, " ")
}

func isBlockedFetchError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "HTTP 401") ||
		strings.Contains(text, "HTTP 403") ||
		strings.Contains(text, "HTTP 429")
}
