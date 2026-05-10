package llm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/domain"

	"github.com/tmc/langchaingo/llms"
)

type recordingJobFilterLLM struct {
	calls           int
	response        string
	responses       []string
	generationInfos []map[string]any
}

func (m *recordingJobFilterLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	m.calls++
	response := m.response
	if len(m.responses) > 0 {
		response = m.responses[0]
		m.responses = m.responses[1:]
	}
	if response == "" {
		response = `{
			"matches": true,
			"compensation_extracted": "$165k base",
			"remote_eligibility": "Remote",
			"why_it_matches": ["LLM reviewed ambiguous posting"]
		}`
	}
	var generationInfo map[string]any
	if len(m.generationInfos) > 0 {
		generationInfo = m.generationInfos[0]
		m.generationInfos = m.generationInfos[1:]
	}
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			Content:        response,
			GenerationInfo: generationInfo,
		}},
	}, nil
}

func (m *recordingJobFilterLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	m.calls++
	return `{"matches":true}`, nil
}

type promptAwareJobFilterLLM struct {
	calls int
}

func (m *promptAwareJobFilterLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	m.calls++
	promptText := fmt.Sprint(messages)
	response := `{
		"matches": true,
		"compensation_extracted": "$165k base",
		"remote_eligibility": "Remote",
		"why_it_matches": ["Single reviewed oversized posting"]
	}`
	if strings.Contains(promptText, "Return one result for every job id") {
		response = `{
			"results": {
				"0": {"matches": true, "compensation_extracted": "$165k base", "remote_eligibility": "Remote", "why_it_matches": ["Batch reviewed Acme"]},
				"1": {"matches": true, "compensation_extracted": "$166k base", "remote_eligibility": "Remote", "why_it_matches": ["Batch reviewed Beta"]},
				"2": {"matches": true, "compensation_extracted": "$167k base", "remote_eligibility": "Remote", "why_it_matches": ["Batch reviewed Gamma"]}
			}
		}`
	}
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			Content: response,
		}},
	}, nil
}

func (m *promptAwareJobFilterLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	m.calls++
	return `{"matches":true}`, nil
}

func TestSelectJobsForLLMFilteringOnlySelectsAmbiguousNonLLMJobs(t *testing.T) {
	criteria := jobFilterTestCriteria()
	jobs := []Job{
		jobFilterTestJob("LLM source", "LLM: generated search", "$190k base", "Remote", jobFilterUsableDescription()),
		jobFilterHighConfidenceDeterministicJob(),
		jobFilterTestJob("Unknown", "RSS: Remote Feed", "", "Remote", jobFilterUsableDescription()),
		jobFilterTestJob("Ambiguous", "RSS: Remote Feed", "", "", jobFilterUsableDescription()),
	}

	selection := selectJobsForLLMFiltering(jobs, criteria)

	if got, want := selection.indexes, []int{3}; !equalInts(got, want) {
		t.Fatalf("selectJobsForLLMFiltering(...).indexes = %#v, want %#v", got, want)
	}
	wantReasons := map[string]int{
		"llm_generated":          1,
		"deterministic_complete": 1,
		"weak_identity":          1,
		"needs_fit_review":       1,
	}
	for reason, want := range wantReasons {
		if got := selection.stats.Reasons[reason]; got != want {
			t.Fatalf("selection.stats.Reasons[%q] = %d, want %d; all reasons = %#v", reason, got, want, selection.stats.Reasons)
		}
	}
}

func TestFilterJobsWithLLMModelEvaluatesOnlySelectedJobs(t *testing.T) {
	criteria := jobFilterTestCriteria()
	jobs := []Job{
		jobFilterTestJob("LLM source", "LLM web: openai", "$190k base", "Remote", jobFilterUsableDescription()),
		jobFilterHighConfidenceDeterministicJob(),
		jobFilterTestJob("Unknown", "RSS: Remote Feed", "", "Remote", jobFilterUsableDescription()),
		jobFilterTestJob("Ambiguous", "Site Search: Example", "", "", jobFilterUsableDescription()),
	}
	selection := selectJobsForLLMFiltering(jobs, criteria)
	fakeLLM := &recordingJobFilterLLM{}

	filtered := filterJobsWithLLMModel(context.Background(), fakeLLM, criteria, jobs, selection)

	if got, want := fakeLLM.calls, 1; got != want {
		t.Fatalf("fake LLM calls = %d, want %d", got, want)
	}
	if got, want := len(filtered), len(jobs); got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
	if got, want := filtered[0].Compensation, jobs[0].Compensation; got != want {
		t.Fatalf("LLM-source job compensation = %q, want original %q", got, want)
	}
	if got, want := filtered[1].Compensation, jobs[1].Compensation; got != want {
		t.Fatalf("deterministic job compensation = %q, want original %q", got, want)
	}
	if got, want := filtered[2].Company, jobs[2].Company; got != want {
		t.Fatalf("weak-identity job company = %q, want original %q", got, want)
	}
	if got, want := filtered[3].Compensation, "$165k base"; got != want {
		t.Fatalf("ambiguous job compensation = %q, want %q", got, want)
	}
	if got, want := filtered[3].WhyMatches, []string{"LLM reviewed ambiguous posting"}; !equalStrings(got, want) {
		t.Fatalf("ambiguous job WhyMatches = %#v, want %#v", got, want)
	}
}

func TestFilterJobsWithLLMModelPreservesExistingFieldsWhenResponseOmitsThem(t *testing.T) {
	criteria := jobFilterTestCriteria()
	jobs := []Job{
		jobFilterTestJob("Ambiguous", "Site Search: Example", "$170k base", "Hybrid", jobFilterUsableDescription()),
	}
	selection := llmJobFilterSelection{indexes: []int{0}}
	fakeLLM := &recordingJobFilterLLM{
		response: `{
			"matches": true,
			"why_it_matches": ["LLM reviewed ambiguous posting"]
		}`,
	}

	filtered := filterJobsWithLLMModel(context.Background(), fakeLLM, criteria, jobs, selection)

	if got, want := filtered[0].Compensation, "$170k base"; got != want {
		t.Fatalf("Compensation = %q, want preserved %q", got, want)
	}
	if got, want := filtered[0].Remote, "Hybrid"; got != want {
		t.Fatalf("Remote = %q, want preserved %q", got, want)
	}
}

func TestFilterJobsWithLLMModelBatchesSameSourceJobs(t *testing.T) {
	criteria := jobFilterTestCriteria()
	jobs := []Job{
		jobFilterTestJob("Acme", "Site Search: Built In", "", "", jobFilterUsableDescription()),
		jobFilterTestJob("Beta", "Site Search: Built In", "", "", jobFilterUsableDescription()),
	}
	selection := selectJobsForLLMFiltering(jobs, criteria)
	fakeLLM := &recordingJobFilterLLM{
		response: `{
			"results": {
				"0": {
					"matches": true,
					"compensation_extracted": "$180k base",
					"remote_eligibility": "Remote",
					"why_it_matches": ["Batch reviewed Acme"]
				},
				"1": {
					"matches": false,
					"why_rejected": ["Batch rejected Beta"]
				}
			}
		}`,
	}

	filtered := filterJobsWithLLMModel(context.Background(), fakeLLM, criteria, jobs, selection)

	if got, want := fakeLLM.calls, 1; got != want {
		t.Fatalf("fake LLM calls = %d, want %d", got, want)
	}
	if got, want := len(filtered), 1; got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
	if got, want := filtered[0].Company, "Acme"; got != want {
		t.Fatalf("filtered[0].Company = %q, want %q", got, want)
	}
	if got, want := filtered[0].Compensation, "$180k base"; got != want {
		t.Fatalf("filtered[0].Compensation = %q, want %q", got, want)
	}
}

func TestFilterJobsWithLLMModelSplitsOversizedSameSourceBatches(t *testing.T) {
	criteria := jobFilterTestCriteria()
	largeDescription := strings.Repeat("Build production infrastructure for distributed systems and developer platforms. ", 500)
	jobs := []Job{
		jobFilterTestJob("Acme", "RSS: Remote Feed", "", "", largeDescription),
		jobFilterTestJob("Beta", "RSS: Remote Feed", "", "", largeDescription),
		jobFilterTestJob("Gamma", "RSS: Remote Feed", "", "", largeDescription),
	}
	selection := selectJobsForLLMFiltering(jobs, criteria)
	fakeLLM := &promptAwareJobFilterLLM{}

	filtered := filterJobsWithLLMModel(context.Background(), fakeLLM, criteria, jobs, selection)

	if got, want := fakeLLM.calls, 3; got != want {
		t.Fatalf("fake LLM calls = %d, want %d oversized jobs evaluated separately", got, want)
	}
	if got, want := len(filtered), len(jobs); got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
}

func TestFilterJobsWithLLMModelFallsBackWhenBatchIsIncomplete(t *testing.T) {
	criteria := jobFilterTestCriteria()
	jobs := []Job{
		jobFilterTestJob("Acme", "Site Search: Built In", "", "", jobFilterUsableDescription()),
		jobFilterTestJob("Beta", "Site Search: Built In", "", "", jobFilterUsableDescription()),
	}
	selection := selectJobsForLLMFiltering(jobs, criteria)
	fakeLLM := &recordingJobFilterLLM{
		responses: []string{
			`{
				"results": {
					"0": {
						"matches": true,
						"compensation_extracted": "$180k base",
						"remote_eligibility": "Remote",
						"why_it_matches": ["Batch reviewed Acme"]
					}
				}
			}`,
			`{"matches":true,"compensation_extracted":"$181k base","remote_eligibility":"Remote","why_it_matches":["Fallback kept Acme"]}`,
			`{"matches":true,"compensation_extracted":"$182k base","remote_eligibility":"Remote","why_it_matches":["Fallback kept Beta"]}`,
		},
	}

	filtered := filterJobsWithLLMModel(context.Background(), fakeLLM, criteria, jobs, selection)

	if got, want := fakeLLM.calls, 3; got != want {
		t.Fatalf("fake LLM calls = %d, want %d", got, want)
	}
	if got, want := len(filtered), len(jobs); got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
}

func TestFilterJobsWithLLMModelLogsAggregateTokenUsage(t *testing.T) {
	restoreDebug := ConfigureDebug(true, filepath.Join(t.TempDir(), "debug.log"))
	defer restoreDebug()

	criteria := jobFilterTestCriteria()
	jobs := []Job{
		jobFilterTestJob("Acme", "Site Search: Built In", "", "", jobFilterUsableDescription()),
		jobFilterTestJob("Beta", "Site Search: Built In", "", "", jobFilterUsableDescription()),
		jobFilterTestJob("Gamma", "RSS: Remote Feed", "", "", jobFilterUsableDescription()),
	}
	selection := selectJobsForLLMFiltering(jobs, criteria)
	fakeLLM := &recordingJobFilterLLM{
		responses: []string{
			`{
				"results": {
					"0": {
						"matches": true,
						"compensation_extracted": "$180k base",
						"remote_eligibility": "Remote",
						"why_it_matches": ["Batch reviewed Acme"]
					},
					"1": {
						"matches": false,
						"why_rejected": ["Batch rejected Beta"]
					}
				}
			}`,
			`{"matches":true,"compensation_extracted":"$181k base","remote_eligibility":"Remote","why_it_matches":["Single reviewed Gamma"]}`,
		},
		generationInfos: []map[string]any{
			{
				"PromptTokens":     10,
				"CompletionTokens": 2,
				"TotalTokens":      12,
				"CachedTokens":     1,
				"ReasoningTokens":  3,
				"ThinkingTokens":   4,
			},
			{
				"PromptTokens":     5,
				"CompletionTokens": 6,
				"TotalTokens":      11,
				"CachedTokens":     1,
				"ReasoningTokens":  2,
				"ThinkingTokens":   3,
			},
		},
	}

	filtered := filterJobsWithLLMModel(context.Background(), fakeLLM, criteria, jobs, selection)

	if got, want := len(filtered), 2; got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
	logBytes, err := os.ReadFile(llmDebug.path)
	if err != nil {
		t.Fatalf("ReadFile(debug log): %v", err)
	}
	logText := string(logBytes)
	for _, want := range []string{
		"job filtering: run summary single_calls=1 batch_calls=1 selected=3 evaluated=3 kept=2 dropped=1",
		"token_usage input=15 output=8 total=23 cached=2 reasoning=5 thinking=7",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("debug log missing %q:\n%s", want, logText)
		}
	}
}

func TestJobFilterPersistedDiffShowsChangedFields(t *testing.T) {
	before := jobFilterTestJob("Acme", "Site Search: Built In", "", "", jobFilterUsableDescription())
	after := before
	after.Compensation = "$180k base"
	after.Remote = "Remote"
	after.WhyMatches = []string{"LLM reviewed ambiguous posting"}

	got := jobFilterPersistedDiff(before, after)

	for _, want := range []string{
		`compensation: "" -> "$180k base"`,
		`remote: "" -> "Remote"`,
		`why_matches: 0 -> 1`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("jobFilterPersistedDiff(...) = %q; want to contain %q", got, want)
		}
	}
}

func TestJobFilterRejectedDiffShowsExtractedFields(t *testing.T) {
	job := jobFilterTestJob("Acme", "Site Search: Built In", "", "", jobFilterUsableDescription())
	result := &LLMEvaluationResult{
		CompensationExtracted: "$120k base",
		RemoteEligibility:     "UK Remote",
		WhyRejected:           []string{"Below salary floor", "Not US remote"},
	}

	got := jobFilterRejectedDiff(job, result)

	for _, want := range []string{
		`compensation_extracted: "" -> "$120k base"`,
		`remote_eligibility: "" -> "UK Remote"`,
		`why_rejected: 2`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("jobFilterRejectedDiff(...) = %q; want to contain %q", got, want)
		}
	}
}

func jobFilterTestCriteria() *CriteriaConfig {
	var criteria CriteriaConfig
	criteria.Filters.MinBaseUSD = 150000
	criteria.Filters.WorkSettings.Remote = true
	return &criteria
}

func jobFilterTestJob(company, source, compensation, remote, description string) Job {
	return Job{
		Company:      company,
		Title:        "Senior Platform Engineer",
		Remote:       remote,
		Compensation: compensation,
		Source:       source,
		ApplyURL:     "https://jobs.example/apply",
		Description:  description,
	}
}

func jobFilterHighConfidenceDeterministicJob() Job {
	job := jobFilterTestJob("Deterministic fit", "RSS: Remote Feed", "$185k base", "Remote", jobFilterUsableDescription())
	job.CompanyWebsite = "https://deterministic.example"
	job.CompanySummary = "Deterministic fit builds deployment automation software for infrastructure teams operating reliable distributed systems."
	job.CompanyIndustry = "Developer Tools"
	job.SetCompanyIdentityEvidence("website", domain.JobIdentityEvidence{Value: job.CompanyWebsite, Confidence: "high"})
	job.SetCompanyIdentityEvidence("summary", domain.JobIdentityEvidence{Value: job.CompanySummary, Confidence: "high"})
	job.SetCompanyIdentityEvidence("industry", domain.JobIdentityEvidence{Value: job.CompanyIndustry, Confidence: "high"})
	return job
}

func jobFilterUsableDescription() string {
	return "Build reliable infrastructure for distributed systems while collaborating with product teams on incident response, observability, and deployment automation. " +
		strings.Repeat("This posting includes enough detail for fit review. ", 3)
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
