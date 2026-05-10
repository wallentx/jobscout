package health

import (
	"sort"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/storage"
)

func CacheKeyForIdentity(identity domain.CompanyHealthContext) string {
	if domain := domain.CompanyHealthContextDomain(identity); domain != "" {
		return "domain:" + domain
	}
	return strings.TrimSpace(identity.Company)
}

func CacheKeyForJob(job domain.Job) string {
	return CacheKeyForIdentity(domain.CompanyHealthContext{
		Company:  job.Company,
		Website:  job.CompanyWebsite,
		Summary:  job.CompanySummary,
		Industry: job.CompanyIndustry,
	})
}

func SourceStringFromMap(sources map[string]any, group string, key string) string {
	if sources == nil {
		return ""
	}
	raw, ok := sources[group]
	if !ok {
		return ""
	}
	values, ok := raw.(map[string]string)
	if ok {
		return strings.TrimSpace(values[key])
	}
	anyValues, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	value, _ := anyValues[key].(string)
	return strings.TrimSpace(value)
}

func UniqueJobsFromJobs(jobs []domain.Job) []domain.Job {
	seen := make(map[string]bool, len(jobs))
	out := make([]domain.Job, 0, len(jobs))
	for _, job := range jobs {
		company := strings.TrimSpace(job.Company)
		if company == "" {
			continue
		}
		key := CacheKeyForJob(job)
		if key == "" {
			key = company
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, job)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Company) < strings.ToLower(out[j].Company)
	})
	return out
}

func UniqueCompaniesFromJobs(jobs []domain.Job) []string {
	seen := make(map[string]bool, len(jobs))
	out := make([]string, 0, len(jobs))
	for _, job := range jobs {
		company := strings.TrimSpace(job.Company)
		if company == "" || seen[company] {
			continue
		}
		seen[company] = true
		out = append(out, company)
	}
	sort.Strings(out)
	return out
}

func CachedHealthForJob(cache storage.HealthCache, job domain.Job) *domain.CompanyHealthResult {
	if cached := cachedHealth(cache, CacheKeyForJob(job)); cached != nil {
		return cached
	}
	return cachedHealth(cache, job.Company)
}

func StoredHealthForJob(cache storage.HealthCache, job domain.Job) *domain.CompanyHealthResult {
	if stored := storedHealth(cache, CacheKeyForJob(job)); stored != nil {
		return stored
	}
	return storedHealth(cache, job.Company)
}

func SetCachedHealth(cache storage.HealthCache, company string, result *domain.CompanyHealthResult) {
	cache[company] = storage.HealthCacheEntry{Result: result, Timestamp: time.Now()}
}

func IsCacheFresh(ts time.Time) bool {
	return IsHealthCacheFresh(ts, nil)
}

func IsHealthCacheFresh(ts time.Time, result *domain.CompanyHealthResult) bool {
	if ts.IsZero() {
		return false
	}
	return time.Since(ts) <= healthCacheTTL(result)
}

func healthCacheTTL(result *domain.CompanyHealthResult) time.Duration {
	if result == nil || healthResultHasVolatileSignals(result) {
		return 24 * time.Hour
	}
	return 7 * 24 * time.Hour
}

func healthResultHasVolatileSignals(result *domain.CompanyHealthResult) bool {
	if result == nil {
		return true
	}
	if len(result.LayoffSignals) > 0 {
		return true
	}
	if result.EmploymentRisk != nil && result.EmploymentRisk.Score >= 25 {
		return true
	}
	for _, flag := range result.Flags {
		flag = strings.ToLower(strings.TrimSpace(flag))
		if strings.Contains(flag, "layoff") ||
			strings.Contains(flag, "negative_news") ||
			strings.Contains(flag, "sec_risk") {
			return true
		}
	}
	return false
}

func cachedHealth(cache storage.HealthCache, company string) *domain.CompanyHealthResult {
	entry, ok := cache[company]
	if !ok || !IsHealthCacheFresh(entry.Timestamp, entry.Result) {
		return nil
	}
	return entry.Result
}

func storedHealth(cache storage.HealthCache, company string) *domain.CompanyHealthResult {
	entry, ok := cache[company]
	if !ok {
		return nil
	}
	return entry.Result
}
