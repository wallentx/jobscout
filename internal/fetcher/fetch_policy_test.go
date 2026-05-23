package fetcher

import "testing"

func TestSiteCandidateLimiterCapsCandidatesPerSource(t *testing.T) {
	limiter := newSiteCandidateLimiter(3)
	first, skipped := limiter.take("https://www.indeed.com/jobs", []siteSearchCandidate{
		{Title: "One"},
		{Title: "Two"},
	})
	if len(first) != 2 || skipped != 0 {
		t.Fatalf("first take len=%d skipped=%d; want len=2 skipped=0", len(first), skipped)
	}

	second, skipped := limiter.take("https://www.indeed.com/jobs", []siteSearchCandidate{
		{Title: "Three"},
		{Title: "Four"},
		{Title: "Five"},
	})
	if len(second) != 1 || skipped != 2 {
		t.Fatalf("second take len=%d skipped=%d; want len=1 skipped=2", len(second), skipped)
	}
}

func TestCapAcceptedFetchedJobsKeepsConfiguredLimit(t *testing.T) {
	summary := newFetchSummary()
	jobs := []Job{
		{Company: "One", Title: "Engineer"},
		{Company: "Two", Title: "Engineer"},
		{Company: "Three", Title: "Engineer"},
	}

	got := capAcceptedFetchedJobs(jobs, 2, &summary)
	if len(got) != 2 {
		t.Fatalf("capAcceptedFetchedJobs len = %d; want 2", len(got))
	}
	if got := len(summary.Filtered["accepted limit reached"]); got != 1 {
		t.Fatalf("accepted limit filtered count = %d; want 1", got)
	}
}
