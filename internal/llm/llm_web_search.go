package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	defaultOpenAIResponsesEndpoint = "https://api.openai.com/v1/responses"
	defaultOpenAIWebSearchTimeout  = 90 * time.Second
)

func executeLLMWebSearch(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error) {
	provider, providerCfg, ok := effectiveLLMProviderForTask(appCfg, llmTaskJobSearch)
	if !ok {
		return nil, fmt.Errorf("no effective llm provider is configured")
	}
	logDebug("llm web search: provider=%s model=%s", provider, providerCfg.Model)

	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "gemini", "googleai":
		return executeGeminiWebSearch(ctx, appCfg, providerCfg, prompt)
	case "openai":
		return executeOpenAIWebSearch(ctx, appCfg, providerCfg, prompt)
	default:
		llm, restore, err := initConfiguredLLMForTask(ctx, appCfg, llmTaskJobSearch)
		if err != nil {
			return nil, err
		}
		defer restore()
		return executeLLMSearch(ctx, llm, prompt)
	}
}

func executeGeminiWebSearch(ctx context.Context, appCfg *AppConfig, providerCfg LLMProviderConfig, prompt string) ([]Job, error) {
	restore, err := applyResolvedLLMAuth(appCfg)
	if err != nil {
		return nil, err
	}
	defer restore()

	apiKey := strings.TrimSpace(os.Getenv(envVarForProvider("gemini")))
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is not set", envVarForProvider("gemini"))
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create Gemini web-search client: %w", err)
	}

	temperature := float32(0.2)
	result, err := client.Models.GenerateContent(ctx,
		providerCfg.Model,
		genai.Text(prompt),
		&genai.GenerateContentConfig{
			Temperature:     &temperature,
			MaxOutputTokens: 8192,
			Tools: []*genai.Tool{
				{GoogleSearch: &genai.GoogleSearch{}},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("gemini web search failed: %w", err)
	}
	usage := tokenUsageFromGeminiUsageMetadata(result.UsageMetadata)
	logDebug("gemini web search: token_usage %s", formatTokenUsageForLog(usage))
	return parseLLMJobsJSON(result.Text())
}

func tokenUsageFromGeminiUsageMetadata(metadata *genai.GenerateContentResponseUsageMetadata) LLMTokenUsage {
	if metadata == nil {
		return LLMTokenUsage{}
	}
	usage := LLMTokenUsage{
		InputTokens:     addOptionalInts(nonZeroTokenPtr(metadata.PromptTokenCount), nonZeroTokenPtr(metadata.ToolUsePromptTokenCount)),
		OutputTokens:    nonZeroTokenPtr(metadata.CandidatesTokenCount),
		TotalTokens:     nonZeroTokenPtr(metadata.TotalTokenCount),
		CachedTokens:    nonZeroTokenPtr(metadata.CachedContentTokenCount),
		ThinkingTokens:  nonZeroTokenPtr(metadata.ThoughtsTokenCount),
		ReasoningTokens: nil,
	}
	if usage.TotalTokens == nil && usage.InputTokens != nil && usage.OutputTokens != nil {
		usage.TotalTokens = intPtr(*usage.InputTokens + *usage.OutputTokens)
	}
	return usage
}

func nonZeroTokenPtr(value int32) *int {
	if value == 0 {
		return nil
	}
	return intPtr(int(value))
}

type openAIWebSearchRequest struct {
	Model           string                 `json:"model"`
	Input           string                 `json:"input"`
	Instructions    string                 `json:"instructions,omitempty"`
	Tools           []map[string]any       `json:"tools"`
	ToolChoice      string                 `json:"tool_choice"`
	Include         []string               `json:"include,omitempty"`
	MaxOutputTokens int                    `json:"max_output_tokens,omitempty"`
	MaxToolCalls    int                    `json:"max_tool_calls,omitempty"`
	Temperature     *float32               `json:"temperature,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type openAIWebSearchResponse struct {
	ID                string                        `json:"id"`
	Status            string                        `json:"status"`
	Error             *openAIWebSearchError         `json:"error"`
	IncompleteDetails *openAIWebSearchIncomplete    `json:"incomplete_details"`
	Output            []openAIWebSearchResponseItem `json:"output"`
	Metadata          map[string]interface{}        `json:"metadata"`
	Usage             map[string]interface{}        `json:"usage"`
}

type openAIWebSearchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

type openAIWebSearchIncomplete struct {
	Reason string `json:"reason"`
}

type openAIWebSearchResponseItem struct {
	Type    string                           `json:"type"`
	Status  string                           `json:"status"`
	Action  openAIWebSearchAction            `json:"action"`
	Content []openAIWebSearchResponseContent `json:"content"`
}

type openAIWebSearchAction struct {
	Type    string                  `json:"type"`
	Query   string                  `json:"query"`
	Queries []string                `json:"queries"`
	Sources []openAIWebSearchSource `json:"sources"`
}

type openAIWebSearchSource struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type openAIWebSearchResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIWebSearchBatch struct {
	Query   string
	Domains []string
}

func executeOpenAIWebSearch(ctx context.Context, appCfg *AppConfig, providerCfg LLMProviderConfig, prompt string) ([]Job, error) {
	restore, err := applyResolvedLLMAuth(appCfg)
	if err != nil {
		return nil, err
	}
	defer restore()

	apiKey := strings.TrimSpace(os.Getenv(envVarForProvider("openai")))
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is not set", envVarForProvider("openai"))
	}
	model := strings.TrimSpace(providerCfg.Model)
	if model == "" {
		model = defaultModelForProvider("openai")
	}

	batches := openAIWebSearchBatches(prompt)
	if len(batches) == 0 {
		batches = []openAIWebSearchBatch{{Query: "matching public job postings"}}
	}
	logDebug("openai web search: endpoint=%s model=%s batches=%d", openAIResponsesEndpoint(providerCfg), model, len(batches))

	allJobs, lastErr := executeOpenAIWebSearchBatches(ctx, providerCfg, apiKey, model, prompt, batches)
	if len(allJobs) == 0 {
		targeted := openAIWebSearchTargetedBatches(prompt)
		if len(targeted) > 0 {
			logDebug("openai web search: broad batches returned no jobs; running targeted fallback batches=%d", len(targeted))
			targetedJobs, targetedErr := executeOpenAIWebSearchBatches(ctx, providerCfg, apiKey, model, prompt, targeted)
			if len(targetedJobs) > 0 {
				return targetedJobs, nil
			}
			if targetedErr != nil {
				lastErr = targetedErr
			}
		}
	}
	if len(allJobs) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return allJobs, nil
}

func executeOpenAIWebSearchBatches(ctx context.Context, providerCfg LLMProviderConfig, apiKey string, model string, prompt string, batches []openAIWebSearchBatch) ([]Job, error) {
	var allJobs []Job
	var lastErr error
	seen := make(map[string]bool)
	for i, batch := range batches {
		jobs, err := executeOpenAIWebSearchBatch(ctx, providerCfg, apiKey, model, prompt, batch, i+1)
		if err != nil {
			lastErr = err
			logDebug("openai web search: batch %d/%d query=%q failed: %v", i+1, len(batches), batch.Query, err)
			continue
		}
		for _, job := range jobs {
			key := openAIWebSearchJobKey(job)
			if seen[key] {
				continue
			}
			seen[key] = true
			allJobs = append(allJobs, job)
		}
	}
	return allJobs, lastErr
}

func executeOpenAIWebSearchBatch(ctx context.Context, providerCfg LLMProviderConfig, apiKey string, model string, basePrompt string, batch openAIWebSearchBatch, batchIndex int) ([]Job, error) {
	tool := map[string]any{
		"type":                "web_search",
		"search_context_size": "low",
	}
	if len(batch.Domains) > 0 {
		tool["filters"] = map[string]any{
			"allowed_domains": batch.Domains,
		}
	}
	requestBody := openAIWebSearchRequest{
		Model:           model,
		Input:           openAIWebSearchBatchPrompt(basePrompt, batch),
		Instructions:    "You are a JSON-only job search assistant. Use web search before deciding no current matching jobs exist.",
		Tools:           []map[string]any{tool},
		ToolChoice:      "required",
		Include:         []string{"web_search_call.action.sources"},
		MaxOutputTokens: 4096,
		MaxToolCalls:    2,
		Temperature:     providerCfg.Temperature,
		Metadata: map[string]interface{}{
			"feature": "jobscout_llm_web",
		},
	}
	logDebug(
		"openai web search: batch=%d query=%q domains=%s model=%s tool=web_search max_tool_calls=%d max_output_tokens=%d",
		batchIndex,
		batch.Query,
		debugOpenAIDomains(batch.Domains),
		model,
		requestBody.MaxToolCalls,
		requestBody.MaxOutputTokens,
	)

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAI web-search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIResponsesEndpoint(providerCfg), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create OpenAI web-search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := openAIWebSearchHTTPClient(providerCfg).Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI web search request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read OpenAI web-search response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenAI web search failed: %s: %s", resp.Status, summarizeOpenAIResponseBody(body))
	}

	var apiResp openAIWebSearchResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse OpenAI web-search response: %w", err)
	}
	if apiResp.Error != nil && strings.TrimSpace(apiResp.Error.Message) != "" {
		return nil, fmt.Errorf("OpenAI web search failed: %s", apiResp.Error.Message)
	}
	usage := ExtractTokenUsage(apiResp.Usage)

	text := openAIResponseOutputText(apiResp)
	logDebug(
		"openai web search: batch=%d response_id=%s status=%s output_items=%d web_search_calls=%d sources=%d token_usage %s output=%s",
		batchIndex,
		apiResp.ID,
		apiResp.Status,
		len(apiResp.Output),
		countOpenAIWebSearchCalls(apiResp),
		countOpenAIWebSearchSources(apiResp),
		formatTokenUsageForLog(usage),
		summarizeOpenAIOutputText(text),
	)
	if strings.TrimSpace(text) == "" && apiResp.IncompleteDetails != nil && strings.TrimSpace(apiResp.IncompleteDetails.Reason) != "" {
		return nil, fmt.Errorf("OpenAI web search response incomplete: %s", apiResp.IncompleteDetails.Reason)
	}
	return parseLLMJobsJSON(text)
}

func openAIWebSearchBatches(prompt string) []openAIWebSearchBatch {
	domains := extractOpenAIAllowedDomains(prompt)
	queries := extractOpenAIWebSearchQueries(prompt)
	if len(queries) == 0 {
		return nil
	}

	batches := make([]openAIWebSearchBatch, 0, len(queries))
	seen := make(map[string]bool)
	for _, query := range queries {
		titleQuery := openAITitleQueryFromSiteQuery(query)
		if titleQuery == "" {
			titleQuery = query
		}
		key := strings.ToLower(titleQuery)
		if seen[key] {
			continue
		}
		seen[key] = true
		batches = append(batches, openAIWebSearchBatch{
			Query:   titleQuery,
			Domains: domains,
		})
	}
	return batches
}

func openAIWebSearchTargetedBatches(prompt string) []openAIWebSearchBatch {
	queries := extractOpenAIWebSearchQueries(prompt)
	if len(queries) == 0 {
		return nil
	}

	batches := make([]openAIWebSearchBatch, 0, len(queries))
	seen := make(map[string]bool)
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		domain := openAISiteQueryDomain(query)
		key := strings.ToLower(query + "|" + domain)
		if seen[key] {
			continue
		}
		seen[key] = true
		batch := openAIWebSearchBatch{Query: query}
		if domain != "" {
			batch.Domains = []string{domain}
		}
		batches = append(batches, batch)
	}
	return batches
}

func extractOpenAIWebSearchQueries(prompt string) []string {
	return extractPromptBulletSection(prompt, "Search only these public-web queries:")
}

func extractOpenAIAllowedDomains(prompt string) []string {
	return extractPromptBulletSection(prompt, "Allowed source domains for providers that support domain filters:")
}

func extractPromptBulletSection(prompt string, header string) []string {
	lines := strings.Split(prompt, "\n")
	inSection := false
	var values []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == header {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if trimmed == "" {
			break
		}
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func openAITitleQueryFromSiteQuery(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(fields[0]), "site:") {
		return strings.TrimSpace(strings.Join(fields[1:], " "))
	}
	return strings.TrimSpace(query)
}

func openAISiteQueryDomain(query string) string {
	fields := strings.Fields(strings.TrimSpace(query))
	if len(fields) == 0 || !strings.HasPrefix(strings.ToLower(fields[0]), "site:") {
		return ""
	}
	domain := strings.TrimPrefix(strings.ToLower(fields[0]), "site:")
	if idx := strings.IndexAny(domain, " /?"); idx >= 0 {
		domain = domain[:idx]
	}
	domain = strings.Trim(domain, ".")
	if domain == "" {
		return ""
	}
	if !strings.Contains(domain, "*") {
		return domain
	}
	parts := strings.Split(domain, ".")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.Contains(part, "*") {
			continue
		}
		cleaned = append(cleaned, part)
	}
	if len(cleaned) >= 2 {
		return strings.Join(cleaned[len(cleaned)-2:], ".")
	}
	return strings.Join(cleaned, ".")
}

func openAIWebSearchBatchPrompt(basePrompt string, batch openAIWebSearchBatch) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Use web search to find current public job postings for this exact role query: %q.\n", batch.Query)
	if len(batch.Domains) > 0 {
		fmt.Fprintf(&b, "The web search tool is restricted to these allowed domains: %s.\n", strings.Join(batch.Domains, ", "))
	}
	b.WriteString("Return only direct job postings or ATS application pages that match this role query and the criteria below.\n")
	b.WriteString("For each result, apply_url must be a direct job detail URL, not a careers home, search result, company page, or placeholder.\n")
	b.WriteString("Include company_website, company_summary, and company_industry when the result provides them, but do not invent missing values.\n")
	if criteria := openAIWebCriteriaSection(basePrompt); criteria != "" {
		b.WriteString(criteria)
	} else {
		b.WriteString(basePrompt)
	}
	b.WriteString("\n\nReturn only a valid JSON array. Do not include citations, markdown, or prose outside the JSON array.")
	return b.String()
}

func openAIWebCriteriaSection(prompt string) string {
	idx := strings.Index(prompt, "Only include roles that match the criteria below.")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(prompt[idx:])
}

func openAIWebSearchJobKey(job Job) string {
	if applyURL := strings.ToLower(strings.TrimSpace(job.ApplyURL)); applyURL != "" {
		return "url:" + applyURL
	}
	return "job:" + strings.ToLower(strings.TrimSpace(job.Company)) + "|" + strings.ToLower(strings.TrimSpace(job.Title))
}

func debugOpenAIDomains(domains []string) string {
	if len(domains) == 0 {
		return "<none>"
	}
	if len(domains) > 8 {
		return strings.Join(domains[:8], ", ") + fmt.Sprintf(", ... (+%d)", len(domains)-8)
	}
	return strings.Join(domains, ", ")
}

func openAIResponsesEndpoint(providerCfg LLMProviderConfig) string {
	endpoint := strings.TrimSpace(providerCfg.Endpoint)
	if endpoint == "" {
		return defaultOpenAIResponsesEndpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/responses") {
		return endpoint
	}
	if strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/responses"
	}
	return endpoint + "/v1/responses"
}

func openAIWebSearchHTTPClient(providerCfg LLMProviderConfig) *http.Client {
	timeout := defaultOpenAIWebSearchTimeout
	if configured := strings.TrimSpace(providerCfg.Timeout); configured != "" {
		if parsed, err := time.ParseDuration(configured); err == nil && parsed > 0 {
			timeout = parsed
		}
	}
	return &http.Client{Timeout: timeout}
}

func summarizeOpenAIResponseBody(body []byte) string {
	var parsed struct {
		Error *openAIWebSearchError `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return parsed.Error.Message
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		text = text[:500] + "..."
	}
	return text
}

func openAIResponseOutputText(resp openAIWebSearchResponse) string {
	var out strings.Builder
	for _, item := range resp.Output {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Type == "output_text" {
				out.WriteString(content.Text)
			}
		}
	}
	return out.String()
}

func countOpenAIWebSearchCalls(resp openAIWebSearchResponse) int {
	count := 0
	for _, item := range resp.Output {
		if item.Type == "web_search_call" {
			count++
		}
	}
	return count
}

func countOpenAIWebSearchSources(resp openAIWebSearchResponse) int {
	count := 0
	for _, item := range resp.Output {
		if item.Type == "web_search_call" {
			count += len(item.Action.Sources)
		}
	}
	return count
}

func summarizeOpenAIOutputText(text string) string {
	text = strings.TrimSpace(text)
	switch text {
	case "":
		return "<empty>"
	case "[]":
		return "[]"
	default:
		runes := []rune(text)
		if len(runes) > 120 {
			runes = runes[:120]
		}
		return fmt.Sprintf("%d chars prefix=%q", len(text), string(runes))
	}
}

func parseLLMJobsJSON(content string) ([]Job, error) {
	jsonStr := stripLLMJSON(content)
	if strings.TrimSpace(jsonStr) == "" {
		return nil, fmt.Errorf("failed to parse LLM JSON output: empty response")
	}

	var jobs []Job
	if err := json.Unmarshal([]byte(jsonStr), &jobs); err != nil {
		if jobs, ok := decodeFirstJSONJobArray(jsonStr); ok {
			return jobs, nil
		}
		return nil, fmt.Errorf("failed to parse LLM JSON output: %v", err)
	}
	return jobs, nil
}

func decodeFirstJSONJobArray(content string) ([]Job, bool) {
	for i, r := range content {
		if r != '[' {
			continue
		}
		var jobs []Job
		decoder := json.NewDecoder(strings.NewReader(content[i:]))
		if err := decoder.Decode(&jobs); err == nil {
			return jobs, true
		}
	}
	return nil, false
}

func ExecuteLLMWebSearch(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error) {
	return executeLLMWebSearch(ctx, appCfg, prompt)
}
