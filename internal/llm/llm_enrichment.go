package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

const maxCompanyHealthRejectedEvidenceInPrompt = 12

// LLMEvaluationResult holds the parsed JSON output from the LLM
type LLMEvaluationResult struct {
	Matches               bool           `json:"matches"`
	CompensationExtracted string         `json:"compensation_extracted"`
	RemoteEligibility     string         `json:"remote_eligibility"`
	WhyItMatches          []string       `json:"why_it_matches"`
	WhyRejected           []string       `json:"why_rejected"`
	TokenUsage            *LLMTokenUsage `json:"token_usage,omitempty"`
}

type jobIdentityLLMInput struct {
	Company          string `json:"company"`
	Title            string `json:"title"`
	ApplyURL         string `json:"apply_url"`
	Source           string `json:"source"`
	ExistingWebsite  string `json:"existing_company_website,omitempty"`
	ExistingSummary  string `json:"existing_company_summary,omitempty"`
	ExistingIndustry string `json:"existing_company_industry,omitempty"`
	PageURL          string `json:"page_url"`
	PageText         string `json:"page_text"`
}

type companyHealthLLMInput struct {
	Company              string                            `json:"company"`
	CompanyIdentity      map[string]any                    `json:"company_identity,omitempty"`
	Score                int                               `json:"score"`
	Confidence           string                            `json:"confidence"`
	Public               *bool                             `json:"public,omitempty"`
	FoundedYear          *int                              `json:"founded_year,omitempty"`
	AgeYears             *int                              `json:"age_years,omitempty"`
	EstimatedEmployees   *int                              `json:"estimated_employees,omitempty"`
	DiscoveredTicker     string                            `json:"discovered_ticker,omitempty"`
	EmploymentRisk       *EmploymentRisk                   `json:"employment_risk,omitempty"`
	Flags                []string                          `json:"flags,omitempty"`
	Notes                []string                          `json:"notes,omitempty"`
	LayoffHeadlines      []string                          `json:"layoff_headlines,omitempty"`
	HackerNewsHighlights []string                          `json:"hacker_news_highlights,omitempty"`
	EmployerReviews      []string                          `json:"employer_reviews,omitempty"`
	RejectedEvidence     []string                          `json:"rejected_evidence,omitempty"`
	RejectedOmitted      []rejectedEvidenceOmissionSummary `json:"rejected_evidence_omitted_summary,omitempty"`
}

type rejectedEvidenceOmissionSummary struct {
	Source          string `json:"source"`
	RejectionReason string `json:"rejection_reason"`
	Count           int    `json:"count"`
}

type companyHealthPromptStats struct {
	RejectedEvidenceTotal    int
	RejectedEvidenceIncluded int
	RejectedEvidenceOmitted  int
}

func initLLM(ctx context.Context, provider string, providerCfg LLMProviderConfig) (llms.Model, error) {
	modelName := providerCfg.Model
	switch strings.ToLower(provider) {
	case "openai":
		if os.Getenv("OPENAI_API_KEY") == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable is not set")
		}
		opts := []openai.Option{}
		if modelName != "" {
			opts = append(opts, openai.WithModel(modelName))
		}
		return openai.New(opts...)
	case "anthropic":
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set")
		}
		opts := []anthropic.Option{}
		if modelName != "" {
			opts = append(opts, anthropic.WithModel(modelName))
		}
		return anthropic.New(opts...)
	case "gemini", "googleai":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
		}
		opts := []googleai.Option{
			googleai.WithAPIKey(apiKey),
			googleai.WithDefaultMaxTokens(2048),
		}
		if modelName != "" {
			opts = append(opts, googleai.WithDefaultModel(modelName))
		}
		return googleai.New(ctx, opts...)
	case "openrouter":
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
		}
		opts := []openai.Option{
			openai.WithToken(apiKey),
			openai.WithBaseURL("https://openrouter.ai/api/v1"),
		}
		if endpoint := strings.TrimSpace(providerCfg.Endpoint); endpoint != "" {
			opts = append(opts, openai.WithBaseURL(endpoint))
		}
		if modelName != "" {
			opts = append(opts, openai.WithModel(modelName))
		}
		return openai.New(opts...)
	case "ollama":
		opts := []ollama.Option{}
		if modelName != "" {
			opts = append(opts, ollama.WithModel(modelName))
		}
		if endpoint := strings.TrimSpace(providerCfg.Endpoint); endpoint != "" {
			opts = append(opts, ollama.WithServerURL(endpoint))
		}
		return ollama.New(opts...)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s. Supported providers: openai, anthropic, gemini, openrouter, ollama", provider)
	}
}

func executeLLMSearch(ctx context.Context, llm llms.Model, prompt string) ([]Job, error) {
	// We append a JSON requirement so the Go app can parse the result,
	// but we do absolutely no "prompt engineering" on how or where it searches.
	fullPrompt := prompt + "\n\nCRITICAL SYSTEM INSTRUCTION: You must return your response ONLY as a valid JSON array of objects. Each object must include these string keys: \"company\", \"title\", \"remote\", \"compensation\", \"apply_url\", and \"description\". When available, include \"company_website\" for the actual company's website, not the job board or application URL, \"company_summary\" for a brief factual company summary, and \"company_industry\" for the company's industry. For any list of reasons it matches, put them in a string array under the key \"why_matches\". Do NOT wrap the JSON in markdown blocks, return ONLY the raw JSON array."

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, fullPrompt),
	}

	logDebug("llm job search: generation start prompt_chars=%d", len(fullPrompt))
	resp, err := llm.GenerateContent(ctx, messages, llms.WithTemperature(0.7), llms.WithMaxTokens(8192))
	if err != nil {
		logDebug("llm job search: generation failed: %v", err)
		return nil, fmt.Errorf("LLM generation failed: %v", err)
	}
	usage := ExtractTokenUsageFromContentResponse(resp)
	logLLMTokenUsage("llm job search", usage)

	if len(resp.Choices) == 0 {
		logDebug("llm job search: generation returned no choices")
		return nil, fmt.Errorf("LLM returned no choices")
	}

	jobs, err := parseLLMJobsJSON(resp.Choices[0].Content)
	if err != nil {
		logDebug("llm job search: parse failed response_chars=%d error=%v", len(resp.Choices[0].Content), err)
		return nil, err
	}
	logDebug("llm job search: parsed jobs=%d response_chars=%d", len(jobs), len(resp.Choices[0].Content))
	return jobs, nil
}

func ExecuteLLMSearch(ctx context.Context, llm llms.Model, prompt string) ([]Job, error) {
	return executeLLMSearch(ctx, llm, prompt)
}

const (
	maxJobIdentityPageTextRunes = 14000
	maxJobIdentityPromptChars   = 18000
	minJobIdentityPageTextRunes = 1000
)

func buildJobIdentityPrompt(job Job, page JobIdentityPage) string {
	pageTextRunes := maxJobIdentityPageTextRunes
	for {
		prompt := buildJobIdentityPromptWithPageText(job, page, truncateForLLMPrompt(page.Text, pageTextRunes))
		if len(prompt) <= maxJobIdentityPromptChars || pageTextRunes <= minJobIdentityPageTextRunes {
			return prompt
		}
		nextLimit := pageTextRunes * maxJobIdentityPromptChars / len(prompt)
		if nextLimit >= pageTextRunes {
			nextLimit = pageTextRunes - 1000
		}
		if nextLimit < minJobIdentityPageTextRunes {
			nextLimit = minJobIdentityPageTextRunes
		}
		pageTextRunes = nextLimit
	}
}

func buildJobIdentityPromptWithPageText(job Job, page JobIdentityPage, pageText string) string {
	input := jobIdentityLLMInput{
		Company:          job.Company,
		Title:            job.Title,
		ApplyURL:         job.ApplyURL,
		Source:           job.Source,
		ExistingWebsite:  job.CompanyWebsite,
		ExistingSummary:  job.CompanySummary,
		ExistingIndustry: job.CompanyIndustry,
		PageURL:          page.URL,
		PageText:         pageText,
	}
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		data = []byte("{}")
	}

	var prompt strings.Builder
	prompt.WriteString("Extract company identity facts for a job listing. ")
	prompt.WriteString("Use only the supplied page text and URLs. Do not browse or invent facts. ")
	prompt.WriteString("Prefer the actual company website, not the job board, ATS, application form, social profile, CDN asset, privacy page, or careers-only posting URL. ")
	prompt.WriteString("If a field is unavailable, return an empty string. ")
	prompt.WriteString("Use confidence values high, medium, or low. ")
	prompt.WriteString("Set industry_provisional to true when the industry is inferred rather than explicitly stated.\n\n")
	prompt.WriteString("Input JSON:\n")
	prompt.Write(data)
	prompt.WriteString("\n\nReturn ONLY valid JSON matching this schema:\n")
	prompt.WriteString("{\n")
	prompt.WriteString(`  "company_website": string,` + "\n")
	prompt.WriteString(`  "company_summary": string,` + "\n")
	prompt.WriteString(`  "company_industry": string,` + "\n")
	prompt.WriteString(`  "website_confidence": "high" | "medium" | "low" | "",` + "\n")
	prompt.WriteString(`  "summary_confidence": "high" | "medium" | "low" | "",` + "\n")
	prompt.WriteString(`  "industry_confidence": "high" | "medium" | "low" | "",` + "\n")
	prompt.WriteString(`  "industry_provisional": boolean,` + "\n")
	prompt.WriteString(`  "company_website_reason": string,` + "\n")
	prompt.WriteString(`  "company_summary_reason": string,` + "\n")
	prompt.WriteString(`  "company_industry_reason": string` + "\n")
	prompt.WriteString("}\n")
	return prompt.String()
}

func truncateForLLMPrompt(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "\n\n[truncated]"
}

func enrichJobIdentityWithLLM(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, error) {
	result, _, err := enrichJobIdentityWithLLMUsage(ctx, llm, job, page)
	return result, err
}

func enrichJobIdentityWithLLMUsage(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
	prompt := buildJobIdentityPrompt(job, page)
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a JSON-only extraction API. Return only valid JSON."),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	logDebug("llm job identity: generation start company=%q title=%q page_url=%q page_chars=%d prompt_chars=%d", job.Company, job.Title, page.URL, len(page.Text), len(prompt))
	resp, err := llm.GenerateContent(ctx, messages, llms.WithTemperature(0.1), llms.WithMaxTokens(2048))
	if err != nil {
		logDebug("llm job identity: generation failed company=%q title=%q error=%v", job.Company, job.Title, err)
		return nil, LLMTokenUsage{}, fmt.Errorf("LLM generation failed: %v", err)
	}
	usage := ExtractTokenUsageFromContentResponse(resp)
	logDebug(
		"llm job identity: company=%q title=%q token_usage %s",
		job.Company,
		job.Title,
		formatTokenUsageForLog(usage),
	)
	if len(resp.Choices) == 0 {
		logDebug("llm job identity: generation returned no choices company=%q title=%q", job.Company, job.Title)
		return nil, usage, fmt.Errorf("LLM returned no choices")
	}

	jsonStr := stripLLMJSON(resp.Choices[0].Content)
	var result JobIdentityEnrichment
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		logDebug("llm job identity: parse failed company=%q title=%q response_chars=%d error=%v", job.Company, job.Title, len(resp.Choices[0].Content), err)
		return nil, usage, fmt.Errorf("failed to parse LLM job identity JSON output: %v", err)
	}
	logDebug(
		"llm job identity: parsed company=%q title=%q website=%q summary_len=%d industry=%q",
		job.Company,
		job.Title,
		result.CompanyWebsite,
		len(strings.TrimSpace(result.CompanySummary)),
		result.CompanyIndustry,
	)
	return &result, usage, nil
}

func EnrichJobIdentityWithLLM(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, error) {
	return enrichJobIdentityWithLLM(ctx, llm, job, page)
}

func EnrichJobIdentityWithLLMUsage(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
	return enrichJobIdentityWithLLMUsage(ctx, llm, job, page)
}

func buildCompanyHealthLLMPrompt(result *CompanyHealthResult) string {
	prompt, _ := buildCompanyHealthLLMPromptWithStats(result)
	return prompt
}

func buildCompanyHealthLLMPromptWithStats(result *CompanyHealthResult) (string, companyHealthPromptStats) {
	input := companyHealthLLMInput{}
	stats := companyHealthPromptStats{}
	if result != nil {
		input.Company = result.Company
		if identity, ok := result.Sources["company_identity"].(map[string]string); ok {
			input.CompanyIdentity = map[string]any{
				"website":  identity["website"],
				"summary":  identity["summary"],
				"industry": identity["industry"],
			}
		} else if identity, ok := result.Sources["company_identity"].(map[string]any); ok {
			input.CompanyIdentity = identity
		}
		input.Score = result.Score
		input.Confidence = result.Confidence
		input.Public = result.Public
		input.FoundedYear = result.FoundedYear
		input.AgeYears = result.AgeYears
		input.EstimatedEmployees = result.EstimatedEmployees
		input.DiscoveredTicker = result.DiscoveredTicker
		input.EmploymentRisk = result.EmploymentRisk
		input.Flags = append([]string(nil), result.Flags...)
		input.Notes = append([]string(nil), result.Notes...)
		for _, signal := range result.LayoffSignals {
			input.LayoffHeadlines = append(input.LayoffHeadlines, signal.Title)
		}
		for _, signal := range result.HNSignals {
			input.HackerNewsHighlights = append(input.HackerNewsHighlights, fmt.Sprintf("%s (%d points, %d comments)", signal.Title, signal.Points, signal.NumComments))
		}
		for _, signal := range result.EmployerReviews {
			input.EmployerReviews = append(input.EmployerReviews, formatEmployerReviewSignal(signal))
		}
		input.RejectedEvidence, input.RejectedOmitted, stats = rejectedHealthEvidenceForPrompt(result.RejectedEvidence)
	}

	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		data = []byte("{}")
	}

	var prompt strings.Builder
	prompt.WriteString("Review this deterministic company health assessment for a job seeker. ")
	prompt.WriteString("Do not invent facts. Use only the supplied signals. ")
	prompt.WriteString("Do not rely on rejected evidence; it is included only to explain disambiguation decisions. ")
	prompt.WriteString("Explain whether this company looks worth further investigation as a potential employer.\n\n")
	prompt.WriteString("Assessment JSON:\n")
	prompt.Write(data)
	prompt.WriteString("\n\nReturn ONLY valid JSON matching this schema:\n")
	prompt.WriteString("{\n")
	prompt.WriteString(`  "summary": string,` + "\n")
	prompt.WriteString(`  "recommendation": string,` + "\n")
	prompt.WriteString(`  "risk_level": string,` + "\n")
	prompt.WriteString(`  "positive_signals": [string],` + "\n")
	prompt.WriteString(`  "concerns": [string],` + "\n")
	prompt.WriteString(`  "follow_up_questions": [string]` + "\n")
	prompt.WriteString("}\n")
	return prompt.String(), stats
}

func rejectedHealthEvidenceForPrompt(evidence []CompanyHealthEvidence) ([]string, []rejectedEvidenceOmissionSummary, companyHealthPromptStats) {
	stats := companyHealthPromptStats{RejectedEvidenceTotal: len(evidence)}
	includedCount := len(evidence)
	if includedCount > maxCompanyHealthRejectedEvidenceInPrompt {
		includedCount = maxCompanyHealthRejectedEvidenceInPrompt
	}
	stats.RejectedEvidenceIncluded = includedCount
	stats.RejectedEvidenceOmitted = len(evidence) - includedCount

	included := make([]string, 0, includedCount)
	for _, item := range evidence[:includedCount] {
		included = append(included, formatRejectedHealthEvidence(item))
	}
	if stats.RejectedEvidenceOmitted == 0 {
		return included, nil, stats
	}

	type omissionKey struct {
		source string
		reason string
	}
	counts := make(map[omissionKey]int)
	for _, item := range evidence[includedCount:] {
		key := omissionKey{
			source: normalizedRejectedEvidenceSource(item.Source),
			reason: normalizedRejectedEvidenceReason(item.Reason),
		}
		counts[key]++
	}

	keys := make([]omissionKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source == keys[j].source {
			return keys[i].reason < keys[j].reason
		}
		return keys[i].source < keys[j].source
	})

	summary := make([]rejectedEvidenceOmissionSummary, 0, len(keys))
	for _, key := range keys {
		summary = append(summary, rejectedEvidenceOmissionSummary{
			Source:          key.source,
			RejectionReason: key.reason,
			Count:           counts[key],
		})
	}
	return included, summary, stats
}

func normalizedRejectedEvidenceSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "unknown"
	}
	return source
}

func normalizedRejectedEvidenceReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "unspecified"
	}
	return reason
}

func formatRejectedHealthEvidence(evidence CompanyHealthEvidence) string {
	parts := []string{evidence.Source}
	if evidence.Value != "" {
		parts = append(parts, evidence.Value)
	}
	if evidence.Reason != "" {
		parts = append(parts, "rejected: "+evidence.Reason)
	}
	if evidence.URL != "" {
		parts = append(parts, evidence.URL)
	}
	return strings.Join(parts, " | ")
}

func formatEmployerReviewSignal(signal EmployerReviewSignal) string {
	parts := []string{signal.Source}
	if signal.Rating != "" {
		parts = append(parts, "rating "+signal.Rating)
	}
	if signal.Title != "" {
		parts = append(parts, signal.Title)
	}
	if signal.Snippet != "" {
		parts = append(parts, signal.Snippet)
	}
	if len(signal.Flags) > 0 {
		parts = append(parts, "flags: "+strings.Join(signal.Flags, ", "))
	}
	if signal.URL != "" {
		parts = append(parts, signal.URL)
	}
	return strings.Join(parts, " | ")
}

func evaluateCompanyHealthWithLLM(ctx context.Context, llm llms.Model, result *CompanyHealthResult) (*LLMCompanyHealthAssessment, error) {
	prompt, promptStats := buildCompanyHealthLLMPromptWithStats(result)
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a JSON-only API. You must return only valid JSON."),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	company := ""
	if result != nil {
		company = result.Company
	}
	logDebug(
		"llm company health: generation start company=%q prompt_chars=%d rejected_evidence_total=%d rejected_evidence_included=%d rejected_evidence_omitted=%d",
		company,
		len(prompt),
		promptStats.RejectedEvidenceTotal,
		promptStats.RejectedEvidenceIncluded,
		promptStats.RejectedEvidenceOmitted,
	)
	resp, err := llm.GenerateContent(ctx, messages, llms.WithTemperature(0.1), llms.WithMaxTokens(2048))
	if err != nil {
		logDebug("llm company health: generation failed company=%q error=%v", company, err)
		return nil, fmt.Errorf("LLM generation failed: %v", err)
	}
	usage := ExtractTokenUsageFromContentResponse(resp)
	logDebug("llm company health: company=%q token_usage %s", company, formatTokenUsageForLog(usage))
	if len(resp.Choices) == 0 {
		logDebug("llm company health: generation returned no choices company=%q", company)
		return nil, fmt.Errorf("LLM returned no choices")
	}

	jsonStr := stripLLMJSON(resp.Choices[0].Content)
	var assessment LLMCompanyHealthAssessment
	if err := json.Unmarshal([]byte(jsonStr), &assessment); err != nil {
		logDebug("llm company health: parse failed company=%q response_chars=%d error=%v", company, len(resp.Choices[0].Content), err)
		return nil, fmt.Errorf("failed to parse LLM company health JSON output: %v", err)
	}
	assessment.TokenUsage = tokenUsagePtr(usage)
	logDebug(
		"llm company health: parsed company=%q risk=%q positive=%d concerns=%d followups=%d",
		company,
		assessment.RiskLevel,
		len(assessment.PositiveSignals),
		len(assessment.Concerns),
		len(assessment.FollowUpQuestions),
	)
	return &assessment, nil
}

func EvaluateCompanyHealthWithLLM(ctx context.Context, llm llms.Model, result *CompanyHealthResult) (*LLMCompanyHealthAssessment, error) {
	return evaluateCompanyHealthWithLLM(ctx, llm, result)
}

func buildPrompt(job Job, criteria *CriteriaConfig) string {
	var promptBuilder strings.Builder

	evalPromptBytes, err := os.ReadFile("EVAL_PROMPT.md")
	if err == nil {
		promptBuilder.WriteString(string(evalPromptBytes))
		promptBuilder.WriteString("\n\n")
	} else {
		// Fallback if EVAL_PROMPT.md is missing
		promptBuilder.WriteString("Evaluate this job posting against the user's criteria. ")
	}

	promptBuilder.WriteString("### User Criteria:\n")
	promptBuilder.WriteString(formatCriteriaForLLMPrompt(criteria))
	promptBuilder.WriteString("\n")

	promptBuilder.WriteString("### Job Posting to Evaluate:\n")
	fmt.Fprintf(&promptBuilder, "Company: %s\n", job.Company)
	fmt.Fprintf(&promptBuilder, "Title: %s\n", job.Title)
	fmt.Fprintf(&promptBuilder, "Location/Remote Raw: %s\n", job.Remote)
	fmt.Fprintf(&promptBuilder, "Compensation Raw: %s\n", job.Compensation)
	fmt.Fprintf(&promptBuilder, "Description: %s\n\n", job.Description)

	promptBuilder.WriteString("### Output Format:\n")
	promptBuilder.WriteString("Instead of a text list, return ONLY valid JSON matching this schema:\n")
	promptBuilder.WriteString("{\n")
	promptBuilder.WriteString(`  "matches": boolean, // true if it meets all hard criteria, false otherwise` + "\n")
	promptBuilder.WriteString(`  "compensation_extracted": string, // The clear compensation string, e.g. "$180k-$220k base"` + "\n")
	promptBuilder.WriteString(`  "remote_eligibility": string, // E.g. "US-Remote", "Hybrid", "Onsite"` + "\n")
	promptBuilder.WriteString(`  "why_it_matches": [string], // Array of bullet points explaining why it matches priority signals` + "\n")
	promptBuilder.WriteString(`  "why_rejected": [string] // If matches is false, explain which hard criteria failed` + "\n")
	promptBuilder.WriteString("}\n")

	return promptBuilder.String()
}

func formatCriteriaForLLMPrompt(criteria *CriteriaConfig) string {
	if criteria == nil {
		return "No criteria were provided.\n\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Candidate location: %s\n", joinNonEmpty(", ", criteria.Candidate.City, criteria.Candidate.State, criteria.Candidate.CountryCode))
	if criteria.Candidate.YearsOfExperience > 0 {
		fmt.Fprintf(&b, "Years of experience: %d\n", criteria.Candidate.YearsOfExperience)
	}
	if len(criteria.Filters.TitleRequires) > 0 {
		fmt.Fprintf(&b, "Required title prefixes/levels: %s\n", strings.Join(criteria.Filters.TitleRequires, ", "))
	}
	if len(criteria.Filters.TitleIncludes) > 0 {
		fmt.Fprintf(&b, "Target title names: %s\n", strings.Join(criteria.Filters.TitleIncludes, ", "))
	}
	if len(criteria.Filters.TitleExcludes) > 0 {
		fmt.Fprintf(&b, "Excluded title terms: %s\n", strings.Join(criteria.Filters.TitleExcludes, ", "))
	}
	if workSettings := selectedWorkSettings(criteria.Filters.WorkSettings); len(workSettings) > 0 {
		fmt.Fprintf(&b, "Allowed work settings: %s\n", strings.Join(workSettings, ", "))
	}
	if criteria.Filters.MaxDistanceMiles > 0 {
		fmt.Fprintf(&b, "Maximum commute distance: %d miles\n", criteria.Filters.MaxDistanceMiles)
	}
	if criteria.Filters.MinBaseUSD > 0 {
		fmt.Fprintf(&b, "Minimum base salary: $%d USD\n", criteria.Filters.MinBaseUSD)
	}
	if len(criteria.Filters.IndustryIncludes) > 0 {
		fmt.Fprintf(&b, "Preferred industries: %s\n", strings.Join(criteria.Filters.IndustryIncludes, ", "))
	}
	if len(criteria.Filters.IndustryExcludes) > 0 {
		fmt.Fprintf(&b, "Excluded industries: %s\n", strings.Join(criteria.Filters.IndustryExcludes, ", "))
	}
	if len(criteria.RoleFamilies) > 0 {
		roles := make([]string, 0, len(criteria.RoleFamilies))
		for _, role := range criteria.RoleFamilies {
			roles = append(roles, string(role))
		}
		fmt.Fprintf(&b, "Role families: %s\n", strings.Join(roles, ", "))
	}
	if len(criteria.PrioritySignals) > 0 {
		fmt.Fprintf(&b, "Priority signals: %s\n", strings.Join(criteria.PrioritySignals, ", "))
	}
	b.WriteString("\n")
	return b.String()
}

func joinNonEmpty(sep string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		return "unspecified"
	}
	return strings.Join(parts, sep)
}

func evaluateJobWithLLM(ctx context.Context, llm llms.Model, job Job, criteria *CriteriaConfig) (*LLMEvaluationResult, error) {
	prompt := buildPrompt(job, criteria)

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a JSON-only API. You must return only valid JSON."),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	logDebug("job evaluation: generation start company=%q title=%q prompt_chars=%d", job.Company, job.Title, len(prompt))
	resp, err := llm.GenerateContent(ctx, messages, llms.WithTemperature(0.1), llms.WithMaxTokens(8192))
	if err != nil {
		logDebug("job evaluation: generation failed company=%q title=%q error=%v", job.Company, job.Title, err)
		return nil, fmt.Errorf("LLM generation failed: %v", err)
	}
	usage := ExtractTokenUsageFromContentResponse(resp)
	logDebug(
		"job evaluation: company=%q title=%q token_usage %s",
		job.Company,
		job.Title,
		formatTokenUsageForLog(usage),
	)

	if len(resp.Choices) == 0 {
		logDebug("job evaluation: generation returned no choices company=%q title=%q", job.Company, job.Title)
		return nil, fmt.Errorf("LLM returned no choices")
	}

	jsonStr := strings.TrimSpace(resp.Choices[0].Content)
	jsonStr = stripLLMJSON(jsonStr)

	var result LLMEvaluationResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		logDebug("job evaluation: parse failed company=%q title=%q response_chars=%d error=%v", job.Company, job.Title, len(resp.Choices[0].Content), err)
		return nil, fmt.Errorf("failed to parse LLM JSON output: %v", err)
	}
	result.TokenUsage = tokenUsagePtr(usage)

	logDebug("job evaluation: parsed company=%q title=%q matches=%t why_matches=%d why_rejected=%d", job.Company, job.Title, result.Matches, len(result.WhyItMatches), len(result.WhyRejected))
	return &result, nil
}

func EvaluateJobWithLLM(ctx context.Context, llm llms.Model, job Job, criteria *CriteriaConfig) (*LLMEvaluationResult, error) {
	return evaluateJobWithLLM(ctx, llm, job, criteria)
}

func stripLLMJSON(content string) string {
	jsonStr := strings.TrimSpace(content)
	if strings.HasPrefix(jsonStr, "```json") {
		jsonStr = strings.TrimPrefix(jsonStr, "```json")
		jsonStr = strings.TrimSuffix(jsonStr, "```")
	} else if strings.HasPrefix(jsonStr, "```") {
		jsonStr = strings.TrimPrefix(jsonStr, "```")
		jsonStr = strings.TrimSuffix(jsonStr, "```")
	}
	return strings.TrimSpace(jsonStr)
}
