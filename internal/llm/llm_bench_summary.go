package llm

import (
	"fmt"
	"sort"
)

func printBenchmarkRecordSummary(record llmBenchmarkRunRecord) {
	status := "ok"
	if record.Error != "" {
		status = "error"
	}
	fmt.Printf(
		"  %s %-32s %-40s score=%5.1f json=%t fields=%t latency=%dms\n",
		status,
		record.Model,
		record.CaseID,
		record.FinalScore,
		record.JSONValid,
		record.RequiredFieldsPresent,
		record.LatencyMS,
	)
	if record.Error != "" {
		fmt.Printf("  error: %s\n", record.Error)
	}
}

func printBenchmarkRunSummary(records []llmBenchmarkRunRecord) {
	if len(records) == 0 {
		return
	}

	fmt.Println("\nSummary")
	fmt.Println("Best by task:")
	for _, summary := range benchmarkTaskSummaries(records) {
		valueText := "value=unavailable"
		if summary.BestValueModel != "" {
			valueText = fmt.Sprintf("value=%s scorePerDollar=%.0f", summary.BestValueModel, summary.BestValueScorePerDollar)
		}
		fmt.Printf(
			"  %-24s quality=%-32s score=%5.1f avgLatency=%dms parseFailures=%d/%d %s\n",
			summary.Task,
			summary.BestQualityModel,
			summary.BestQualityScore,
			summary.AvgLatencyMS,
			summary.ParseFailures,
			summary.Total,
			valueText,
		)
	}

	models := benchmarkModelSummaries(records)
	if len(models) == 0 {
		return
	}
	best := models[0]
	fmt.Printf(
		"Best average successful model: %s score=%5.1f avgLatency=%dms ok=%d errors=%d\n",
		best.Model,
		best.AvgScore,
		best.AvgLatencyMS,
		best.OK,
		best.Errors,
	)
}

type benchmarkTaskSummary struct {
	Task                    string
	Total                   int
	ParseFailures           int
	AvgLatencyMS            int64
	CostRecords             int
	BestQualityModel        string
	BestQualityScore        float64
	BestQualityLatencyMS    int64
	BestValueModel          string
	BestValueScorePerDollar float64
}

func benchmarkTaskSummaries(records []llmBenchmarkRunRecord) []benchmarkTaskSummary {
	byTask := make(map[string][]llmBenchmarkRunRecord)
	for _, record := range records {
		byTask[record.Task] = append(byTask[record.Task], record)
	}

	tasks := make([]string, 0, len(byTask))
	for task := range byTask {
		tasks = append(tasks, task)
	}
	sort.Strings(tasks)

	summaries := make([]benchmarkTaskSummary, 0, len(tasks))
	for _, task := range tasks {
		summaries = append(summaries, summarizeBenchmarkTask(task, byTask[task]))
	}
	return summaries
}

func summarizeBenchmarkTask(task string, records []llmBenchmarkRunRecord) benchmarkTaskSummary {
	summary := benchmarkTaskSummary{
		Task:  task,
		Total: len(records),
	}
	var latencySum int64
	var latencyCount int64
	var bestQualitySet bool
	for _, record := range records {
		if !record.JSONValid {
			summary.ParseFailures++
		}
		if record.LatencyMS > 0 {
			latencySum += record.LatencyMS
			latencyCount++
		}
		if record.Error == "" && (!bestQualitySet || record.FinalScore > summary.BestQualityScore) {
			bestQualitySet = true
			summary.BestQualityModel = record.Model
			summary.BestQualityScore = record.FinalScore
			summary.BestQualityLatencyMS = record.LatencyMS
		}
		if record.EstimatedCostUSD == nil || *record.EstimatedCostUSD <= 0 || record.Error != "" {
			continue
		}
		summary.CostRecords++
		scorePerDollar := record.FinalScore / *record.EstimatedCostUSD
		if summary.BestValueModel == "" || scorePerDollar > summary.BestValueScorePerDollar {
			summary.BestValueModel = record.Model
			summary.BestValueScorePerDollar = scorePerDollar
		}
	}
	if latencyCount > 0 {
		summary.AvgLatencyMS = latencySum / latencyCount
	}
	return summary
}

type benchmarkModelSummary struct {
	Model        string
	OK           int
	Errors       int
	AvgScore     float64
	AvgLatencyMS int64
}

func benchmarkModelSummaries(records []llmBenchmarkRunRecord) []benchmarkModelSummary {
	type accumulator struct {
		ok        int
		errors    int
		score     float64
		latencyMS int64
	}
	byModel := make(map[string]*accumulator)
	for _, record := range records {
		acc := byModel[record.Model]
		if acc == nil {
			acc = &accumulator{}
			byModel[record.Model] = acc
		}
		if record.Error != "" {
			acc.errors++
			continue
		}
		acc.ok++
		acc.score += record.FinalScore
		acc.latencyMS += record.LatencyMS
	}

	summaries := make([]benchmarkModelSummary, 0, len(byModel))
	for model, acc := range byModel {
		if acc.ok == 0 {
			summaries = append(summaries, benchmarkModelSummary{
				Model:  model,
				Errors: acc.errors,
			})
			continue
		}
		summaries = append(summaries, benchmarkModelSummary{
			Model:        model,
			OK:           acc.ok,
			Errors:       acc.errors,
			AvgScore:     acc.score / float64(acc.ok),
			AvgLatencyMS: acc.latencyMS / int64(acc.ok),
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].AvgScore == summaries[j].AvgScore {
			return summaries[i].AvgLatencyMS < summaries[j].AvgLatencyMS
		}
		return summaries[i].AvgScore > summaries[j].AvgScore
	})
	return summaries
}
