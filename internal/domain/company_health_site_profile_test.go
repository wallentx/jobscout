package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestShouldUseBrowserCompanyProfile(t *testing.T) {
	foundedYear := 2018
	employees := 120

	tests := []struct {
		name   string
		result *CompanyHealthResult
		want   bool
	}{
		{
			name:   "nil result",
			result: nil,
			want:   false,
		},
		{
			name:   "missing founded year",
			result: &CompanyHealthResult{EstimatedEmployees: &employees, Confidence: "medium"},
			want:   true,
		},
		{
			name:   "missing employees",
			result: &CompanyHealthResult{FoundedYear: &foundedYear, Confidence: "medium"},
			want:   true,
		},
		{
			name:   "low confidence",
			result: &CompanyHealthResult{FoundedYear: &foundedYear, EstimatedEmployees: &employees, Confidence: "low"},
			want:   true,
		},
		{
			name:   "complete medium confidence",
			result: &CompanyHealthResult{FoundedYear: &foundedYear, EstimatedEmployees: &employees, Confidence: "medium"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldUseBrowserCompanyProfile(tt.result); got != tt.want {
				t.Errorf("ShouldUseBrowserCompanyProfile(%#v) = %v, want %v", tt.result, got, tt.want)
			}
		})
	}
}

func TestEnrichCompanyHealthContextFromSiteProfile(t *testing.T) {
	identity := CompanyHealthContext{
		Company: "Acme Cloud",
		Aliases: []string{"AcmeCloud"},
		Website: "https://acmecloud.example",
	}
	profile := &CompanySiteProfile{
		WebsiteURL: "https://www.acmecloud.example/",
		Summary:    "Acme Cloud builds deployment automation for software teams.",
		Industry:   "Developer Tools",
	}

	got := EnrichCompanyHealthContextFromSiteProfile(identity, profile)

	if got.Company != identity.Company {
		t.Fatalf("Company = %q; want %q", got.Company, identity.Company)
	}
	if got.Website != identity.Website {
		t.Fatalf("Website = %q; want existing website %q", got.Website, identity.Website)
	}
	if got.Summary != profile.Summary {
		t.Fatalf("Summary = %q; want %q", got.Summary, profile.Summary)
	}
	if got.Industry != profile.Industry {
		t.Fatalf("Industry = %q; want %q", got.Industry, profile.Industry)
	}
	if len(got.Aliases) != 1 || got.Aliases[0] != "AcmeCloud" {
		t.Fatalf("Aliases = %#v; want existing aliases preserved", got.Aliases)
	}
}

func TestEnrichCompanyHealthContextFromSiteProfileFillsMissingWebsite(t *testing.T) {
	identity := CompanyHealthContext{Company: "Acme Cloud"}
	profile := &CompanySiteProfile{
		WebsiteURL: "https://www.acmecloud.example/",
	}

	got := EnrichCompanyHealthContextFromSiteProfile(identity, profile)

	if got.Website != profile.WebsiteURL {
		t.Fatalf("Website = %q; want %q", got.Website, profile.WebsiteURL)
	}
}

func TestCompanyHealthWithDataSourcesUsesLateSiteProfileForNewsContext(t *testing.T) {
	originalHTTPGet := httpGet
	originalFetchGoogleNewsRSS := fetchGoogleNewsRSS
	defer func() {
		httpGet = originalHTTPGet
		fetchGoogleNewsRSS = originalFetchGoogleNewsRSS
	}()

	httpGet = func(rawURL string) ([]byte, error) {
		return nil, fmt.Errorf("blocked test HTTP fetch: %s", rawURL)
	}
	fetchGoogleNewsRSS = func(query string) ([]RSSItem, error) {
		if strings.Contains(strings.ToLower(query), "layoffs") {
			return nil, nil
		}
		return []RSSItem{{
			Title: "Texas grocery chain announces expansion and new hiring plans",
			Link:  "https://example.com/business/texas-grocery-chain-expansion",
		}}, nil
	}

	result, err := CompanyHealthWithDataSources(CompanyHealthContext{
		Company: "Regional Market",
	}, "", true, CompanyHealthDataSources{
		FetchCompanySiteProfile: func(identity CompanyHealthContext) (*CompanySiteProfile, error) {
			return &CompanySiteProfile{
				WebsiteURL: "https://regionalmarket.example",
				Summary:    "Regional Market is a Texas grocery store chain.",
				Industry:   "Grocery Stores",
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("CompanyHealthWithDataSources() error = %v", err)
	}
	if len(result.RejectedEvidence) != 0 {
		t.Fatalf("RejectedEvidence = %#v, want contextual news accepted", result.RejectedEvidence)
	}
	newsSource, ok := result.Sources["news"].(map[string]any)
	if !ok {
		t.Fatalf("news source = %#v, want map[string]any", result.Sources["news"])
	}
	titles, ok := newsSource["titles"].([]string)
	if !ok || len(titles) != 1 || !strings.Contains(titles[0], "Texas grocery chain") {
		t.Fatalf("news titles = %#v, want contextual grocery headline accepted", newsSource["titles"])
	}
}

func TestRecordCompanySiteProfileFetchErrorAddsBrowserNotice(t *testing.T) {
	result := &CompanyHealthResult{}

	recordCompanySiteProfileFetchError(result, errors.New("browser binary (Chrome/Chromium/Edge) not found"))

	if len(result.Notices) != 1 {
		t.Fatalf("Notices = %#v; want one browser notice", result.Notices)
	}
	if !strings.Contains(result.Notices[0], "Install Chrome or Chromium") {
		t.Fatalf("notice = %q; want browser install guidance", result.Notices[0])
	}
}

func TestApplyCompanySiteProfileAddsCompanySiteEvidence(t *testing.T) {
	result := &CompanyHealthResult{
		Confidence:  "low",
		SignalsUsed: []string{},
		Notes:       []string{},
		Sources:     make(map[string]any),
	}
	initCompanyHealthAssessments(result)

	profile := &CompanySiteProfile{
		SearchQuery: "Acme official site",
		SearchURL:   "https://search.example/?q=Acme",
		WebsiteURL:  "https://www.acme.example",
		AboutURL:    "https://www.acme.example/about",
		WebsiteText: "Acme makes anvils.",
		AboutText:   "Founded in 2012, Acme has 1,200 employees worldwide.",
	}

	ApplyCompanySiteProfile(result, profile)

	if len(result.SignalsUsed) != 1 || result.SignalsUsed[0] != "browser_company_site" {
		t.Fatalf("ApplyCompanySiteProfile() SignalsUsed = %#v, want browser_company_site", result.SignalsUsed)
	}
	source, ok := result.Sources["company_site"].(map[string]any)
	if !ok {
		t.Fatalf("ApplyCompanySiteProfile() company_site source = %#v, want map[string]any", result.Sources["company_site"])
	}
	wantSource := map[string]string{
		"search_query": profile.SearchQuery,
		"search_url":   profile.SearchURL,
		"website_url":  profile.WebsiteURL,
		"about_url":    profile.AboutURL,
	}
	for key, want := range wantSource {
		if got := source[key]; got != want {
			t.Errorf("ApplyCompanySiteProfile() source[%q] = %v, want %q", key, got, want)
		}
	}

	if result.FoundedYear == nil || *result.FoundedYear != 2012 {
		t.Fatalf("ApplyCompanySiteProfile() FoundedYear = %#v, want 2012", result.FoundedYear)
	}
	if result.EstimatedEmployees == nil || *result.EstimatedEmployees != 1200 {
		t.Fatalf("ApplyCompanySiteProfile() EstimatedEmployees = %#v, want 1200", result.EstimatedEmployees)
	}
	if result.Confidence != "medium" {
		t.Errorf("ApplyCompanySiteProfile() Confidence = %q, want medium", result.Confidence)
	}

	foundedAssessment := result.FieldAssessments["founded_year"]
	if foundedAssessment == nil || foundedAssessment.Source != "company_site" || foundedAssessment.URL != profile.AboutURL {
		t.Fatalf("ApplyCompanySiteProfile() founded_year assessment = %#v, want company_site with about URL", foundedAssessment)
	}
	employeesAssessment := result.FieldAssessments["estimated_employees"]
	if employeesAssessment == nil || employeesAssessment.Source != "company_site" || employeesAssessment.URL != profile.AboutURL {
		t.Fatalf("ApplyCompanySiteProfile() estimated_employees assessment = %#v, want company_site with about URL", employeesAssessment)
	}
}

func TestApplyCompanySiteProfileAppliesPublicProfileFactsAndReviews(t *testing.T) {
	employees := 3000
	foundedYear := 2019
	reviewCount := 512
	recommendPercent := 84
	ceoApprovalPercent := 92
	result := &CompanyHealthResult{
		Confidence:  "low",
		Score:       50,
		SignalsUsed: []string{},
		Notes:       []string{},
		Sources:     make(map[string]any),
	}
	initCompanyHealthAssessments(result)

	ApplyCompanySiteProfile(result, &CompanySiteProfile{
		PublicProfiles: []CompanyPublicProfile{
			{
				Source:             "linkedin",
				URL:                "https://www.linkedin.com/company/ramp/",
				Industry:           "Financial Services",
				EmployeeRange:      "1,001-5,000 employees",
				EstimatedEmployees: &employees,
				FoundedYear:        &foundedYear,
				Headquarters:       "New York, NY",
			},
			{
				Source:             "glassdoor",
				URL:                "https://www.glassdoor.com/Overview/Working-at-Ramp-EI_IE4211228.11,15.htm",
				Rating:             "4.2/5",
				ReviewCount:        &reviewCount,
				RecommendPercent:   &recommendPercent,
				CEOApprovalPercent: &ceoApprovalPercent,
				Snippet:            "Employees praise the product momentum and mention some long hours.",
			},
		},
	})

	if result.EstimatedEmployees == nil || *result.EstimatedEmployees != employees {
		t.Fatalf("EstimatedEmployees = %#v; want %d", result.EstimatedEmployees, employees)
	}
	if result.FoundedYear == nil || *result.FoundedYear != foundedYear {
		t.Fatalf("FoundedYear = %#v; want %d", result.FoundedYear, foundedYear)
	}
	if len(result.EmployerReviews) != 1 {
		t.Fatalf("EmployerReviews = %#v; want one Glassdoor review signal", result.EmployerReviews)
	}
	review := result.EmployerReviews[0]
	if review.Rating != "4.2/5" || review.ReviewCount == nil || *review.ReviewCount != reviewCount {
		t.Fatalf("EmployerReviews[0] = %#v; want rating and review count", review)
	}
	if _, ok := result.Sources["public_profiles"]; !ok {
		t.Fatalf("Sources = %#v; want public_profiles source", result.Sources)
	}
}

func TestApplyEmployerReviewHealthSignalsPenalizesWeakReviewMetrics(t *testing.T) {
	reviewCount := 420
	recommendPercent := 38
	result := &CompanyHealthResult{
		Company: "Acme",
		Score:   70,
		Flags:   []string{},
		Notes:   []string{},
		EmploymentRisk: &EmploymentRisk{
			Level:   "Low",
			Score:   0,
			Factors: []string{},
		},
		EmployerReviews: []EmployerReviewSignal{{
			Source:           "glassdoor",
			Rating:           "2.8/5",
			ReviewCount:      &reviewCount,
			RecommendPercent: &recommendPercent,
		}},
	}

	applyEmployerReviewHealthSignals(result)

	if result.Score >= 70 {
		t.Fatalf("Score = %d; want weak review metrics to lower score", result.Score)
	}
	if result.EmploymentRisk.Score == 0 {
		t.Fatalf("EmploymentRisk.Score = 0; want weak review metrics to raise risk")
	}
	if !companyHealthSignalUsed(result.SignalsUsed, "employer_review_health") {
		t.Fatalf("SignalsUsed = %#v; want employer_review_health", result.SignalsUsed)
	}
}

func TestApplyCompanySiteProfileAddsFieldGapsWithoutSiteEvidence(t *testing.T) {
	result := &CompanyHealthResult{
		Confidence:  "medium",
		SignalsUsed: []string{},
		Notes:       []string{},
		Sources:     make(map[string]any),
	}
	initCompanyHealthAssessments(result)

	ApplyCompanySiteProfile(result, &CompanySiteProfile{
		WebsiteURL: "://invalid",
	})

	foundedAssessment := result.FieldAssessments["founded_year"]
	if foundedAssessment == nil || len(foundedAssessment.Notes) != 1 {
		t.Fatalf("ApplyCompanySiteProfile() founded_year assessment = %#v, want one gap note", foundedAssessment)
	}
	if want := "Browser company-site lookup found no trustworthy founded-year evidence."; foundedAssessment.Notes[0] != want {
		t.Errorf("ApplyCompanySiteProfile() founded_year gap note = %q, want %q", foundedAssessment.Notes[0], want)
	}

	employeesAssessment := result.FieldAssessments["estimated_employees"]
	if employeesAssessment == nil || len(employeesAssessment.Notes) != 1 {
		t.Fatalf("ApplyCompanySiteProfile() estimated_employees assessment = %#v, want one gap note", employeesAssessment)
	}
	if want := "Browser company-site lookup found no trustworthy employee-count evidence."; employeesAssessment.Notes[0] != want {
		t.Errorf("ApplyCompanySiteProfile() estimated_employees gap note = %q, want %q", employeesAssessment.Notes[0], want)
	}
}
