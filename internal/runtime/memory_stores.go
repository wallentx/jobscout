package runtime

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/storage"
)

type memoryJobStore struct {
	mu   sync.Mutex
	jobs []storage.Job
}

func (s *memoryJobStore) LoadJobs() ([]storage.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]storage.Job(nil), s.jobs...), nil
}

func (s *memoryJobStore) SaveJobs(jobs []storage.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs = append([]storage.Job(nil), jobs...)
	return nil
}

type memoryHealthStore struct {
	mu    sync.Mutex
	cache storage.HealthCache
}

func (s *memoryHealthStore) LoadHealthCache() (storage.HealthCache, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return cloneHealthCache(s.cache), nil
}

func (s *memoryHealthStore) SaveHealthCache(cache storage.HealthCache) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache = cloneHealthCache(cache)
	return nil
}

func (s *memoryHealthStore) GetHealth(company string) (*storage.CompanyHealthResult, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.cache[company]
	if !ok {
		return nil, time.Time{}, nil
	}
	return entry.Result, entry.Timestamp, nil
}

func (s *memoryHealthStore) SetHealth(company string, result *storage.CompanyHealthResult, fetchedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache == nil {
		s.cache = make(storage.HealthCache)
	}
	s.cache[company] = storage.HealthCacheEntry{
		Result:    result,
		Timestamp: fetchedAt,
	}
	return nil
}

func (s *memoryHealthStore) ClearHealthCache() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache = make(storage.HealthCache)
	return nil
}

func cloneHealthCache(cache storage.HealthCache) storage.HealthCache {
	out := make(storage.HealthCache, len(cache))
	for company, entry := range cache {
		out[company] = entry
	}
	return out
}

type memoryCompanyIdentityStore struct {
	mu         sync.Mutex
	identities map[string]domain.CompanyIdentity
}

func (s *memoryCompanyIdentityStore) GetCompanyIdentity(ctx context.Context, companyName string, websiteOrDomain string) (*domain.CompanyIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range companyIdentityMemoryKeys(companyName, websiteOrDomain) {
		identity, ok := s.identities[key]
		if !ok {
			continue
		}
		clone := identity
		clone.NameAliases = append([]string(nil), identity.NameAliases...)
		clone.DomainAliases = append([]string(nil), identity.DomainAliases...)
		if identity.IdentityEvidence != nil {
			clone.IdentityEvidence = append([]byte(nil), identity.IdentityEvidence...)
		}
		return &clone, nil
	}
	return nil, nil
}

func (s *memoryCompanyIdentityStore) UpsertCompanyIdentity(ctx context.Context, identity domain.CompanyIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.identities == nil {
		s.identities = make(map[string]domain.CompanyIdentity)
	}
	clone := identity
	clone.NameAliases = append([]string(nil), identity.NameAliases...)
	clone.DomainAliases = append([]string(nil), identity.DomainAliases...)
	if identity.IdentityEvidence != nil {
		clone.IdentityEvidence = append([]byte(nil), identity.IdentityEvidence...)
	}
	for _, key := range companyIdentityMemoryKeys(identity.DisplayName, identity.Website) {
		s.identities[key] = clone
	}
	for _, alias := range identity.NameAliases {
		for _, key := range companyIdentityMemoryKeys(alias, "") {
			s.identities[key] = clone
		}
	}
	for _, alias := range identity.DomainAliases {
		for _, key := range companyIdentityMemoryKeys("", alias) {
			s.identities[key] = clone
		}
	}
	return nil
}

func companyIdentityMemoryKeys(companyName string, websiteOrDomain string) []string {
	var keys []string
	if domain := normalizeMemoryDomain(websiteOrDomain); domain != "" {
		keys = append(keys, "domain:"+domain)
	}
	if name := strings.ToLower(strings.Join(strings.Fields(companyName), " ")); name != "" {
		keys = append(keys, "name:"+name)
	}
	return keys
}

func normalizeMemoryDomain(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

func InMemoryStores() Stores {
	return Stores{
		Jobs:            &memoryJobStore{},
		Health:          &memoryHealthStore{cache: make(storage.HealthCache)},
		CompanyIdentity: &memoryCompanyIdentityStore{identities: make(map[string]domain.CompanyIdentity)},
	}
}
