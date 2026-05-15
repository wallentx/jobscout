package llm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wallentx/jobscout/internal/cliui"
)

type benchmarkReportOptions struct {
	Latest bool
	JSON   bool
	Format string
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

	switch opts.Format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(records); err != nil {
			fmt.Printf("Benchmark report JSON output error: %v\n", err)
			os.Exit(1)
		}
		return
	case "md":
		fmt.Print(benchmarkMarkdownReport(records, len(files)))
		return
	}

	fmt.Printf("%s %d benchmark records from %d file(s).\n", cliui.Style("Loaded", cliui.Cyan, cliui.Bold), len(records), len(files))
	printBenchmarkRunSummary(records)
	printBenchmarkModelTable(records)
}

func RunLLMBenchmarkReportCLI(args []string) {
	runLLMBenchmarkReportCLI(args)
}

func parseBenchmarkReportOptions(args []string) (benchmarkReportOptions, error) {
	var opts benchmarkReportOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--latest":
			opts.Latest = true
		case arg == "--json":
			if err := setBenchmarkReportFormat(&opts, "json"); err != nil {
				return opts, err
			}
		case arg == "--format":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--format requires one of: text, md, json")
			}
			if err := setBenchmarkReportFormat(&opts, args[i+1]); err != nil {
				return opts, err
			}
			i++
		case strings.HasPrefix(arg, "--format="):
			if err := setBenchmarkReportFormat(&opts, strings.TrimPrefix(arg, "--format=")); err != nil {
				return opts, err
			}
		default:
			return opts, fmt.Errorf("unknown benchmark report option %q", arg)
		}
	}
	if opts.Format == "" {
		opts.Format = "text"
	}
	return opts, nil
}

func setBenchmarkReportFormat(opts *benchmarkReportOptions, value string) error {
	format, err := normalizeBenchmarkReportFormat(value)
	if err != nil {
		return err
	}
	if opts.Format != "" && opts.Format != format {
		return fmt.Errorf("conflicting benchmark report output formats %q and %q", opts.Format, format)
	}
	opts.Format = format
	opts.JSON = format == "json"
	return nil
}

func normalizeBenchmarkReportFormat(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text", "plain":
		return "text", nil
	case "md", "markdown":
		return "md", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("--format requires one of: text, md, json")
	}
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

	fmt.Printf("\n%s\n", cliui.Style("Models:", cliui.Cyan, cliui.Bold))
	for _, summary := range summaries {
		fmt.Printf(
			"  %-32s score=%s avgLatency=%dms ok=%d errors=%d\n",
			truncateBenchmarkColumn(summary.Model, 32),
			colorBenchmarkScore(summary.AvgScore),
			summary.AvgLatencyMS,
			summary.OK,
			summary.Errors,
		)
	}
}

func benchmarkMarkdownReport(records []llmBenchmarkRunRecord, fileCount int) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# Jobscout LLM Benchmark Report\n\n")
	fmt.Fprintf(&out, "Loaded %d benchmark records from %d file(s).\n\n", len(records), fileCount)

	comparisons := benchmarkTaskModelComparisons(records)
	out.WriteString("## Task winners\n\n")
	out.WriteString("| Task | Best quality | Score | Fastest usable | Latency | Lowest token use | Avg tokens | Lowest estimated cost | Avg USD |\n")
	out.WriteString("| --- | --- | ---: | --- | ---: | --- | ---: | --- | ---: |\n")
	for _, comparison := range comparisons {
		bestQuality := "n/a"
		bestScore := "n/a"
		fastest := "n/a"
		fastestLatency := "n/a"
		lowestToken := "n/a"
		lowestTokenValue := "n/a"
		lowestCost := "n/a"
		lowestCostValue := "n/a"
		if comparison.BestQuality.ModelName() != "" {
			bestQuality = comparison.BestQuality.ModelName()
			bestScore = fmt.Sprintf("%.1f", comparison.BestQuality.AvgScore)
		}
		if comparison.Fastest.ModelName() != "" {
			fastest = comparison.Fastest.ModelName()
			fastestLatency = formatBenchmarkDurationMS(comparison.Fastest.AvgLatencyMS)
		}
		if comparison.LowestToken.ModelName() != "" {
			lowestToken = comparison.LowestToken.ModelName()
			lowestTokenValue = formatBenchmarkInt(comparison.LowestToken.AvgTotalTokens)
		}
		if comparison.LowestCost.ModelName() != "" {
			lowestCost = comparison.LowestCost.ModelName()
			lowestCostValue = formatBenchmarkCost(comparison.LowestCost.AvgEstimatedCostUSD, comparison.LowestCost.CostRecords)
		}
		fmt.Fprintf(
			&out,
			"| %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			benchmarkMarkdownCell(comparison.Label),
			benchmarkMarkdownCell(bestQuality),
			benchmarkMarkdownCell(bestScore),
			benchmarkMarkdownCell(fastest),
			benchmarkMarkdownCell(fastestLatency),
			benchmarkMarkdownCell(lowestToken),
			benchmarkMarkdownCell(lowestTokenValue),
			benchmarkMarkdownCell(lowestCost),
			benchmarkMarkdownCell(lowestCostValue),
		)
	}

	writeBenchmarkMarkdownAdvisories(&out, records)
	writeBenchmarkMarkdownErrorSummary(&out, records, 20)

	out.WriteString("\n## Task details\n")
	for _, comparison := range comparisons {
		fmt.Fprintf(&out, "\n## %s\n\n", benchmarkMarkdownHeading(comparison.Label))
		if len(comparison.Models) == 0 {
			out.WriteString("_No benchmark records yet for this task._\n")
			continue
		}
		out.WriteString("| Model | Runs | OK | Err | Score | Accuracy | JSON | Grounding | Speed | Cost | Stability | Latency | Avg tokens | Avg USD | Parse failures | Missing fields | Common error |\n")
		out.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n")
		for _, model := range comparison.Models {
			fmt.Fprintf(
				&out,
				"| %s | %d | %d | %d | %.1f | %.1f | %.1f | %.1f | %.1f | %.1f | %.1f | %s | %s | %s | %d | %d | %s |\n",
				benchmarkMarkdownCell(model.ModelName()),
				model.Total,
				model.OK,
				model.Errors,
				model.AvgScore,
				model.AvgAccuracyScore,
				model.AvgJSONScore,
				model.AvgGroundingScore,
				model.AvgSpeedScore,
				model.AvgCostScore,
				model.AvgStabilityScore,
				benchmarkMarkdownCell(formatBenchmarkDurationMS(model.AvgLatencyMS)),
				benchmarkMarkdownCell(formatBenchmarkInt(model.AvgTotalTokens)),
				benchmarkMarkdownCell(formatBenchmarkCost(model.AvgEstimatedCostUSD, model.CostRecords)),
				model.JSONFailures,
				model.MissingFields,
				benchmarkMarkdownCell(truncateBenchmarkColumn(model.CommonError, 180)),
			)
		}
	}
	return out.String()
}

func writeBenchmarkMarkdownAdvisories(out *strings.Builder, records []llmBenchmarkRunRecord) {
	advisories := benchmarkTaskModelAdvisories(records)
	if len(advisories) == 0 {
		return
	}
	out.WriteString("\n## Not recommended models\n\n")
	out.WriteString("| Task | Model | Recommendation | Reason |\n")
	out.WriteString("| --- | --- | --- | --- |\n")
	for _, advisory := range advisories {
		fmt.Fprintf(
			out,
			"| %s | %s | %s | %s |\n",
			benchmarkMarkdownCell(advisory.Label),
			benchmarkMarkdownCell(advisory.ModelName()),
			benchmarkMarkdownCell(advisory.Recommendation),
			benchmarkMarkdownCell(advisory.Reason),
		)
	}
}

func writeBenchmarkMarkdownErrorSummary(out *strings.Builder, records []llmBenchmarkRunRecord, limit int) {
	errors := benchmarkErrorSummaries(records)
	if len(errors) == 0 {
		return
	}
	if limit > 0 && len(errors) > limit {
		errors = errors[:limit]
	}
	out.WriteString("\n## Error summary\n\n")
	out.WriteString("| Count | Task | Model | Error |\n")
	out.WriteString("| ---: | --- | --- | --- |\n")
	for _, summary := range errors {
		fmt.Fprintf(
			out,
			"| %d | %s | %s | %s |\n",
			summary.Count,
			benchmarkMarkdownCode(summary.Task),
			benchmarkMarkdownCell(summary.ModelName()),
			benchmarkMarkdownCell(truncateBenchmarkColumn(summary.Error, 220)),
		)
	}
}

func benchmarkMarkdownCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "n/a"
	}
	return "`" + strings.ReplaceAll(value, "`", "'") + "`"
}

func benchmarkMarkdownHeading(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func benchmarkMarkdownCell(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.TrimSpace(value)
	if value == "" {
		return "n/a"
	}
	return value
}
