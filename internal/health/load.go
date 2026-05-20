package health

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/fetcher"
	"github.com/wallentx/jobscout/internal/storage"
)

type LoadResult struct {
	Company   string
	Result    *domain.CompanyHealthResult
	Err       error
	FetchedAt time.Time
	FromCache bool
}

func CompanyHealthWithContext(ctx context.Context, identity domain.CompanyHealthContext, ticker string, includeNews bool) (*domain.CompanyHealthResult, error) {
	start := time.Now()
	logDebug(
		"data fetch start company=%q website=%q industry=%q include_news=%t",
		identity.Company,
		identity.Website,
		identity.Industry,
		includeNews,
	)
	result, err := domain.CompanyHealthWithDataSources(identity, ticker, includeNews, domain.CompanyHealthDataSources{
		FetchCompanySiteProfile: func(identity domain.CompanyHealthContext) (*domain.CompanySiteProfile, error) {
			return fetchBrowserCompanySiteProfileThrottled(ctx, identity)
		},
		FetchLayoffsFYI: fetcher.FetchLayoffsFYISignals,
	})
	if err != nil {
		logDebug("data fetch failed company=%q duration=%s error=%v", identity.Company, time.Since(start).Round(time.Millisecond), err)
		return nil, err
	}
	logDebug(
		"timing company=%q step=data_sources duration=%s sources=%s wikidata=%t employer_reviews=%d layoff_signals=%d hn_signals=%d",
		result.Company,
		time.Since(start).Round(time.Millisecond),
		debugSourceKeys(result.Sources),
		result.Sources != nil && result.Sources["wikidata"] != nil,
		len(result.EmployerReviews),
		len(result.LayoffSignals),
		len(result.HNSignals),
	)
	logCompanyHealthResult("data fetch complete", result, time.Since(start))
	return result, nil
}

var companyHealthWithContext = CompanyHealthWithContext

func fetchBrowserCompanySiteProfileThrottled(ctx context.Context, identity domain.CompanyHealthContext) (*domain.CompanySiteProfile, error) {
	var profile *domain.CompanySiteProfile
	company := strings.TrimSpace(identity.Company)
	start := time.Now()
	logDebug("browser company profile lookup start company=%q website=%q", company, identity.Website)
	err := runThrottledHealthStep(ctx, companyHealthBrowserSem, "browser company profile", company, func() error {
		var fetchErr error
		if session := browserSessionFromContext(ctx); session != nil {
			logDebug("browser company profile lookup using shared browser company=%q", company)
			profile, fetchErr = session.FetchCompanySiteProfile(ctx, identity)
		} else {
			profile, fetchErr = fetcher.FetchBrowserCompanySiteProfileForIdentity(identity)
		}
		return fetchErr
	})
	switch {
	case err == nil && profile != nil:
		logDebug(
			"browser company profile lookup succeeded company=%q website_url=%q about_url=%q summary=%t industry=%q duration=%s",
			company,
			profile.WebsiteURL,
			profile.AboutURL,
			strings.TrimSpace(profile.Summary) != "",
			profile.Industry,
			time.Since(start).Round(time.Millisecond),
		)
	case errors.Is(err, fetcher.ErrBrowserNotInstalled):
		logDebug("browser company profile lookup skipped company=%q reason=browser_not_installed duration=%s", company, time.Since(start).Round(time.Millisecond))
	case err != nil:
		logDebug("browser company profile lookup failed company=%q duration=%s error=%v", company, time.Since(start).Round(time.Millisecond), err)
	default:
		logDebug("browser company profile lookup returned no profile company=%q duration=%s", company, time.Since(start).Round(time.Millisecond))
	}
	return profile, err
}

func LoadCompanyHealth(ctx context.Context, identity domain.CompanyHealthContext, forceRefresh bool, store storage.HealthStore, appCfg *config.AppConfig) LoadResult {
	start := time.Now()
	company := strings.TrimSpace(identity.Company)
	cacheKey := CacheKeyForIdentity(identity)
	if cacheKey == "" {
		cacheKey = company
	}
	initialCacheKey := cacheKey
	logDebug(
		"load start company=%q cache_key=%q website=%q industry=%q force_refresh=%t llm_company_health=%t",
		company,
		cacheKey,
		identity.Website,
		identity.Industry,
		forceRefresh,
		LLMCompanyHealthEnabled(appCfg),
	)
	if identity.RequireResolvedIdentity && domain.CompanyHealthContextDomain(identity) == "" && !LLMCompanyHealthEnabled(appCfg) {
		logDebug("load skipped company=%q reason=identity_unresolved duration=%s", company, time.Since(start).Round(time.Millisecond))
		return LoadResult{
			Company: company,
			Err:     NewIdentityUnresolvedError(company),
		}
	}

	if !forceRefresh {
		if cached, ok := loadCachedCompanyHealth(ctx, appCfg, store, cacheKey, company, start); ok {
			return cached
		}
	}

	fetchIdentity := ApplyOptionalLLMCompanyIdentity(ctx, appCfg, identity)
	if fetchIdentity.RequireResolvedIdentity && domain.CompanyHealthContextDomain(fetchIdentity) == "" {
		logDebug("load skipped company=%q reason=identity_unresolved_after_llm duration=%s", company, time.Since(start).Round(time.Millisecond))
		return LoadResult{
			Company: company,
			Err:     NewIdentityUnresolvedError(company),
		}
	}
	if enrichedCacheKey := CacheKeyForIdentity(fetchIdentity); enrichedCacheKey != "" {
		cacheKey = enrichedCacheKey
	}
	if !forceRefresh && cacheKey != initialCacheKey {
		if cached, ok := loadCachedCompanyHealth(ctx, appCfg, store, cacheKey, company, start); ok {
			return cached
		}
	}
	result, err := companyHealthWithContext(ctx, fetchIdentity, "", true)
	fetchedAt := time.Now()
	if err == nil && result != nil {
		ApplyOptionalLLMCompanyHealth(ctx, appCfg, result)
		if err := store.SetHealth(cacheKey, result, fetchedAt); err != nil {
			logDebug("cache write failed company=%q cache_key=%q error=%v", company, cacheKey, err)
		} else {
			logDebug("cache write succeeded company=%q cache_key=%q", company, cacheKey)
		}
		logCompanyHealthResult("load fresh result", result, time.Since(start))
	} else if err != nil {
		logDebug("load fresh failed company=%q cache_key=%q duration=%s error=%v", company, cacheKey, time.Since(start).Round(time.Millisecond), err)
	}
	return LoadResult{
		Company:   company,
		Result:    result,
		Err:       err,
		FetchedAt: fetchedAt,
	}
}

func RefreshCompanyHealth(ctx context.Context, identity domain.CompanyHealthContext, appCfg *config.AppConfig) LoadResult {
	start := time.Now()
	company := strings.TrimSpace(identity.Company)
	logDebug(
		"refresh start company=%q website=%q industry=%q llm_company_health=%t",
		company,
		identity.Website,
		identity.Industry,
		LLMCompanyHealthEnabled(appCfg),
	)
	fetchIdentity := ApplyOptionalLLMCompanyIdentity(ctx, appCfg, identity)
	if fetchIdentity.RequireResolvedIdentity && domain.CompanyHealthContextDomain(fetchIdentity) == "" {
		logDebug("refresh skipped company=%q reason=identity_unresolved duration=%s", company, time.Since(start).Round(time.Millisecond))
		return LoadResult{
			Company: company,
			Err:     NewIdentityUnresolvedError(company),
		}
	}
	result, err := companyHealthWithContext(ctx, fetchIdentity, "", true)
	fetchedAt := time.Now()
	if err == nil && result != nil {
		ApplyOptionalLLMCompanyHealth(ctx, appCfg, result)
		logCompanyHealthResult("refresh result", result, time.Since(start))
	} else if err != nil {
		logDebug("refresh failed company=%q duration=%s error=%v", company, time.Since(start).Round(time.Millisecond), err)
	}
	return LoadResult{
		Company:   company,
		Result:    result,
		Err:       err,
		FetchedAt: fetchedAt,
	}
}

func logCompanyHealthResult(stage string, result *domain.CompanyHealthResult, elapsed time.Duration) {
	if result == nil {
		logDebug("%s result=nil duration=%s", stage, elapsed.Round(time.Millisecond))
		return
	}
	logDebug(
		"%s company=%q discovered_name=%q ticker=%q score=%d confidence=%q signals=%s flags=%s notes=%d notices=%d rejected_evidence=%d sources=%s llm_assessment=%t duration=%s",
		stage,
		result.Company,
		result.DiscoveredName,
		result.DiscoveredTicker,
		result.Score,
		result.Confidence,
		debugStringList(result.SignalsUsed),
		debugStringList(result.Flags),
		len(result.Notes),
		len(result.Notices),
		len(result.RejectedEvidence),
		debugSourceKeys(result.Sources),
		result.LLMAssessment != nil,
		elapsed.Round(time.Millisecond),
	)
	logDebug(
		"%s fields company=%q founded_year=%s employees=%s employment_risk=%d/%q hn_signals=%d layoff_signals=%d employer_reviews=%d",
		stage,
		result.Company,
		debugOptionalInt(result.FoundedYear),
		debugOptionalInt(result.EstimatedEmployees),
		debugEmploymentRiskScore(result.EmploymentRisk),
		debugEmploymentRiskLevel(result.EmploymentRisk),
		len(result.HNSignals),
		len(result.LayoffSignals),
		len(result.EmployerReviews),
	)
	if len(result.RejectedEvidence) > 0 {
		logDebug("%s rejected_evidence_summary company=%q summary=%s", stage, result.Company, debugRejectedEvidenceSummary(result.RejectedEvidence))
	}
	for _, notice := range result.Notices {
		logDebug("%s notice company=%q message=%q", stage, result.Company, notice)
	}
	fields := make([]string, 0, len(result.FieldAssessments))
	for field := range result.FieldAssessments {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		assessment := result.FieldAssessments[field]
		if assessment == nil {
			continue
		}
		logDebug(
			"%s field=%q status=%q confidence=%q source=%q evidence=%d notes=%d",
			stage,
			field,
			assessment.Status,
			assessment.Confidence,
			assessment.Source,
			len(assessment.Evidence),
			len(assessment.Notes),
		)
	}
}

func loadCachedCompanyHealth(ctx context.Context, appCfg *config.AppConfig, store storage.HealthStore, cacheKey string, company string, start time.Time) (LoadResult, bool) {
	cached, fetchedAt, err := store.GetHealth(cacheKey)
	if err == nil && cached != nil && IsHealthCacheFresh(fetchedAt, cached) {
		logDebug("cache hit company=%q cache_key=%q fetched_at=%s age=%s", company, cacheKey, fetchedAt.Format(time.RFC3339), time.Since(fetchedAt).Round(time.Second))
		if cached.LLMAssessment == nil && LLMCompanyHealthEnabled(appCfg) {
			logDebug("cache hit missing LLM assessment; applying LLM company health company=%q", company)
			ApplyOptionalLLMCompanyHealth(ctx, appCfg, cached)
			if cached.LLMAssessment != nil {
				if err := store.SetHealth(cacheKey, cached, fetchedAt); err != nil {
					logDebug("cache update after LLM assessment failed company=%q cache_key=%q error=%v", company, cacheKey, err)
				} else {
					logDebug("cache update after LLM assessment succeeded company=%q cache_key=%q", company, cacheKey)
				}
			}
		}
		logCompanyHealthResult("load cache result", cached, time.Since(start))
		return LoadResult{
			Company:   company,
			Result:    cached,
			FetchedAt: fetchedAt,
			FromCache: true,
		}, true
	}
	if err != nil {
		logDebug("cache lookup failed company=%q cache_key=%q error=%v", company, cacheKey, err)
	} else if cached == nil {
		logDebug("cache miss company=%q cache_key=%q", company, cacheKey)
	} else {
		logDebug("cache stale company=%q cache_key=%q fetched_at=%s age=%s", company, cacheKey, fetchedAt.Format(time.RFC3339), time.Since(fetchedAt).Round(time.Second))
	}
	return LoadResult{}, false
}

func debugOptionalInt(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func debugEmploymentRiskScore(risk *domain.EmploymentRisk) int {
	if risk == nil {
		return 0
	}
	return risk.Score
}

func debugEmploymentRiskLevel(risk *domain.EmploymentRisk) string {
	if risk == nil {
		return ""
	}
	return risk.Level
}

func debugStringList(values []string) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return "[]"
	}
	return "[" + strings.Join(out, ", ") + "]"
}

func debugSourceKeys(sources map[string]any) string {
	if len(sources) == 0 {
		return "[]"
	}
	keys := make([]string, 0, len(sources))
	for key := range sources {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return "[]"
	}
	sort.Strings(keys)
	return "[" + strings.Join(keys, ", ") + "]"
}

func debugRejectedEvidenceSummary(evidence []domain.CompanyHealthEvidence) string {
	if len(evidence) == 0 {
		return "[]"
	}
	counts := make(map[string]int)
	for _, item := range evidence {
		source := strings.TrimSpace(item.Source)
		if source == "" {
			source = "unknown"
		}
		reason := strings.TrimSpace(item.Reason)
		if reason == "" {
			reason = "unspecified"
		}
		counts[source+"/"+reason]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}
