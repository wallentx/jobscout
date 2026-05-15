package llm

import (
	"strings"
	"testing"
)

func TestParseLLMJobsJSONStripsMarkdownFence(t *testing.T) {
	got, err := parseLLMJobsJSON("```json\n[{\"company\":\"Acme\",\"title\":\"Software Engineer\",\"apply_url\":\"https://example.com/jobs/1\"}]\n```")
	if err != nil {
		t.Fatalf("parseLLMJobsJSON(...) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("parseLLMJobsJSON(...) len = %d, want 1", len(got))
	}
	if got[0].Company != "Acme" {
		t.Fatalf("parseLLMJobsJSON(...)[0].Company = %q, want %q", got[0].Company, "Acme")
	}
}

func TestParseLLMJobsJSONExtractsArrayFromText(t *testing.T) {
	got, err := parseLLMJobsJSON("Here are the matching jobs:\n[{\"company\":\"Acme\",\"title\":\"Software Engineer\",\"apply_url\":\"https://example.com/jobs/1\"}]\nGrounding links omitted.")
	if err != nil {
		t.Fatalf("parseLLMJobsJSON(...) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("parseLLMJobsJSON(...) len = %d, want 1", len(got))
	}
	if got[0].Title != "Software Engineer" {
		t.Fatalf("parseLLMJobsJSON(...)[0].Title = %q, want %q", got[0].Title, "Software Engineer")
	}
}

func TestParseLLMJobsJSONAcceptsJobsObject(t *testing.T) {
	got, err := parseLLMJobsJSON(`{"jobs":[{"company":"Acme","title":"Software Engineer","apply_url":"https://example.com/jobs/1"}],"count":1}`)
	if err != nil {
		t.Fatalf("parseLLMJobsJSON(...) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("parseLLMJobsJSON(...) len = %d, want 1", len(got))
	}
	if got[0].Company != "Acme" {
		t.Fatalf("parseLLMJobsJSON(...)[0].Company = %q, want Acme", got[0].Company)
	}
}

func TestParseLLMJobsJSONAcceptsSingleJobObject(t *testing.T) {
	got, err := parseLLMJobsJSON(`{"company":"Acme","title":"Software Engineer","apply_url":"https://example.com/jobs/1"}`)
	if err != nil {
		t.Fatalf("parseLLMJobsJSON(...) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("parseLLMJobsJSON(...) len = %d, want 1", len(got))
	}
	if got[0].Title != "Software Engineer" {
		t.Fatalf("parseLLMJobsJSON(...)[0].Title = %q, want Software Engineer", got[0].Title)
	}
}

func TestParseLLMJobsJSONEmptyResponse(t *testing.T) {
	if _, err := parseLLMJobsJSON("   "); err == nil {
		t.Fatal("parseLLMJobsJSON(empty) error = nil, want error")
	}
}

func TestOpenAIResponsesEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{name: "default", endpoint: "", want: defaultOpenAIResponsesEndpoint},
		{name: "base", endpoint: "https://api.example.test", want: "https://api.example.test/v1/responses"},
		{name: "v1", endpoint: "https://api.example.test/v1", want: "https://api.example.test/v1/responses"},
		{name: "responses", endpoint: "https://api.example.test/v1/responses", want: "https://api.example.test/v1/responses"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := openAIResponsesEndpoint(LLMProviderConfig{Endpoint: tt.endpoint})
			if got != tt.want {
				t.Fatalf("openAIResponsesEndpoint(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestOpenAIResponseOutputText(t *testing.T) {
	resp := openAIWebSearchResponse{
		Output: []openAIWebSearchResponseItem{
			{
				Type: "web_search_call",
				Action: openAIWebSearchAction{
					Sources: []openAIWebSearchSource{{URL: "https://example.com/job"}},
				},
			},
			{
				Type: "message",
				Content: []openAIWebSearchResponseContent{
					{Type: "output_text", Text: "[{\"company\":\"Acme\"}]"},
				},
			},
		},
	}

	if got, want := openAIResponseOutputText(resp), "[{\"company\":\"Acme\"}]"; got != want {
		t.Fatalf("openAIResponseOutputText(...) = %q, want %q", got, want)
	}
	if got, want := countOpenAIWebSearchCalls(resp), 1; got != want {
		t.Fatalf("countOpenAIWebSearchCalls(...) = %d, want %d", got, want)
	}
	if got, want := countOpenAIWebSearchSources(resp), 1; got != want {
		t.Fatalf("countOpenAIWebSearchSources(...) = %d, want %d", got, want)
	}
}

func TestOpenAIWebSearchBatchesGroupsByTitleWithAllowedDomains(t *testing.T) {
	prompt := strings.Join([]string{
		"Search only these public-web queries:",
		"- site:jobs.lever.co Software Engineer",
		"- site:job-boards.greenhouse.io Software Engineer",
		"- site:jobs.ashbyhq.com Software Developer",
		"",
		"Allowed source domains for providers that support domain filters:",
		"- jobs.lever.co",
		"- job-boards.greenhouse.io",
		"- jobs.ashbyhq.com",
		"",
		"Only include roles that match the criteria below.",
	}, "\n")

	got := openAIWebSearchBatches(prompt)
	if len(got) != 2 {
		t.Fatalf("openAIWebSearchBatches(...) len = %d, want 2: %#v", len(got), got)
	}
	if got[0].Query != "Software Engineer" {
		t.Fatalf("openAIWebSearchBatches(...)[0].Query = %q, want Software Engineer", got[0].Query)
	}
	if got[1].Query != "Software Developer" {
		t.Fatalf("openAIWebSearchBatches(...)[1].Query = %q, want Software Developer", got[1].Query)
	}
	if len(got[0].Domains) != 3 {
		t.Fatalf("openAIWebSearchBatches(...)[0].Domains = %#v, want 3 domains", got[0].Domains)
	}
}

func TestOpenAIWebSearchTargetedBatchesUsesOriginalSiteQueries(t *testing.T) {
	prompt := strings.Join([]string{
		"Search only these public-web queries:",
		"- site:jobs.lever.co Software Engineer",
		"- site:careers-*.icims.com Software Engineer",
		"- site:*.bamboohr.com/jobs Software Developer",
		"",
		"Only include roles that match the criteria below.",
	}, "\n")

	got := openAIWebSearchTargetedBatches(prompt)
	if len(got) != 3 {
		t.Fatalf("openAIWebSearchTargetedBatches(...) len = %d, want 3: %#v", len(got), got)
	}
	if got[0].Query != "site:jobs.lever.co Software Engineer" {
		t.Fatalf("openAIWebSearchTargetedBatches(...)[0].Query = %q, want original site query", got[0].Query)
	}
	if len(got[0].Domains) != 1 || got[0].Domains[0] != "jobs.lever.co" {
		t.Fatalf("openAIWebSearchTargetedBatches(...)[0].Domains = %#v, want jobs.lever.co", got[0].Domains)
	}
	if len(got[1].Domains) != 1 || got[1].Domains[0] != "icims.com" {
		t.Fatalf("openAIWebSearchTargetedBatches(...)[1].Domains = %#v, want icims.com", got[1].Domains)
	}
	if len(got[2].Domains) != 1 || got[2].Domains[0] != "bamboohr.com" {
		t.Fatalf("openAIWebSearchTargetedBatches(...)[2].Domains = %#v, want bamboohr.com", got[2].Domains)
	}
}

func TestOpenAIWebSearchBatchPromptUsesSingleQuery(t *testing.T) {
	prompt := "Search only these public-web queries:\n- site:jobs.lever.co Software Engineer\n\nOnly include roles that match the criteria below.\nLocation: Austin"
	got := openAIWebSearchBatchPrompt(prompt, openAIWebSearchBatch{
		Query:   "Software Engineer",
		Domains: []string{"jobs.lever.co"},
	})
	for _, want := range []string{
		`"Software Engineer"`,
		"jobs.lever.co",
		"Location: Austin",
		"do not invent missing values",
		"Return only a valid JSON array",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("openAIWebSearchBatchPrompt(...) = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "Omit the result if either field is unavailable") {
		t.Fatalf("openAIWebSearchBatchPrompt(...) = %q, want prompt to keep partial direct job results", got)
	}
}
