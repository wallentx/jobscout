package fetcher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSourceProfileEnricherSingleflightsDuplicateProfileFetches(t *testing.T) {
	enricher := newSourceProfileEnricher()
	fetches := 0
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		fetches++
		time.Sleep(20 * time.Millisecond)
		return acmeBuiltInProfileHTML(), nil
	}

	jobs := []Job{
		{Company: "Acme", Title: "Staff Platform Engineer"},
		{Company: "Acme", Title: "Senior SRE"},
	}
	var wg sync.WaitGroup
	for i := range jobs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			enricher.Enrich(context.Background(), &jobs[idx], "https://builtin.com/company/acme?utm=ignored", nil, "")
		}(i)
	}
	wg.Wait()

	if fetches != 1 {
		t.Fatalf("profile fetches = %d; want 1", fetches)
	}
	for i, job := range jobs {
		if job.CompanyWebsite != "https://acme.example" {
			t.Fatalf("jobs[%d].CompanyWebsite = %q; want cached profile website", i, job.CompanyWebsite)
		}
		if !strings.Contains(job.CompanySummary, "deployment automation") {
			t.Fatalf("jobs[%d].CompanySummary = %q; want cached profile summary", i, job.CompanySummary)
		}
	}
}

func TestSourceProfileEnricherSkipsLLMWhenDeterministicProfileCompletesIdentity(t *testing.T) {
	enricher := newSourceProfileEnricher()
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		return affirmBuiltInProfileHTML(), nil
	}
	job := Job{Company: "Affirm", Title: "Software Engineer"}
	llmCalls := 0
	llmEnrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		llmCalls++
		return &JobIdentityEnrichment{
			CompanyWebsite:  "https://llm.example",
			CompanySummary:  "LLM should not be needed for this profile.",
			CompanyIndustry: "Artificial Intelligence",
		}, LLMTokenUsage{}, nil
	}

	if ok := enricher.Enrich(context.Background(), &job, "https://builtin.com/company/affirm", llmEnrich, "llm_builtin_company_profile"); !ok {
		t.Fatal("Enrich() = false; want deterministic profile identity")
	}

	if llmCalls != 0 {
		t.Fatalf("LLM identity calls = %d; want 0 when deterministic source-profile parsing completes identity", llmCalls)
	}
	if job.CompanyWebsite != "https://www.affirm.com" {
		t.Fatalf("CompanyWebsite = %q; want deterministic profile website", job.CompanyWebsite)
	}
	if !strings.Contains(job.CompanySummary, "pay-over-time") {
		t.Fatalf("CompanySummary = %q; want deterministic profile summary", job.CompanySummary)
	}
	if job.CompanyIndustry != "Fintech" {
		t.Fatalf("CompanyIndustry = %q; want deterministic profile industry", job.CompanyIndustry)
	}
}

func TestSourceProfileEnricherUsesCachedHTMLForLaterLLMWithoutRefetch(t *testing.T) {
	enricher := newSourceProfileEnricher()
	fetches := 0
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		fetches++
		return `<html><body><a href="https://acme.example">Website</a><p>Acme builds deployment automation.</p></body></html>`, nil
	}

	first := Job{Company: "Acme", Title: "Platform Engineer"}
	if ok := enricher.Enrich(context.Background(), &first, "https://builtin.com/company/acme", nil, ""); !ok {
		t.Fatal("initial Enrich() = false; want deterministic cache entry")
	}

	second := Job{Company: "Acme", Title: "Platform Engineer"}
	llmCalls := 0
	llmEnrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		llmCalls++
		if !strings.Contains(page.Text, "deployment automation") {
			t.Fatalf("cached LLM page text = %q; want cached profile text", page.Text)
		}
		return &JobIdentityEnrichment{
			CompanySummary:    "Acme builds deployment automation software for platform teams.",
			SummaryConfidence: "high",
		}, LLMTokenUsage{}, nil
	}

	if ok := enricher.Enrich(context.Background(), &second, "https://builtin.com/company/acme", llmEnrich, "llm_builtin_company_profile"); !ok {
		t.Fatal("second Enrich() = false; want cached profile identity")
	}

	if fetches != 1 {
		t.Fatalf("profile fetches = %d; want cached HTML reuse without refetch", fetches)
	}
	if llmCalls != 1 {
		t.Fatalf("LLM identity calls = %d; want 1 using cached profile HTML", llmCalls)
	}
	if second.CompanySummary != "Acme builds deployment automation software for platform teams." {
		t.Fatalf("second.CompanySummary = %q; want LLM summary from cached HTML", second.CompanySummary)
	}
}

func TestSourceProfileRunRegistrySharesFetchProfileWithAcceptedEnrichment(t *testing.T) {
	sourceProfileRunMu.Lock()
	sourceProfileRun = nil
	sourceProfileRunMu.Unlock()
	t.Cleanup(func() {
		sourceProfileRunMu.Lock()
		sourceProfileRun = nil
		sourceProfileRunMu.Unlock()
	})

	fetchRunProfiles := beginSourceProfileRun()
	fetches := 0
	fetchRunProfiles.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		fetches++
		return `<html><body><a href="https://acme.example">Website</a><p>Acme builds deployment automation.</p></body></html>`, nil
	}

	fetchedJob := Job{Company: "Acme", Title: "Platform Engineer"}
	if ok := fetchRunProfiles.Enrich(context.Background(), &fetchedJob, "https://builtin.com/company/acme", nil, ""); !ok {
		t.Fatal("fetch-run Enrich() = false; want source profile cached")
	}

	acceptedRunProfiles := currentSourceProfileRun()
	acceptedJob := Job{Company: "Acme", Title: "Platform Engineer"}
	llmCalls := 0
	llmEnrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		llmCalls++
		return &JobIdentityEnrichment{
			CompanySummary:    "Acme builds deployment automation software for platform teams.",
			SummaryConfidence: "high",
		}, LLMTokenUsage{}, nil
	}

	if ok := acceptedRunProfiles.Enrich(context.Background(), &acceptedJob, "https://builtin.com/company/acme", llmEnrich, "llm_builtin_company_profile"); !ok {
		t.Fatal("accepted-run Enrich() = false; want shared source profile cache")
	}

	if fetches != 1 {
		t.Fatalf("profile fetches = %d; want accepted enrichment to reuse fetch-run profile", fetches)
	}
	if llmCalls != 1 {
		t.Fatalf("LLM identity calls = %d; want one cached-HTML LLM pass", llmCalls)
	}
	if acceptedJob.CompanySummary != "Acme builds deployment automation software for platform teams." {
		t.Fatalf("acceptedJob.CompanySummary = %q; want LLM summary from shared cached profile", acceptedJob.CompanySummary)
	}
}

func TestSourceProfileEnricherDoesNotRetryBuiltInBlock(t *testing.T) {
	prevInterval := builtInSourceProfileInterval
	builtInSourceProfileInterval = time.Millisecond
	t.Cleanup(func() {
		builtInSourceProfileInterval = prevInterval
	})

	enricher := newSourceProfileEnricher()
	fetches := 0
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		fetches++
		return "", fmt.Errorf("HTTP 403")
	}

	first := Job{Company: "Acme", Title: "Staff Platform Engineer"}
	if ok := enricher.Enrich(context.Background(), &first, "https://builtin.com/company/acme", nil, ""); ok {
		t.Fatal("Enrich() = true; want false after HTTP 403")
	}

	if fetches != 1 {
		t.Fatalf("profile fetches = %d; want no retry after HTTP 403", fetches)
	}
}

func TestSourceProfileEnricherDisablesBuiltInProfilesAfterBlock(t *testing.T) {
	prevInterval := builtInSourceProfileInterval
	builtInSourceProfileInterval = time.Millisecond
	t.Cleanup(func() {
		builtInSourceProfileInterval = prevInterval
	})

	enricher := newSourceProfileEnricher()
	fetches := 0
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		fetches++
		return "", fmt.Errorf("HTTP 403")
	}
	job := Job{Company: "Affirm", Title: "Software Engineer"}

	if ok := enricher.Enrich(context.Background(), &job, "https://builtin.com/company/affirm", nil, ""); ok {
		t.Fatal("first Enrich() = true; want false after HTTP 403")
	}
	next := Job{Company: "Toast", Title: "Software Engineer"}
	if ok := enricher.Enrich(context.Background(), &next, "https://builtin.com/company/toast", nil, ""); ok {
		t.Fatal("second Enrich() = true; want false after Built In host disabled")
	}

	if fetches != 1 {
		t.Fatalf("profile fetches = %d; want only first blocked fetch", fetches)
	}
}

func TestSourceProfileEnricherQueuedBuiltInFetchSkipsAfterBlock(t *testing.T) {
	prevInterval := builtInSourceProfileInterval
	builtInSourceProfileInterval = 50 * time.Millisecond
	t.Cleanup(func() {
		builtInSourceProfileInterval = prevInterval
	})

	enricher := newSourceProfileEnricher()
	firstFetchStarted := make(chan struct{})
	releaseFirstFetch := make(chan struct{})
	fetches := 0
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		fetches++
		if fetches == 1 {
			close(firstFetchStarted)
			<-releaseFirstFetch
			return "", fmt.Errorf("HTTP 403")
		}
		return affirmBuiltInProfileHTML(), nil
	}

	first := Job{Company: "Block", Title: "Software Engineer"}
	second := Job{Company: "Toast", Title: "Software Engineer"}
	firstDone := make(chan bool, 1)
	secondDone := make(chan bool, 1)

	go func() {
		firstDone <- enricher.Enrich(context.Background(), &first, "https://builtin.com/company/block-inc", nil, "")
	}()
	<-firstFetchStarted
	go func() {
		secondDone <- enricher.Enrich(context.Background(), &second, "https://builtin.com/company/toast", nil, "")
	}()

	close(releaseFirstFetch)

	if ok := <-firstDone; ok {
		t.Fatal("first Enrich() = true; want false after HTTP 403")
	}
	if ok := <-secondDone; ok {
		t.Fatal("second Enrich() = true; want false after queued fetch observes host block")
	}
	if fetches != 1 {
		t.Fatalf("profile fetches = %d; want queued worker to skip without fetching", fetches)
	}
}

func TestEnrichBuiltInListingJobProfilesUsesCompanyProfileOnce(t *testing.T) {
	enricher := newSourceProfileEnricher()
	fetches := 0
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		fetches++
		if profileURL != "https://builtin.com/company/affirm" {
			return "", fmt.Errorf("unexpected profile URL %q", profileURL)
		}
		return affirmBuiltInProfileHTML(), nil
	}

	listingJobs := []builtInListingJob{
		{
			profileURL: "https://builtin.com/company/affirm?utm=ignored",
			job: Job{
				Company:  "Affirm",
				Title:    "Software Engineer",
				ApplyURL: "https://builtin.com/job/software-engineer/1000001",
			},
		},
		{
			profileURL: "https://builtin.com/company/affirm",
			job: Job{
				Company:  "Affirm",
				Title:    "Application Developer",
				ApplyURL: "https://builtin.com/job/application-developer/1000002",
			},
		},
	}

	enrichBuiltInListingJobProfiles(context.Background(), listingJobs, enricher)

	if fetches != 1 {
		t.Fatalf("profile fetches = %d; want 1", fetches)
	}
	for i, listingJob := range listingJobs {
		if listingJob.job.CompanyWebsite != "https://www.affirm.com" {
			t.Fatalf("listingJobs[%d].job.CompanyWebsite = %q; want https://www.affirm.com", i, listingJob.job.CompanyWebsite)
		}
		if !strings.Contains(listingJob.job.CompanySummary, "pay-over-time") {
			t.Fatalf("listingJobs[%d].job.CompanySummary = %q; want Built In company profile summary", i, listingJob.job.CompanySummary)
		}
		if listingJob.job.CompanyIndustry != "Fintech" {
			t.Fatalf("listingJobs[%d].job.CompanyIndustry = %q; want Fintech", i, listingJob.job.CompanyIndustry)
		}
		if listingJob.job.CompanyIdentity == nil || listingJob.job.CompanyIdentity.Website == nil {
			t.Fatalf("listingJobs[%d].job.CompanyIdentity = %#v; want website evidence", i, listingJob.job.CompanyIdentity)
		}
	}
}

func TestEnrichBuiltInListingJobProfilesKeepsJobWhenProfileFetchFails(t *testing.T) {
	prevInterval := builtInSourceProfileInterval
	builtInSourceProfileInterval = time.Millisecond
	t.Cleanup(func() {
		builtInSourceProfileInterval = prevInterval
	})

	enricher := newSourceProfileEnricher()
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		return "", fmt.Errorf("HTTP 403")
	}
	listingJobs := []builtInListingJob{
		{
			profileURL: "https://builtin.com/company/affirm",
			job: Job{
				Company:  "Affirm",
				Title:    "Software Engineer",
				ApplyURL: "https://builtin.com/job/software-engineer/1000001",
			},
		},
	}

	enrichBuiltInListingJobProfiles(context.Background(), listingJobs, enricher)

	if listingJobs[0].job.Company != "Affirm" || listingJobs[0].job.Title != "Software Engineer" {
		t.Fatalf("listing job = %#v; want original job preserved after profile fetch failure", listingJobs[0].job)
	}
	if listingJobs[0].job.CompanyWebsite != "" {
		t.Fatalf("CompanyWebsite = %q; want empty after failed profile fetch", listingJobs[0].job.CompanyWebsite)
	}
}

func TestBuiltInListingProfileHydrationRunsAfterCardFilter(t *testing.T) {
	enricher := newSourceProfileEnricher()
	fetches := 0
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		fetches++
		return affirmBuiltInProfileHTML(), nil
	}
	criteria := &CriteriaConfig{}
	criteria.Filters.TitleIncludes = []string{"Engineer"}
	listingJobs := []builtInListingJob{
		{
			profileURL: "https://builtin.com/company/affirm",
			job: Job{
				Company:  "Affirm",
				Title:    "Software Engineer",
				ApplyURL: "https://builtin.com/job/software-engineer/1000001",
			},
		},
		{
			profileURL: "https://builtin.com/company/notion",
			job: Job{
				Company:  "Notion",
				Title:    "Product Manager",
				ApplyURL: "https://builtin.com/job/product-manager/1000002",
			},
		},
	}

	accepted, filtered := filterBuiltInListingJobsWithProfiles(listingJobs, criteria)
	enrichBuiltInListingJobProfiles(context.Background(), accepted, enricher)

	if len(accepted) != 1 {
		t.Fatalf("accepted len = %d; want 1", len(accepted))
	}
	if got := countFilteredJobs(filtered); got != 1 {
		t.Fatalf("filtered count = %d; want 1", got)
	}
	if fetches != 1 {
		t.Fatalf("profile fetches = %d; want 1 for accepted card only", fetches)
	}
	if accepted[0].job.CompanyWebsite != "https://www.affirm.com" {
		t.Fatalf("accepted[0].job.CompanyWebsite = %q; want https://www.affirm.com", accepted[0].job.CompanyWebsite)
	}
}

func TestBuiltInListingProfileHydrationSkipsExistingJobs(t *testing.T) {
	enricher := newSourceProfileEnricher()
	fetches := 0
	enricher.fetchHTML = func(ctx context.Context, profileURL string) (string, error) {
		fetches++
		return affirmBuiltInProfileHTML(), nil
	}
	existing := newExistingJobIndex([]Job{{
		Company:  "Affirm",
		Title:    "Software Engineer",
		ApplyURL: "https://builtin.com/job/software-engineer/1000001",
	}})
	listingJobs := []builtInListingJob{
		{
			profileURL: "https://builtin.com/company/affirm",
			job: Job{
				Company:  "Affirm",
				Title:    "Software Engineer",
				ApplyURL: "https://www.builtinseattle.com/job/software-engineer/1000001",
			},
		},
		{
			profileURL: "https://builtin.com/company/beta",
			job: Job{
				Company:  "Beta",
				Title:    "Software Engineer",
				ApplyURL: "https://builtin.com/job/software-engineer/1000002",
			},
		},
	}

	kept, skipped := skipExistingBuiltInListingJobs(listingJobs, existing)
	enrichBuiltInListingJobProfiles(context.Background(), kept, enricher)

	if len(kept) != 1 || kept[0].job.Company != "Beta" {
		t.Fatalf("kept = %#v; want only new Beta listing", kept)
	}
	if len(skipped) != 1 || skipped[0].Company != "Affirm" {
		t.Fatalf("skipped = %#v; want existing Affirm job", skipped)
	}
	if fetches != 1 {
		t.Fatalf("profile fetches = %d; want only new listing profile fetched", fetches)
	}
}

func acmeBuiltInProfileHTML() string {
	return `<html><body>
		<a href="https://acme.example">Website</a>
		<h2>What We Do</h2>
		<p>Acme builds deployment automation software for infrastructure teams operating reliable distributed systems.</p>
		<h2>Why Work With Us</h2>
	</body></html>`
}

func affirmBuiltInProfileHTML() string {
	return `<html><body>
		<h1>Affirm</h1>
		<a href="https://www.affirm.com">Website</a>
		<div>Industry: Fintech Website Headquarters</div>
		<h2>What We Do</h2>
		<p>Affirm builds pay-over-time financial products for consumers and merchants.</p>
		<h2>Why Work With Us</h2>
	</body></html>`
}
