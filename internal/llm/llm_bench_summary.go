package llm

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/wallentx/jobscout/internal/cliui"
)

func printBenchmarkRecordSummary(record llmBenchmarkRunRecord) {
	fmt.Print(formatBenchmarkRecordSummary(record))
}

func formatBenchmarkRecordSummary(record llmBenchmarkRunRecord) string {
	var out strings.Builder
	status := cliui.Style("ok", cliui.Green, cliui.Bold)
	if record.Error != "" {
		status = cliui.Style("error", cliui.Red, cliui.Bold)
	}
	model := formatBenchmarkString(record.Model)
	fmt.Fprintf(&out, "  %s %s\n", status, cliui.Style(truncateBenchmarkColumn(model, 60), cliui.Bold))
	fmt.Fprintf(&out, "     %s %s\n", cliui.Style("case:", cliui.Dim), truncateBenchmarkColumn(record.CaseID, 68))
	fmt.Fprintf(
		&out,
		"     %s %s  %s %s  %s %s\n",
		cliui.Style("task:", cliui.Dim),
		normalizeBenchmarkTaskName(record.Task),
		cliui.Style("score:", cliui.Dim),
		colorBenchmarkScore(record.FinalScore),
		cliui.Style("latency:", cliui.Dim),
		formatBenchmarkDurationMS(record.LatencyMS),
	)
	fmt.Fprintf(
		&out,
		"     %s %s  %s %s\n",
		cliui.Style("json:", cliui.Dim),
		colorBenchmarkBool(record.JSONValid),
		cliui.Style("fields:", cliui.Dim),
		colorBenchmarkBool(record.RequiredFieldsPresent),
	)
	if record.Error != "" {
		fmt.Fprintf(&out, "     %s %s\n", cliui.Style("error:", cliui.Red), truncateBenchmarkColumn(record.Error, 88))
	}
	return out.String()
}

func printBenchmarkRunSummary(records []llmBenchmarkRunRecord) {
	if len(records) == 0 {
		return
	}

	fmt.Printf("\n%s\n", cliui.Style("Summary", cliui.Cyan, cliui.Bold))
	printBenchmarkTaskModelComparisons(records)
	printBenchmarkTaskModelAdvisories(records)
	printBenchmarkErrorSummary(records, 20)

	models := benchmarkModelSummaries(records)
	if len(models) == 0 {
		return
	}
	best := models[0]
	fmt.Printf(
		"%s %s score=%s avgLatency=%dms ok=%d errors=%d\n",
		cliui.Style("Best average successful model:", cliui.Bold),
		cliui.Style(best.Model, cliui.Green),
		colorBenchmarkScore(best.AvgScore),
		best.AvgLatencyMS,
		best.OK,
		best.Errors,
	)
}

func printBenchmarkErrorSummary(records []llmBenchmarkRunRecord, limit int) {
	errors := benchmarkErrorSummaries(records)
	if len(errors) == 0 {
		return
	}
	if limit > 0 && len(errors) > limit {
		errors = errors[:limit]
	}
	fmt.Printf("\n%s\n", cliui.Style("Errors:", cliui.Red, cliui.Bold))
	for _, summary := range errors {
		fmt.Printf(
			"  %4d %-24s %-34s %s\n",
			summary.Count,
			truncateBenchmarkColumn(normalizeBenchmarkTaskName(summary.Task), 24),
			truncateBenchmarkColumn(summary.ModelName(), 34),
			truncateBenchmarkColumn(summary.Error, 140),
		)
	}
}

func printBenchmarkTaskModelComparisons(records []llmBenchmarkRunRecord) {
	comparisons := benchmarkTaskModelComparisons(records)
	if len(comparisons) == 0 {
		return
	}

	fmt.Println(cliui.Style("Task model comparisons:", cliui.Cyan, cliui.Bold))
	for _, comparison := range comparisons {
		fmt.Printf("\n%s %s\n", cliui.Style(comparison.Label, cliui.Bold), cliui.Style("("+comparison.TaskKey+")", cliui.Dim))
		if len(comparison.Models) == 0 {
			fmt.Println("  No benchmark records yet for this task.")
			continue
		}
		if comparison.BestQuality.ModelName() != "" {
			fmt.Printf("  %s %s score=%s latency=%s tokens=%s cost=%s\n",
				cliui.Style("Best quality:", cliui.Bold),
				cliui.Style(comparison.BestQuality.ModelName(), cliui.Green),
				colorBenchmarkScore(comparison.BestQuality.AvgScore),
				formatBenchmarkDurationMS(comparison.BestQuality.AvgLatencyMS),
				formatBenchmarkInt(comparison.BestQuality.AvgTotalTokens),
				formatBenchmarkCost(comparison.BestQuality.AvgEstimatedCostUSD, comparison.BestQuality.CostRecords),
			)
		} else {
			fmt.Println("  Best quality: n/a")
		}
		if comparison.Fastest.ModelName() != "" {
			fmt.Printf("  %s %s latency=%s score=%s\n",
				cliui.Style("Fastest usable:", cliui.Bold),
				cliui.Style(comparison.Fastest.ModelName(), cliui.Green),
				formatBenchmarkDurationMS(comparison.Fastest.AvgLatencyMS),
				colorBenchmarkScore(comparison.Fastest.AvgScore),
			)
		}
		if comparison.LowestToken.ModelName() != "" {
			fmt.Printf("  %s %s tokens=%s score=%s\n",
				cliui.Style("Lowest token use:", cliui.Bold),
				cliui.Style(comparison.LowestToken.ModelName(), cliui.Green),
				formatBenchmarkInt(comparison.LowestToken.AvgTotalTokens),
				colorBenchmarkScore(comparison.LowestToken.AvgScore),
			)
		}
		if comparison.LowestCost.ModelName() != "" {
			fmt.Printf("  %s %s cost=%s score=%s\n",
				cliui.Style("Lowest estimated cost:", cliui.Bold),
				cliui.Style(comparison.LowestCost.ModelName(), cliui.Green),
				formatBenchmarkCost(comparison.LowestCost.AvgEstimatedCostUSD, comparison.LowestCost.CostRecords),
				colorBenchmarkScore(comparison.LowestCost.AvgScore),
			)
		}
		fmt.Println(cliui.Style("  model                              runs ok err score   acc  json ground speed cost stable latency avgTokens avgUSD parseFail missingFields commonError", cliui.Dim))
		for _, model := range comparison.Models {
			fmt.Printf(
				"  %-34s %4d %2d %3d %6.1f %5.1f %5.1f %6.1f %5.1f %4.1f %6.1f %7s %9s %7s %9d %13d %s\n",
				truncateBenchmarkColumn(model.ModelName(), 34),
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
				formatBenchmarkDurationMS(model.AvgLatencyMS),
				formatBenchmarkInt(model.AvgTotalTokens),
				formatBenchmarkCost(model.AvgEstimatedCostUSD, model.CostRecords),
				model.JSONFailures,
				model.MissingFields,
				truncateBenchmarkColumn(formatBenchmarkString(model.CommonError), 96),
			)
		}
	}
}

func printBenchmarkTaskModelAdvisories(records []llmBenchmarkRunRecord) {
	advisories := benchmarkTaskModelAdvisories(records)
	if len(advisories) == 0 {
		return
	}
	fmt.Printf("\n%s\n", cliui.Style("Not recommended:", cliui.Red, cliui.Bold))
	for _, advisory := range advisories {
		fmt.Printf(
			"  %-18s %-34s %s: %s\n",
			truncateBenchmarkColumn(advisory.Label, 18),
			truncateBenchmarkColumn(advisory.ModelName(), 34),
			cliui.Style(advisory.Recommendation, cliui.Red),
			advisory.Reason,
		)
	}
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

type benchmarkTaskModelGroup struct {
	Label   string
	TaskKey string
}

type benchmarkTaskModelComparison struct {
	Label       string
	TaskKey     string
	Models      []benchmarkTaskModelSummary
	BestQuality benchmarkTaskModelSummary
	Fastest     benchmarkTaskModelSummary
	LowestToken benchmarkTaskModelSummary
	LowestCost  benchmarkTaskModelSummary
}

type benchmarkTaskModelAdvisory struct {
	Label          string
	TaskKey        string
	Provider       string
	Model          string
	Total          int
	OK             int
	Errors         int
	AvgScore       float64
	Recommendation string
	Reason         string
}

type benchmarkTaskModelSummary struct {
	Provider            string
	Model               string
	Total               int
	OK                  int
	Errors              int
	JSONFailures        int
	MissingFields       int
	AvgScore            float64
	AvgAccuracyScore    float64
	AvgJSONScore        float64
	AvgGroundingScore   float64
	AvgSpeedScore       float64
	AvgCostScore        float64
	AvgStabilityScore   float64
	AvgLatencyMS        int64
	AvgInputTokens      int
	AvgOutputTokens     int
	AvgTotalTokens      int
	AvgEstimatedCostUSD float64
	CostRecords         int
	CommonError         string
}

const (
	benchmarkAdvisoryAvoid    = "Avoid for this task"
	benchmarkAvoidScoreCutoff = 75.0
)

type benchmarkErrorSummary struct {
	Task     string
	Provider string
	Model    string
	Error    string
	Count    int
}

func (s benchmarkErrorSummary) ModelName() string {
	provider := strings.TrimSpace(s.Provider)
	model := strings.TrimSpace(s.Model)
	if provider == "" {
		return model
	}
	if model == "" {
		return provider
	}
	return provider + "/" + model
}

func (s benchmarkTaskModelSummary) ModelName() string {
	provider := strings.TrimSpace(s.Provider)
	model := strings.TrimSpace(s.Model)
	if provider == "" {
		return model
	}
	if model == "" {
		return provider
	}
	return provider + "/" + model
}

func (s benchmarkTaskModelAdvisory) ModelName() string {
	provider := strings.TrimSpace(s.Provider)
	model := strings.TrimSpace(s.Model)
	if provider == "" {
		return model
	}
	if model == "" {
		return provider
	}
	return provider + "/" + model
}

func benchmarkErrorSummaries(records []llmBenchmarkRunRecord) []benchmarkErrorSummary {
	byKey := make(map[string]*benchmarkErrorSummary)
	for _, record := range records {
		message := strings.TrimSpace(record.Error)
		if message == "" {
			continue
		}
		task := normalizeBenchmarkTaskName(record.Task)
		key := strings.ToLower(task) + "\x00" + benchmarkModelKey(record.Provider, record.Model) + "\x00" + message
		summary := byKey[key]
		if summary == nil {
			summary = &benchmarkErrorSummary{
				Task:     task,
				Provider: strings.TrimSpace(record.Provider),
				Model:    strings.TrimSpace(record.Model),
				Error:    message,
			}
			byKey[key] = summary
		}
		summary.Count++
	}
	out := make([]benchmarkErrorSummary, 0, len(byKey))
	for _, summary := range byKey {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			if out[i].Task == out[j].Task {
				if out[i].ModelName() == out[j].ModelName() {
					return out[i].Error < out[j].Error
				}
				return out[i].ModelName() < out[j].ModelName()
			}
			return out[i].Task < out[j].Task
		}
		return out[i].Count > out[j].Count
	})
	return out
}

var benchmarkTaskModelGroups = []benchmarkTaskModelGroup{
	{
		Label:   "Resume parsing",
		TaskKey: llmTaskResumeCriteria,
	},
	{
		Label:   "Job search",
		TaskKey: llmTaskJobSearch,
	},
	{
		Label:   "Job filtering",
		TaskKey: llmTaskFiltering,
	},
	{
		Label:   "Job enrichment",
		TaskKey: llmTaskJobIdentity,
	},
	{
		Label:   "Company Health",
		TaskKey: llmTaskCompanyHealth,
	},
}

func benchmarkTaskModelComparisons(records []llmBenchmarkRunRecord) []benchmarkTaskModelComparison {
	out := make([]benchmarkTaskModelComparison, 0, len(benchmarkTaskModelGroups))
	for _, group := range benchmarkTaskModelGroups {
		groupRecords := filterBenchmarkRecordsForTask(records, group.TaskKey)
		models := summarizeBenchmarkTaskModels(groupRecords)
		comparison := benchmarkTaskModelComparison{
			Label:   group.Label,
			TaskKey: group.TaskKey,
			Models:  models,
		}
		comparison.BestQuality = bestQualityBenchmarkTaskModel(models)
		comparison.Fastest = fastestBenchmarkTaskModel(models)
		comparison.LowestToken = lowestTokenBenchmarkTaskModel(models)
		comparison.LowestCost = lowestCostBenchmarkTaskModel(models)
		out = append(out, comparison)
	}
	return out
}

func benchmarkTaskModelAdvisories(records []llmBenchmarkRunRecord) []benchmarkTaskModelAdvisory {
	comparisons := benchmarkTaskModelComparisons(records)
	advisories := make([]benchmarkTaskModelAdvisory, 0)
	for _, comparison := range comparisons {
		for _, model := range comparison.Models {
			reason, ok := benchmarkTaskModelAvoidReason(model)
			if !ok {
				continue
			}
			advisories = append(advisories, benchmarkTaskModelAdvisory{
				Label:          comparison.Label,
				TaskKey:        comparison.TaskKey,
				Provider:       model.Provider,
				Model:          model.Model,
				Total:          model.Total,
				OK:             model.OK,
				Errors:         model.Errors,
				AvgScore:       model.AvgScore,
				Recommendation: benchmarkAdvisoryAvoid,
				Reason:         reason,
			})
		}
	}
	return advisories
}

func benchmarkTaskModelAvoidReason(model benchmarkTaskModelSummary) (string, bool) {
	reasons := make([]string, 0, 2)
	if model.OK == 0 {
		reasons = append(reasons, "no successful runs")
	} else if model.AvgScore < benchmarkAvoidScoreCutoff {
		reasons = append(reasons, fmt.Sprintf("average score %.1f below %.1f", model.AvgScore, benchmarkAvoidScoreCutoff))
	}
	if model.Errors > 0 {
		reasons = append(reasons, fmt.Sprintf("%d/%d runs failed", model.Errors, model.Total))
	}
	if len(reasons) == 0 {
		return "", false
	}
	return strings.Join(reasons, "; "), true
}

func filterBenchmarkRecordsForTask(records []llmBenchmarkRunRecord, task string) []llmBenchmarkRunRecord {
	task = normalizeBenchmarkTaskName(task)
	out := make([]llmBenchmarkRunRecord, 0)
	for _, record := range records {
		if normalizeBenchmarkTaskName(record.Task) != task {
			continue
		}
		record.Task = task
		out = append(out, record)
	}
	return out
}

func summarizeBenchmarkTaskModels(records []llmBenchmarkRunRecord) []benchmarkTaskModelSummary {
	type accumulator struct {
		summary        benchmarkTaskModelSummary
		score          float64
		accuracy       float64
		jsonScore      float64
		groundingScore float64
		speedScore     float64
		costScore      float64
		stabilityScore float64
		latencyMS      int64
		latencyRecords int
		inputTokens    int
		outputTokens   int
		totalTokens    int
		tokenRecords   int
		cost           float64
		errors         map[string]int
	}
	byModel := make(map[string]*accumulator)
	for _, record := range records {
		key := benchmarkModelKey(record.Provider, record.Model)
		acc := byModel[key]
		if acc == nil {
			acc = &accumulator{}
			acc.summary.Provider = strings.TrimSpace(record.Provider)
			acc.summary.Model = strings.TrimSpace(record.Model)
			byModel[key] = acc
		}
		acc.summary.Total++
		if record.Error != "" {
			acc.summary.Errors++
			if acc.errors == nil {
				acc.errors = make(map[string]int)
			}
			acc.errors[record.Error]++
			continue
		}
		acc.summary.OK++
		acc.score += record.FinalScore
		acc.accuracy += float64(record.AccuracyScore)
		acc.jsonScore += float64(record.JSONScore)
		acc.groundingScore += float64(record.GroundingScore)
		acc.speedScore += float64(record.SpeedScore)
		acc.costScore += float64(record.CostScore)
		acc.stabilityScore += float64(record.StabilityScore)
		if !record.JSONValid {
			acc.summary.JSONFailures++
		}
		if !record.RequiredFieldsPresent {
			acc.summary.MissingFields++
		}
		if record.LatencyMS > 0 {
			acc.latencyMS += record.LatencyMS
			acc.latencyRecords++
		}
		if totalTokens, ok := benchmarkTotalTokens(record); ok {
			acc.totalTokens += totalTokens
			acc.tokenRecords++
		}
		if record.InputTokens != nil {
			acc.inputTokens += *record.InputTokens
		}
		if record.OutputTokens != nil {
			acc.outputTokens += *record.OutputTokens
		}
		if cost, ok := benchmarkRecordCostUSD(record); ok {
			acc.cost += cost
			acc.summary.CostRecords++
		}
	}

	summaries := make([]benchmarkTaskModelSummary, 0, len(byModel))
	for _, acc := range byModel {
		acc.summary.CommonError = commonBenchmarkError(acc.errors)
		if acc.summary.OK > 0 {
			ok := float64(acc.summary.OK)
			acc.summary.AvgScore = acc.score / ok
			acc.summary.AvgAccuracyScore = acc.accuracy / ok
			acc.summary.AvgJSONScore = acc.jsonScore / ok
			acc.summary.AvgGroundingScore = acc.groundingScore / ok
			acc.summary.AvgSpeedScore = acc.speedScore / ok
			acc.summary.AvgCostScore = acc.costScore / ok
			acc.summary.AvgStabilityScore = acc.stabilityScore / ok
			if acc.latencyRecords > 0 {
				acc.summary.AvgLatencyMS = acc.latencyMS / int64(acc.latencyRecords)
			}
			if acc.tokenRecords > 0 {
				acc.summary.AvgInputTokens = int(math.Round(float64(acc.inputTokens) / float64(acc.tokenRecords)))
				acc.summary.AvgOutputTokens = int(math.Round(float64(acc.outputTokens) / float64(acc.tokenRecords)))
				acc.summary.AvgTotalTokens = int(math.Round(float64(acc.totalTokens) / float64(acc.tokenRecords)))
			}
			if acc.summary.CostRecords > 0 {
				acc.summary.AvgEstimatedCostUSD = acc.cost / float64(acc.summary.CostRecords)
			}
		}
		summaries = append(summaries, acc.summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].AvgScore == summaries[j].AvgScore {
			if summaries[i].AvgLatencyMS == summaries[j].AvgLatencyMS {
				return summaries[i].ModelName() < summaries[j].ModelName()
			}
			return summaries[i].AvgLatencyMS < summaries[j].AvgLatencyMS
		}
		return summaries[i].AvgScore > summaries[j].AvgScore
	})
	return summaries
}

func bestQualityBenchmarkTaskModel(models []benchmarkTaskModelSummary) benchmarkTaskModelSummary {
	for _, model := range models {
		if model.OK > 0 {
			return model
		}
	}
	return benchmarkTaskModelSummary{}
}

func benchmarkModelKey(provider string, model string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + "\x00" + strings.ToLower(strings.TrimSpace(model))
}

func commonBenchmarkError(errors map[string]int) string {
	best := ""
	bestCount := 0
	for message, count := range errors {
		if count > bestCount || (count == bestCount && message < best) {
			best = message
			bestCount = count
		}
	}
	return best
}

func benchmarkTotalTokens(record llmBenchmarkRunRecord) (int, bool) {
	if record.Details != nil {
		if total, ok := benchmarkIntFromAny(record.Details["total_tokens"]); ok && total > 0 {
			return total, true
		}
	}
	if record.InputTokens != nil && record.OutputTokens != nil {
		return *record.InputTokens + *record.OutputTokens, true
	}
	return 0, false
}

func benchmarkIntFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed), true
		}
	}
	return 0, false
}

func fastestBenchmarkTaskModel(models []benchmarkTaskModelSummary) benchmarkTaskModelSummary {
	var best benchmarkTaskModelSummary
	bestSet := false
	for _, model := range models {
		if model.OK == 0 || model.AvgLatencyMS <= 0 {
			continue
		}
		if !bestSet || model.AvgLatencyMS < best.AvgLatencyMS || (model.AvgLatencyMS == best.AvgLatencyMS && model.AvgScore > best.AvgScore) {
			best = model
			bestSet = true
		}
	}
	return best
}

func lowestTokenBenchmarkTaskModel(models []benchmarkTaskModelSummary) benchmarkTaskModelSummary {
	var best benchmarkTaskModelSummary
	bestSet := false
	for _, model := range models {
		if model.OK == 0 || model.AvgTotalTokens <= 0 {
			continue
		}
		if !bestSet || model.AvgTotalTokens < best.AvgTotalTokens || (model.AvgTotalTokens == best.AvgTotalTokens && model.AvgScore > best.AvgScore) {
			best = model
			bestSet = true
		}
	}
	return best
}

func lowestCostBenchmarkTaskModel(models []benchmarkTaskModelSummary) benchmarkTaskModelSummary {
	var best benchmarkTaskModelSummary
	bestSet := false
	for _, model := range models {
		if model.OK == 0 || model.CostRecords == 0 || model.AvgEstimatedCostUSD <= 0 {
			continue
		}
		if !bestSet || model.AvgEstimatedCostUSD < best.AvgEstimatedCostUSD || (model.AvgEstimatedCostUSD == best.AvgEstimatedCostUSD && model.AvgScore > best.AvgScore) {
			best = model
			bestSet = true
		}
	}
	return best
}

func formatBenchmarkDurationMS(value int64) string {
	if value <= 0 {
		return "n/a"
	}
	return fmt.Sprintf("%dms", value)
}

func formatBenchmarkInt(value int) string {
	if value <= 0 {
		return "n/a"
	}
	return fmt.Sprintf("%d", value)
}

func formatBenchmarkCost(value float64, records int) string {
	if records == 0 || value <= 0 {
		return "n/a"
	}
	return fmt.Sprintf("$%.6f", value)
}

func formatBenchmarkString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "n/a"
	}
	return value
}

func colorBenchmarkScore(score float64) string {
	text := fmt.Sprintf("%.1f", score)
	switch {
	case score >= 85:
		return cliui.Style(text, cliui.Green)
	case score >= 60:
		return cliui.Style(text, cliui.Yellow)
	default:
		return cliui.Style(text, cliui.Red)
	}
}

func colorBenchmarkBool(ok bool) string {
	if ok {
		return cliui.Style("ok", cliui.Green)
	}
	return cliui.Style("no", cliui.Red)
}

func truncateBenchmarkColumn(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func benchmarkTaskSummaries(records []llmBenchmarkRunRecord) []benchmarkTaskSummary {
	byTask := make(map[string][]llmBenchmarkRunRecord)
	for _, record := range records {
		task := normalizeBenchmarkTaskName(record.Task)
		record.Task = task
		byTask[task] = append(byTask[task], record)
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
		if record.Error != "" {
			continue
		}
		cost, ok := benchmarkRecordCostUSD(record)
		if !ok {
			continue
		}
		summary.CostRecords++
		scorePerDollar := record.FinalScore / cost
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
