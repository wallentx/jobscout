package fetcher

import (
	"strings"
	"testing"
)

func TestLLMWebSearchQueriesBuildsTargetedMatrix(t *testing.T) {
	criteria := &CriteriaConfig{}
	criteria.Filters.TitleRequires = []string{"Staff"}
	criteria.Filters.TitleIncludes = []string{"DevOps Engineer", "Platform Engineer"}

	got := llmWebSearchQueries(criteria, []string{"job-boards.greenhouse.io", "site:jobs.lever.co"}, 3)
	want := []string{
		"site:job-boards.greenhouse.io Staff DevOps Engineer",
		"site:jobs.lever.co Staff DevOps Engineer",
		"site:job-boards.greenhouse.io Staff Platform Engineer",
	}
	if len(got) != len(want) {
		t.Fatalf("llmWebSearchQueries() = %#v; want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("llmWebSearchQueries()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestBuildLLMWebSearchPromptIncludesProviderWebInstruction(t *testing.T) {
	criteria := &CriteriaConfig{}
	criteria.Candidate.City = "Seattle"
	criteria.Candidate.State = "WA"
	criteria.Filters.TitleIncludes = []string{"Software Engineer"}
	criteria.Filters.WorkSettings.Remote = true

	prompt, queries := buildLLMWebSearchPrompt(criteria, []string{"site:jobs.ashbyhq.com"})
	if len(queries) != 1 {
		t.Fatalf("buildLLMWebSearchPrompt() queries = %#v; want one query", queries)
	}
	for _, want := range []string{
		"Use provider-backed web search",
		"use it before deciding there are no matching jobs",
		"site:jobs.ashbyhq.com Software Engineer",
		"Allowed source domains",
		"jobs.ashbyhq.com",
		"company industry",
		"Seattle, WA",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("buildLLMWebSearchPrompt() = %q; want %q", prompt, want)
		}
	}
}

func TestLLMWebSearchDomainsNormalizesTargets(t *testing.T) {
	got := llmWebSearchDomains([]string{
		"site:job-boards.greenhouse.io",
		"site:careers-*.icims.com",
		"site:*.bamboohr.com/jobs",
		"site:job-boards.greenhouse.io",
	})
	want := []string{"job-boards.greenhouse.io", "icims.com", "bamboohr.com"}
	if len(got) != len(want) {
		t.Fatalf("llmWebSearchDomains(...) = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("llmWebSearchDomains(...)[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
