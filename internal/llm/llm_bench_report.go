package llm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type benchmarkReportOptions struct {
	Latest bool
	JSON   bool
}

func runLLMBenchmarkReportCLI(args []string) {
	opts, err := parseBenchmarkReportOptions(args)
	if err != nil {
		fmt.Printf("Benchmark report argument error: %v\n", err)
		os.Exit(1)
	}

	files, err := benchmarkReportFiles(opts)
	if err != nil {
		fmt.Printf("Benchmark report error: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Printf("No benchmark records found under %s\n", benchmarkRecordsDir())
		return
	}

	records, err := loadBenchmarkRecords(files)
	if err != nil {
		fmt.Printf("Benchmark report error: %v\n", err)
		os.Exit(1)
	}
	if len(records) == 0 {
		fmt.Printf("No benchmark records found under %s\n", benchmarkRecordsDir())
		return
	}

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(records); err != nil {
			fmt.Printf("Benchmark report JSON output error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Printf("Loaded %d benchmark records from %d file(s).\n", len(records), len(files))
	printBenchmarkRunSummary(records)
	printBenchmarkModelTable(records)
}

func RunLLMBenchmarkReportCLI(args []string) {
	runLLMBenchmarkReportCLI(args)
}

func parseBenchmarkReportOptions(args []string) (benchmarkReportOptions, error) {
	var opts benchmarkReportOptions
	for _, arg := range args {
		switch arg {
		case "--latest":
			opts.Latest = true
		case "--json":
			opts.JSON = true
		default:
			return opts, fmt.Errorf("unknown benchmark report option %q", arg)
		}
	}
	return opts, nil
}

func benchmarkReportFiles(opts benchmarkReportOptions) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(benchmarkRecordsDir(), "llm-bench-*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	if opts.Latest && len(matches) > 1 {
		return matches[len(matches)-1:], nil
	}
	return matches, nil
}

func benchmarkRecordsDir() string {
	return filepath.Join(defaultRuntimeDir(), "benchmarks")
}

func loadBenchmarkRecords(files []string) ([]llmBenchmarkRunRecord, error) {
	var records []llmBenchmarkRunRecord
	for _, path := range files {
		fileRecords, err := loadBenchmarkRecordsFile(path)
		if err != nil {
			return nil, err
		}
		records = append(records, fileRecords...)
	}
	return records, nil
}

func loadBenchmarkRecordsFile(path string) ([]llmBenchmarkRunRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	var records []llmBenchmarkRunRecord
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record llmBenchmarkRunRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("parse %s:%d: %w", path, lineNo, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return records, nil
}

func printBenchmarkModelTable(records []llmBenchmarkRunRecord) {
	summaries := benchmarkModelSummaries(records)
	if len(summaries) == 0 {
		return
	}

	fmt.Println("\nModels:")
	for _, summary := range summaries {
		fmt.Printf(
			"  %-32s score=%5.1f avgLatency=%dms ok=%d errors=%d\n",
			summary.Model,
			summary.AvgScore,
			summary.AvgLatencyMS,
			summary.OK,
			summary.Errors,
		)
	}
}
