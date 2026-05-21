package domain

import (
	"time"
)

// LayoffSignal represents a news item about layoffs
type LayoffSignal struct {
	Title         string     `json:"title"`
	Date          *time.Time `json:"date,omitempty"`
	URL           string     `json:"url,omitempty"`
	EmployeeCount *int       `json:"employee_count,omitempty"`
	PercentageStr string     `json:"percentage_str,omitempty"`
}

// HNSignal represents a Hacker News story
type HNSignal struct {
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Points      int       `json:"points"`
	NumComments int       `json:"num_comments"`
	Date        time.Time `json:"date"`
}

// CompanyHealthConcernStory is a recent accepted news/HN story worth deeper review.
type CompanyHealthConcernStory struct {
	Source  string     `json:"source"`
	Title   string     `json:"title"`
	URL     string     `json:"url,omitempty"`
	Date    *time.Time `json:"date,omitempty"`
	Concern string     `json:"concern,omitempty"`
}

// CompanyHealthConcernArticle is transient page text fetched for LLM review.
type CompanyHealthConcernArticle struct {
	Source  string     `json:"source"`
	Title   string     `json:"title"`
	URL     string     `json:"url,omitempty"`
	Date    *time.Time `json:"date,omitempty"`
	Concern string     `json:"concern,omitempty"`
	Text    string     `json:"text,omitempty"`
}

// EmployerReviewSignal represents browser-discovered employer review evidence
type EmployerReviewSignal struct {
	Source             string   `json:"source"`
	Title              string   `json:"title"`
	URL                string   `json:"url,omitempty"`
	Rating             string   `json:"rating,omitempty"`
	ReviewCount        *int     `json:"review_count,omitempty"`
	RecommendPercent   *int     `json:"recommend_percent,omitempty"`
	CEOApprovalPercent *int     `json:"ceo_approval_percent,omitempty"`
	Snippet            string   `json:"snippet,omitempty"`
	Flags              []string `json:"flags,omitempty"`
}

// EmploymentRisk represents calculated employment risk
type EmploymentRisk struct {
	Level          string     `json:"level"` // Low, Medium, High, Critical
	Score          int        `json:"score"` // 0-100
	Factors        []string   `json:"factors"`
	LastLayoffDate *time.Time `json:"last_layoff_date,omitempty"`
}

// CompanyHealthResult contains the full health assessment
type CompanyHealthResult struct {
	Company              string                                   `json:"company"`
	Score                int                                      `json:"score"`
	Confidence           string                                   `json:"confidence"` // high, medium, low
	Public               *bool                                    `json:"public,omitempty"`
	FoundedYear          *int                                     `json:"founded_year,omitempty"`
	AgeYears             *int                                     `json:"age_years,omitempty"`
	EstimatedEmployees   *int                                     `json:"estimated_employees,omitempty"`
	SignalsUsed          []string                                 `json:"signals_used"`
	Flags                []string                                 `json:"flags"`
	Notes                []string                                 `json:"notes"`
	DiscoveredTicker     string                                   `json:"discovered_ticker,omitempty"`
	DiscoveredName       string                                   `json:"discovered_name,omitempty"`
	LayoffSignals        []LayoffSignal                           `json:"layoff_signals,omitempty"`
	HNSignals            []HNSignal                               `json:"hn_signals,omitempty"`
	ConcernStories       []CompanyHealthConcernStory              `json:"concern_stories,omitempty"`
	ConcernStoryArticles []CompanyHealthConcernArticle            `json:"-"`
	EmployerReviews      []EmployerReviewSignal                   `json:"employer_reviews,omitempty"`
	EmploymentRisk       *EmploymentRisk                          `json:"employment_risk,omitempty"`
	LLMAssessment        *LLMCompanyHealthAssessment              `json:"llm_assessment,omitempty"`
	RejectedEvidence     []CompanyHealthEvidence                  `json:"rejected_evidence,omitempty"`
	Sources              map[string]any                           `json:"sources"`
	FieldAssessments     map[string]*CompanyHealthFieldAssessment `json:"field_assessments,omitempty"`
	Notices              []string                                 `json:"notices,omitempty"`
}

// LLMTokenUsage is the normalized token accounting surfaced by LLM providers.
type LLMTokenUsage struct {
	InputTokens     *int `json:"input_tokens,omitempty"`
	OutputTokens    *int `json:"output_tokens,omitempty"`
	TotalTokens     *int `json:"total_tokens,omitempty"`
	CachedTokens    *int `json:"cached_tokens,omitempty"`
	ReasoningTokens *int `json:"reasoning_tokens,omitempty"`
	ThinkingTokens  *int `json:"thinking_tokens,omitempty"`
}

// Available reports whether any token usage field was found.
func (u LLMTokenUsage) Available() bool {
	return u.InputTokens != nil ||
		u.OutputTokens != nil ||
		u.TotalTokens != nil ||
		u.CachedTokens != nil ||
		u.ReasoningTokens != nil ||
		u.ThinkingTokens != nil
}

type LLMCompanyHealthAssessment struct {
	Summary                string                          `json:"summary"`
	Recommendation         string                          `json:"recommendation"`
	RiskLevel              string                          `json:"risk_level"`
	PositiveSignals        []string                        `json:"positive_signals"`
	Concerns               []string                        `json:"concerns"`
	FollowUpQuestions      []string                        `json:"follow_up_questions"`
	ArticleReviews         []LLMCompanyHealthArticleReview `json:"article_reviews,omitempty"`
	StoryInsight           string                          `json:"story_insight,omitempty"`
	ScoreModifier          *int                            `json:"score_modifier,omitempty"`
	ScoreModifierReason    string                          `json:"score_modifier_reason,omitempty"`
	ScoreModifierNovelFact string                          `json:"score_modifier_novel_fact,omitempty"`
	ScoreModifierSources   []string                        `json:"score_modifier_sources,omitempty"`
	TokenUsage             *LLMTokenUsage                  `json:"token_usage,omitempty"`
}

type LLMCompanyHealthArticleReview struct {
	URL             string `json:"url"`
	Source          string `json:"source,omitempty"`
	Related         bool   `json:"related"`
	RelevanceReason string `json:"relevance_reason,omitempty"`
	NovelFact       string `json:"novel_fact,omitempty"`
}

type CompanyHealthEvidence struct {
	Value      string `json:"value"`
	Source     string `json:"source"`
	URL        string `json:"url,omitempty"`
	Confidence string `json:"confidence"`
	Accepted   bool   `json:"accepted"`
	Reason     string `json:"reason,omitempty"`
}

type CompanyHealthFieldAssessment struct {
	Status     string                  `json:"status"`
	Confidence string                  `json:"confidence,omitempty"`
	Source     string                  `json:"source,omitempty"`
	URL        string                  `json:"url,omitempty"`
	Notes      []string                `json:"notes,omitempty"`
	Evidence   []CompanyHealthEvidence `json:"evidence,omitempty"`
}

type CompanyHealthContext struct {
	Company                 string
	Aliases                 []string
	Website                 string
	Summary                 string
	Industry                string
	Industries              []string
	Ticker                  string
	EstimatedEmployees      *int
	FoundedYear             *int
	RequireResolvedIdentity bool
}
