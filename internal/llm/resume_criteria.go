package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wallentx/jobscout/internal/domain"

	"github.com/tmc/langchaingo/llms"
)

const maxResumeTextBytes = 120_000

type resumeCriteriaResponse struct {
	Candidate struct {
		City              string `json:"city"`
		State             string `json:"state"`
		CountryCode       string `json:"country_code"`
		YearsOfExperience int    `json:"years_of_experience"`
	} `json:"candidate"`
	RoleFamilies    []string `json:"role_families"`
	TitleRequires   []string `json:"title_requires"`
	TitleIncludes   []string `json:"title_includes"`
	TitleExcludes   []string `json:"title_excludes"`
	WorkSettings    []string `json:"work_settings"`
	MinBaseUSD      int      `json:"min_base_usd"`
	PrioritySignals []string `json:"priority_signals"`
}

func evaluateResumeCriteriaWithLLM(ctx context.Context, llm llms.Model, resumeText string) (*CriteriaConfig, error) {
	criteria, _, err := evaluateResumeCriteriaWithLLMUsage(ctx, llm, resumeText)
	return criteria, err
}

func evaluateResumeCriteriaWithLLMUsage(ctx context.Context, llm llms.Model, resumeText string) (*CriteriaConfig, LLMTokenUsage, error) {
	prompt := buildResumeCriteriaPrompt(resumeText)
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a JSON-only API. You must return only valid JSON."),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	logDebug("resume criteria: generation start resume_chars=%d prompt_chars=%d", len(resumeText), len(prompt))
	resp, err := llm.GenerateContent(ctx, messages, llmJSONCallOptions(0.1, 4096)...)
	if err != nil {
		logDebug("resume criteria: generation failed error=%v", err)
		return nil, LLMTokenUsage{}, fmt.Errorf("LLM generation failed: %v", err)
	}
	usage := ExtractTokenUsageFromContentResponse(resp)
	logLLMTokenUsage("resume criteria", usage)
	if len(resp.Choices) == 0 {
		logDebug("resume criteria: generation returned no choices")
		return nil, usage, fmt.Errorf("LLM returned no choices")
	}

	var parsed resumeCriteriaResponse
	if err := json.Unmarshal([]byte(stripLLMJSON(resp.Choices[0].Content)), &parsed); err != nil {
		logDebug("resume criteria: parse failed response_chars=%d error=%v", len(resp.Choices[0].Content), err)
		return nil, usage, fmt.Errorf("failed to parse LLM resume criteria JSON output: %v", err)
	}

	criteria, err := criteriaFromResumeResponse(parsed)
	if err != nil {
		return nil, usage, err
	}
	logDebug("resume criteria: parsed role_families=%d title_includes=%d priority_signals=%d", len(criteria.RoleFamilies), len(criteria.Filters.TitleIncludes), len(criteria.PrioritySignals))
	return criteria, usage, nil
}

func GenerateCriteriaFromResume(ctx context.Context, appCfg AppConfig, resumePath string) (*CriteriaConfig, error) {
	resumeText, err := domain.ExtractResumeText(resumePath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(resumeText) == "" {
		return nil, fmt.Errorf("resume did not contain readable text")
	}

	model, restoreAuth, err := initConfiguredLLMForTask(ctx, &appCfg, llmTaskResumeCriteria)
	if err != nil {
		return nil, err
	}
	defer restoreAuth()

	return evaluateResumeCriteriaWithLLM(ctx, model, resumeText)
}

func buildResumeCriteriaPrompt(resumeText string) string {
	resumeText = strings.TrimSpace(resumeText)
	if len(resumeText) > maxResumeTextBytes {
		resumeText = truncateValidUTF8(resumeText, maxResumeTextBytes)
	}

	roleIDs := make([]string, 0, len(domain.RoleFamilySpecs()))
	for _, spec := range domain.RoleFamilySpecs() {
		roleIDs = append(roleIDs, fmt.Sprintf("%s (%s)", spec.ID, spec.Label))
	}

	var prompt strings.Builder
	prompt.WriteString("Read this resume and infer a starting job-search criteria profile. ")
	prompt.WriteString("Use only facts and reasonable role-search terms supported by the resume. ")
	prompt.WriteString("Leave unknown personal location fields empty instead of guessing. ")
	prompt.WriteString("Use title_requires only for title levels that commonly appear in job titles, such as Junior, Senior, Staff, or Principal; do not use Mid-Level. ")
	prompt.WriteString("Leave title_requires empty if no title level is required. ")
	prompt.WriteString("Use title_includes for common public job-board title names, such as Software Engineer, Frontend Developer, Backend Engineer, Platform Engineer, Data Engineer, Product Designer, or Product Manager. ")
	prompt.WriteString("Do not put skills, tools, industries, adjectives, or generic standalone words like Engineer or Developer in title_includes; put skills and tools in priority_signals. ")
	prompt.WriteString("For role_families, use only these IDs: ")
	prompt.WriteString(strings.Join(roleIDs, ", "))
	prompt.WriteString(".\n\nReturn ONLY valid JSON matching this schema:\n")
	prompt.WriteString("{\n")
	prompt.WriteString(`  "candidate": {"city": string, "state": string, "country_code": string, "years_of_experience": number},` + "\n")
	prompt.WriteString(`  "role_families": [string],` + "\n")
	prompt.WriteString(`  "title_requires": [string],` + "\n")
	prompt.WriteString(`  "title_includes": [string],` + "\n")
	prompt.WriteString(`  "title_excludes": [string],` + "\n")
	prompt.WriteString(`  "work_settings": [string],` + "\n")
	prompt.WriteString(`  "min_base_usd": number,` + "\n")
	prompt.WriteString(`  "priority_signals": [string]` + "\n")
	prompt.WriteString("}\n\nResume text:\n")
	prompt.WriteString(resumeText)
	return prompt.String()
}

func truncateValidUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	truncated := value[:maxBytes]
	for !utf8.ValidString(truncated) && len(truncated) > 0 {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}

func criteriaFromResumeResponse(parsed resumeCriteriaResponse) (*CriteriaConfig, error) {
	var cfg CriteriaConfig
	cfg.Candidate.City = strings.TrimSpace(parsed.Candidate.City)
	cfg.Candidate.State = strings.TrimSpace(parsed.Candidate.State)
	cfg.Candidate.CountryCode = domain.NormalizeCountryCode(parsed.Candidate.CountryCode)
	cfg.Candidate.YearsOfExperience = parsed.Candidate.YearsOfExperience
	domain.NormalizeCriteriaLocation(&cfg)
	cfg.Filters.TitleRequires = domain.NormalizeTitlePrefixes(cleanStringList(parsed.TitleRequires))
	cfg.Filters.TitleExcludes = cleanStringList(parsed.TitleExcludes)
	cfg.Filters.WorkSettings = domain.ParseWorkSettings(strings.Join(parsed.WorkSettings, ", "))
	cfg.Filters.MinBaseUSD = parsed.MinBaseUSD
	cfg.Filters.IndustryIncludes = []string{}
	cfg.Filters.IndustryExcludes = []string{}
	cfg.PrioritySignals = cleanStringList(parsed.PrioritySignals)

	roles, err := domain.ParseRoleFamilyCSV(strings.Join(parsed.RoleFamilies, ", "))
	if err != nil {
		return nil, err
	}
	cfg.RoleFamilies = roles
	cfg.Filters.TitleIncludes = domain.NormalizeTargetTitleNames(cleanStringList(parsed.TitleIncludes), cfg.RoleFamilies)
	return &cfg, nil
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}
