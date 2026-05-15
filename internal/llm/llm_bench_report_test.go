package llm

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBenchmarkRecordsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llm-bench.jsonl")
	data := strings.Join([]string{
		`{"provider":"gemini","model":"gemini-2.5-flash-lite","task":"job_filter","case_id":"case1","final_score":91.5}`,
		`{"provider":"gemini","model":"gemini-2.5-flash","task":"resume_to_criteria","case_id":"case2","final_score":82.0,"error":"boom"}`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}

	records, err := loadBenchmarkRecordsFile(path)
	if err != nil {
		t.Fatalf("loadBenchmarkRecordsFile(%q) error = %v", path, err)
	}
	if len(records) != 2 {
		t.Fatalf("loadBenchmarkRecordsFile(%q) len = %d, want 2", path, len(records))
	}
	if records[0].Model != "gemini-2.5-flash-lite" {
		t.Fatalf("loadBenchmarkRecordsFile(%q)[0].Model = %q, want gemini-2.5-flash-lite", path, records[0].Model)
	}
	if records[1].Error != "boom" {
		t.Fatalf("loadBenchmarkRecordsFile(%q)[1].Error = %q, want boom", path, records[1].Error)
	}
}

func TestLoadBenchmarkRecordsFileRejectsMalformedJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llm-bench.jsonl")
	if err := os.WriteFile(path, []byte("{bad json}\n"), 0600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}

	_, err := loadBenchmarkRecordsFile(path)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("loadBenchmarkRecordsFile(%q) error = %v, want parse error", path, err)
	}
}

func TestParseBenchmarkReportOptions(t *testing.T) {
	opts, err := parseBenchmarkReportOptions([]string{"--latest", "--json"})
	if err != nil {
		t.Fatalf("parseBenchmarkReportOptions(--latest --json) error = %v", err)
	}
	if !opts.Latest {
		t.Fatalf("parseBenchmarkReportOptions(--latest --json).Latest = false, want true")
	}
	if !opts.JSON {
		t.Fatalf("parseBenchmarkReportOptions(--latest --json).JSON = false, want true")
	}

	opts, err = parseBenchmarkReportOptions([]string{"--latest", "--format", "md"})
	if err != nil {
		t.Fatalf("parseBenchmarkReportOptions(--latest --format md) error = %v", err)
	}
	if !opts.Latest {
		t.Fatalf("parseBenchmarkReportOptions(--latest --format md).Latest = false, want true")
	}
	if opts.Format != "md" {
		t.Fatalf("parseBenchmarkReportOptions(--latest --format md).Format = %q, want md", opts.Format)
	}

	opts, err = parseBenchmarkReportOptions([]string{"--format=markdown"})
	if err != nil {
		t.Fatalf("parseBenchmarkReportOptions(--format=markdown) error = %v", err)
	}
	if opts.Format != "md" {
		t.Fatalf("parseBenchmarkReportOptions(--format=markdown).Format = %q, want md", opts.Format)
	}

	opts, err = parseBenchmarkReportOptions([]string{"--format", "json"})
	if err != nil {
		t.Fatalf("parseBenchmarkReportOptions(--format json) error = %v", err)
	}
	if !opts.JSON || opts.Format != "json" {
		t.Fatalf("parseBenchmarkReportOptions(--format json) = JSON %t Format %q, want JSON true Format json", opts.JSON, opts.Format)
	}

	_, err = parseBenchmarkReportOptions([]string{"--json", "--format", "md"})
	if err == nil || !strings.Contains(err.Error(), "conflicting benchmark report output formats") {
		t.Fatalf("parseBenchmarkReportOptions(--json --format md) error = %v, want format conflict", err)
	}

	_, err = parseBenchmarkReportOptions([]string{"--bogus"})
	if err == nil || !strings.Contains(err.Error(), "unknown benchmark report option") {
		t.Fatalf("parseBenchmarkReportOptions(--bogus) error = %v, want unknown option", err)
	}
}

func TestBenchmarkTaskSummariesIncludeFailureRateLatencyAndValue(t *testing.T) {
	cheapCost := 0.001
	expensiveCost := 0.01
	records := []llmBenchmarkRunRecord{
		{
			Task:             "llm_job_filtering",
			Model:            "cheap-good",
			FinalScore:       90,
			LatencyMS:        1000,
			JSONValid:        true,
			EstimatedCostUSD: &cheapCost,
		},
		{
			Task:             "llm_job_filtering",
			Model:            "expensive-best",
			FinalScore:       95,
			LatencyMS:        3000,
			JSONValid:        true,
			EstimatedCostUSD: &expensiveCost,
		},
		{
			Task:      "llm_job_filtering",
			Model:     "bad-json",
			LatencyMS: 500,
			JSONValid: false,
		},
		{
			Task:       "resume_to_criteria",
			Model:      "no-cost",
			FinalScore: 80,
			LatencyMS:  2000,
			JSONValid:  true,
		},
	}

	summaries := benchmarkTaskSummaries(records)
	if len(summaries) != 2 {
		t.Fatalf("benchmarkTaskSummaries(...) len = %d, want 2", len(summaries))
	}

	jobFilter := summaries[0]
	if jobFilter.Task != "llm_job_filtering" {
		t.Fatalf("benchmarkTaskSummaries(...)[0].Task = %q, want llm_job_filtering", jobFilter.Task)
	}
	if jobFilter.BestQualityModel != "expensive-best" {
		t.Fatalf("job_filter BestQualityModel = %q, want expensive-best", jobFilter.BestQualityModel)
	}
	if jobFilter.BestValueModel != "cheap-good" {
		t.Fatalf("job_filter BestValueModel = %q, want cheap-good", jobFilter.BestValueModel)
	}
	if jobFilter.ParseFailures != 1 || jobFilter.Total != 3 {
		t.Fatalf("job_filter parse failures = %d/%d, want 1/3", jobFilter.ParseFailures, jobFilter.Total)
	}
	if jobFilter.AvgLatencyMS != 1500 {
		t.Fatalf("job_filter AvgLatencyMS = %d, want 1500", jobFilter.AvgLatencyMS)
	}

	resume := summaries[1]
	if resume.Task != "resume_to_criteria" {
		t.Fatalf("benchmarkTaskSummaries(...)[1].Task = %q, want resume_to_criteria", resume.Task)
	}
	if resume.BestValueModel != "" {
		t.Fatalf("resume_to_criteria BestValueModel = %q, want empty without cost data", resume.BestValueModel)
	}
	if resume.CostRecords != 0 {
		t.Fatalf("resume_to_criteria CostRecords = %d, want 0", resume.CostRecords)
	}
}

func TestBenchmarkTaskModelComparisonsGroupRecordsByTaskKeys(t *testing.T) {
	fastCost1 := 0.002
	fastCost2 := 0.003
	accurateCost := 0.006
	searchCost := 0.001
	records := []llmBenchmarkRunRecord{
		{
			Provider:              "gemini",
			Model:                 "fast-filter",
			Task:                  "llm_job_filtering",
			CaseID:                "single",
			LatencyMS:             800,
			InputTokens:           benchmarkTestIntPtr(100),
			OutputTokens:          benchmarkTestIntPtr(50),
			EstimatedCostUSD:      &fastCost1,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         80,
			JSONScore:             100,
			GroundingScore:        75,
			SpeedScore:            95,
			CostScore:             98,
			StabilityScore:        90,
			FinalScore:            88,
			Details:               map[string]any{"total_tokens": 150},
		},
		{
			Provider:              "gemini",
			Model:                 "fast-filter",
			Task:                  "llm_job_filtering",
			CaseID:                "batch",
			LatencyMS:             1000,
			InputTokens:           benchmarkTestIntPtr(200),
			OutputTokens:          benchmarkTestIntPtr(80),
			EstimatedCostUSD:      &fastCost2,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         90,
			JSONScore:             100,
			GroundingScore:        85,
			SpeedScore:            90,
			CostScore:             96,
			StabilityScore:        90,
			FinalScore:            92,
			Details:               map[string]any{"total_tokens": 280},
		},
		{
			Provider:              "gemini",
			Model:                 "accurate-filter",
			Task:                  "llm_job_filtering",
			CaseID:                "single",
			LatencyMS:             2000,
			InputTokens:           benchmarkTestIntPtr(400),
			OutputTokens:          benchmarkTestIntPtr(120),
			EstimatedCostUSD:      &accurateCost,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         98,
			JSONScore:             100,
			GroundingScore:        95,
			SpeedScore:            65,
			CostScore:             75,
			StabilityScore:        95,
			FinalScore:            95,
			Details:               map[string]any{"total_tokens": 520},
		},
		{
			Provider:              "openai",
			Model:                 "resume-model",
			Task:                  "resume_to_criteria",
			CaseID:                "resume",
			LatencyMS:             1500,
			InputTokens:           benchmarkTestIntPtr(300),
			OutputTokens:          benchmarkTestIntPtr(90),
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         86,
			JSONScore:             100,
			GroundingScore:        80,
			SpeedScore:            82,
			CostScore:             90,
			StabilityScore:        88,
			FinalScore:            86,
		},
		{
			Provider:              "anthropic",
			Model:                 "health-model",
			Task:                  "llm_company_health",
			CaseID:                "health",
			LatencyMS:             2500,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         91,
			JSONScore:             100,
			GroundingScore:        90,
			SpeedScore:            70,
			CostScore:             80,
			StabilityScore:        93,
			FinalScore:            91,
		},
		{
			Provider:              "ollama",
			Model:                 "identity-model",
			Task:                  "job_identity",
			CaseID:                "identity",
			LatencyMS:             1700,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         89,
			JSONScore:             100,
			GroundingScore:        85,
			SpeedScore:            78,
			CostScore:             100,
			StabilityScore:        90,
			FinalScore:            89,
		},
		{
			Provider:              "openai",
			Model:                 "search-model",
			Task:                  "llm_job_search",
			CaseID:                "search",
			LatencyMS:             1200,
			InputTokens:           benchmarkTestIntPtr(250),
			OutputTokens:          benchmarkTestIntPtr(100),
			EstimatedCostUSD:      &searchCost,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         92,
			JSONScore:             100,
			GroundingScore:        90,
			SpeedScore:            84,
			CostScore:             95,
			StabilityScore:        90,
			FinalScore:            91,
			Details:               map[string]any{"total_tokens": 350},
		},
	}

	comparisons := benchmarkTaskModelComparisons(records)
	if len(comparisons) != len(benchmarkTaskModelGroups) {
		t.Fatalf("benchmarkTaskModelComparisons(...) len = %d, want %d", len(comparisons), len(benchmarkTaskModelGroups))
	}

	filtering := benchmarkTestComparisonByLabel(t, comparisons, "Job filtering")
	if filtering.TaskKey != llmTaskFiltering {
		t.Fatalf("Job filtering TaskKey = %q, want %q", filtering.TaskKey, llmTaskFiltering)
	}
	if filtering.BestQuality.Model != "accurate-filter" {
		t.Fatalf("Job filtering BestQuality.Model = %q, want accurate-filter", filtering.BestQuality.Model)
	}
	if filtering.Fastest.Model != "fast-filter" {
		t.Fatalf("Job filtering Fastest.Model = %q, want fast-filter", filtering.Fastest.Model)
	}
	if filtering.LowestToken.Model != "fast-filter" {
		t.Fatalf("Job filtering LowestToken.Model = %q, want fast-filter", filtering.LowestToken.Model)
	}
	if filtering.LowestCost.Model != "fast-filter" {
		t.Fatalf("Job filtering LowestCost.Model = %q, want fast-filter", filtering.LowestCost.Model)
	}

	fastFilter := benchmarkTestModelSummary(t, filtering.Models, "gemini", "fast-filter")
	if fastFilter.OK != 2 {
		t.Fatalf("fast-filter OK = %d, want 2", fastFilter.OK)
	}
	if fastFilter.AvgLatencyMS != 900 {
		t.Fatalf("fast-filter AvgLatencyMS = %d, want 900", fastFilter.AvgLatencyMS)
	}
	if fastFilter.AvgTotalTokens != 215 {
		t.Fatalf("fast-filter AvgTotalTokens = %d, want 215", fastFilter.AvgTotalTokens)
	}
	benchmarkTestFloatClose(t, "fast-filter AvgSpeedScore", fastFilter.AvgSpeedScore, 92.5)
	benchmarkTestFloatClose(t, "fast-filter AvgCostScore", fastFilter.AvgCostScore, 97)
	benchmarkTestFloatClose(t, "fast-filter AvgEstimatedCostUSD", fastFilter.AvgEstimatedCostUSD, 0.0025)

	resume := benchmarkTestComparisonByLabel(t, comparisons, "Resume parsing")
	if resume.TaskKey != llmTaskResumeCriteria {
		t.Fatalf("Resume parsing TaskKey = %q, want %q", resume.TaskKey, llmTaskResumeCriteria)
	}
	if resume.BestQuality.Model != "resume-model" {
		t.Fatalf("Resume parsing BestQuality.Model = %q, want resume-model", resume.BestQuality.Model)
	}

	jobSearch := benchmarkTestComparisonByLabel(t, comparisons, "Job search")
	if jobSearch.TaskKey != llmTaskJobSearch {
		t.Fatalf("Job search TaskKey = %q, want %q", jobSearch.TaskKey, llmTaskJobSearch)
	}
	if jobSearch.BestQuality.Model != "search-model" {
		t.Fatalf("Job search BestQuality.Model = %q, want search-model", jobSearch.BestQuality.Model)
	}

	enrichment := benchmarkTestComparisonByLabel(t, comparisons, "Job enrichment")
	if enrichment.BestQuality.Model != "identity-model" {
		t.Fatalf("Job enrichment BestQuality.Model = %q, want identity-model", enrichment.BestQuality.Model)
	}

	health := benchmarkTestComparisonByLabel(t, comparisons, "Company Health")
	if health.TaskKey != llmTaskCompanyHealth {
		t.Fatalf("Company Health TaskKey = %q, want %q", health.TaskKey, llmTaskCompanyHealth)
	}
	if health.BestQuality.Model != "health-model" {
		t.Fatalf("Company Health BestQuality.Model = %q, want health-model", health.BestQuality.Model)
	}
}

func TestBenchmarkTaskModelComparisonsDoNotPickFailedOnlyBestQuality(t *testing.T) {
	records := []llmBenchmarkRunRecord{
		{
			Provider: "gemini",
			Model:    "failing-filter",
			Task:     "llm_job_filtering",
			CaseID:   "single",
			Error:    "provider unavailable",
		},
		{
			Provider: "openai",
			Model:    "also-failing-filter",
			Task:     "llm_job_filtering",
			CaseID:   "batch",
			Error:    "request failed",
		},
	}

	comparisons := benchmarkTaskModelComparisons(records)
	filtering := benchmarkTestComparisonByLabel(t, comparisons, "Job filtering")
	if len(filtering.Models) != 2 {
		t.Fatalf("Job filtering Models len = %d, want 2", len(filtering.Models))
	}
	if filtering.BestQuality.ModelName() != "" {
		t.Fatalf("Job filtering BestQuality = %q, want empty for failed-only task", filtering.BestQuality.ModelName())
	}
	if filtering.Fastest.ModelName() != "" {
		t.Fatalf("Job filtering Fastest = %q, want empty for failed-only task", filtering.Fastest.ModelName())
	}

	report := benchmarkMarkdownReport(records, 1)
	want := "| Job filtering | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |"
	if !strings.Contains(report, want) {
		t.Fatalf("benchmarkMarkdownReport(...) missing failed-only n/a winner row %q in:\n%s", want, report)
	}
	if !strings.Contains(report, "provider unavailable") {
		t.Fatalf("benchmarkMarkdownReport(...) missing failed-only error summary in:\n%s", report)
	}
}

func TestBenchmarkTaskModelAdvisoriesFlagFailuresAndLowScores(t *testing.T) {
	records := []llmBenchmarkRunRecord{
		{
			Provider:              "openai",
			Model:                 "good-search",
			Task:                  "llm_job_search",
			CaseID:                "search",
			JSONValid:             true,
			RequiredFieldsPresent: true,
			FinalScore:            91,
		},
		{
			Provider:              "openai",
			Model:                 "low-score-search",
			Task:                  "llm_job_search",
			CaseID:                "search",
			JSONValid:             true,
			RequiredFieldsPresent: true,
			FinalScore:            72,
		},
		{
			Provider: "openai",
			Model:    "broken-search",
			Task:     "llm_job_search",
			CaseID:   "search",
			Error:    "parse failed",
		},
	}

	advisories := benchmarkTaskModelAdvisories(records)
	if len(advisories) != 2 {
		t.Fatalf("benchmarkTaskModelAdvisories(...) len = %d, want 2: %#v", len(advisories), advisories)
	}
	lowScore := benchmarkTestAdvisoryByModel(t, advisories, "openai/low-score-search")
	if lowScore.Recommendation != benchmarkAdvisoryAvoid {
		t.Fatalf("low-score Recommendation = %q, want %q", lowScore.Recommendation, benchmarkAdvisoryAvoid)
	}
	if !strings.Contains(lowScore.Reason, "average score 72.0 below 75.0") {
		t.Fatalf("low-score Reason = %q, want score threshold", lowScore.Reason)
	}
	broken := benchmarkTestAdvisoryByModel(t, advisories, "openai/broken-search")
	if !strings.Contains(broken.Reason, "no successful runs") || !strings.Contains(broken.Reason, "1/1 runs failed") {
		t.Fatalf("broken Reason = %q, want failure reasons", broken.Reason)
	}
}

func TestBenchmarkMarkdownReportIncludesNotRecommendedModels(t *testing.T) {
	records := []llmBenchmarkRunRecord{
		{
			Provider: "openai",
			Model:    "broken-search",
			Task:     "llm_job_search",
			CaseID:   "search",
			Error:    "parse failed",
		},
	}

	report := benchmarkMarkdownReport(records, 1)
	for _, want := range []string{
		"## Not recommended models",
		"| Task | Model | Recommendation | Reason |",
		"| Job search | openai/broken-search | Avoid for this task | no successful runs; 1/1 runs failed |",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("benchmarkMarkdownReport(...) missing %q in:\n%s", want, report)
		}
	}
}

func TestBenchmarkTaskModelComparisonsNormalizesLegacyTaskNames(t *testing.T) {
	records := []llmBenchmarkRunRecord{
		{Provider: "gemini", Model: "legacy-filter", Task: "job_filter", CaseID: "single", JSONValid: true, RequiredFieldsPresent: true, FinalScore: 90},
		{Provider: "gemini", Model: "legacy-filter", Task: "job_filter_batch", CaseID: "batch", JSONValid: true, RequiredFieldsPresent: true, FinalScore: 92},
		{Provider: "openai", Model: "legacy-search", Task: "autonomous_job_search", CaseID: "search", JSONValid: true, RequiredFieldsPresent: true, FinalScore: 91},
		{Provider: "anthropic", Model: "legacy-health", Task: "company_health_summary", CaseID: "health", JSONValid: true, RequiredFieldsPresent: true, FinalScore: 89},
	}

	comparisons := benchmarkTaskModelComparisons(records)
	filtering := benchmarkTestComparisonByLabel(t, comparisons, "Job filtering")
	if filtering.TaskKey != llmTaskFiltering || len(filtering.Models) != 1 || filtering.Models[0].OK != 2 {
		t.Fatalf("legacy filtering records were not normalized into one task: %#v", filtering)
	}
	search := benchmarkTestComparisonByLabel(t, comparisons, "Job search")
	if search.BestQuality.Model != "legacy-search" {
		t.Fatalf("legacy search BestQuality.Model = %q, want legacy-search", search.BestQuality.Model)
	}
	health := benchmarkTestComparisonByLabel(t, comparisons, "Company Health")
	if health.BestQuality.Model != "legacy-health" {
		t.Fatalf("legacy health BestQuality.Model = %q, want legacy-health", health.BestQuality.Model)
	}
}

func TestBenchmarkTaskModelComparisonsEstimateCostFromTokenUsage(t *testing.T) {
	records := []llmBenchmarkRunRecord{
		{
			Provider:              "openai",
			Model:                 "gpt-4o-mini",
			Task:                  "resume_to_criteria",
			CaseID:                "resume",
			LatencyMS:             1000,
			InputTokens:           benchmarkTestIntPtr(1000),
			OutputTokens:          benchmarkTestIntPtr(500),
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         90,
			JSONScore:             100,
			GroundingScore:        90,
			SpeedScore:            100,
			CostScore:             100,
			StabilityScore:        100,
			FinalScore:            94,
			Details:               map[string]any{"total_tokens": 1500},
		},
	}

	resume := benchmarkTestComparisonByLabel(t, benchmarkTaskModelComparisons(records), "Resume parsing")
	if resume.LowestCost.Model != "gpt-4o-mini" {
		t.Fatalf("Resume parsing LowestCost.Model = %q, want gpt-4o-mini", resume.LowestCost.Model)
	}
	benchmarkTestFloatClose(t, "Resume parsing AvgEstimatedCostUSD", resume.LowestCost.AvgEstimatedCostUSD, 0.00045)
}

func TestBenchmarkMarkdownReportIncludesTaskComparisonTables(t *testing.T) {
	fastCost1 := 0.002
	fastCost2 := 0.003
	accurateCost := 0.006
	searchCost := 0.001
	records := []llmBenchmarkRunRecord{
		{
			Provider:              "gemini",
			Model:                 "fast-filter",
			Task:                  "llm_job_filtering",
			CaseID:                "single",
			LatencyMS:             800,
			InputTokens:           benchmarkTestIntPtr(100),
			OutputTokens:          benchmarkTestIntPtr(50),
			EstimatedCostUSD:      &fastCost1,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         80,
			JSONScore:             100,
			GroundingScore:        75,
			SpeedScore:            95,
			CostScore:             98,
			StabilityScore:        90,
			FinalScore:            88,
			Details:               map[string]any{"total_tokens": 150},
		},
		{
			Provider:              "gemini",
			Model:                 "fast-filter",
			Task:                  "llm_job_filtering",
			CaseID:                "batch",
			LatencyMS:             1000,
			InputTokens:           benchmarkTestIntPtr(200),
			OutputTokens:          benchmarkTestIntPtr(80),
			EstimatedCostUSD:      &fastCost2,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         90,
			JSONScore:             100,
			GroundingScore:        85,
			SpeedScore:            90,
			CostScore:             96,
			StabilityScore:        90,
			FinalScore:            92,
			Details:               map[string]any{"total_tokens": 280},
		},
		{
			Provider:              "gemini",
			Model:                 "accurate-filter",
			Task:                  "llm_job_filtering",
			CaseID:                "single",
			LatencyMS:             2000,
			InputTokens:           benchmarkTestIntPtr(400),
			OutputTokens:          benchmarkTestIntPtr(120),
			EstimatedCostUSD:      &accurateCost,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         98,
			JSONScore:             100,
			GroundingScore:        95,
			SpeedScore:            65,
			CostScore:             75,
			StabilityScore:        95,
			FinalScore:            95,
			Details:               map[string]any{"total_tokens": 520},
		},
		{
			Provider:              "openai",
			Model:                 "search-model",
			Task:                  "llm_job_search",
			CaseID:                "search",
			LatencyMS:             1200,
			InputTokens:           benchmarkTestIntPtr(250),
			OutputTokens:          benchmarkTestIntPtr(100),
			EstimatedCostUSD:      &searchCost,
			JSONValid:             true,
			RequiredFieldsPresent: true,
			AccuracyScore:         92,
			JSONScore:             100,
			GroundingScore:        90,
			SpeedScore:            84,
			CostScore:             95,
			StabilityScore:        90,
			FinalScore:            91,
			Details:               map[string]any{"total_tokens": 350},
		},
	}

	report := benchmarkMarkdownReport(records, 1)
	for _, want := range []string{
		"# Jobscout LLM Benchmark Report",
		"Loaded 4 benchmark records from 1 file(s).",
		"| Task | Best quality | Score | Fastest usable | Latency | Lowest token use | Avg tokens | Lowest estimated cost | Avg USD |",
		"| Job filtering | gemini/accurate-filter | 95.0 | gemini/fast-filter | 900ms | gemini/fast-filter | 215 | gemini/fast-filter | $0.002500 |",
		"| Job search | openai/search-model | 91.0 | openai/search-model | 1200ms | openai/search-model | 350 | openai/search-model | $0.001000 |",
		"## Job filtering",
		"| Model | Runs | OK | Err | Score | Accuracy | JSON | Grounding | Speed | Cost | Stability | Latency | Avg tokens | Avg USD | Parse failures | Missing fields | Common error |",
		"| gemini/fast-filter | 2 | 2 | 0 | 90.0 | 85.0 | 100.0 | 80.0 | 92.5 | 97.0 | 90.0 | 900ms | 215 | $0.002500 | 0 | 0 | n/a |",
		"## Job search",
		"| openai/search-model | 1 | 1 | 0 | 91.0 | 92.0 | 100.0 | 90.0 | 84.0 | 95.0 | 90.0 | 1200ms | 350 | $0.001000 | 0 | 0 | n/a |",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("benchmarkMarkdownReport(...) missing %q in:\n%s", want, report)
		}
	}
}

func benchmarkTestIntPtr(value int) *int {
	return &value
}

func benchmarkTestComparisonByLabel(t *testing.T, comparisons []benchmarkTaskModelComparison, label string) benchmarkTaskModelComparison {
	t.Helper()
	for _, comparison := range comparisons {
		if comparison.Label == label {
			return comparison
		}
	}
	t.Fatalf("benchmarkTaskModelComparisons(...) missing label %q", label)
	return benchmarkTaskModelComparison{}
}

func benchmarkTestModelSummary(t *testing.T, summaries []benchmarkTaskModelSummary, provider string, model string) benchmarkTaskModelSummary {
	t.Helper()
	for _, summary := range summaries {
		if summary.Provider == provider && summary.Model == model {
			return summary
		}
	}
	t.Fatalf("benchmarkTaskModelComparisons(...) missing model %s/%s", provider, model)
	return benchmarkTaskModelSummary{}
}

func benchmarkTestAdvisoryByModel(t *testing.T, advisories []benchmarkTaskModelAdvisory, model string) benchmarkTaskModelAdvisory {
	t.Helper()
	for _, advisory := range advisories {
		if advisory.ModelName() == model {
			return advisory
		}
	}
	t.Fatalf("benchmarkTaskModelAdvisories(...) missing model %s", model)
	return benchmarkTaskModelAdvisory{}
}

func benchmarkTestFloatClose(t *testing.T, name string, got float64, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
