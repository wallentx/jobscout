package llm

import (
	"encoding/json"
)

type llmBenchmarkCase struct {
	ID      string          `json:"id"`
	Task    string          `json:"task"`
	Version int             `json:"version"`
	Input   json.RawMessage `json:"input"`
	Checks  benchmarkChecks `json:"checks"`
}

type benchmarkChecks struct {
	JSONRequired          bool                             `json:"json_required"`
	RequiredFields        []string                         `json:"required_fields"`
	ExpectedValues        map[string]any                   `json:"expected_values"`
	ExpectedContains      map[string][]any                 `json:"expected_contains"`
	EnumValues            map[string][]any                 `json:"enum_values" yaml:"enum_values"`
	NumericMinimums       map[string]float64               `json:"numeric_minimums"`
	NumericMaximums       map[string]float64               `json:"numeric_maximums" yaml:"numeric_maximums"`
	NumericRanges         map[string]benchmarkNumericRange `json:"numeric_ranges" yaml:"numeric_ranges"`
	MustInclude           []string                         `json:"must_include"`
	MustNotInclude        []string                         `json:"must_not_include"`
	HallucinationPatterns []string                         `json:"hallucination_patterns" yaml:"hallucination_patterns"`
	GroundingRules        []string                         `json:"grounding_rules"`
}

type benchmarkNumericRange struct {
	Min *float64 `json:"min,omitempty" yaml:"min,omitempty"`
	Max *float64 `json:"max,omitempty" yaml:"max,omitempty"`
}

type llmBenchmarkRunRecord struct {
	Timestamp             string         `json:"timestamp"`
	BenchmarkVersion      int            `json:"benchmark_version"`
	Provider              string         `json:"provider"`
	Model                 string         `json:"model"`
	Task                  string         `json:"task"`
	CaseID                string         `json:"case_id"`
	LatencyMS             int64          `json:"latency_ms"`
	InputTokens           *int           `json:"input_tokens,omitempty"`
	OutputTokens          *int           `json:"output_tokens,omitempty"`
	EstimatedCostUSD      *float64       `json:"estimated_cost_usd,omitempty"`
	JSONValid             bool           `json:"json_valid"`
	RequiredFieldsPresent bool           `json:"required_fields_present"`
	AccuracyScore         int            `json:"accuracy_score"`
	JSONScore             int            `json:"json_score"`
	GroundingScore        int            `json:"grounding_score"`
	SpeedScore            int            `json:"speed_score"`
	CostScore             int            `json:"cost_score"`
	StabilityScore        int            `json:"stability_score"`
	FinalScore            float64        `json:"final_score"`
	ScoreCap              int            `json:"score_cap,omitempty"`
	Error                 string         `json:"error,omitempty"`
	RawOutput             string         `json:"raw_output,omitempty"`
	Details               map[string]any `json:"details,omitempty"`
}

type benchmarkJobFilterInput struct {
	Criteria CriteriaConfig `yaml:"criteria"`
	Job      Job            `yaml:"job"`
}

type benchmarkJobFilterBatchInput struct {
	Criteria CriteriaConfig                 `yaml:"criteria" json:"criteria"`
	Source   string                         `yaml:"source" json:"source"`
	Jobs     []benchmarkJobFilterBatchEntry `yaml:"jobs" json:"jobs"`
}

type benchmarkJobFilterBatchEntry struct {
	ID  string `yaml:"id" json:"id"`
	Job Job    `yaml:"job" json:"job"`
}

type benchmarkJobIdentityInput struct {
	Job  Job             `yaml:"job" json:"job"`
	Page JobIdentityPage `yaml:"page" json:"page"`
}

type benchmarkJobIdentityOutput struct {
	CompanyWebsite        string `json:"company_website"`
	CompanySummary        string `json:"company_summary"`
	CompanyIndustry       string `json:"company_industry"`
	WebsiteConfidence     string `json:"website_confidence"`
	SummaryConfidence     string `json:"summary_confidence"`
	IndustryConfidence    string `json:"industry_confidence"`
	IndustryProvisional   bool   `json:"industry_provisional"`
	CompanyWebsiteReason  string `json:"company_website_reason"`
	CompanySummaryReason  string `json:"company_summary_reason"`
	CompanyIndustryReason string `json:"company_industry_reason"`
}

type jobFilterBatchOutput struct {
	Results    map[string]LLMEvaluationResult `json:"results"`
	TokenUsage *LLMTokenUsage                 `json:"token_usage,omitempty"`
}

type benchmarkResumeInput struct {
	ResumeText string `yaml:"resume_text"`
}

type benchmarkCompanyHealthInput struct {
	Result CompanyHealthResult `yaml:"result"`
}

type benchmarkJobSearchInput struct {
	Prompt string `yaml:"prompt" json:"prompt"`
}

type benchmarkJobSearchOutput struct {
	Jobs  []Job `json:"jobs"`
	Count int   `json:"count"`
}

const benchmarkVersion = 1
