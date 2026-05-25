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

func (s *memoryHealthStore) DeleteHealth(company string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.cache, company)
	delete(s.cache, strings.ToLower(strings.TrimSpace(company)))
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

type memoryCandidateStore struct {
	mu         sync.Mutex
	candidates map[string]domain.JobCandidate
	decisions  map[domain.JobCandidateDecisionKey]domain.JobCandidateDecision
}

func (s *memoryCandidateStore) UpsertJobCandidate(ctx context.Context, candidate domain.JobCandidate) error {
	return s.UpsertJobCandidates(ctx, []domain.JobCandidate{candidate})
}

func (s *memoryCandidateStore) UpsertJobCandidates(ctx context.Context, candidates []domain.JobCandidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.candidates == nil {
		s.candidates = make(map[string]domain.JobCandidate)
	}
	for _, candidate := range candidates {
		key := strings.TrimSpace(candidate.Key)
		if key == "" {
			continue
		}
		now := time.Now()
		if candidate.FirstSeen.IsZero() {
			if existing, ok := s.candidates[key]; ok && !existing.FirstSeen.IsZero() {
				candidate.FirstSeen = existing.FirstSeen
			} else {
				candidate.FirstSeen = now
			}
		}
		if candidate.LastSeen.IsZero() {
			candidate.LastSeen = now
		}
		candidate.Key = key
		candidate.Job.Metadata = domain.NormalizeJobMetadata(candidate.Job.Metadata)
		s.candidates[key] = candidate
	}
	return nil
}

func (s *memoryCandidateStore) GetJobCandidate(ctx context.Context, candidateKey string) (*domain.JobCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	candidate, ok := s.candidates[strings.TrimSpace(candidateKey)]
	if !ok {
		return nil, nil
	}
	candidate.Job.Metadata = domain.NormalizeJobMetadata(domain.CloneJobMetadata(candidate.Job.Metadata))
	return &candidate, nil
}

func (s *memoryCandidateStore) UpsertJobCandidateDecision(ctx context.Context, decision domain.JobCandidateDecision) error {
	return s.UpsertJobCandidateDecisions(ctx, []domain.JobCandidateDecision{decision})
}

func (s *memoryCandidateStore) UpsertJobCandidateDecisions(ctx context.Context, decisions []domain.JobCandidateDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.decisions == nil {
		s.decisions = make(map[domain.JobCandidateDecisionKey]domain.JobCandidateDecision)
	}
	for _, decision := range decisions {
		key := normalizeMemoryCandidateDecisionKey(decision.JobCandidateDecisionKey)
		if key.CandidateKey == "" || key.CriteriaHash == "" || key.Stage == "" || key.DecisionVersion == "" {
			continue
		}
		if strings.TrimSpace(decision.Decision) == "" {
			continue
		}
		if decision.DecidedAt.IsZero() {
			decision.DecidedAt = time.Now()
		}
		decision.JobCandidateDecisionKey = key
		decision.Job.Metadata = domain.NormalizeJobMetadata(decision.Job.Metadata)
		s.decisions[key] = decision
	}
	return nil
}

func (s *memoryCandidateStore) GetJobCandidateDecision(ctx context.Context, key domain.JobCandidateDecisionKey) (*domain.JobCandidateDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key = normalizeMemoryCandidateDecisionKey(key)
	decision, ok := s.decisions[key]
	if !ok {
		return nil, nil
	}
	if !decision.ExpiresAt.IsZero() && decision.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	decision.Job.Metadata = domain.NormalizeJobMetadata(domain.CloneJobMetadata(decision.Job.Metadata))
	return &decision, nil
}

func (s *memoryCandidateStore) PruneJobCandidateCache(ctx context.Context, olderThan time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if olderThan.IsZero() {
		return 0, nil
	}
	removed := 0
	for key, candidate := range s.candidates {
		if candidate.LastSeen.Before(olderThan) {
			delete(s.candidates, key)
			removed++
		}
	}
	for key := range s.decisions {
		if _, ok := s.candidates[key.CandidateKey]; !ok {
			delete(s.decisions, key)
		}
	}
	return removed, nil
}

func normalizeMemoryCandidateDecisionKey(key domain.JobCandidateDecisionKey) domain.JobCandidateDecisionKey {
	return domain.JobCandidateDecisionKey{
		CandidateKey:    strings.TrimSpace(key.CandidateKey),
		CriteriaHash:    strings.TrimSpace(key.CriteriaHash),
		Stage:           strings.TrimSpace(key.Stage),
		DecisionVersion: strings.TrimSpace(key.DecisionVersion),
		LLMProvider:     strings.ToLower(strings.TrimSpace(key.LLMProvider)),
		LLMModel:        strings.TrimSpace(key.LLMModel),
	}
}

func InMemoryStores() Stores {
	candidateStore := &memoryCandidateStore{
		candidates: make(map[string]domain.JobCandidate),
		decisions:  make(map[domain.JobCandidateDecisionKey]domain.JobCandidateDecision),
	}
	return Stores{
		Jobs:            &memoryJobStore{},
		Health:          &memoryHealthStore{cache: make(storage.HealthCache)},
		CompanyIdentity: &memoryCompanyIdentityStore{identities: make(map[string]domain.CompanyIdentity)},
		Candidates:      candidateStore,
	}
}
