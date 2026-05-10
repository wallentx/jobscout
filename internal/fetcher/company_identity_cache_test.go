package fetcher

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
)

func TestCompanyIdentityCache_GetSet(t *testing.T) {
	cache := NewCompanyIdentityCache()
	company := " Acme Enterprises "

	// Ensure missing company returns false
	if _, ok := cache.Get(company); ok {
		t.Fatal("Expected Get on empty cache to return false")
	}

	record := CompanyIdentityRecord{
		Website:  "https://acme.com",
		Summary:  "Anvils and explosives.",
		Industry: "Manufacturing",
		Identity: &domain.JobIdentityMetadata{
			Website: &domain.JobIdentityEvidence{Source: "test"},
		},
	}
	cache.Set(company, record)

	// Ensure normalization works
	got, ok := cache.Get("Acme Enterprises")
	if !ok {
		t.Fatal("Expected Get to find record using normalized name")
	}
	if got.Website != record.Website || got.Summary != record.Summary || got.Industry != record.Industry {
		t.Fatalf("Got record %+v, want %#v", got, record)
	}
	if got.Identity == nil || got.Identity.Website == nil || got.Identity.Website.Source != "test" {
		t.Fatal("Expected Identity evidence to be retrieved")
	}
}

func TestEnrichJobsFromApplyPagesHydratesPersistentCompanyIdentity(t *testing.T) {
	evidence := domain.JobIdentityMetadata{
		Website:  &domain.JobIdentityEvidence{Value: "https://www.acme.example", Source: "company_identity_store", Confidence: "high"},
		Summary:  &domain.JobIdentityEvidence{Value: "Acme builds deployment automation software for infrastructure teams.", Source: "company_identity_store", Confidence: "high"},
		Industry: &domain.JobIdentityEvidence{Value: "Developer Tools", Source: "company_identity_store", Confidence: "medium"},
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		t.Fatalf("json.Marshal(evidence) error = %v", err)
	}
	store := &fakePersistentCompanyIdentityStore{
		record: domain.CompanyIdentity{
			DisplayName:      "Acme",
			Website:          "https://www.acme.example",
			Summary:          "Acme builds deployment automation software for infrastructure teams.",
			Industry:         "Developer Tools",
			IdentityEvidence: evidenceJSON,
		},
	}
	jobs := []Job{{
		Company:      "Acme",
		Title:        "Software Engineer",
		Compensation: "$100,000",
		ApplyURL:     "https://jobs.example/acme/1",
	}}

	enriched := enrichJobsFromApplyPagesWithLLMStoreAndProgress(context.Background(), jobs, nil, store, nil, nil)

	if got, want := store.getCalls, 1; got != want {
		t.Fatalf("persistent store get calls = %d, want %d", got, want)
	}
	if got, want := enriched[0].CompanyWebsite, "https://www.acme.example"; got != want {
		t.Errorf("CompanyWebsite = %q, want %q", got, want)
	}
	if got, want := enriched[0].CompanySummary, "Acme builds deployment automation software for infrastructure teams."; got != want {
		t.Errorf("CompanySummary = %q, want %q", got, want)
	}
	if got, want := enriched[0].CompanyIndustry, "Developer Tools"; got != want {
		t.Errorf("CompanyIndustry = %q, want %q", got, want)
	}
	if enriched[0].CompanyIdentity == nil || enriched[0].CompanyIdentity.Website == nil || enriched[0].CompanyIdentity.Website.Source != "company_identity_store" {
		t.Fatalf("CompanyIdentity.Website = %#v, want persistent store evidence", enriched[0].CompanyIdentity)
	}
}

func TestEnrichJobsFromApplyPagesPersistsTrustedCompanyIdentity(t *testing.T) {
	store := &fakePersistentCompanyIdentityStore{}
	jobs := []Job{{
		Company:         "Acme",
		Title:           "Platform Engineer",
		Compensation:    "$120,000",
		ApplyURL:        "https://jobs.example/acme/1",
		CompanyWebsite:  "https://www.acme.example",
		CompanySummary:  "Acme builds deployment automation software for infrastructure teams.",
		CompanyIndustry: "Developer Tools",
		CompanyIdentity: &domain.JobIdentityMetadata{
			Website:  &domain.JobIdentityEvidence{Value: "https://www.acme.example", Source: "source_profile", URL: "https://jobs.example/acme/company", Confidence: "high"},
			Summary:  &domain.JobIdentityEvidence{Value: "Acme builds deployment automation software for infrastructure teams.", Source: "source_profile", URL: "https://jobs.example/acme/company", Confidence: "high"},
			Industry: &domain.JobIdentityEvidence{Value: "Developer Tools", Source: "source_profile", URL: "https://jobs.example/acme/company", Confidence: "medium"},
		},
	}}

	_ = enrichJobsFromApplyPagesWithLLMStoreAndProgress(context.Background(), jobs, nil, store, nil, nil)

	if got, want := store.setCalls, 1; got != want {
		t.Fatalf("persistent store set calls = %d, want %d", got, want)
	}
	if got, want := store.saved.Website, "https://www.acme.example"; got != want {
		t.Errorf("saved Website = %q, want %q", got, want)
	}
	if got, want := store.saved.Summary, "Acme builds deployment automation software for infrastructure teams."; got != want {
		t.Errorf("saved Summary = %q, want %q", got, want)
	}
	if got, want := store.saved.Industry, "Developer Tools"; got != want {
		t.Errorf("saved Industry = %q, want %q", got, want)
	}
	var savedEvidence domain.JobIdentityMetadata
	if err := json.Unmarshal(store.saved.IdentityEvidence, &savedEvidence); err != nil {
		t.Fatalf("json.Unmarshal(saved IdentityEvidence) error = %v", err)
	}
	if savedEvidence.Website == nil || savedEvidence.Website.Source != "source_profile" {
		t.Fatalf("saved IdentityEvidence.Website = %#v, want trusted source_profile evidence", savedEvidence.Website)
	}
}

type fakePersistentCompanyIdentityStore struct {
	record domain.CompanyIdentity
	ok     bool

	getCalls int
	setCalls int
	saved    domain.CompanyIdentity
}

func (s *fakePersistentCompanyIdentityStore) GetCompanyIdentity(ctx context.Context, company string, website string) (*domain.CompanyIdentity, error) {
	s.getCalls++
	if s.record.Website != "" || s.record.Summary != "" || s.record.Industry != "" || len(s.record.IdentityEvidence) > 0 {
		record := s.record
		return &record, nil
	}
	if s.ok {
		return &domain.CompanyIdentity{}, nil
	}
	return nil, nil
}

func (s *fakePersistentCompanyIdentityStore) UpsertCompanyIdentity(ctx context.Context, identity domain.CompanyIdentity) error {
	s.setCalls++
	s.saved = identity
	return nil
}

func TestApplyCachedIdentity(t *testing.T) {
	cache := NewCompanyIdentityCache()

	job := Job{
		Company: "Stark Industries",
		Title:   "Engineer",
	}

	record := CompanyIdentityRecord{
		Website:  "https://stark.com",
		Summary:  "Defense and clean energy.",
		Industry: "Defense",
		Identity: &domain.JobIdentityMetadata{
			Website:  &domain.JobIdentityEvidence{Value: "https://stark.com", Source: "cache-website", Confidence: "medium"},
			Summary:  &domain.JobIdentityEvidence{Value: "Defense and clean energy.", Source: "cache-summary", Confidence: "medium"},
			Industry: &domain.JobIdentityEvidence{Value: "Defense", Source: "cache-industry", Confidence: "medium"},
		},
	}
	cache.Set("Stark Industries", record)

	gotRecord, _ := cache.Get("Stark Industries")
	ApplyCachedIdentity(&job, gotRecord)

	if job.CompanyWebsite != "https://stark.com" {
		t.Errorf("job.CompanyWebsite = %q; want %q", job.CompanyWebsite, "https://stark.com")
	}
	if job.CompanySummary != "Defense and clean energy." {
		t.Errorf("job.CompanySummary = %q; want %q", job.CompanySummary, "Defense and clean energy.")
	}
	if job.CompanyIndustry != "Defense" {
		t.Errorf("job.CompanyIndustry = %q; want %q", job.CompanyIndustry, "Defense")
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Website == nil || job.CompanyIdentity.Website.Source != "cache-website" {
		t.Errorf("job.CompanyIdentity.Website missing cached evidence")
	}
}

func TestCompanyIdentityCache_Concurrency(t *testing.T) {
	cache := NewCompanyIdentityCache()
	var wg sync.WaitGroup

	// Simulate multiple goroutines trying to enrich the same company
	// but using a cache.

	workers := 10
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()

			job := Job{Company: "Acme"}

			// Simulate a concurrent cache check
			if record, ok := cache.Get(job.Company); ok {
				ApplyCachedIdentity(&job, record)
			} else {
				// Simulate slow enrichment
				time.Sleep(10 * time.Millisecond)

				job.CompanyWebsite = "https://acme.com"
				job.CompanySummary = "Acme summary. Acme summary. Acme summary. Acme summary. Acme summary."
				job.CompanyIndustry = "Tech"
				// Ensure Company is set correctly to be valid
				job.Company = "Acme"

				if cache.IsIdentityComplete(job) {
					cache.Set(job.Company, CompanyIdentityRecord{
						Website:  job.CompanyWebsite,
						Summary:  job.CompanySummary,
						Industry: job.CompanyIndustry,
						Identity: &domain.JobIdentityMetadata{
							Website:  &domain.JobIdentityEvidence{Value: job.CompanyWebsite, Source: "test", Confidence: "medium"},
							Summary:  &domain.JobIdentityEvidence{Value: job.CompanySummary, Source: "test", Confidence: "medium"},
							Industry: &domain.JobIdentityEvidence{Value: job.CompanyIndustry, Source: "test", Confidence: "medium"},
						},
					})
				}
			}

			if job.CompanyWebsite != "https://acme.com" {
				t.Errorf("worker %d: website not set", id)
			}
		}(i)
	}

	wg.Wait()

	if record, ok := cache.Get("Acme"); !ok || record.Website != "https://acme.com" {
		t.Fatalf("Cache not populated correctly at the end. ok=%t, record=%+v", ok, record)
	}
}

func TestPropagateSameCompanyIdentityFillsMissingFieldsWithCopiedEvidence(t *testing.T) {
	jobs := []Job{
		{
			Company:         "Acme Inc.",
			Title:           "Staff Platform Engineer",
			CompanyWebsite:  "https://acme.example",
			CompanySummary:  "Acme builds deployment tooling for software engineering teams and release automation.",
			CompanyIndustry: "Developer Tools",
			CompanyIdentity: &domain.JobIdentityMetadata{
				Website: &domain.JobIdentityEvidence{
					Value:      "https://acme.example",
					Source:     "llm_apply_page",
					URL:        "https://jobs.example/acme/platform",
					Confidence: "high",
					Reason:     "Website extracted by LLM from supplied page text.",
				},
				Summary: &domain.JobIdentityEvidence{
					Value:      "Acme builds deployment tooling for software engineering teams and release automation.",
					Source:     "llm_company_about",
					URL:        "https://acme.example/about",
					Confidence: "high",
					Reason:     "Company summary extracted by LLM from supplied page text.",
				},
				Industry: &domain.JobIdentityEvidence{
					Value:      "Developer Tools",
					Source:     "company_about",
					URL:        "https://acme.example/about",
					Confidence: "medium",
					Reason:     "Industry came from an explicit page label.",
				},
			},
		},
		{
			Company: "Acme",
			Title:   "Site Reliability Engineer",
		},
	}

	propagateSameCompanyIdentity(jobs)

	if jobs[1].CompanyWebsite != "https://acme.example" {
		t.Fatalf("CompanyWebsite = %q; want copied website", jobs[1].CompanyWebsite)
	}
	if jobs[1].CompanySummary != "Acme builds deployment tooling for software engineering teams and release automation." {
		t.Fatalf("CompanySummary = %q; want copied summary", jobs[1].CompanySummary)
	}
	if jobs[1].CompanyIndustry != "Developer Tools" {
		t.Fatalf("CompanyIndustry = %q; want copied industry", jobs[1].CompanyIndustry)
	}
	if jobs[1].CompanyIdentity == nil || jobs[1].CompanyIdentity.Website == nil {
		t.Fatalf("CompanyIdentity.Website = %#v; want copied evidence", jobs[1].CompanyIdentity)
	}
	if jobs[1].CompanyIdentity.Website.Source != "same_company_identity_copy" {
		t.Fatalf("CompanyIdentity.Website.Source = %q; want same_company_identity_copy", jobs[1].CompanyIdentity.Website.Source)
	}
	if !strings.Contains(jobs[1].CompanyIdentity.Website.Reason, "llm_apply_page") {
		t.Fatalf("CompanyIdentity.Website.Reason = %q; want original source mentioned", jobs[1].CompanyIdentity.Website.Reason)
	}
	if jobs[1].CompanyIdentity.Website.URL != "https://jobs.example/acme/platform" {
		t.Fatalf("CompanyIdentity.Website.URL = %q; want original evidence URL", jobs[1].CompanyIdentity.Website.URL)
	}
}

func TestPropagateSameCompanyIdentityDoesNotCopyWeakProvisionalData(t *testing.T) {
	jobs := []Job{
		{
			Company:         "Acme",
			Title:           "Staff Platform Engineer",
			CompanyWebsite:  "https://acme.example",
			CompanySummary:  "Acme builds deployment tooling for software engineering teams and release automation.",
			CompanyIndustry: "Developer Tools",
			CompanyIdentity: &domain.JobIdentityMetadata{
				Website: &domain.JobIdentityEvidence{
					Value:       "https://acme.example",
					Source:      "source_payload",
					URL:         "https://jobs.example/acme/platform",
					Confidence:  "low",
					Provisional: true,
				},
				Summary: &domain.JobIdentityEvidence{
					Value:       "Acme builds deployment tooling for software engineering teams and release automation.",
					Source:      "source_payload",
					URL:         "https://jobs.example/acme/platform",
					Confidence:  "low",
					Provisional: true,
				},
				Industry: &domain.JobIdentityEvidence{
					Value:       "Developer Tools",
					Source:      "company_summary_inference",
					URL:         "https://jobs.example/acme/platform",
					Confidence:  "low",
					Provisional: true,
				},
			},
		},
		{
			Company: "Acme Inc.",
			Title:   "Site Reliability Engineer",
		},
	}

	propagateSameCompanyIdentity(jobs)

	if jobs[1].CompanyWebsite != "" || jobs[1].CompanySummary != "" || jobs[1].CompanyIndustry != "" {
		t.Fatalf("copied identity = website %q summary %q industry %q; want weak provisional identity ignored", jobs[1].CompanyWebsite, jobs[1].CompanySummary, jobs[1].CompanyIndustry)
	}
	if jobs[1].CompanyIdentity != nil {
		t.Fatalf("CompanyIdentity = %#v; want no copied evidence", jobs[1].CompanyIdentity)
	}
}

func TestSeedCompanyIdentityFromTrustedJobsCopiesBeforeEnrichment(t *testing.T) {
	jobs := []Job{
		{
			Company:         "Affirm",
			Title:           "Software Engineer II, Frontend",
			CompanyWebsite:  "https://www.affirm.com",
			CompanySummary:  "Affirm builds financial products for consumer purchases.",
			CompanyIndustry: "Fintech",
			CompanyIdentity: &domain.JobIdentityMetadata{
				Website: &domain.JobIdentityEvidence{
					Value:      "https://www.affirm.com",
					Source:     "builtin_card_company_label",
					URL:        "https://builtin.com/company/affirm",
					Confidence: "high",
				},
				Summary: &domain.JobIdentityEvidence{
					Value:      "Affirm builds financial products for consumer purchases.",
					Source:     "builtin_company_profile",
					URL:        "https://builtin.com/company/affirm",
					Confidence: "high",
				},
				Industry: &domain.JobIdentityEvidence{
					Value:      "Fintech",
					Source:     "builtin_card_industry",
					URL:        "https://builtin.com/company/affirm",
					Confidence: "medium",
				},
			},
		},
		{
			Company: "Affirm",
			Title:   "Software Engineer I, Backend",
		},
	}
	cache := NewCompanyIdentityCache()

	copied := seedCompanyIdentityFromTrustedJobs(jobs, cache)

	if got, want := copied, 3; got != want {
		t.Fatalf("seedCompanyIdentityFromTrustedJobs(jobs, cache) = %d, want %d", got, want)
	}
	if got, want := jobs[1].CompanyWebsite, "https://www.affirm.com"; got != want {
		t.Errorf("jobs[1].CompanyWebsite = %q, want %q", got, want)
	}
	if got, want := jobs[1].CompanySummary, "Affirm builds financial products for consumer purchases."; got != want {
		t.Errorf("jobs[1].CompanySummary = %q, want %q", got, want)
	}
	if got, want := jobs[1].CompanyIndustry, "Fintech"; got != want {
		t.Errorf("jobs[1].CompanyIndustry = %q, want %q", got, want)
	}
	if jobs[1].CompanyIdentity == nil || jobs[1].CompanyIdentity.Website == nil || jobs[1].CompanyIdentity.Website.Source != "same_company_identity_copy" {
		t.Fatalf("jobs[1].CompanyIdentity.Website = %#v, want same_company_identity_copy evidence", jobs[1].CompanyIdentity)
	}

	record, ok := cache.Get("Affirm")
	if !ok {
		t.Fatal("cache.Get(\"Affirm\") ok = false, want true")
	}
	if got, want := record.Website, "https://www.affirm.com"; got != want {
		t.Errorf("cache.Get(\"Affirm\").Website = %q, want %q", got, want)
	}
}
