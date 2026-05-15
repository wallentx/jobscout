package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tmc/langchaingo/llms"
	"gopkg.in/yaml.v3"
)

func runLLMBenchmarkCase(
	ctx context.Context,
	llm llms.Model,
	provider string,
	model string,
	benchCase llmBenchmarkCase,
) llmBenchmarkRunRecord {
	record := llmBenchmarkRunRecord{
		Timestamp:             time.Now().Format(time.RFC3339),
		BenchmarkVersion:      benchmarkVersion,
		Provider:              provider,
		Model:                 model,
		Task:                  normalizeBenchmarkTaskName(benchCase.Task),
		CaseID:                benchCase.ID,
		RequiredFieldsPresent: true,
		SpeedScore:            100,
		CostScore:             100,
		StabilityScore:        100,
	}

	start := time.Now()
	output, usage, err := executeBenchmarkTask(ctx, llm, benchCase)
	record.LatencyMS = time.Since(start).Milliseconds()
	record.RawOutput = output
	if err != nil {
		record.Error = err.Error()
		record.JSONValid = false
		record.RequiredFieldsPresent = false
		record.ScoreCap = 40
		record.FinalScore = 0
		return record
	}

	scoreBenchmarkOutput(&record, benchCase.Checks, output)
	applyBenchmarkTokenUsage(&record, usage)
	return record
}

func executeBenchmarkTask(ctx context.Context, llm llms.Model, benchCase llmBenchmarkCase) (string, LLMTokenUsage, error) {
	switch normalizeBenchmarkTaskName(benchCase.Task) {
	case llmTaskFiltering:
		return executeBenchmarkJobFilteringTask(ctx, llm, benchCase)
	case "job_identity":
		var input benchmarkJobIdentityInput
		if err := yaml.Unmarshal(benchCase.Input, &input); err != nil {
			return "", LLMTokenUsage{}, fmt.Errorf("decode job_identity input: %w", err)
		}
		result, usage, err := enrichJobIdentityWithLLMUsage(ctx, llm, input.Job, input.Page)
		if err != nil {
			return "", usage, err
		}
		output, err := marshalBenchmarkOutput(benchmarkJobIdentityOutputFromEnrichment(result))
		return output, usage, err
	case llmTaskResumeCriteria:
		var input benchmarkResumeInput
		if err := yaml.Unmarshal(benchCase.Input, &input); err != nil {
			return "", LLMTokenUsage{}, fmt.Errorf("decode resume_to_criteria input: %w", err)
		}
		result, usage, err := evaluateResumeCriteriaWithLLMUsage(ctx, llm, input.ResumeText)
		if err != nil {
			return "", usage, err
		}
		output, err := marshalBenchmarkOutput(result)
		return output, usage, err
	case llmTaskCompanyHealth:
		var input benchmarkCompanyHealthInput
		if err := yaml.Unmarshal(benchCase.Input, &input); err != nil {
			return "", LLMTokenUsage{}, fmt.Errorf("decode %s input: %w", llmTaskCompanyHealth, err)
		}
		result, err := evaluateCompanyHealthWithLLM(ctx, llm, &input.Result)
		if err != nil {
			return "", LLMTokenUsage{}, err
		}
		output, err := marshalBenchmarkOutput(result)
		return output, usageFromCompanyHealthAssessment(result), err
	case llmTaskJobSearch:
		var input benchmarkJobSearchInput
		if err := yaml.Unmarshal(benchCase.Input, &input); err != nil {
			return "", LLMTokenUsage{}, fmt.Errorf("decode %s input: %w", llmTaskJobSearch, err)
		}
		jobs, usage, err := executeLLMSearchWithUsage(ctx, llm, input.Prompt)
		if err != nil {
			return "", usage, err
		}
		output, err := marshalBenchmarkOutput(benchmarkJobSearchOutput{
			Jobs:  jobs,
			Count: len(jobs),
		})
		return output, usage, err
	default:
		return "", LLMTokenUsage{}, fmt.Errorf("unsupported benchmark task %q", benchCase.Task)
	}
}

func executeBenchmarkJobFilteringTask(ctx context.Context, llm llms.Model, benchCase llmBenchmarkCase) (string, LLMTokenUsage, error) {
	var batchInput benchmarkJobFilterBatchInput
	if err := yaml.Unmarshal(benchCase.Input, &batchInput); err == nil && len(batchInput.Jobs) > 0 {
		result, err := evaluateJobFilterBatchWithLLM(ctx, llm, batchInput)
		if err != nil {
			return "", LLMTokenUsage{}, err
		}
		output, err := marshalBenchmarkOutput(result)
		return output, usageFromJobFilterBatchOutput(result), err
	}

	var input benchmarkJobFilterInput
	if err := yaml.Unmarshal(benchCase.Input, &input); err != nil {
		return "", LLMTokenUsage{}, fmt.Errorf("decode %s input: %w", llmTaskFiltering, err)
	}
	result, err := evaluateJobWithLLM(ctx, llm, input.Job, &input.Criteria)
	if err != nil {
		return "", LLMTokenUsage{}, err
	}
	output, err := marshalBenchmarkOutput(result)
	return output, usageFromLLMEvaluationResult(result), err
}

func applyBenchmarkTokenUsage(record *llmBenchmarkRunRecord, usage LLMTokenUsage) {
	if record == nil || !usage.Available() {
		return
	}
	record.InputTokens = cloneIntPtr(usage.InputTokens)
	record.OutputTokens = cloneIntPtr(usage.OutputTokens)
	if record.Details == nil {
		record.Details = make(map[string]any)
	}
	if usage.TotalTokens != nil {
		record.Details["total_tokens"] = *usage.TotalTokens
	}
	if usage.CachedTokens != nil {
		record.Details["cached_tokens"] = *usage.CachedTokens
	}
	if usage.ReasoningTokens != nil {
		record.Details["reasoning_tokens"] = *usage.ReasoningTokens
	}
	if usage.ThinkingTokens != nil {
		record.Details["thinking_tokens"] = *usage.ThinkingTokens
	}
	if cost, ok := benchmarkRecordCostUSD(*record); ok {
		record.EstimatedCostUSD = &cost
	}
}

func usageFromLLMEvaluationResult(result *LLMEvaluationResult) LLMTokenUsage {
	if result == nil || result.TokenUsage == nil {
		return LLMTokenUsage{}
	}
	return *result.TokenUsage
}

func usageFromJobFilterBatchOutput(result *jobFilterBatchOutput) LLMTokenUsage {
	if result == nil || result.TokenUsage == nil {
		return LLMTokenUsage{}
	}
	return *result.TokenUsage
}

func usageFromCompanyHealthAssessment(result *LLMCompanyHealthAssessment) LLMTokenUsage {
	if result == nil || result.TokenUsage == nil {
		return LLMTokenUsage{}
	}
	return *result.TokenUsage
}

func benchmarkJobIdentityOutputFromEnrichment(result *JobIdentityEnrichment) benchmarkJobIdentityOutput {
	if result == nil {
		return benchmarkJobIdentityOutput{}
	}
	return benchmarkJobIdentityOutput{
		CompanyWebsite:        result.CompanyWebsite,
		CompanySummary:        result.CompanySummary,
		CompanyIndustry:       result.CompanyIndustry,
		WebsiteConfidence:     result.WebsiteConfidence,
		SummaryConfidence:     result.SummaryConfidence,
		IndustryConfidence:    result.IndustryConfidence,
		IndustryProvisional:   result.IndustryProvisional,
		CompanyWebsiteReason:  result.CompanyWebsiteReason,
		CompanySummaryReason:  result.CompanySummaryReason,
		CompanyIndustryReason: result.CompanyIndustryReason,
	}
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func marshalBenchmarkOutput(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
