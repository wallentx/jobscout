package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

func evaluateJobFilterBatchWithLLM(ctx context.Context, llm llms.Model, input benchmarkJobFilterBatchInput) (*jobFilterBatchOutput, error) {
	if len(input.Jobs) == 0 {
		return nil, fmt.Errorf("%s batch input has no jobs", llmTaskFiltering)
	}
	for _, entry := range input.Jobs {
		if strings.TrimSpace(entry.ID) == "" {
			return nil, fmt.Errorf("%s batch input contains a job with empty id", llmTaskFiltering)
		}
	}

	prompt := buildJobFilterBatchPrompt(input)
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a JSON-only API. You must return only valid JSON."),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}
	logDebug("job filter batch: generation start jobs=%d source=%q prompt_chars=%d", len(input.Jobs), input.Source, len(prompt))
	resp, err := llm.GenerateContent(ctx, messages, llmJSONCallOptions(0.1, 8192)...)
	if err != nil {
		logDebug("job filter batch: generation failed jobs=%d source=%q error=%v", len(input.Jobs), input.Source, err)
		return nil, fmt.Errorf("LLM generation failed: %v", err)
	}
	usage := ExtractTokenUsageFromContentResponse(resp)
	logDebug("job filter batch: jobs=%d source=%q token_usage %s", len(input.Jobs), input.Source, formatTokenUsageForLog(usage))
	if len(resp.Choices) == 0 {
		logDebug("job filter batch: generation returned no choices jobs=%d source=%q", len(input.Jobs), input.Source)
		return nil, fmt.Errorf("LLM returned no choices")
	}

	jsonStr := stripLLMJSON(resp.Choices[0].Content)
	var output jobFilterBatchOutput
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		return nil, fmt.Errorf("failed to parse LLM JSON output: %v", err)
	}
	if output.Results == nil {
		output.Results = make(map[string]LLMEvaluationResult)
	}
	output.TokenUsage = tokenUsagePtr(usage)
	return &output, nil
}

func buildJobFilterBatchPrompt(input benchmarkJobFilterBatchInput) string {
	payload, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		payload = []byte("{}")
	}

	var prompt strings.Builder
	prompt.WriteString("Evaluate each job independently against the criteria. ")
	prompt.WriteString("The jobs in this batch are from the same source; do not copy facts, compensation, remote status, or reasons between jobs. ")
	prompt.WriteString("Use only the fields supplied for each individual job. ")
	prompt.WriteString("Return one result for every job id.\n\n")
	prompt.WriteString("Return ONLY valid JSON in this shape:\n")
	prompt.WriteString(`{"results":{"<job_id>":{"matches":true,"compensation_extracted":"...","remote_eligibility":"...","why_it_matches":["..."]}}}` + "\n\n")
	prompt.WriteString("Input:\n")
	prompt.Write(payload)
	return prompt.String()
}
