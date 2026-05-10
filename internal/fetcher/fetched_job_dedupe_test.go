package fetcher

import "testing"

func TestDedupeFetchedJobsUsesBuiltInJobIDAcrossHosts(t *testing.T) {
	jobs := []Job{
		{
			Company:        "Affirm",
			Title:          "Software Engineer II, Frontend",
			ApplyURL:       "https://builtin.com/job/software-engineer-ii-frontend/1234567",
			CompanySummary: "Affirm builds financial products for consumers and merchants.",
		},
		{
			Company:         "Affirm",
			Title:           "Software Engineer II, Frontend",
			ApplyURL:        "https://www.builtinseattle.com/job/software-engineer-ii-frontend/1234567?utm=ignored",
			CompanyWebsite:  "https://www.affirm.com",
			CompanyIndustry: "Fintech",
		},
	}

	deduped, duplicates := dedupeFetchedJobs(jobs)

	if len(deduped) != 1 {
		t.Fatalf("dedupeFetchedJobs(...) deduped len = %d; want 1", len(deduped))
	}
	if len(duplicates) != 1 {
		t.Fatalf("dedupeFetchedJobs(...) duplicates len = %d; want 1", len(duplicates))
	}
	if deduped[0].CompanyWebsite != "https://www.affirm.com" {
		t.Fatalf("deduped[0].CompanyWebsite = %q; want merged duplicate website", deduped[0].CompanyWebsite)
	}
	if deduped[0].CompanyIndustry != "Fintech" {
		t.Fatalf("deduped[0].CompanyIndustry = %q; want merged duplicate industry", deduped[0].CompanyIndustry)
	}
}

func TestDedupeFetchedJobsUsesCanonicalApplyURL(t *testing.T) {
	jobs := []Job{
		{
			Company:  "Acme",
			Title:    "Software Engineer",
			ApplyURL: "https://jobs.example.com/acme/software-engineer/",
		},
		{
			Company:  "Acme",
			Title:    "Software Engineer",
			ApplyURL: "https://JOBS.EXAMPLE.com/acme/software-engineer",
		},
	}

	deduped, duplicates := dedupeFetchedJobs(jobs)

	if len(deduped) != 1 {
		t.Fatalf("dedupeFetchedJobs(...) deduped len = %d; want 1", len(deduped))
	}
	if len(duplicates) != 1 {
		t.Fatalf("dedupeFetchedJobs(...) duplicates len = %d; want 1", len(duplicates))
	}
}

func TestDedupeFetchedJobsUsesCanonicalApplyURLAcrossIdentityDrift(t *testing.T) {
	jobs := []Job{
		{
			Company:  "Acme Inc.",
			Title:    "Software Engineer",
			ApplyURL: "https://jobs.example.com/acme/software-engineer/",
		},
		{
			Company:  "Acme",
			Title:    "Software Developer",
			ApplyURL: "https://JOBS.EXAMPLE.com/acme/software-engineer",
		},
	}

	deduped, duplicates := dedupeFetchedJobs(jobs)

	if len(deduped) != 1 {
		t.Fatalf("dedupeFetchedJobs(...) deduped len = %d; want 1", len(deduped))
	}
	if len(duplicates) != 1 {
		t.Fatalf("dedupeFetchedJobs(...) duplicates len = %d; want 1", len(duplicates))
	}
}

func TestDedupeFetchedJobsKeepsDistinctBuiltInJobIDs(t *testing.T) {
	jobs := []Job{
		{
			Company:  "Affirm",
			Title:    "Software Engineer",
			ApplyURL: "https://builtin.com/job/software-engineer/1000001",
		},
		{
			Company:  "Affirm",
			Title:    "Software Engineer",
			ApplyURL: "https://builtin.com/job/software-engineer/1000002",
		},
	}

	deduped, duplicates := dedupeFetchedJobs(jobs)

	if len(deduped) != 2 {
		t.Fatalf("dedupeFetchedJobs(...) deduped len = %d; want 2", len(deduped))
	}
	if len(duplicates) != 0 {
		t.Fatalf("dedupeFetchedJobs(...) duplicates len = %d; want 0", len(duplicates))
	}
}

func TestExistingJobIndexMatchesBuiltInJobIDAcrossHosts(t *testing.T) {
	index := newExistingJobIndex([]Job{{
		Company:  "Affirm",
		Title:    "Software Engineer II, Frontend",
		ApplyURL: "https://builtin.com/job/software-engineer-ii-frontend/1234567",
	}})

	job := Job{
		Company:  "Affirm",
		Title:    "Software Engineer II, Frontend",
		ApplyURL: "https://www.builtinseattle.com/job/software-engineer-ii-frontend/1234567?utm=ignored",
	}

	if !index.contains(job) {
		t.Fatalf("existing index did not match Built In job across hosts")
	}
}

func TestSkipExistingFetchedJobsRemovesExistingAndKeepsNew(t *testing.T) {
	index := newExistingJobIndex([]Job{{
		Company:  "Acme",
		Title:    "Software Engineer",
		ApplyURL: "https://jobs.example/acme/software-engineer",
	}})
	jobs := []Job{
		{Company: "Acme", Title: "Software Engineer", ApplyURL: "https://jobs.example/acme/software-engineer"},
		{Company: "Beta", Title: "Platform Engineer", ApplyURL: "https://jobs.example/beta/platform-engineer"},
	}

	kept, skipped := skipExistingFetchedJobs(jobs, index)

	if len(kept) != 1 || kept[0].Company != "Beta" {
		t.Fatalf("kept = %#v; want only new Beta job", kept)
	}
	if len(skipped) != 1 || skipped[0].Company != "Acme" {
		t.Fatalf("skipped = %#v; want existing Acme job", skipped)
	}
}

func TestSkipExistingFetchedJobsMatchesCanonicalApplyURLAcrossIdentityDrift(t *testing.T) {
	index := newExistingJobIndex([]Job{{
		Company:  "Acme Inc.",
		Title:    "Software Engineer",
		ApplyURL: "https://jobs.example/acme/software-engineer",
	}})
	jobs := []Job{
		{Company: "Acme", Title: "Software Developer", ApplyURL: "https://jobs.example/acme/software-engineer/"},
		{Company: "Beta", Title: "Platform Engineer", ApplyURL: "https://jobs.example/beta/platform-engineer"},
	}

	kept, skipped := skipExistingFetchedJobs(jobs, index)

	if len(kept) != 1 || kept[0].Company != "Beta" {
		t.Fatalf("kept = %#v; want only new Beta job", kept)
	}
	if len(skipped) != 1 || skipped[0].Company != "Acme" {
		t.Fatalf("skipped = %#v; want existing Acme job", skipped)
	}
}
