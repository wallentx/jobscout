package health

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
)

type fakeBrowserSession struct {
	profileCalls int
	reviewCalls  int
	articleCalls int
	profile      *domain.CompanySiteProfile
	reviews      []domain.EmployerReviewSignal
	articleText  string
}

func (s *fakeBrowserSession) FetchCompanySiteProfile(ctx context.Context, identity domain.CompanyHealthContext) (*domain.CompanySiteProfile, error) {
	s.profileCalls++
	return s.profile, nil
}

func (s *fakeBrowserSession) FetchEmployerReviewSignals(ctx context.Context, company string) ([]domain.EmployerReviewSignal, error) {
	s.reviewCalls++
	return s.reviews, nil
}

func (s *fakeBrowserSession) FetchArticleText(ctx context.Context, rawURL string) (string, error) {
	s.articleCalls++
	return s.articleText, nil
}

func TestBrowserFetchesUseContextBrowserSession(t *testing.T) {
	session := &fakeBrowserSession{
		profile: &domain.CompanySiteProfile{
			WebsiteURL: "https://acme.example/",
			Summary:    "Acme builds deployment tooling.",
		},
		reviews: []domain.EmployerReviewSignal{
			{Source: "glassdoor", Title: "Acme reviews"},
		},
	}
	ctx := ContextWithBrowserSession(context.Background(), session)

	profile, err := fetchBrowserCompanySiteProfileThrottled(ctx, domain.CompanyHealthContext{
		Company: "Acme",
		Website: "https://acme.example",
	})
	if err != nil {
		t.Fatalf("fetchBrowserCompanySiteProfileThrottled() error = %v", err)
	}
	if profile != session.profile {
		t.Fatalf("fetchBrowserCompanySiteProfileThrottled() = %#v, want session profile %#v", profile, session.profile)
	}

	reviews, err := fetchBrowserEmployerReviewSignalsThrottled(ctx, "Acme")
	if err != nil {
		t.Fatalf("fetchBrowserEmployerReviewSignalsThrottled() error = %v", err)
	}
	if len(reviews) != 1 || reviews[0].Source != "glassdoor" {
		t.Fatalf("fetchBrowserEmployerReviewSignalsThrottled() = %#v, want glassdoor review", reviews)
	}
	if session.profileCalls != 1 {
		t.Fatalf("session profile calls = %d, want 1", session.profileCalls)
	}
	if session.reviewCalls != 1 {
		t.Fatalf("session review calls = %d, want 1", session.reviewCalls)
	}
}

func TestApplyOptionalLLMCompanyHealthSkipsEmployerReviewsForStrongResult(t *testing.T) {
	originalInit := initConfiguredLLMForTask
	originalEvaluate := evaluateCompanyHealthWithLLM
	defer func() {
		initConfiguredLLMForTask = originalInit
		evaluateCompanyHealthWithLLM = originalEvaluate
	}()

	initConfiguredLLMForTask = func(ctx context.Context, appCfg *config.AppConfig, task string) (llms.Model, func(), error) {
		return fakeHealthLLM{}, func() {}, nil
	}
	assessmentCalled := false
	evaluateCompanyHealthWithLLM = func(ctx context.Context, model llms.Model, result *domain.CompanyHealthResult) (*domain.LLMCompanyHealthAssessment, error) {
		assessmentCalled = true
		return &domain.LLMCompanyHealthAssessment{RiskLevel: "Low"}, nil
	}

	session := &fakeBrowserSession{
		reviews: []domain.EmployerReviewSignal{
			{Source: "glassdoor", Title: "Acme reviews"},
		},
	}
	ctx := ContextWithBrowserSession(context.Background(), session)
	result := &domain.CompanyHealthResult{
		Company:     "Acme",
		Score:       82,
		Confidence:  "high",
		SignalsUsed: []string{"job_company_identity", "wikidata_ticker", "sec_edgar", "sec_10k_headcount", "google_news_rss"},
		Sources: map[string]any{
			"company_identity": map[string]string{},
			"wikidata":         map[string]string{},
			"sec":              map[string]string{},
			"stock_history":    map[string]string{},
			"news":             map[string]string{},
		},
		EmploymentRisk: &domain.EmploymentRisk{Level: "Low", Score: 5},
	}

	ApplyOptionalLLMCompanyHealth(ctx, &config.AppConfig{
		LLM: config.LLMConfig{
			Enabled:       true,
			CompanyHealth: true,
			Provider:      "openai",
		},
	}, result)

	if session.reviewCalls != 0 {
		t.Fatalf("review calls = %d, want 0 for strong result", session.reviewCalls)
	}
	if len(result.EmployerReviews) != 0 {
		t.Fatalf("EmployerReviews = %#v, want none", result.EmployerReviews)
	}
	if !assessmentCalled {
		t.Fatal("LLM assessment was not called")
	}
	if result.LLMAssessment == nil {
		t.Fatal("LLMAssessment is nil")
	}
}

func TestApplyOptionalLLMCompanyHealthFetchesEmployerReviewsForWeakResult(t *testing.T) {
	originalInit := initConfiguredLLMForTask
	originalEvaluate := evaluateCompanyHealthWithLLM
	defer func() {
		initConfiguredLLMForTask = originalInit
		evaluateCompanyHealthWithLLM = originalEvaluate
	}()

	initConfiguredLLMForTask = func(ctx context.Context, appCfg *config.AppConfig, task string) (llms.Model, func(), error) {
		return fakeHealthLLM{}, func() {}, nil
	}
	evaluateCompanyHealthWithLLM = func(ctx context.Context, model llms.Model, result *domain.CompanyHealthResult) (*domain.LLMCompanyHealthAssessment, error) {
		return &domain.LLMCompanyHealthAssessment{RiskLevel: "Medium"}, nil
	}

	session := &fakeBrowserSession{
		reviews: []domain.EmployerReviewSignal{
			{Source: "glassdoor", Title: "Acme reviews"},
		},
	}
	ctx := ContextWithBrowserSession(context.Background(), session)
	result := &domain.CompanyHealthResult{
		Company:        "Acme",
		Score:          62,
		Confidence:     "medium",
		SignalsUsed:    []string{"job_company_identity", "google_news_rss"},
		Sources:        map[string]any{"news": map[string]string{}},
		EmploymentRisk: &domain.EmploymentRisk{Level: "Medium", Score: 25},
	}

	ApplyOptionalLLMCompanyHealth(ctx, &config.AppConfig{
		LLM: config.LLMConfig{
			Enabled:       true,
			CompanyHealth: true,
			Provider:      "openai",
		},
	}, result)

	if session.reviewCalls != 1 {
		t.Fatalf("review calls = %d, want 1 for weak result", session.reviewCalls)
	}
	if len(result.EmployerReviews) != 1 {
		t.Fatalf("EmployerReviews = %#v, want one review", result.EmployerReviews)
	}
}

func TestApplyOptionalLLMCompanyHealthFetchesConcernArticlesAndAppliesNovelModifier(t *testing.T) {
	originalInit := initConfiguredLLMForTask
	originalEvaluate := evaluateCompanyHealthWithLLM
	defer func() {
		initConfiguredLLMForTask = originalInit
		evaluateCompanyHealthWithLLM = originalEvaluate
	}()

	initConfiguredLLMForTask = func(ctx context.Context, appCfg *config.AppConfig, task string) (llms.Model, func(), error) {
		return fakeHealthLLM{}, func() {}, nil
	}
	storyURL := "https://news.example/acme-investigation"
	evaluateCompanyHealthWithLLM = func(ctx context.Context, model llms.Model, result *domain.CompanyHealthResult) (*domain.LLMCompanyHealthAssessment, error) {
		if len(result.ConcernStoryArticles) != 1 {
			t.Fatalf("len(ConcernStoryArticles) = %d, want 1", len(result.ConcernStoryArticles))
		}
		if !strings.Contains(result.ConcernStoryArticles[0].Text, "credit line was suspended") {
			t.Fatalf("ConcernStoryArticles[0].Text = %q; want fetched page text", result.ConcernStoryArticles[0].Text)
		}
		delta := -6
		return &domain.LLMCompanyHealthAssessment{
			StoryInsight: "Recent coverage describes a credit-line suspension not visible in headline scoring.",
			ArticleReviews: []domain.LLMCompanyHealthArticleReview{
				{
					URL:       storyURL,
					Source:    "google_news_rss",
					Related:   true,
					NovelFact: "The article says Acme's credit line was suspended after regulatory review.",
				},
			},
			ScoreModifier:          &delta,
			ScoreModifierReason:    "The page adds a concrete financing concern.",
			ScoreModifierNovelFact: "The article says Acme's credit line was suspended after regulatory review.",
			ScoreModifierSources:   []string{storyURL},
			FollowUpQuestions:      []string{"Ask whether financing has been restored."},
			PositiveSignals:        []string{},
			Concerns:               []string{"Financing risk needs follow-up."},
			RiskLevel:              "medium",
			Recommendation:         "Ask about the financing issue before applying.",
		}, nil
	}

	published := time.Now().AddDate(0, -1, 0)
	session := &fakeBrowserSession{
		articleText: "Acme's credit line was suspended after a regulatory review of its lending controls.",
	}
	ctx := ContextWithBrowserSession(context.Background(), session)
	result := &domain.CompanyHealthResult{
		Company:        "Acme",
		Score:          70,
		Confidence:     "medium",
		Sources:        map[string]any{},
		EmploymentRisk: &domain.EmploymentRisk{Level: "Low", Score: 5},
		ConcernStories: []domain.CompanyHealthConcernStory{
			{
				Source:  "google_news_rss",
				Title:   "Acme faces regulatory investigation",
				URL:     storyURL,
				Date:    &published,
				Concern: "negative news keyword: investigation",
			},
		},
	}

	ApplyOptionalLLMCompanyHealth(ctx, &config.AppConfig{
		LLM: config.LLMConfig{
			Enabled:       true,
			CompanyHealth: true,
			Provider:      "openai",
		},
	}, result)

	if session.articleCalls != 1 {
		t.Fatalf("article calls = %d, want 1", session.articleCalls)
	}
	if result.Score != 64 {
		t.Fatalf("Score = %d, want 64 after LLM modifier", result.Score)
	}
	if result.LLMAssessment == nil || result.LLMAssessment.StoryInsight == "" {
		t.Fatalf("LLMAssessment = %#v; want story insight", result.LLMAssessment)
	}
}
