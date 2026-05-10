package storage

import "time"

// HealthCacheEntry represents a cached health result with timestamp
type HealthCacheEntry struct {
	Result    *CompanyHealthResult `json:"result"`
	Timestamp time.Time            `json:"timestamp"`
}

// HealthCache maps company names to cached health results
type HealthCache map[string]HealthCacheEntry
