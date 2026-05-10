package fetcher

import (
	"strings"
	"testing"
)

func TestFormatFetchReviewSummaryDebugIncludesSearchOutcomes(t *testing.T) {
	session := NewReviewSession(nil, nil, FetchSummary{
		Searches: map[string]string{
			FetchSearchLLM:  "disabled in config",
			FetchSearchRSS:  "executed; found 0 results",
			FetchSearchAPI:  "execution failed: boom",
			FetchSearchSite: "executed; found 2 results",
		},
	}, false)

	rendered := session.Summary(true)

	for _, expected := range []string{
		"Added",
		"  LLM",
		"    skipped: disabled in config",
		"  RSS",
		"    no results",
		"  Configured Source",
		"    failed: execution failed: boom",
		"Filtered",
		"  none",
		"Searches",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("ReviewSession.Summary() = %q; want %q", rendered, expected)
		}
	}
}

func TestFormatFetchReviewSummaryDebugTreatsJobSearchDisabledAsSkipped(t *testing.T) {
	session := NewReviewSession(nil, nil, FetchSummary{
		Searches: map[string]string{
			FetchSearchLLM: "LLM job search disabled in config",
		},
	}, false)

	rendered := session.Summary(true)

	if !strings.Contains(rendered, "    skipped: LLM job search disabled in config") {
		t.Fatalf("ReviewSession.Summary() = %q; want skipped LLM job search placeholder", rendered)
	}
}

func TestFormatFetchReviewSummaryHidesDebugSectionsWithoutDebug(t *testing.T) {
	job := Job{Company: "Acme", Title: "Staff Platform Engineer", Source: "LLM: llm_job_search"}
	session := NewReviewSession(nil, []Job{job}, FetchSummary{
		Notices: []string{"LLM job search failed: boom"},
		Rejected: map[string]map[string][]string{
			FetchSearchSite: {
				"missing title": {"No Title Co - "},
			},
		},
		Filtered: map[string][]Job{
			"duplicate": {
				{Company: "Acme", Title: "Staff Platform Engineer", Source: "LLM: llm_job_search"},
			},
		},
		Searches: map[string]string{
			FetchSearchSite: "executed; found 1 results",
		},
	}, false)

	rendered := session.Summary(false)

	for _, expected := range []string{
		"Fetched 1 results. Added 1 new jobs.",
		"Added",
		"    + Acme - Staff Platform Engineer",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("ReviewSession.Summary() = %q; want %q", rendered, expected)
		}
	}
	for _, unexpected := range []string{
		"Rejected",
		"Filtered",
		"Searches",
		"Notes",
		"LLM job search failed",
		"missing title",
		"duplicate",
	} {
		if strings.Contains(rendered, unexpected) {
			t.Fatalf("ReviewSession.Summary() = %q; did not want %q without debug", rendered, unexpected)
		}
	}
}

func TestFormatFetchReviewSummaryDebugGroupsFilteredByMethodAndSource(t *testing.T) {
	session := NewReviewSession(nil, nil, FetchSummary{
		Filtered: map[string][]Job{
			"duplicate": {
				{Company: "Acme", Title: "Staff Platform Engineer", Source: "LLM: llm_job_search"},
				{Company: "Bravo", Title: "Senior DevOps Engineer", Source: "RSS: We Work Remotely DevOps RSS"},
			},
		},
	}, false)

	rendered := session.Summary(true)

	for _, expected := range []string{
		"Filtered",
		"  duplicate (2)",
		"    LLM",
		"      llm_job_search (1)",
		"        - Acme - Staff Platform Engineer",
		"    RSS",
		"      We Work Remotely DevOps (1)",
		"        - Bravo - Senior DevOps Engineer",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("ReviewSession.Summary() = %q; want %q", rendered, expected)
		}
	}
}

func TestFormatFetchReviewSummaryDebugShowsLLMFilterBypassReasons(t *testing.T) {
	session := NewReviewSession(nil, []Job{{Company: "Acme", Title: "Staff Platform Engineer", Source: "RSS: We Work Remotely"}}, FetchSummary{
		LLMFilterBypass: map[string][]Job{
			"deterministic_complete": {
				{Company: "Acme", Title: "Staff Platform Engineer", Source: "RSS: We Work Remotely"},
			},
			"weak_identity": {
				{Company: "UNKNOWN", Title: "Backend Engineer", Source: "Site Search: LinkedIn"},
			},
		},
	}, false)

	rendered := session.Summary(true)

	for _, expected := range []string{
		"LLM Filter Bypass",
		"  deterministic_complete (1)",
		"    RSS",
		"      We Work Remotely (1)",
		"        - Acme - Staff Platform Engineer",
		"  weak_identity (1)",
		"    Site Search",
		"      LinkedIn (1)",
		"        - UNKNOWN - Backend Engineer",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("ReviewSession.Summary() = %q; want %q", rendered, expected)
		}
	}
}

func TestFormatExecutedSearchStatusIncludesFilteredAndRejectedCandidates(t *testing.T) {
	tests := []struct {
		accepted int
		filtered int
		rejected int
		want     string
	}{
		{accepted: 0, filtered: 0, rejected: 0, want: "executed; found 0 results"},
		{accepted: 4, filtered: 0, rejected: 0, want: "executed; found 4 results"},
		{accepted: 0, filtered: 2, rejected: 0, want: "executed; accepted 0 results; filtered 2; rejected 0"},
		{accepted: 3, filtered: 129, rejected: 4, want: "executed; accepted 3 results; filtered 129; rejected 4"},
	}

	for _, tt := range tests {
		got := formatExecutedSearchStatus(tt.accepted, tt.filtered, tt.rejected)
		if got != tt.want {
			t.Fatalf("formatExecutedSearchStatus(%d, %d, %d) = %q; want %q", tt.accepted, tt.filtered, tt.rejected, got, tt.want)
		}
	}
}
