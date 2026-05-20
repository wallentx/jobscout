package health

import (
	"context"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
	"github.com/wallentx/jobscout/internal/storage"
)

type fakeLoadHealthStore struct {
	entries map[string]storage.HealthCacheEntry
}

func (s *fakeLoadHealthStore) LoadHealthCache() (storage.HealthCache, error) {
	return nil, nil
}

func (s *fakeLoadHealthStore) SaveHealthCache(storage.HealthCache) error {
	return nil
}

func (s *fakeLoadHealthStore) GetHealth(company string) (*domain.CompanyHealthResult, time.Time, error) {
	entry, ok := s.entries[company]
	if !ok {
		return nil, time.Time{}, nil
	}
	return entry.Result, entry.Timestamp, nil
}

func (s *fakeLoadHealthStore) SetHealth(company string, result *domain.CompanyHealthResult, fetchedAt time.Time) error {
	if s.entries == nil {
		s.entries = make(map[string]storage.HealthCacheEntry)
	}
	s.entries[company] = storage.HealthCacheEntry{Result: result, Timestamp: fetchedAt}
	return nil
}

func (s *fakeLoadHealthStore) DeleteHealth(company string) error {
	delete(s.entries, company)
	return nil
}

func (s *fakeLoadHealthStore) ClearHealthCache() error {
	s.entries = make(map[string]storage.HealthCacheEntry)
	return nil
}

func TestLoadCompanyHealthRechecksCacheAfterLLMIdentityEnrichment(t *testing.T) {
	originalInit := initConfiguredLLMForTask
	originalEnrich := enrichCompanyHealthIdentityWithLLM
	originalCompanyHealthWithContext := companyHealthWithContext
	defer func() {
		initConfiguredLLMForTask = originalInit
		enrichCompanyHealthIdentityWithLLM = originalEnrich
		companyHealthWithContext = originalCompanyHealthWithContext
	}()

	initConfiguredLLMForTask = func(ctx context.Context, appCfg *config.AppConfig, task string) (llms.Model, func(), error) {
		return fakeHealthLLM{}, func() {}, nil
	}
	enrichCompanyHealthIdentityWithLLM = func(ctx context.Context, model llms.Model, identity domain.CompanyHealthContext) (*llmpkg.CompanyIdentitySearchResult, llmpkg.LLMTokenUsage, error) {
		return &llmpkg.CompanyIdentitySearchResult{
			Website:  "https://www.acmecloud.example",
			Industry: "Developer Tools",
			Summary:  "Acme Cloud builds deployment automation for software teams.",
		}, llmpkg.LLMTokenUsage{}, nil
	}
	companyHealthWithContext = func(ctx context.Context, identity domain.CompanyHealthContext, ticker string, includeNews bool) (*domain.CompanyHealthResult, error) {
		t.Fatal("fresh company health fetch should not run when enriched cache key is fresh")
		return nil, nil
	}

	cached := &domain.CompanyHealthResult{
		Company:       "Acme Cloud",
		Score:         88,
		Confidence:    "high",
		SignalsUsed:   []string{},
		Flags:         []string{},
		Notes:         []string{},
		Sources:       map[string]any{},
		LLMAssessment: &domain.LLMCompanyHealthAssessment{RiskLevel: "low"},
	}
	store := &fakeLoadHealthStore{
		entries: map[string]storage.HealthCacheEntry{
			"domain:acmecloud.example": {
				Result:    cached,
				Timestamp: time.Now(),
			},
		},
	}

	loaded := LoadCompanyHealth(context.Background(), domain.CompanyHealthContext{
		Company: "Acme Cloud",
	}, false, store, &config.AppConfig{
		LLM: config.LLMConfig{
			Enabled:       true,
			CompanyHealth: true,
			Provider:      "openai",
		},
	})

	if !loaded.FromCache {
		t.Fatalf("LoadCompanyHealth(...).FromCache = false, want enriched cache hit")
	}
	if loaded.Result != cached {
		t.Fatalf("LoadCompanyHealth(...).Result = %#v, want cached result", loaded.Result)
	}
}
