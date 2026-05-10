package llm

import (
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
			Task:             "job_filter",
			Model:            "cheap-good",
			FinalScore:       90,
			LatencyMS:        1000,
			JSONValid:        true,
			EstimatedCostUSD: &cheapCost,
		},
		{
			Task:             "job_filter",
			Model:            "expensive-best",
			FinalScore:       95,
			LatencyMS:        3000,
			JSONValid:        true,
			EstimatedCostUSD: &expensiveCost,
		},
		{
			Task:      "job_filter",
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
	if jobFilter.Task != "job_filter" {
		t.Fatalf("benchmarkTaskSummaries(...)[0].Task = %q, want job_filter", jobFilter.Task)
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
