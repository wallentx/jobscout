package llm

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/wallentx/jobscout/internal/domain"
)

type fakeContentLLM struct {
	content        string
	generationInfo map[string]any
}

func (m fakeContentLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: m.content, GenerationInfo: m.generationInfo}},
	}, nil
}

func (m fakeContentLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return m.content, nil
}

func TestBuildPromptIncludesCriteria(t *testing.T) {
	var criteria CriteriaConfig
	criteria.Candidate.City = "Example City"
	criteria.Candidate.State = "EX"
	criteria.Candidate.CountryCode = "US"
	criteria.Candidate.YearsOfExperience = 3
	criteria.Filters.TitleRequires = []string{"Engineer"}
	criteria.Filters.TitleIncludes = []string{"backend", "systems"}
	criteria.Filters.TitleExcludes = []string{"contract"}
	criteria.Filters.WorkSettings.Remote = true
	criteria.Filters.MinBaseUSD = 100000
	criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}
	criteria.PrioritySignals = []string{"reliability", "automation"}

	prompt := buildPrompt(Job{
		Company:      "Acme",
		Title:        "Staff Platform Engineer",
		Remote:       "Remote",
		Compensation: "$220k",
		Description:  "Build reliable infrastructure.",
	}, &criteria)

	for _, expected := range []string{
		"### User Criteria:",
		"Candidate location: Example City, EX, US",
		"Years of experience: 3",
		"Required title prefixes/levels: Engineer",
		"Target title names: backend, systems",
		"Excluded title terms: contract",
		"Allowed work settings: remote",
		"Minimum base salary: $100000 USD",
		"Role families: devops_sre_systems",
		"Priority signals: reliability, automation",
		"### Job Posting to Evaluate:",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("buildPrompt() missing %q in:\n%s", expected, prompt)
		}
	}
}

func TestEvaluateJobWithLLMParsesRejectionReasons(t *testing.T) {
	llm := fakeContentLLM{content: `{
		"matches": false,
		"compensation_extracted": "$70k base",
		"remote_eligibility": "Onsite",
		"why_it_matches": [],
		"why_rejected": ["Below minimum salary", "Wrong work setting"]
	}`}

	result, err := EvaluateJobWithLLM(context.Background(), llm, Job{
		Company:      "Acme",
		Title:        "Software Engineer",
		Remote:       "Onsite",
		Compensation: "$70k base",
		Description:  "Build software.",
	}, nil)
	if err != nil {
		t.Fatalf("EvaluateJobWithLLM(...) error = %v", err)
	}
	if result.Matches {
		t.Fatal("EvaluateJobWithLLM(...).Matches = true, want false")
	}
	if len(result.WhyRejected) != 2 || result.WhyRejected[0] != "Below minimum salary" {
		t.Fatalf("EvaluateJobWithLLM(...).WhyRejected = %#v, want rejection reasons", result.WhyRejected)
	}
}

func TestEvaluateCompanyHealthWithLLM(t *testing.T) {
	result := &CompanyHealthResult{
		Company:    "Acme",
		Score:      62,
		Confidence: "medium",
		Flags:      []string{"negative_news_signal"},
		Notes:      []string{"News titles contain some negative-risk keywords."},
	}
	llm := fakeContentLLM{content: `{
		"summary": "Mixed signals, but not an automatic reject.",
		"recommendation": "Investigate before applying.",
		"risk_level": "medium",
		"positive_signals": ["Some stabilizing signals exist."],
		"concerns": ["Recent negative news needs review."],
		"follow_up_questions": ["Ask about team stability."]
	}`}

	assessment, err := evaluateCompanyHealthWithLLM(context.Background(), llm, result)
	if err != nil {
		t.Fatalf("evaluateCompanyHealthWithLLM() error = %v", err)
	}
	if assessment.Summary != "Mixed signals, but not an automatic reject." {
		t.Fatalf("assessment.Summary = %q, want mixed-signals summary", assessment.Summary)
	}
	if len(assessment.Concerns) != 1 || assessment.Concerns[0] != "Recent negative news needs review." {
		t.Fatalf("assessment.Concerns = %#v, want one concern", assessment.Concerns)
	}
}

func TestEnrichJobIdentityWithLLM(t *testing.T) {
	llm := fakeContentLLM{content: `{
		"company_website": "https://www.acme.example",
		"company_summary": "Acme builds deployment tooling for software teams.",
		"company_industry": "Developer Tools",
		"website_confidence": "high",
		"summary_confidence": "high",
		"industry_confidence": "medium",
		"industry_provisional": false,
		"company_website_reason": "The page links to the company website.",
		"company_summary_reason": "The page describes Acme's product.",
		"company_industry_reason": "The page explicitly describes developer tooling."
	}`}

	result, err := enrichJobIdentityWithLLM(context.Background(), llm, Job{
		Company:  "Acme",
		Title:    "Staff Platform Engineer",
		ApplyURL: "https://jobs.example/acme/platform",
	}, JobIdentityPage{
		URL:  "https://jobs.example/acme/platform",
		Text: "Acme builds deployment tooling for software teams. Website: https://www.acme.example",
	})
	if err != nil {
		t.Fatalf("enrichJobIdentityWithLLM() error = %v", err)
	}
	if result.CompanyWebsite != "https://www.acme.example" {
		t.Fatalf("result.CompanyWebsite = %q; want https://www.acme.example", result.CompanyWebsite)
	}
	if result.CompanyIndustry != "Developer Tools" {
		t.Fatalf("result.CompanyIndustry = %q; want Developer Tools", result.CompanyIndustry)
	}
	if result.IndustryProvisional {
		t.Fatalf("result.IndustryProvisional = true, want false")
	}
}

func TestBuildCompanyIdentitySearchPromptIncludesIdentityAndSourceRules(t *testing.T) {
	prompt := buildCompanyIdentitySearchPrompt(domain.CompanyHealthContext{
		Company:  "Acme Cloud",
		Aliases:  []string{"AcmeCloud"},
		Website:  "https://www.acmecloud.example",
		Summary:  "Acme Cloud builds deployment automation for software teams.",
		Industry: "Developer Tools",
	})

	for _, expected := range []string{
		"Acme Cloud",
		"AcmeCloud",
		"https://www.acmecloud.example",
		"Developer Tools",
		"Do not score company health",
		"Website/domain is the strongest identity anchor",
		"source URL",
		"Return ONLY valid JSON",
		`"canonical_name"`,
		`"sources"`,
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("buildCompanyIdentitySearchPrompt() missing %q in:\n%s", expected, prompt)
		}
	}
}

func TestEnrichCompanyHealthIdentityWithLLMParsesSourcedFacts(t *testing.T) {
	llm := fakeContentLLM{content: `{
		"canonical_name": "Acme Cloud",
		"aliases": ["AcmeCloud"],
		"website": "https://www.acmecloud.example",
		"industry": "Developer Tools",
		"summary": "Acme Cloud builds deployment automation for software teams.",
		"sources": [
			{
				"url": "https://www.acmecloud.example/about",
				"supports": ["website", "summary", "industry"]
			}
		]
	}`}

	result, _, err := enrichCompanyHealthIdentityWithLLM(context.Background(), llm, domain.CompanyHealthContext{
		Company: "Acme Cloud",
	})
	if err != nil {
		t.Fatalf("enrichCompanyHealthIdentityWithLLM() error = %v", err)
	}
	if result.CanonicalName != "Acme Cloud" {
		t.Fatalf("result.CanonicalName = %q, want Acme Cloud", result.CanonicalName)
	}
	if result.Website != "https://www.acmecloud.example" {
		t.Fatalf("result.Website = %q, want https://www.acmecloud.example", result.Website)
	}
	if result.Industry != "Developer Tools" {
		t.Fatalf("result.Industry = %q, want Developer Tools", result.Industry)
	}
	if len(result.Sources) != 1 || result.Sources[0].URL != "https://www.acmecloud.example/about" {
		t.Fatalf("result.Sources = %#v, want sourced about URL", result.Sources)
	}
}

func TestApplyCompanyIdentitySearchResultPreservesExistingFields(t *testing.T) {
	identity := applyCompanyIdentitySearchResult(domain.CompanyHealthContext{
		Company: "Acme Cloud",
		Aliases: []string{"Acme"},
		Website: "https://known.example",
	}, &CompanyIdentitySearchResult{
		CanonicalName: "Acme Cloud Inc.",
		Aliases:       []string{"AcmeCloud", "Acme"},
		Website:       "https://wrong.example",
		Industry:      "Developer Tools",
		Summary:       "Acme Cloud builds deployment automation for software teams.",
	})

	if identity.Website != "https://known.example" {
		t.Fatalf("identity.Website = %q, want existing website", identity.Website)
	}
	if identity.Industry != "Developer Tools" {
		t.Fatalf("identity.Industry = %q, want Developer Tools", identity.Industry)
	}
	if identity.Summary == "" {
		t.Fatal("identity.Summary is empty, want LLM summary")
	}
	for _, expected := range []string{"Acme", "AcmeCloud", "Acme Cloud Inc."} {
		if !slices.Contains(identity.Aliases, expected) {
			t.Fatalf("identity.Aliases = %#v, want %q", identity.Aliases, expected)
		}
	}
}

func TestApplyCompanyIdentitySearchResultRejectsSharedProfileWebsite(t *testing.T) {
	identity := applyCompanyIdentitySearchResult(domain.CompanyHealthContext{
		Company: "Acme Cloud",
	}, &CompanyIdentitySearchResult{
		CanonicalName: "Acme Cloud Inc.",
		Website:       "https://www.linkedin.com/company/acme-cloud/",
		Industry:      "Developer Tools",
		Summary:       "Acme Cloud builds deployment automation for software teams.",
	})

	if identity.Website != "" {
		t.Fatalf("identity.Website = %q, want rejected shared profile host", identity.Website)
	}
	if identity.Industry != "Developer Tools" {
		t.Fatalf("identity.Industry = %q, want non-website fields preserved", identity.Industry)
	}
	if identity.Summary == "" {
		t.Fatal("identity.Summary is empty, want non-website fields preserved")
	}
}

func TestBuildJobIdentityPromptCapsNoisyPageText(t *testing.T) {
	pageText := "Concept Plus builds software for federal clients. " + strings.Repeat(`"`, 20000)

	prompt := buildJobIdentityPrompt(Job{
		Company:  "Concept Plus",
		Title:    "Jr. Full Stack Developer",
		ApplyURL: "https://builtin.com/job/full-stack-developer/123",
	}, JobIdentityPage{
		URL:  "https://conceptplus.com/",
		Text: pageText,
	})

	if got, wantMax := len(prompt), 18000; got > wantMax {
		t.Fatalf("len(buildJobIdentityPrompt(...)) = %d, want <= %d", got, wantMax)
	}
	if !strings.Contains(prompt, "[truncated]") {
		t.Fatalf("buildJobIdentityPrompt(...) missing truncation marker")
	}
}

func TestEvaluateResumeCriteriaWithLLM(t *testing.T) {
	llm := fakeContentLLM{content: `{
		"candidate": {
			"city": "Chicago",
			"state": "Texas",
			"country_code": "United States",
			"years_of_experience": 8
		},
		"role_families": ["backend", "devops_sre_systems"],
		"title_requires": ["Engineer"],
		"title_includes": ["Platform", "Infrastructure"],
		"title_excludes": ["Manager"],
		"work_settings": ["remote", "hybrid"],
		"min_base_usd": 150000,
		"priority_signals": ["Kubernetes", "Go", "reliability"]
	}`}

	criteria, err := evaluateResumeCriteriaWithLLM(context.Background(), llm, "Example resume")
	if err != nil {
		t.Fatalf("evaluateResumeCriteriaWithLLM() error = %v", err)
	}
	if criteria.Candidate.City != "Chicago" || criteria.Candidate.YearsOfExperience != 8 {
		t.Fatalf("criteria.Candidate = %#v, want Chicago with 8 years", criteria.Candidate)
	}
	if criteria.Candidate.State != "TX" {
		t.Fatalf("criteria.Candidate.State = %q, want TX", criteria.Candidate.State)
	}
	if criteria.Candidate.CountryCode != "US" {
		t.Fatalf("criteria.Candidate.CountryCode = %q, want US", criteria.Candidate.CountryCode)
	}
	if len(criteria.RoleFamilies) != 2 || criteria.RoleFamilies[0] != RoleBackendEngineering || criteria.RoleFamilies[1] != RoleDevOpsSRESystems {
		t.Fatalf("criteria.RoleFamilies = %#v, want backend and devops_sre_systems", criteria.RoleFamilies)
	}
	if !criteria.Filters.WorkSettings.Remote || !criteria.Filters.WorkSettings.Hybrid {
		t.Fatalf("criteria.Filters.WorkSettings = %#v, want remote and hybrid", criteria.Filters.WorkSettings)
	}
	if criteria.Filters.MinBaseUSD != 150000 {
		t.Fatalf("criteria.Filters.MinBaseUSD = %d, want 150000", criteria.Filters.MinBaseUSD)
	}
}

func TestBuildCompanyHealthLLMPromptIncludesSignals(t *testing.T) {
	result := &CompanyHealthResult{
		Company:    "Acme",
		Score:      62,
		Confidence: "medium",
		LayoffSignals: []LayoffSignal{
			{Title: "Acme announces restructuring"},
		},
		HNSignals: []HNSignal{
			{Title: "Acme launches new developer platform", Points: 120, NumComments: 40},
		},
		EmployerReviews: []EmployerReviewSignal{
			{
				Source:           "glassdoor",
				Title:            "Acme Reviews",
				Rating:           "3.7/5",
				ReviewCount:      intPtr(512),
				RecommendPercent: intPtr(84),
				Snippet:          "Employees mention good culture and long hours.",
				Flags:            []string{"long hours", "positive culture"},
			},
		},
	}

	prompt := buildCompanyHealthLLMPrompt(result)
	for _, expected := range []string{
		"Review this deterministic company health assessment",
		`"company": "Acme"`,
		"Acme announces restructuring",
		"Acme launches new developer platform",
		"glassdoor | rating 3.7/5",
		"512 reviews",
		"84% recommend",
		"long hours",
		"Return ONLY valid JSON",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("buildCompanyHealthLLMPrompt() missing %q in:\n%s", expected, prompt)
		}
	}
}

func TestBuildCompanyHealthLLMPromptIncludesConcernArticleContext(t *testing.T) {
	published := time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)
	result := &CompanyHealthResult{
		Company: "Acme",
		ConcernStoryArticles: []domain.CompanyHealthConcernArticle{
			{
				Source:  "google_news_rss",
				Title:   "Acme faces regulatory investigation",
				URL:     "https://news.example/acme-investigation",
				Date:    &published,
				Concern: "negative news keyword: investigation",
				Text:    "Acme disclosed that a regulator opened an investigation into its hiring and safety practices.",
			},
		},
	}

	input := parseCompanyHealthPromptInput(t, buildCompanyHealthLLMPrompt(result))
	if len(input.ArticleContext) != 1 {
		t.Fatalf("len(input.ArticleContext) = %d, want 1", len(input.ArticleContext))
	}
	article := input.ArticleContext[0]
	if article.URL != "https://news.example/acme-investigation" || article.Concern != "negative news keyword: investigation" {
		t.Fatalf("ArticleContext[0] = %#v; want story metadata", article)
	}
	if !strings.Contains(article.Text, "regulator opened an investigation") {
		t.Fatalf("ArticleContext[0].Text = %q; want fetched article text", article.Text)
	}
}

func TestEvaluateCompanyHealthWithLLMParsesStoryInsightAndModifier(t *testing.T) {
	llm := fakeContentLLM{content: `{
		"summary": "Recent coverage adds one concrete risk.",
		"recommendation": "Ask about the investigation before applying.",
		"risk_level": "medium",
		"positive_signals": [],
		"concerns": ["Regulatory follow-up is warranted."],
		"follow_up_questions": ["Has the matter been resolved?"],
		"article_reviews": [{
			"url": "https://news.example/acme-investigation",
			"source": "google_news_rss",
			"related": true,
			"relevance_reason": "The page names Acme and its security software business.",
			"novel_fact": "The article says a regulator opened an investigation into Acme's hiring and safety practices."
		}],
		"story_insight": "Recent coverage points to an unresolved regulatory issue that is broader than the headline alone.",
		"score_modifier": -6,
		"score_modifier_reason": "The page describes an active regulator investigation.",
		"score_modifier_novel_fact": "The article says a regulator opened an investigation into Acme's hiring and safety practices.",
		"score_modifier_sources": ["https://news.example/acme-investigation"]
	}`}

	assessment, err := evaluateCompanyHealthWithLLM(context.Background(), llm, &CompanyHealthResult{Company: "Acme"})
	if err != nil {
		t.Fatalf("evaluateCompanyHealthWithLLM() error = %v", err)
	}
	if assessment.StoryInsight == "" {
		t.Fatalf("StoryInsight = empty; want article insight")
	}
	if assessment.ScoreModifier == nil || *assessment.ScoreModifier != -6 {
		t.Fatalf("ScoreModifier = %#v; want -6", assessment.ScoreModifier)
	}
	if len(assessment.ArticleReviews) != 1 || !assessment.ArticleReviews[0].Related {
		t.Fatalf("ArticleReviews = %#v; want one related review", assessment.ArticleReviews)
	}
	if assessment.ScoreModifierNovelFact == "" || len(assessment.ScoreModifierSources) != 1 {
		t.Fatalf("assessment = %#v; want novel fact and source", assessment)
	}
}

func TestBuildCompanyHealthLLMPromptIncludesAllSmallRejectedEvidence(t *testing.T) {
	result := &CompanyHealthResult{
		Company: "Acme",
		RejectedEvidence: []CompanyHealthEvidence{
			{Source: "news", Value: "Acme unrelated article", Reason: "domain mismatch", URL: "https://news.example/1"},
			{Source: "hacker_news", Value: "Wrong Acme thread", Reason: "company name mismatch"},
		},
	}

	input := parseCompanyHealthPromptInput(t, buildCompanyHealthLLMPrompt(result))
	if got, want := len(input.RejectedEvidence), 2; got != want {
		t.Fatalf("len(input.RejectedEvidence) = %d, want %d", got, want)
	}
	if got := len(input.RejectedOmitted); got != 0 {
		t.Fatalf("len(input.RejectedOmitted) = %d, want 0", got)
	}
	for _, expected := range []string{
		"news | Acme unrelated article | rejected: domain mismatch | https://news.example/1",
		"hacker_news | Wrong Acme thread | rejected: company name mismatch",
	} {
		if !containsString(input.RejectedEvidence, expected) {
			t.Fatalf("input.RejectedEvidence = %#v, want to contain %q", input.RejectedEvidence, expected)
		}
	}
}

func TestBuildCompanyHealthLLMPromptCapsRejectedEvidenceAndSummarizesOmitted(t *testing.T) {
	rejected := make([]CompanyHealthEvidence, 0, maxCompanyHealthRejectedEvidenceInPrompt+4)
	for i := 0; i < maxCompanyHealthRejectedEvidenceInPrompt; i++ {
		rejected = append(rejected, CompanyHealthEvidence{
			Source: "included_source",
			Value:  "included evidence",
			Reason: "included reason",
		})
	}
	rejected = append(rejected,
		CompanyHealthEvidence{Source: "news", Value: "omitted news 1", Reason: "domain mismatch"},
		CompanyHealthEvidence{Source: "news", Value: "omitted news 2", Reason: "domain mismatch"},
		CompanyHealthEvidence{Source: "hacker_news", Value: "omitted hn", Reason: "company name mismatch"},
		CompanyHealthEvidence{Value: "omitted unknown"},
	)

	prompt, stats := buildCompanyHealthLLMPromptWithStats(&CompanyHealthResult{
		Company:          "Acme",
		RejectedEvidence: rejected,
	})
	if got, want := stats.RejectedEvidenceTotal, maxCompanyHealthRejectedEvidenceInPrompt+4; got != want {
		t.Fatalf("stats.RejectedEvidenceTotal = %d, want %d", got, want)
	}
	if got, want := stats.RejectedEvidenceIncluded, maxCompanyHealthRejectedEvidenceInPrompt; got != want {
		t.Fatalf("stats.RejectedEvidenceIncluded = %d, want %d", got, want)
	}
	if got, want := stats.RejectedEvidenceOmitted, 4; got != want {
		t.Fatalf("stats.RejectedEvidenceOmitted = %d, want %d", got, want)
	}

	input := parseCompanyHealthPromptInput(t, prompt)
	if got, want := len(input.RejectedEvidence), maxCompanyHealthRejectedEvidenceInPrompt; got != want {
		t.Fatalf("len(input.RejectedEvidence) = %d, want %d", got, want)
	}
	if strings.Contains(strings.Join(input.RejectedEvidence, "\n"), "omitted news") {
		t.Fatalf("input.RejectedEvidence = %#v, want omitted evidence values excluded", input.RejectedEvidence)
	}

	gotSummary := rejectedEvidenceSummaryBySourceReason(input.RejectedOmitted)
	wantSummary := map[string]int{
		"hacker_news\x00company name mismatch": 1,
		"news\x00domain mismatch":              2,
		"unknown\x00unspecified":               1,
	}
	if len(gotSummary) != len(wantSummary) {
		t.Fatalf("rejectedEvidenceSummaryBySourceReason(...) = %#v, want %#v", gotSummary, wantSummary)
	}
	for key, want := range wantSummary {
		if got := gotSummary[key]; got != want {
			t.Fatalf("rejectedEvidenceSummaryBySourceReason(...)[%q] = %d, want %d", key, got, want)
		}
	}
}

func parseCompanyHealthPromptInput(t *testing.T, prompt string) companyHealthLLMInput {
	t.Helper()

	const startMarker = "Assessment JSON:\n"
	const endMarker = "\n\nReturn ONLY valid JSON"
	start := strings.Index(prompt, startMarker)
	if start < 0 {
		t.Fatalf("buildCompanyHealthLLMPrompt(...) missing %q in:\n%s", startMarker, prompt)
	}
	start += len(startMarker)
	end := strings.Index(prompt[start:], endMarker)
	if end < 0 {
		t.Fatalf("buildCompanyHealthLLMPrompt(...) missing %q in:\n%s", endMarker, prompt)
	}

	var input companyHealthLLMInput
	if err := json.Unmarshal([]byte(prompt[start:start+end]), &input); err != nil {
		t.Fatalf("json.Unmarshal(company health prompt input) error = %v", err)
	}
	return input
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func rejectedEvidenceSummaryBySourceReason(summary []rejectedEvidenceOmissionSummary) map[string]int {
	result := make(map[string]int, len(summary))
	for _, item := range summary {
		result[item.Source+"\x00"+item.RejectionReason] = item.Count
	}
	return result
}
