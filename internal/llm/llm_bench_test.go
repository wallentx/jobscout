package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
	"gopkg.in/yaml.v3"
)

var errBenchmarkGeneration = errors.New("benchmark generation failed")

type errorLLM struct {
	err error
}

func (m errorLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	return nil, m.err
}

func (m errorLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", m.err
}

func TestLoadLLMBenchmarkCasesFromEmbeddedFixtures(t *testing.T) {
	cases, err := loadLLMBenchmarkCases()
	if err != nil {
		t.Fatalf("loadLLMBenchmarkCases() error = %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("loadLLMBenchmarkCases() returned no cases; want embedded benchcases")
	}

	seenTasks := make(map[string]bool, len(cases))
	for _, benchCase := range cases {
		if strings.TrimSpace(benchCase.ID) == "" {
			t.Fatalf("loadLLMBenchmarkCases() included case with empty ID: %#v", benchCase)
		}
		if strings.TrimSpace(benchCase.Task) == "" {
			t.Fatalf("loadLLMBenchmarkCases() included case %q with empty task", benchCase.ID)
		}
		if len(benchCase.Input) == 0 {
			t.Fatalf("loadLLMBenchmarkCases() included case %q with empty input", benchCase.ID)
		}
		if !json.Valid(benchCase.Input) {
			t.Fatalf("loadLLMBenchmarkCases() case %q input is not valid JSON: %s", benchCase.ID, string(benchCase.Input))
		}
		seenTasks[benchCase.Task] = true
	}

	for _, task := range []string{
		"llm_job_search",
		"llm_company_health",
		"job_identity",
		"llm_job_filtering",
		"resume_to_criteria",
	} {
		if !seenTasks[task] {
			t.Fatalf("loadLLMBenchmarkCases() tasks = %#v; want task %q", benchmarkTaskNames(seenTasks), task)
		}
	}
}

func TestValidateLLMBenchmarkCaseRequiredFields(t *testing.T) {
	validInput := json.RawMessage(`{"value":true}`)
	tests := []struct {
		name      string
		benchCase llmBenchmarkCase
		wantErr   string
	}{
		{
			name: "valid",
			benchCase: llmBenchmarkCase{
				ID:      "case_v1",
				Task:    "job_filter",
				Version: 1,
				Input:   validInput,
			},
		},
		{
			name: "missing ID",
			benchCase: llmBenchmarkCase{
				Task:    "job_filter",
				Version: 1,
				Input:   validInput,
			},
			wantErr: "id is required",
		},
		{
			name: "missing task",
			benchCase: llmBenchmarkCase{
				ID:      "case_v1",
				Version: 1,
				Input:   validInput,
			},
			wantErr: "task is required",
		},
		{
			name: "missing positive version",
			benchCase: llmBenchmarkCase{
				ID:    "case_v1",
				Task:  "job_filter",
				Input: validInput,
			},
			wantErr: "version must be greater than zero",
		},
		{
			name: "missing input",
			benchCase: llmBenchmarkCase{
				ID:      "case_v1",
				Task:    "job_filter",
				Version: 1,
			},
			wantErr: "input is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLLMBenchmarkCase(tt.benchCase)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateLLMBenchmarkCase(%#v) error = %v, want nil", tt.benchCase, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateLLMBenchmarkCase(%#v) error = %v, want containing %q", tt.benchCase, err, tt.wantErr)
			}
		})
	}
}

func TestBenchmarkChecksEnumValuesDecodeJSONAndYAML(t *testing.T) {
	var jsonChecks benchmarkChecks
	if err := json.Unmarshal([]byte(`{
		"enum_values": {
			"risk_level": ["low", "medium", "high"],
			"results.accept_remote_engineer.remote_eligibility": ["Remote", "Onsite"]
		}
	}`), &jsonChecks); err != nil {
		t.Fatalf("json.Unmarshal(enum_values) error = %v", err)
	}
	if got := len(jsonChecks.EnumValues["risk_level"]); got != 3 {
		t.Fatalf("json enum_values[risk_level] len = %d, want 3", got)
	}

	var yamlChecks benchmarkChecks
	if err := yaml.Unmarshal([]byte(`
enum_values:
  risk_level:
    - low
    - medium
  results.accept_remote_engineer.remote_eligibility:
    - Remote
    - Onsite
`), &yamlChecks); err != nil {
		t.Fatalf("yaml.Unmarshal(enum_values) error = %v", err)
	}
	if got := len(yamlChecks.EnumValues["results.accept_remote_engineer.remote_eligibility"]); got != 2 {
		t.Fatalf("yaml enum_values nested len = %d, want 2", got)
	}
}

func TestBenchmarkChecksNumericBoundsDecodeJSONAndYAML(t *testing.T) {
	var jsonChecks benchmarkChecks
	if err := json.Unmarshal([]byte(`{
		"numeric_maximums": {
			"salary.max_base_usd": 150000
		},
		"numeric_ranges": {
			"candidate.years_of_experience": {"min": 1, "max": 3}
		}
	}`), &jsonChecks); err != nil {
		t.Fatalf("json.Unmarshal(numeric bounds) error = %v", err)
	}
	if got := jsonChecks.NumericMaximums["salary.max_base_usd"]; got != 150000 {
		t.Fatalf("json numeric_maximums[salary.max_base_usd] = %v, want 150000", got)
	}
	if got := jsonChecks.NumericRanges["candidate.years_of_experience"].Min; got == nil || *got != 1 {
		t.Fatalf("json numeric_ranges[candidate.years_of_experience].Min = %v, want 1", got)
	}
	if got := jsonChecks.NumericRanges["candidate.years_of_experience"].Max; got == nil || *got != 3 {
		t.Fatalf("json numeric_ranges[candidate.years_of_experience].Max = %v, want 3", got)
	}

	var yamlChecks benchmarkChecks
	if err := yaml.Unmarshal([]byte(`
numeric_maximums:
  salary.max_base_usd: 150000
numeric_ranges:
  candidate.years_of_experience:
    min: 1
    max: 3
`), &yamlChecks); err != nil {
		t.Fatalf("yaml.Unmarshal(numeric bounds) error = %v", err)
	}
	if got := yamlChecks.NumericMaximums["salary.max_base_usd"]; got != 150000 {
		t.Fatalf("yaml numeric_maximums[salary.max_base_usd] = %v, want 150000", got)
	}
	if got := yamlChecks.NumericRanges["candidate.years_of_experience"].Min; got == nil || *got != 1 {
		t.Fatalf("yaml numeric_ranges[candidate.years_of_experience].Min = %v, want 1", got)
	}
	if got := yamlChecks.NumericRanges["candidate.years_of_experience"].Max; got == nil || *got != 3 {
		t.Fatalf("yaml numeric_ranges[candidate.years_of_experience].Max = %v, want 3", got)
	}
}

func TestScoreBenchmarkOutputCapsHallucinationPatterns(t *testing.T) {
	record := llmBenchmarkRunRecord{
		Task:           "llm_job_search",
		SpeedScore:     100,
		CostScore:      100,
		StabilityScore: 100,
	}
	checks := benchmarkChecks{
		JSONRequired: true,
		RequiredFields: []string{
			"jobs",
			"count",
		},
		ExpectedValues: map[string]any{
			"count": float64(1),
		},
		HallucinationPatterns: []string{
			"https://example.com",
			"hypothetical",
		},
	}
	output := `{"jobs":[{"company":"Example Apps","apply_url":"https://example.com/jobs/123","description":"A hypothetical job."}],"count":1}`

	scoreBenchmarkOutput(&record, checks, output)

	if record.ScoreCap != 50 {
		t.Fatalf("ScoreCap = %d, want 50 for hallucination pattern match", record.ScoreCap)
	}
	if record.FinalScore > 50 {
		t.Fatalf("FinalScore = %.1f, want capped at 50", record.FinalScore)
	}
	if got, ok := record.Details["hallucination_patterns_matched"]; !ok || got != 2 {
		t.Fatalf("hallucination_patterns_matched = %#v, %t; want 2, true", got, ok)
	}
}

func TestLoadLLMBenchmarkCasesIncludesVariedResumeFormats(t *testing.T) {
	cases, err := loadLLMBenchmarkCases()
	if err != nil {
		t.Fatalf("loadLLMBenchmarkCases() error = %v", err)
	}
	seen := make(map[string]bool, len(cases))
	for _, benchCase := range cases {
		seen[benchCase.ID] = true
	}
	for _, id := range []string{
		"resume_to_criteria_software_developer_v1",
		"resume_to_criteria_frontend_developer_v1",
		"resume_to_criteria_devops_table_v1",
		"resume_to_criteria_career_changer_v1",
		"resume_to_criteria_contract_history_v1",
	} {
		if !seen[id] {
			t.Fatalf("loadLLMBenchmarkCases() missing resume fixture %q", id)
		}
	}
}

func TestParseBenchmarkCLIOptionsRejectsAllModelsWithModel(t *testing.T) {
	_, err := parseBenchmarkCLIOptions([]string{"--all-models", "--model", "example-model"})
	if err == nil || !strings.Contains(err.Error(), "--all-models cannot be combined with --model") {
		t.Fatalf("parseBenchmarkCLIOptions(--all-models --model example-model) error = %v, want conflict", err)
	}
}

func TestApplyBenchmarkModelOverridesClearsLegacyTopLevelAuth(t *testing.T) {
	appCfg := defaultAppConfig()
	appCfg.LLM.Provider = "gemini"
	appCfg.LLM.Auth = LLMAuthConfig{
		Mode:    llmAuthModeCommand,
		EnvVar:  "GEMINI_API_KEY",
		Command: "cat ~/.ssh/tokens/ai-studio-gemini",
	}
	appCfg.LLM.Providers = defaultLLMProviders()

	applyBenchmarkModelOverrides(&appCfg, benchmarkCLIOptions{Provider: "openai"})

	if appCfg.LLM.Provider != "openai" {
		t.Fatalf("applyBenchmarkModelOverrides(...).LLM.Provider = %q, want openai", appCfg.LLM.Provider)
	}
	if appCfg.LLM.Auth.EnvVar != "OPENAI_API_KEY" {
		t.Fatalf("applyBenchmarkModelOverrides(...).LLM.Auth.EnvVar = %q, want OPENAI_API_KEY", appCfg.LLM.Auth.EnvVar)
	}
	if appCfg.LLM.Auth.Command != "" {
		t.Fatalf("applyBenchmarkModelOverrides(...).LLM.Auth.Command = %q, want empty", appCfg.LLM.Auth.Command)
	}
	if appCfg.LLM.Providers["openai"].Auth.EnvVar != "OPENAI_API_KEY" {
		t.Fatalf("applyBenchmarkModelOverrides(...).Providers[openai].Auth.EnvVar = %q, want OPENAI_API_KEY", appCfg.LLM.Providers["openai"].Auth.EnvVar)
	}
}

func TestFilterBenchmarkModelListRemovesManualOptionAndDuplicates(t *testing.T) {
	got := filterBenchmarkModelList([]string{
		" example-model ",
		manualModelOption,
		"example-model",
		"another-model",
		"",
	})

	if len(got) != 2 {
		t.Fatalf("filterBenchmarkModelList(...) len = %d, want 2: %#v", len(got), got)
	}
	for _, model := range got {
		if model == manualModelOption || strings.TrimSpace(model) == "" {
			t.Fatalf("filterBenchmarkModelList(...) included invalid model %q in %#v", model, got)
		}
	}
}

func TestFilterBenchmarkModelListForProviderRemovesBlockedOpenAIModels(t *testing.T) {
	got := filterBenchmarkModelListForProvider("openai", []string{
		"gpt-4.1",
		"gpt-4o",
		"gpt-4o-2024-11-20",
		"gpt-4.1-nano",
		"gpt-4.1-nano-2025-04-14",
		"gpt-4.5-preview",
		"gpt-4-turbo",
		"gpt-3.5-turbo",
		"chat-latest",
		"gpt-5-chat-latest",
		"o1",
		"o1-pro",
		"o1-pro-2025-03-19",
		"o1-2024-12-17",
		"o3",
		"o3-2025-04-16",
		"o3-pro",
		"o3-pro-2025-06-10",
		"o3-mini",
		"o4-mini",
	})

	for _, model := range []string{
		"gpt-4o-2024-11-20",
		"gpt-4.1-nano",
		"gpt-4.1-nano-2025-04-14",
		"gpt-4.5-preview",
		"gpt-4-turbo",
		"gpt-3.5-turbo",
		"chat-latest",
		"gpt-5-chat-latest",
		"o1",
		"o1-pro",
		"o1-pro-2025-03-19",
		"o1-2024-12-17",
		"o3-2025-04-16",
		"o3-pro",
		"o3-pro-2025-06-10",
		"o3-mini",
		"o4-mini",
	} {
		if stringSliceContains(got, model) {
			t.Fatalf("filterBenchmarkModelListForProvider(openai, ...) included blocked model %q in %#v", model, got)
		}
	}
	for _, model := range []string{"gpt-4.1", "gpt-4o", "o3"} {
		if !stringSliceContains(got, model) {
			t.Fatalf("filterBenchmarkModelListForProvider(openai, ...) omitted usable model %q from %#v", model, got)
		}
	}
}

func TestFilterBenchmarkModelListForProviderRemovesGeminiAliases(t *testing.T) {
	got := filterBenchmarkModelListForProvider("gemini", []string{
		"gemini-2.5-flash",
		"gemini-flash-latest",
		"gemini-flash-lite-latest",
		"gemini-pro-latest",
		"gemini-3-pro-preview",
		"gemini-3.1-flash-lite-preview",
		"gemini-3.1-flash-lite",
		"gemini-3.1-pro-preview",
		"gemini-3-flash-preview",
	})

	for _, model := range []string{"gemini-2.5-flash", "gemini-flash-latest", "gemini-flash-lite-latest", "gemini-pro-latest", "gemini-3-pro-preview", "gemini-3.1-flash-lite-preview"} {
		if stringSliceContains(got, model) {
			t.Fatalf("filterBenchmarkModelListForProvider(gemini, ...) included deprecated model %q in %#v", model, got)
		}
	}
	for _, model := range []string{"gemini-3.1-flash-lite", "gemini-3.1-pro-preview", "gemini-3-flash-preview"} {
		if !stringSliceContains(got, model) {
			t.Fatalf("filterBenchmarkModelListForProvider(gemini, ...) omitted runnable model %q from %#v", model, got)
		}
	}
}

func TestScoreBenchmarkOutputAppliesInvalidJSONCap(t *testing.T) {
	record := benchmarkRecordWithPerfectRunScores("job_filter")
	checks := benchmarkChecks{
		JSONRequired:   true,
		RequiredFields: []string{"matches"},
	}

	scoreBenchmarkOutput(&record, checks, `{"matches":`)

	if record.JSONValid {
		t.Fatalf("scoreBenchmarkOutput(invalid JSON).JSONValid = true, want false")
	}
	if record.ScoreCap != 40 {
		t.Fatalf("scoreBenchmarkOutput(invalid JSON).ScoreCap = %d, want 40", record.ScoreCap)
	}
	if record.FinalScore > 40 {
		t.Fatalf("scoreBenchmarkOutput(invalid JSON).FinalScore = %v, want <= 40", record.FinalScore)
	}
}

func TestFormatBenchmarkRecordSummaryUsesColorAndShortLines(t *testing.T) {
	summary := formatBenchmarkRecordSummary(llmBenchmarkRunRecord{
		Model:                 "gpt-4o-mini",
		Task:                  "autonomous_job_search",
		CaseID:                "llm_job_search_example_apps_v1",
		LatencyMS:             1275,
		JSONValid:             true,
		RequiredFieldsPresent: true,
		FinalScore:            91.25,
	})

	if !strings.Contains(summary, "\x1b[") {
		t.Fatalf("formatBenchmarkRecordSummary(...) missing ANSI color styling:\n%s", summary)
	}
	if !strings.Contains(summary, "llm_job_search") {
		t.Fatalf("formatBenchmarkRecordSummary(...) did not normalize task name:\n%s", summary)
	}
	lines := strings.Split(strings.TrimRight(summary, "\n"), "\n")
	if len(lines) < 4 {
		t.Fatalf("formatBenchmarkRecordSummary(...) lines = %d, want at least 4:\n%s", len(lines), summary)
	}
	for _, line := range lines {
		if len(line) > 120 {
			t.Fatalf("formatBenchmarkRecordSummary(...) line is too long (%d): %q", len(line), line)
		}
	}
}

func TestRunLLMBenchmarkCaseMarksGenerationFailureIncomplete(t *testing.T) {
	record := runLLMBenchmarkCase(
		context.Background(),
		errorLLM{err: errBenchmarkGeneration},
		"test",
		"test-model",
		llmBenchmarkCase{
			ID:      "case_v1",
			Task:    "job_filter",
			Version: 1,
			Input:   json.RawMessage(`{"value":true}`),
		},
	)

	if record.Error == "" {
		t.Fatalf("runLLMBenchmarkCase(error LLM).Error = empty, want error")
	}
	if record.JSONValid {
		t.Fatalf("runLLMBenchmarkCase(error LLM).JSONValid = true, want false")
	}
	if record.RequiredFieldsPresent {
		t.Fatalf("runLLMBenchmarkCase(error LLM).RequiredFieldsPresent = true, want false")
	}
}

func TestExecuteBenchmarkTaskSupportsJobFilterBatch(t *testing.T) {
	benchCase := llmBenchmarkCase{
		ID:      "job_filter_batch_same_source_v1",
		Task:    "llm_job_filtering",
		Version: 1,
		Input: json.RawMessage(`{
			"criteria": {
				"filters": {
					"title_includes": ["Software Engineer"],
					"title_excludes": ["Manager"],
					"work_settings": {"remote": true},
					"min_base_usd": 100000
				}
			},
			"source": "RSS: Example Feed",
			"jobs": [
				{
					"id": "accept_remote_engineer",
					"job": {
						"company": "Example Apps",
						"title": "Software Engineer",
						"remote": "Remote - United States",
						"compensation": "$120,000 base",
						"description": "Build APIs for customer-facing products."
					}
				},
				{
					"id": "reject_manager",
					"job": {
						"company": "Example Consulting",
						"title": "Engineering Manager",
						"remote": "Onsite",
						"compensation": "$90,000 base",
						"description": "Manage a local onsite team."
					}
				}
			]
		}`),
	}
	llm := fakeContentLLM{content: `{
		"results": {
			"accept_remote_engineer": {
				"matches": true,
				"compensation_extracted": "$120,000 base",
				"remote_eligibility": "Remote",
				"why_it_matches": ["software engineer", "remote"]
			},
			"reject_manager": {
				"matches": false,
				"compensation_extracted": "$90,000 base",
				"remote_eligibility": "Onsite",
				"why_it_matches": ["manager", "onsite"]
			}
		}
	}`}

	output, _, err := executeBenchmarkTask(context.Background(), llm, benchCase)
	if err != nil {
		t.Fatalf("executeBenchmarkTask(llm_job_filtering batch) error = %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("executeBenchmarkTask(llm_job_filtering batch) output is invalid JSON: %v\n%s", err, output)
	}
	if got, ok := benchmarkValueAtPath(parsed, "results.accept_remote_engineer.matches"); !ok || got != true {
		t.Fatalf("results.accept_remote_engineer.matches = %#v, %t; want true, true", got, ok)
	}
	if got, ok := benchmarkValueAtPath(parsed, "results.reject_manager.matches"); !ok || got != false {
		t.Fatalf("results.reject_manager.matches = %#v, %t; want false, true", got, ok)
	}
}

func TestExecuteBenchmarkTaskSupportsJobSearch(t *testing.T) {
	benchCase := llmBenchmarkCase{
		ID:      "llm_job_search_example_apps_v1",
		Task:    "llm_job_search",
		Version: 1,
		Input: json.RawMessage(`{
			"prompt": "Find one remote Software Developer job at Example Apps."
		}`),
	}
	llm := fakeContentLLM{
		content: `[{
			"company": "Example Apps",
			"title": "Software Developer",
			"remote": "Remote - United States",
			"compensation": "$120,000 base",
			"apply_url": "https://example.com/jobs/software-developer",
			"description": "Build APIs and database-backed services.",
			"why_matches": ["software developer", "remote", "API work"]
		}]`,
		generationInfo: map[string]any{
			"PromptTokens":     111,
			"CompletionTokens": 44,
			"TotalTokens":      155,
		},
	}

	output, usage, err := executeBenchmarkTask(context.Background(), llm, benchCase)
	if err != nil {
		t.Fatalf("executeBenchmarkTask(llm_job_search) error = %v", err)
	}
	if usage.TotalTokens == nil || *usage.TotalTokens != 155 {
		t.Fatalf("executeBenchmarkTask(llm_job_search) usage.TotalTokens = %v, want 155", intPtrValue(usage.TotalTokens))
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("executeBenchmarkTask(llm_job_search) output is invalid JSON: %v\n%s", err, output)
	}
	if got, ok := benchmarkValueAtPath(parsed, "count"); !ok || got != float64(1) {
		t.Fatalf("count = %#v, %t; want 1, true", got, ok)
	}
	jobs, ok := parsed["jobs"].([]any)
	if !ok || len(jobs) != 1 {
		t.Fatalf("jobs = %#v; want one job", parsed["jobs"])
	}
}

func TestExecuteBenchmarkTaskSupportsJobIdentity(t *testing.T) {
	benchCase := llmBenchmarkCase{
		ID:      "job_identity_example_apps_v1",
		Task:    "job_identity",
		Version: 1,
		Input: json.RawMessage(`{
			"job": {
				"company": "Example Apps",
				"title": "Staff Platform Engineer",
				"source": "Example Careers",
				"apply_url": "https://jobs.example.com/example-apps/staff-platform-engineer"
			},
			"page": {
				"url": "https://jobs.example.com/example-apps/staff-platform-engineer",
				"text": "Example Apps builds workflow automation software for support teams. Learn more at https://www.exampleapps.test. This Staff Platform Engineer role works on reliability for the Example Apps product."
			}
		}`),
		Checks: benchmarkChecks{
			JSONRequired: true,
			RequiredFields: []string{
				"company_website",
				"company_summary",
				"company_industry",
				"website_confidence",
				"summary_confidence",
				"industry_confidence",
				"industry_provisional",
			},
			ExpectedValues: map[string]any{
				"company_website":      "https://www.exampleapps.test",
				"company_industry":     "Workflow Automation",
				"website_confidence":   "high",
				"industry_provisional": false,
			},
			ExpectedContains: map[string][]any{
				"company_summary": {"support teams"},
			},
			EnumValues: map[string][]any{
				"website_confidence":  {"high", "medium", "low", ""},
				"summary_confidence":  {"high", "medium", "low", ""},
				"industry_confidence": {"high", "medium", "low", ""},
			},
			GroundingRules: []string{"Example Apps", "support teams", "workflow automation"},
		},
	}
	llm := fakeContentLLM{
		content: `{
			"company_website": "https://www.exampleapps.test",
			"company_summary": "Example Apps builds workflow automation software for support teams.",
			"company_industry": "Workflow Automation",
			"website_confidence": "high",
			"summary_confidence": "high",
			"industry_confidence": "medium",
			"industry_provisional": false,
			"company_website_reason": "The supplied page text names the website.",
			"company_summary_reason": "The supplied page describes the product and customer audience.",
			"company_industry_reason": "The industry is inferred from the workflow automation product description."
		}`,
		generationInfo: map[string]any{
			"PromptTokens":     160,
			"CompletionTokens": 45,
			"TotalTokens":      205,
		},
	}

	record := runLLMBenchmarkCase(context.Background(), llm, "test", "fake-model", benchCase)
	if record.Error != "" {
		t.Fatalf("runLLMBenchmarkCase(job_identity).Error = %q, want empty", record.Error)
	}
	if !record.JSONValid {
		t.Fatalf("runLLMBenchmarkCase(job_identity).JSONValid = false, want true")
	}
	if !record.RequiredFieldsPresent {
		t.Fatalf("runLLMBenchmarkCase(job_identity).RequiredFieldsPresent = false, want true")
	}
	if record.AccuracyScore != 100 {
		t.Fatalf("runLLMBenchmarkCase(job_identity).AccuracyScore = %d, want 100", record.AccuracyScore)
	}
	if record.GroundingScore != 100 {
		t.Fatalf("runLLMBenchmarkCase(job_identity).GroundingScore = %d, want 100", record.GroundingScore)
	}
	if record.InputTokens == nil || *record.InputTokens != 160 {
		t.Fatalf("runLLMBenchmarkCase(job_identity).InputTokens = %v, want 160", intPtrValue(record.InputTokens))
	}
	if record.OutputTokens == nil || *record.OutputTokens != 45 {
		t.Fatalf("runLLMBenchmarkCase(job_identity).OutputTokens = %v, want 45", intPtrValue(record.OutputTokens))
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(record.RawOutput), &parsed); err != nil {
		t.Fatalf("runLLMBenchmarkCase(job_identity).RawOutput invalid JSON: %v\n%s", err, record.RawOutput)
	}
	if got, ok := benchmarkValueAtPath(parsed, "company_website"); !ok || got != "https://www.exampleapps.test" {
		t.Fatalf("company_website = %#v, %t; want https://www.exampleapps.test, true", got, ok)
	}
}

func TestRunLLMBenchmarkCaseRecordsTokenUsage(t *testing.T) {
	record := runLLMBenchmarkCase(
		context.Background(),
		fakeContentLLM{
			content: `{
				"matches": true,
				"compensation_extracted": "$120k base",
				"remote_eligibility": "Remote",
				"why_it_matches": ["remote"],
				"why_rejected": []
			}`,
			generationInfo: map[string]any{
				"PromptTokens":     120,
				"CompletionTokens": 30,
				"TotalTokens":      150,
			},
		},
		"test",
		"test-model",
		llmBenchmarkCase{
			ID:      "job_filter_usage_v1",
			Task:    "job_filter",
			Version: 1,
			Input: json.RawMessage(`{
				"criteria": {
					"filters": {
						"work_settings": {"remote": true}
					}
				},
				"job": {
					"company": "Example Apps",
					"title": "Software Engineer",
					"remote": "Remote",
					"compensation": "$120k base",
					"description": "Build APIs."
				}
			}`),
			Checks: benchmarkChecks{JSONRequired: true, RequiredFields: []string{"matches"}},
		},
	)

	if record.Error != "" {
		t.Fatalf("runLLMBenchmarkCase(token usage).Error = %q, want empty", record.Error)
	}
	if record.InputTokens == nil || *record.InputTokens != 120 {
		t.Fatalf("runLLMBenchmarkCase(token usage).InputTokens = %v, want 120", intPtrValue(record.InputTokens))
	}
	if record.OutputTokens == nil || *record.OutputTokens != 30 {
		t.Fatalf("runLLMBenchmarkCase(token usage).OutputTokens = %v, want 30", intPtrValue(record.OutputTokens))
	}
	if got := record.Details["total_tokens"]; got != 150 {
		t.Fatalf("runLLMBenchmarkCase(token usage).Details[total_tokens] = %v, want 150", got)
	}
}

func TestScoreBenchmarkOutputAppliesMissingRequiredFieldsCap(t *testing.T) {
	record := benchmarkRecordWithPerfectRunScores("job_filter")
	checks := benchmarkChecks{
		JSONRequired:   true,
		RequiredFields: []string{"matches", "why_it_matches"},
	}

	scoreBenchmarkOutput(&record, checks, `{"matches":true}`)

	if !record.JSONValid {
		t.Fatalf("scoreBenchmarkOutput(missing fields).JSONValid = false, want true")
	}
	if record.RequiredFieldsPresent {
		t.Fatalf("scoreBenchmarkOutput(missing fields).RequiredFieldsPresent = true, want false")
	}
	if record.ScoreCap != 60 {
		t.Fatalf("scoreBenchmarkOutput(missing fields).ScoreCap = %d, want 60", record.ScoreCap)
	}
	if record.FinalScore != 60 {
		t.Fatalf("scoreBenchmarkOutput(missing fields).FinalScore = %v, want 60", record.FinalScore)
	}
}

func TestBenchmarkRequiredFieldsPresent(t *testing.T) {
	parsed := map[string]any{
		"matches":                true,
		"why_it_matches":         "",
		"compensation_extracted": nil,
		"candidate": map[string]any{
			"State":       "OH",
			"CountryCode": "US",
		},
	}

	tests := []struct {
		name     string
		parsed   map[string]any
		required []string
		want     bool
	}{
		{
			name: "no required fields",
			want: true,
		},
		{
			name:     "all required fields present",
			parsed:   parsed,
			required: []string{"matches", "why_it_matches"},
			want:     true,
		},
		{
			name:     "present nil value counts as present",
			parsed:   parsed,
			required: []string{"compensation_extracted"},
			want:     true,
		},
		{
			name:     "nested field path",
			parsed:   parsed,
			required: []string{"candidate.state"},
			want:     true,
		},
		{
			name:     "nested field path case insensitive",
			parsed:   parsed,
			required: []string{"Candidate.State"},
			want:     true,
		},
		{
			name:     "nested field path snake case to camel case",
			parsed:   parsed,
			required: []string{"candidate.country_code"},
			want:     true,
		},
		{
			name:     "missing field",
			parsed:   parsed,
			required: []string{"remote_eligibility"},
			want:     false,
		},
		{
			name:     "nil parsed output with required field",
			required: []string{"matches"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := benchmarkRequiredFieldsPresent(tt.parsed, tt.required)
			if got != tt.want {
				t.Fatalf("benchmarkRequiredFieldsPresent(%#v, %#v) = %t, want %t", tt.parsed, tt.required, got, tt.want)
			}
		})
	}
}

func TestBenchmarkAccuracyScoreUsesNestedExpectedValues(t *testing.T) {
	parsed := map[string]any{
		"candidate": map[string]any{
			"State":       "oh",
			"CountryCode": "US",
		},
		"role_families": []any{"backend", "frontend"},
		"priority_signals": []any{
			"testing",
			"databases",
		},
		"filters": map[string]any{
			"work_settings": map[string]any{
				"remote": true,
			},
			"min_base_usd": float64(110000),
		},
	}
	checks := benchmarkChecks{
		ExpectedValues: map[string]any{
			"candidate.state":               "OH",
			"candidate.country_code":        "US",
			"filters.work_settings.remote":  true,
			"filters.min_base_usd":          float64(110000),
			"filters.work_settings.hybrid":  true,
			"filters.work_settings.manager": false,
		},
		ExpectedContains: map[string][]any{
			"role_families":    {"backend", "mobile"},
			"priority_signals": {"testing"},
		},
		NumericMinimums: map[string]float64{
			"filters.min_base_usd":          100000,
			"candidate.years_of_experience": 4,
		},
	}

	score, details := benchmarkAccuracyScore(parsed, checks, "")
	if score != 63 {
		t.Fatalf("benchmarkAccuracyScore(nested expected values) = %d, want 63", score)
	}
	if details["accuracy_checks_matched"] != 7 {
		t.Fatalf("benchmarkAccuracyScore(...).details[accuracy_checks_matched] = %v, want 7", details["accuracy_checks_matched"])
	}
	if details["accuracy_checks_total"] != 11 {
		t.Fatalf("benchmarkAccuracyScore(...).details[accuracy_checks_total] = %v, want 11", details["accuracy_checks_total"])
	}
}

func TestBenchmarkAccuracyScoreUsesEnumValues(t *testing.T) {
	tests := []struct {
		name        string
		parsed      map[string]any
		wantScore   int
		wantMatched int
	}{
		{
			name: "valid enum values with nested path",
			parsed: map[string]any{
				"risk_level": "HIGH",
				"results": map[string]any{
					"accept_remote_engineer": map[string]any{
						"remote_eligibility": "remote",
					},
				},
			},
			wantScore:   100,
			wantMatched: 2,
		},
		{
			name: "invalid enum values",
			parsed: map[string]any{
				"risk_level": "critical",
				"results": map[string]any{
					"accept_remote_engineer": map[string]any{
						"remote_eligibility": "teleport",
					},
				},
			},
			wantScore:   0,
			wantMatched: 0,
		},
	}
	checks := benchmarkChecks{
		EnumValues: map[string][]any{
			"risk_level": {"low", "medium", "high"},
			"results.accept_remote_engineer.remote_eligibility": {"Remote", "Onsite"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, details := benchmarkAccuracyScore(tt.parsed, checks, "")
			if score != tt.wantScore {
				t.Fatalf("benchmarkAccuracyScore(enum values) = %d, want %d", score, tt.wantScore)
			}
			if details["accuracy_checks_matched"] != tt.wantMatched {
				t.Fatalf("benchmarkAccuracyScore(...).details[accuracy_checks_matched] = %v, want %d", details["accuracy_checks_matched"], tt.wantMatched)
			}
			if details["accuracy_checks_total"] != 2 {
				t.Fatalf("benchmarkAccuracyScore(...).details[accuracy_checks_total] = %v, want 2", details["accuracy_checks_total"])
			}
		})
	}
}

func TestBenchmarkAccuracyScoreUsesNumericMaximumsAndRanges(t *testing.T) {
	parsed := map[string]any{
		"salary": map[string]any{
			"max_base_usd": float64(140000),
		},
		"candidate": map[string]any{
			"years_of_experience": float64(2),
		},
		"risk": map[string]any{
			"score": float64(35),
		},
	}
	minExperience := 1.0
	maxExperience := 3.0
	minSalary := 100000.0
	maxSalary := 130000.0
	checks := benchmarkChecks{
		NumericMaximums: map[string]float64{
			"salary.max_base_usd": 150000,
			"risk.score":          25,
		},
		NumericRanges: map[string]benchmarkNumericRange{
			"candidate.years_of_experience": {Min: &minExperience, Max: &maxExperience},
			"salary.max_base_usd":           {Min: &minSalary, Max: &maxSalary},
		},
	}

	score, details := benchmarkAccuracyScore(parsed, checks, "")
	if score != 50 {
		t.Fatalf("benchmarkAccuracyScore(numeric maximums and ranges) = %d, want 50", score)
	}
	if details["accuracy_checks_matched"] != 2 {
		t.Fatalf("benchmarkAccuracyScore(...).details[accuracy_checks_matched] = %v, want 2", details["accuracy_checks_matched"])
	}
	if details["accuracy_checks_total"] != 4 {
		t.Fatalf("benchmarkAccuracyScore(...).details[accuracy_checks_total] = %v, want 4", details["accuracy_checks_total"])
	}
}

func TestBenchmarkValueContainsSupportsStringSlicesAndSubstrings(t *testing.T) {
	if !benchmarkValueContains([]any{"backend", "frontend"}, "Backend") {
		t.Fatal("benchmarkValueContains([]any{backend, frontend}, Backend) = false, want true")
	}
	if !benchmarkValueContains("remote backend developer", "Backend") {
		t.Fatal("benchmarkValueContains(remote backend developer, Backend) = false, want true")
	}
	if benchmarkValueContains([]any{"backend"}, "mobile") {
		t.Fatal("benchmarkValueContains([]any{backend}, mobile) = true, want false")
	}
}

func TestBenchmarkNumericValue(t *testing.T) {
	tests := []struct {
		input any
		want  float64
		ok    bool
	}{
		{input: float64(4), want: 4, ok: true},
		{input: int(5), want: 5, ok: true},
		{input: json.Number("6.5"), want: 6.5, ok: true},
		{input: "7", ok: false},
	}

	for _, tt := range tests {
		got, ok := benchmarkNumericValue(tt.input)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("benchmarkNumericValue(%#v) = %v, %t; want %v, %t", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestBenchmarkRecordCostUSDEstimatesOpenAITokenCost(t *testing.T) {
	record := llmBenchmarkRunRecord{
		Provider:     "openai",
		Model:        "gpt-4o-mini-2024-07-18",
		InputTokens:  benchmarkTestIntPtr(1000),
		OutputTokens: benchmarkTestIntPtr(500),
		Details: map[string]any{
			"cached_tokens": 200,
		},
	}

	got, ok := benchmarkRecordCostUSD(record)
	if !ok {
		t.Fatal("benchmarkRecordCostUSD(...) ok = false, want true")
	}
	want := 0.000435
	if diff := got - want; diff < -0.0000001 || diff > 0.0000001 {
		t.Fatalf("benchmarkRecordCostUSD(...) = %.8f, want %.8f", got, want)
	}
}

func TestNormalizeBenchmarkRunScoresNormalizesSpeedAndCostWhenAvailable(t *testing.T) {
	cheapCost := 0.002
	expensiveCost := 0.004
	records := []llmBenchmarkRunRecord{
		{
			Task:                  "job_filter",
			CaseID:                "case1",
			Model:                 "fast-expensive",
			LatencyMS:             1000,
			EstimatedCostUSD:      &expensiveCost,
			AccuracyScore:         100,
			JSONScore:             100,
			GroundingScore:        100,
			SpeedScore:            100,
			CostScore:             100,
			StabilityScore:        100,
			FinalScore:            100,
			RequiredFieldsPresent: true,
		},
		{
			Task:                  "job_filter",
			CaseID:                "case1",
			Model:                 "slow-cheap",
			LatencyMS:             2000,
			EstimatedCostUSD:      &cheapCost,
			AccuracyScore:         100,
			JSONScore:             100,
			GroundingScore:        100,
			SpeedScore:            100,
			CostScore:             100,
			StabilityScore:        100,
			FinalScore:            100,
			RequiredFieldsPresent: true,
		},
		{
			Task:                  "job_filter",
			CaseID:                "case1",
			Model:                 "unknown-cost",
			LatencyMS:             1000,
			AccuracyScore:         100,
			JSONScore:             100,
			GroundingScore:        100,
			SpeedScore:            100,
			CostScore:             100,
			StabilityScore:        100,
			FinalScore:            100,
			RequiredFieldsPresent: true,
		},
	}

	normalizeBenchmarkRunScores(records)

	if records[0].SpeedScore != 100 || records[0].CostScore != 50 {
		t.Fatalf("fast-expensive scores = speed %d cost %d, want speed 100 cost 50", records[0].SpeedScore, records[0].CostScore)
	}
	if records[1].SpeedScore != 50 || records[1].CostScore != 100 {
		t.Fatalf("slow-cheap scores = speed %d cost %d, want speed 50 cost 100", records[1].SpeedScore, records[1].CostScore)
	}
	if records[2].SpeedScore != 100 || records[2].CostScore != 100 {
		t.Fatalf("unknown-cost scores = speed %d cost %d, want unchanged cost 100 with speed 100", records[2].SpeedScore, records[2].CostScore)
	}
}

func benchmarkRecordWithPerfectRunScores(task string) llmBenchmarkRunRecord {
	return llmBenchmarkRunRecord{
		Task:           task,
		SpeedScore:     100,
		CostScore:      100,
		StabilityScore: 100,
	}
}

func benchmarkTaskNames(tasks map[string]bool) []string {
	keys := make([]string, 0, len(tasks))
	for key := range tasks {
		keys = append(keys, key)
	}
	return keys
}
