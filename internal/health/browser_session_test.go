package health

import (
	"context"
	"testing"

	"github.com/tmc/langchaingo/llms"
	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
)

type fakeBrowserSession struct {
	profileCalls int
	reviewCalls  int
	profile      *domain.CompanySiteProfile
	reviews      []domain.EmployerReviewSignal
}

func (s *fakeBrowserSession) FetchCompanySiteProfile(ctx context.Context, identity domain.CompanyHealthContext) (*domain.CompanySiteProfile, error) {
	s.profileCalls++
	return s.profile, nil
}

func (s *fakeBrowserSession) FetchEmployerReviewSignals(ctx context.Context, company string) ([]domain.EmployerReviewSignal, error) {
	s.reviewCalls++
	return s.reviews, nil
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
