package health

import (
	"context"
	"reflect"
	"testing"

	"github.com/tmc/langchaingo/llms"
	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
)

type fakeHealthLLM struct{}

func (fakeHealthLLM) GenerateContent(context.Context, []llms.MessageContent, ...llms.CallOption) (*llms.ContentResponse, error) {
	return &llms.ContentResponse{}, nil
}

func (fakeHealthLLM) Call(context.Context, string, ...llms.CallOption) (string, error) {
	return "", nil
}

func TestApplyOptionalLLMCompanyIdentityEnrichesIncompleteIdentity(t *testing.T) {
	originalInit := initConfiguredLLMForTask
	originalEnrich := enrichCompanyHealthIdentityWithLLM
	defer func() {
		initConfiguredLLMForTask = originalInit
		enrichCompanyHealthIdentityWithLLM = originalEnrich
	}()

	initCalled := false
	initConfiguredLLMForTask = func(ctx context.Context, appCfg *config.AppConfig, task string) (llms.Model, func(), error) {
		if task != "company_health" {
			t.Fatalf("task = %q, want company_health", task)
		}
		initCalled = true
		return fakeHealthLLM{}, func() {}, nil
	}
	enrichCompanyHealthIdentityWithLLM = func(ctx context.Context, model llms.Model, identity domain.CompanyHealthContext) (*llmpkg.CompanyIdentitySearchResult, llmpkg.LLMTokenUsage, error) {
		if identity.Company != "Acme Cloud" {
			t.Fatalf("identity.Company = %q, want Acme Cloud", identity.Company)
		}
		return &llmpkg.CompanyIdentitySearchResult{
			CanonicalName: "Acme Cloud Inc.",
			Aliases:       []string{"AcmeCloud"},
			Website:       "https://www.acmecloud.example",
			Industry:      "Developer Tools",
			Summary:       "Acme Cloud builds deployment automation for software teams.",
		}, llmpkg.LLMTokenUsage{}, nil
	}

	got := ApplyOptionalLLMCompanyIdentity(context.Background(), &config.AppConfig{
		LLM: config.LLMConfig{
			Enabled:       true,
			CompanyHealth: true,
			Provider:      "openai",
		},
	}, domain.CompanyHealthContext{Company: "Acme Cloud"})

	if !initCalled {
		t.Fatal("LLM init was not called")
	}
	if got.Website != "https://www.acmecloud.example" {
		t.Fatalf("got.Website = %q, want https://www.acmecloud.example", got.Website)
	}
	if got.Industry != "Developer Tools" {
		t.Fatalf("got.Industry = %q, want Developer Tools", got.Industry)
	}
	if got.Summary == "" {
		t.Fatal("got.Summary is empty, want LLM summary")
	}
	if len(got.Aliases) != 2 {
		t.Fatalf("got.Aliases = %#v, want canonical alias and short alias", got.Aliases)
	}
}

func TestApplyOptionalLLMCompanyIdentitySkipsCompleteIdentity(t *testing.T) {
	originalInit := initConfiguredLLMForTask
	defer func() {
		initConfiguredLLMForTask = originalInit
	}()

	initConfiguredLLMForTask = func(ctx context.Context, appCfg *config.AppConfig, task string) (llms.Model, func(), error) {
		t.Fatal("LLM init should not be called for complete identity")
		return nil, func() {}, nil
	}

	identity := domain.CompanyHealthContext{
		Company:  "Acme Cloud",
		Website:  "https://www.acmecloud.example",
		Summary:  "Acme Cloud builds deployment automation for software teams.",
		Industry: "Developer Tools",
	}

	got := ApplyOptionalLLMCompanyIdentity(context.Background(), &config.AppConfig{
		LLM: config.LLMConfig{
			Enabled:       true,
			CompanyHealth: true,
			Provider:      "openai",
		},
	}, identity)

	if !reflect.DeepEqual(got, identity) {
		t.Fatalf("ApplyOptionalLLMCompanyIdentity() = %#v, want unchanged identity", got)
	}
}

func TestApplyLLMCompanyHealthScoreModifierRequiresOnlyRelatedArticleSources(t *testing.T) {
	storyURL := "https://news.example/acme-risk"
	allowed := map[string]bool{normalizedStoryURL(storyURL): true}

	if !modifierUsesOnlyAllowedSources([]string{storyURL + "/"}, allowed) {
		t.Fatal("modifierUsesOnlyAllowedSources() = false, want true for only related article source")
	}

	for name, sources := range map[string][]string{
		"none":      nil,
		"blank":     {""},
		"unrelated": {"https://other.example/acme-risk"},
		"mixed":     {storyURL, "https://other.example/acme-risk"},
	} {
		t.Run(name, func(t *testing.T) {
			if modifierUsesOnlyAllowedSources(sources, allowed) {
				t.Fatalf("modifierUsesOnlyAllowedSources(%#v) = true, want false", sources)
			}
		})
	}
}

func TestApplyLLMCompanyHealthScoreModifierClearsRejectedArticleFields(t *testing.T) {
	delta := -5
	assessment := &domain.LLMCompanyHealthAssessment{
		StoryInsight: "Unrelated coverage should not be retained as company insight.",
		ArticleReviews: []domain.LLMCompanyHealthArticleReview{
			{
				URL:             "https://other.example/not-acme",
				Related:         true,
				RelevanceReason: "This is not the article context URL.",
				NovelFact:       "This fact came from an unrelated URL.",
			},
		},
		ScoreModifier:          &delta,
		ScoreModifierReason:    "Unrelated article reason.",
		ScoreModifierNovelFact: "Unrelated article fact.",
		ScoreModifierSources:   []string{"https://other.example/not-acme"},
	}
	result := &domain.CompanyHealthResult{
		Company: "Acme",
		Score:   70,
		ConcernStoryArticles: []domain.CompanyHealthConcernArticle{
			{URL: "https://news.example/acme-risk"},
		},
	}

	applyLLMCompanyHealthScoreModifier(result, assessment)

	if result.Score != 70 {
		t.Fatalf("Score = %d, want unchanged 70", result.Score)
	}
	if assessment.StoryInsight != "" || len(assessment.ArticleReviews) != 0 {
		t.Fatalf("article fields = insight %q reviews %#v; want cleared", assessment.StoryInsight, assessment.ArticleReviews)
	}
	if assessment.ScoreModifier != nil || assessment.ScoreModifierReason != "" || assessment.ScoreModifierNovelFact != "" || len(assessment.ScoreModifierSources) != 0 {
		t.Fatalf("modifier fields were not cleared: %#v", assessment)
	}
}
