package llm

import (
	"encoding/json"
	"strings"
)

func normalizeBenchmarkRunScores(records []llmBenchmarkRunRecord) {
	fastestByTask := make(map[string]int64)
	cheapestByTask := make(map[string]float64)
	for _, record := range records {
		key := record.Task + "|" + record.CaseID
		if record.Error == "" && record.LatencyMS > 0 {
			if fastestByTask[key] == 0 || record.LatencyMS < fastestByTask[key] {
				fastestByTask[key] = record.LatencyMS
			}
		}
		if record.Error != "" || record.EstimatedCostUSD == nil || *record.EstimatedCostUSD <= 0 {
			continue
		}
		if cheapestByTask[key] == 0 || *record.EstimatedCostUSD < cheapestByTask[key] {
			cheapestByTask[key] = *record.EstimatedCostUSD
		}
	}

	for i := range records {
		key := records[i].Task + "|" + records[i].CaseID
		fastest := fastestByTask[key]
		if fastest <= 0 || records[i].LatencyMS <= 0 {
			records[i].SpeedScore = 0
		} else {
			score := int(float64(fastest) / float64(records[i].LatencyMS) * 100)
			if score < 0 {
				score = 0
			}
			if score > 100 {
				score = 100
			}
			records[i].SpeedScore = score
		}
		cheapest := cheapestByTask[key]
		if cheapest > 0 && records[i].EstimatedCostUSD != nil && *records[i].EstimatedCostUSD > 0 {
			score := int(cheapest / *records[i].EstimatedCostUSD * 100)
			if score < 0 {
				score = 0
			}
			if score > 100 {
				score = 100
			}
			records[i].CostScore = score
		}
		if records[i].Error != "" {
			continue
		}
		records[i].FinalScore = weightedBenchmarkScore(records[i])
		if records[i].ScoreCap > 0 && records[i].FinalScore > float64(records[i].ScoreCap) {
			records[i].FinalScore = float64(records[i].ScoreCap)
		}
	}
}

func scoreBenchmarkOutput(record *llmBenchmarkRunRecord, checks benchmarkChecks, output string) {
	var parsed map[string]any
	if checks.JSONRequired {
		record.JSONValid = json.Unmarshal([]byte(output), &parsed) == nil
	} else {
		record.JSONValid = true
	}

	record.JSONScore = boolScore(record.JSONValid)
	record.RequiredFieldsPresent = benchmarkRequiredFieldsPresent(parsed, checks.RequiredFields)
	accuracyScore, details := benchmarkAccuracyScore(parsed, checks, output)
	record.AccuracyScore = accuracyScore
	record.GroundingScore = benchmarkGroundingScore(checks, output)
	record.Details = details

	if checks.JSONRequired && !record.JSONValid {
		record.ScoreCap = 40
	}
	if record.JSONValid && !record.RequiredFieldsPresent {
		record.ScoreCap = 60
	}

	record.FinalScore = weightedBenchmarkScore(*record)
	if record.ScoreCap > 0 && record.FinalScore > float64(record.ScoreCap) {
		record.FinalScore = float64(record.ScoreCap)
	}
}

func benchmarkRequiredFieldsPresent(parsed map[string]any, required []string) bool {
	if len(required) == 0 {
		return true
	}
	if parsed == nil {
		return false
	}
	for _, field := range required {
		if _, ok := benchmarkValueAtPath(parsed, field); !ok {
			return false
		}
	}
	return true
}

func benchmarkAccuracyScore(parsed map[string]any, checks benchmarkChecks, output string) (int, map[string]any) {
	details := make(map[string]any)
	score := 100
	matches, total := benchmarkAccuracyMatches(parsed, checks)
	if total > 0 {
		score = int(float64(matches) / float64(total) * 100)
		details["accuracy_checks_matched"] = matches
		details["accuracy_checks_total"] = total
	}

	textScore := benchmarkTextCheckScore(output, checks.MustInclude, checks.MustNotInclude)
	hasTextChecks := len(checks.MustInclude)+len(checks.MustNotInclude) > 0
	if total > 0 && hasTextChecks {
		score = (score + textScore) / 2
	} else if hasTextChecks {
		score = textScore
	}
	return score, details
}

func benchmarkAccuracyMatches(parsed map[string]any, checks benchmarkChecks) (int, int) {
	matches := 0
	total := 0

	for key, expected := range checks.ExpectedValues {
		total++
		got, ok := benchmarkValueAtPath(parsed, key)
		if ok && valuesEquivalent(got, expected) {
			matches++
		}
	}
	for key, expectedItems := range checks.ExpectedContains {
		for _, expected := range expectedItems {
			total++
			got, ok := benchmarkValueAtPath(parsed, key)
			if ok && benchmarkValueContains(got, expected) {
				matches++
			}
		}
	}
	for key, allowedValues := range checks.EnumValues {
		total++
		got, ok := benchmarkValueAtPath(parsed, key)
		if ok && benchmarkValueInEnum(got, allowedValues) {
			matches++
		}
	}
	for key, minimum := range checks.NumericMinimums {
		total++
		got, ok := benchmarkValueAtPath(parsed, key)
		if !ok {
			continue
		}
		number, ok := benchmarkNumericValue(got)
		if ok && number >= minimum {
			matches++
		}
	}
	for key, maximum := range checks.NumericMaximums {
		total++
		got, ok := benchmarkValueAtPath(parsed, key)
		if !ok {
			continue
		}
		number, ok := benchmarkNumericValue(got)
		if ok && number <= maximum {
			matches++
		}
	}
	for key, numericRange := range checks.NumericRanges {
		total++
		got, ok := benchmarkValueAtPath(parsed, key)
		if !ok {
			continue
		}
		number, ok := benchmarkNumericValue(got)
		if ok && benchmarkNumericRangeMatches(number, numericRange) {
			matches++
		}
	}
	return matches, total
}

func benchmarkNumericRangeMatches(number float64, numericRange benchmarkNumericRange) bool {
	if numericRange.Min != nil && number < *numericRange.Min {
		return false
	}
	if numericRange.Max != nil && number > *numericRange.Max {
		return false
	}
	return numericRange.Min != nil || numericRange.Max != nil
}

func benchmarkValueInEnum(value any, allowedValues []any) bool {
	for _, allowed := range allowedValues {
		if valuesEquivalent(value, allowed) {
			return true
		}
	}
	return false
}

func benchmarkValueAtPath(parsed map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" || parsed == nil {
		return nil, false
	}
	current := any(parsed)
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = benchmarkMapValueCaseInsensitive(object, part)
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func benchmarkMapValueCaseInsensitive(values map[string]any, key string) (any, bool) {
	if value, ok := values[key]; ok {
		return value, true
	}
	normalizedKey := normalizeBenchmarkPathKey(key)
	for candidate, value := range values {
		if strings.EqualFold(candidate, key) || normalizeBenchmarkPathKey(candidate) == normalizedKey {
			return value, true
		}
	}
	return nil, false
}

func benchmarkValueContains(value any, expected any) bool {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if valuesEquivalent(item, expected) || benchmarkValueContains(item, expected) {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if valuesEquivalent(item, expected) || benchmarkValueContains(item, expected) {
				return true
			}
		}
	case string:
		expectedString, ok := expected.(string)
		return ok && strings.Contains(strings.ToLower(typed), strings.ToLower(expectedString))
	}
	return valuesEquivalent(value, expected)
}

func benchmarkNumericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	}
	return 0, false
}

func normalizeBenchmarkPathKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func benchmarkTextCheckScore(output string, mustInclude []string, mustNotInclude []string) int {
	total := len(mustInclude) + len(mustNotInclude)
	if total == 0 {
		return 100
	}

	output = strings.ToLower(output)
	passed := 0
	for _, value := range mustInclude {
		if strings.Contains(output, strings.ToLower(value)) {
			passed++
		}
	}
	for _, value := range mustNotInclude {
		if !strings.Contains(output, strings.ToLower(value)) {
			passed++
		}
	}
	return int(float64(passed) / float64(total) * 100)
}

func benchmarkGroundingScore(checks benchmarkChecks, output string) int {
	if len(checks.GroundingRules) == 0 {
		return 100
	}
	return benchmarkTextCheckScore(output, checks.GroundingRules, nil)
}

func valuesEquivalent(got any, want any) bool {
	gotString, gotStringOK := got.(string)
	wantString, wantStringOK := want.(string)
	if gotStringOK && wantStringOK {
		return strings.EqualFold(strings.TrimSpace(gotString), strings.TrimSpace(wantString))
	}
	gotBytes, gotErr := json.Marshal(got)
	wantBytes, wantErr := json.Marshal(want)
	if gotErr != nil || wantErr != nil {
		return false
	}
	return string(gotBytes) == string(wantBytes)
}

func boolScore(ok bool) int {
	if ok {
		return 100
	}
	return 0
}

func weightedBenchmarkScore(record llmBenchmarkRunRecord) float64 {
	weights := benchmarkWeightsForTask(record.Task)
	return float64(record.AccuracyScore)*weights.Accuracy +
		float64(record.JSONScore)*weights.JSON +
		float64(record.GroundingScore)*weights.Grounding +
		float64(record.SpeedScore)*weights.Speed +
		float64(record.CostScore)*weights.Cost +
		float64(record.StabilityScore)*weights.Stability
}

type benchmarkWeights struct {
	Accuracy  float64
	JSON      float64
	Grounding float64
	Speed     float64
	Cost      float64
	Stability float64
}

func benchmarkWeightsForTask(task string) benchmarkWeights {
	switch strings.ToLower(strings.TrimSpace(task)) {
	case "company_health_summary":
		return benchmarkWeights{Accuracy: 0.25, JSON: 0.25, Grounding: 0.30, Speed: 0.10, Cost: 0.05, Stability: 0.05}
	case "job_filter", "job_filter_batch":
		return benchmarkWeights{Accuracy: 0.40, JSON: 0.20, Grounding: 0.10, Speed: 0.10, Cost: 0.10, Stability: 0.10}
	case "resume_to_criteria":
		return benchmarkWeights{Accuracy: 0.35, JSON: 0.20, Grounding: 0.15, Speed: 0.10, Cost: 0.10, Stability: 0.10}
	case "autonomous_job_search":
		return benchmarkWeights{Accuracy: 0.35, JSON: 0.15, Grounding: 0.25, Speed: 0.05, Cost: 0.10, Stability: 0.10}
	case "browser_reputation_research":
		return benchmarkWeights{Accuracy: 0.30, JSON: 0.10, Grounding: 0.30, Speed: 0.05, Cost: 0.10, Stability: 0.15}
	case "benchmark_judge":
		return benchmarkWeights{Accuracy: 0.45, JSON: 0.10, Grounding: 0.20, Speed: 0.05, Cost: 0.05, Stability: 0.15}
	default:
		return benchmarkWeights{Accuracy: 0.35, JSON: 0.20, Grounding: 0.15, Speed: 0.10, Cost: 0.10, Stability: 0.10}
	}
}
